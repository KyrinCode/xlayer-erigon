package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"
)

type dataPair struct {
	k []byte `msg:"k"`
	v []byte `msg:"v"`
}

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	// Command-line argument parsing
	mdbxPath := flag.String("mdbx", "", "Path to the source MDBX database")
	rocksdbPath := flag.String("rocksdb", "", "Path to the target RocksDB database")
	verbose := flag.Bool("verbose", false, "Whether to output detailed logs")
	flag.Parse()

	if *mdbxPath == "" || *rocksdbPath == "" {
		fmt.Println("Usage: mdbx2rocksdb --mdbx /mdbx/db/path --rocksdb /rocksdb/path [--verbose]")
		os.Exit(1)
	}

	// Configure logging
	logger := log.Root()
	if *verbose {
		logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StderrHandler))
	} else {
		logger.SetHandler(log.LvlFilterHandler(log.LvlWarn, log.StderrHandler))
	}

	// Configure logging
	if err := os.MkdirAll(*rocksdbPath, 0755); err != nil {
		logger.Error("Failed to create target directory", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}

	// Open the source MDBX database
	logger.Info("Opening source MDBX database", "path", *mdbxPath)
	srcDB, err := openMDBX(*mdbxPath, logger)
	if err != nil {
		logger.Error("Failed to open MDBX database", "path", *mdbxPath, "error", err)
		os.Exit(1)
	}
	defer srcDB.Close()

	// Start the conversion process
	startTime := time.Now()
	logger.Info("Starting database conversion")

	// Get all table names
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

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	commitrdbPath := filepath.Join(exeDir, "commitrdb")
	log.Info("commitdb path", "path", commitrdbPath)

	// Set up progress reporting variables
	totalRecords := int64(0)

	// Convert data table by table
	for _, table := range tableNames {
		logger.Info("Converting table", "name", table)
		tableStartTime := time.Now()

		recordCount, err := convertTable(commitrdbPath, *rocksdbPath, table, srcDB, logger)
		if err != nil {
			logger.Error("Failed to convert table", "name", table, "error", err)
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

	elapsed := time.Since(startTime)
	logger.Info("Conversion completed successfully",
		"total_records", totalRecords,
		"total_time", elapsed.Round(time.Second),
		"avg_speed", fmt.Sprintf("%.0f records/sec", float64(totalRecords)/elapsed.Seconds()))
}

func openMDBX(path string, logger log.Logger) (kv.RwDB, error) {
	roTxLimit := int64(32)
	roTxsLimiter := semaphore.NewWeighted(roTxLimit)

	opts := mdbx.NewMDBX(logger).
		Path(path).
		RoTxsLimiter(roTxsLimiter).
		GrowthStep(16 * datasize.MB)

	return opts.Open(context.Background())
}

func convertTable(commitrdbPath, rocksdbPath, table string, srcDB kv.RwDB, logger log.Logger) (int64, error) {
	recordCount := int64(0)

	// Process records in batches to avoid a single large transaction
	batchSize := 10000
	batch := make([]dataPair, 0, batchSize)
	index := 0

	// Start data migration transaction
	if err := srcDB.View(context.Background(), func(srcTx kv.Tx) error {
		srcCursor, err := srcTx.Cursor(table)
		if err != nil {
			return err
		}
		defer srcCursor.Close()

		// Iterate over records in the source database
		for k, v, err := srcCursor.First(); k != nil; k, v, err = srcCursor.Next() {
			if err != nil {
				return err
			}

			// Copy key and value since the cursor might reuse the same memory in the next iteration
			keyCopy := make([]byte, len(k))
			valueCopy := make([]byte, len(v))
			copy(keyCopy, k)
			copy(valueCopy, v)

			batch = append(batch, dataPair{k: keyCopy, v: valueCopy})
			recordCount++

			if len(batch) >= batchSize {
				if err := putBatch(commitrdbPath, rocksdbPath, table, index, batch, logger); err != nil {
					return err
				}

				batch = batch[:0]
				index += 1
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	if err := putBatch(commitrdbPath, rocksdbPath, table, index, batch, logger); err != nil {
		return 0, err
	}

	return recordCount, nil
}

func putBatch(commitrdbPath, rocksdbPath, table string, index int, batch []dataPair, logger log.Logger) error {
	if len(batch) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		logger.Info("Put batch", "table", table, "count", len(batch), "cost", time.Since(start))
	}()

	dataPath := fmt.Sprintf("%s-%d.bin", table, index)
	if err := saveDataToFile(dataPath, batch); err != nil {
		panic(err)
	}
	defer os.Remove(dataPath)

	return commit2rocksdb(commitrdbPath, rocksdbPath, table, dataPath, logger)

	/*
		return dstDB.Update(context.Background(), func(dstTx kv.RwTx) error {
			dstCursor, err := dstTx.RwCursor(table)
			if err != nil {
				return err
			}
			defer dstCursor.Close()

			for _, item := range batch {
				if err := dstCursor.Put(item.k, item.v); err != nil {
					return err
				}
			}
			return nil
		}) */
}
func saveDataToFile(filename string, data []dataPair) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := msgpack.NewEncoder(file)
	err = enc.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

func commit2rocksdb(commitrdbPath, rocksdbPath, table, dataPath string, logger log.Logger) error {
	cmd := exec.Command(commitrdbPath, "--rocksdb", rocksdbPath, "--table", table, "--data", dataPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()

	if err != nil {
		logger.Error("子进程执行失败", "err", err)
		if stderr.Len() > 0 {
			logger.Error(fmt.Sprintf("stderr:\n%s", stderr.String()))
		} else {
			logger.Error("没有 stderr 输出（可能是崩溃或信号终止）")
		}
		if stdout.Len() > 0 {
			logger.Error(fmt.Sprintf("stdout:\n%s", stdout.String()))
		}
		return err
	}

	return nil
}
