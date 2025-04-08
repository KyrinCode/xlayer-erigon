package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"
)

func main() {
	// 命令行参数解析
	mdbxPath := flag.String("mdbx", "", "源MDBX数据库路径")
	rocksdbPath := flag.String("rocksdb", "", "目标RocksDB数据库路径")
	verbose := flag.Bool("verbose", false, "是否输出详细日志")
	flag.Parse()

	if *mdbxPath == "" || *rocksdbPath == "" {
		fmt.Println("Usage: mdbx2rocksdb --mdbx /mdbx/db/path --rocksdb /rocksdb/path [--verbose]")
		os.Exit(1)
	}

	// 配置日志
	logger := log.New()
	if *verbose {
		logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StderrHandler))
	} else {
		logger.SetHandler(log.LvlFilterHandler(log.LvlWarn, log.StderrHandler))
	}

	// 确保目标目录存在
	if err := os.MkdirAll(*rocksdbPath, 0755); err != nil {
		logger.Error("Failed to create target directory", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}

	// 打开源MDBX数据库
	logger.Info("Opening source MDBX database", "path", *mdbxPath)
	srcDB, err := openMDBX(*mdbxPath, logger)
	if err != nil {
		logger.Error("Failed to open MDBX database", "path", *mdbxPath, "error", err)
		os.Exit(1)
	}
	defer srcDB.Close()

	// 打开目标RocksDB数据库
	logger.Info("Opening target RocksDB database", "path", *rocksdbPath)
	dstDB, err := openRocksDB(*rocksdbPath, logger)
	if err != nil {
		logger.Error("Failed to open RocksDB database", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}
	defer dstDB.Close()

	// 开始转换过程
	startTime := time.Now()
	logger.Info("Starting database conversion")

	// 获取所有表名
	var tableNames []string
	if err := srcDB.View(context.Background(), func(tx kv.Tx) error {
		tables, err := tx.ListBuckets()
		if err != nil {
			return err
		}
		tableNames = tables
		return nil
	}); err != nil {
		logger.Error("Failed to list buckets", "error", err)
		os.Exit(1)
	}

	// 创建目标表
	if err := dstDB.Update(context.Background(), func(tx kv.RwTx) error {
		for _, table := range tableNames {
			if err := tx.CreateBucket(table); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", table, err)
			}
		}
		return nil
	}); err != nil {
		logger.Error("Failed to create target buckets", "error", err)
		os.Exit(1)
	}

	// 设置进度报告变量
	totalRecords := int64(0)
	processedRecords := int64(0)
	lastProgressReport := time.Now()

	// 逐表转换数据
	for _, table := range tableNames {
		logger.Info("Converting table", "name", table)
		tableStartTime := time.Now()
		recordCount := int64(0)

		// 开始数据迁移事务
		if err := srcDB.View(context.Background(), func(srcTx kv.Tx) error {
			return dstDB.Update(context.Background(), func(dstTx kv.RwTx) error {
				srcCursor, err := srcTx.Cursor(table)
				if err != nil {
					return err
				}
				defer srcCursor.Close()

				dstCursor, err := dstTx.RwCursor(table)
				if err != nil {
					return err
				}
				defer dstCursor.Close()

				// 按批次处理记录，以避免单个大事务
				batchSize := 10000
				batch := make([]struct {
					k, v []byte
				}, 0, batchSize)

				// 遍历源数据库中的记录
				for k, v, err := srcCursor.First(); k != nil; k, v, err = srcCursor.Next() {
					if err != nil {
						return err
					}

					// 复制键值，因为游标可能会在下一次迭代中重用相同的内存
					keyCopy := make([]byte, len(k))
					valueCopy := make([]byte, len(v))
					copy(keyCopy, k)
					copy(valueCopy, v)

					batch = append(batch, struct {
						k, v []byte
					}{keyCopy, valueCopy})

					recordCount++
					processedRecords++

					// 当批次达到大小限制或是最后一批时，提交到目标数据库
					if len(batch) >= batchSize {
						for _, item := range batch {
							if err := dstCursor.Put(item.k, item.v); err != nil {
								return err
							}
						}
						batch = batch[:0] // 清空批次

						// 定期报告进度
						if time.Since(lastProgressReport) > 5*time.Second {
							logger.Info("Conversion progress",
								"processed", processedRecords,
								"table", table,
								"elapsed", time.Since(startTime).Round(time.Second))
							lastProgressReport = time.Now()
						}
					}
				}

				// 处理最后一批
				for _, item := range batch {
					if err := dstCursor.Put(item.k, item.v); err != nil {
						return err
					}
				}

				return nil
			})
		}); err != nil {
			logger.Error("Error during conversion", "table", table, "error", err)
			os.Exit(1)
		}

		totalRecords += recordCount
		tableElapsed := time.Since(tableStartTime)
		logger.Info("Table conversion completed",
			"table", table,
			"records", recordCount,
			"time", tableElapsed.Round(time.Second),
			"speed", fmt.Sprintf("%.0f records/sec", float64(recordCount)/tableElapsed.Seconds()))
	}

	// 完成转换
	elapsed := time.Since(startTime)
	logger.Info("Conversion completed successfully",
		"total_records", totalRecords,
		"total_time", elapsed.Round(time.Second),
		"avg_speed", fmt.Sprintf("%.0f records/sec", float64(totalRecords)/elapsed.Seconds()))
}

// 打开MDBX数据库
func openMDBX(path string, logger log.Logger) (kv.RwDB, error) {
	roTxLimit := int64(32)
	roTxsLimiter := semaphore.NewWeighted(roTxLimit)

	opts := mdbx.NewMDBX(logger).
		Path(path).
		RoTxsLimiter(roTxsLimiter).
		GrowthStep(16 * datasize.MB)

	return opts.Open(context.Background())
}

// 打开RocksDB数据库
func openRocksDB(path string, logger log.Logger) (kv.RwDB, error) {
	targetSemCount := int64(runtime.GOMAXPROCS(-1)) - 1
	if targetSemCount <= 0 {
		targetSemCount = 1
	}

	readTxLimit := int64(32)
	roTxsLimiter := semaphore.NewWeighted(readTxLimit)
	writeTxLimiter := semaphore.NewWeighted(targetSemCount)

	return rocksdb.NewRocksDB(path, logger, kv.ChaindataTablesCfg, kv.ChainDB, roTxsLimiter, writeTxLimiter, false)
}
