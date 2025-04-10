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

type dataPair struct {
	k []byte
	v []byte
}

func main() {
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
	logger := log.New()
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

	// Open the target RocksDB database
	logger.Info("Opening target RocksDB database", "path", *rocksdbPath)
	dstDB, err := openRocksDB(*rocksdbPath, logger)
	if err != nil {
		logger.Error("Failed to open RocksDB database", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}
	defer dstDB.Close()

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

	// Create target tables
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

	// Set up progress reporting variables
	totalRecords := int64(0)

	// Convert data table by table
	for _, table := range tableNames {
		logger.Info("Converting table", "name", table)
		tableStartTime := time.Now()

		recordCount, err := convertTable(table, srcDB, dstDB)
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

func convertTable(table string, srcDB kv.RwDB, dstDB kv.RwDB) (int64, error) {
	recordCount := int64(0)

	// Process records in batches to avoid a single large transaction
	batchSize := 10000
	batch := make([]dataPair, 0, batchSize)

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
				if err := putBatch(dstDB, table, batch); err != nil {
					return err
				}

				batch = batch[:0]
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	if err := putBatch(dstDB, table, batch); err != nil {
		return 0, err
	}

	return recordCount, nil
}

func putBatch(db kv.RwDB, table string, batch []dataPair) error {
	if len(batch) == 0 {
		return nil
	}
	return db.Update(context.Background(), func(dstTx kv.RwTx) error {
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
	})
}
