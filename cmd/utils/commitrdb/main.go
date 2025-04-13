package main

import (
	"context"
	"flag"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
	"github.com/ledgerwatch/log/v3"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/sync/semaphore"
	"os"
	"runtime"
)

type dataPair struct {
	k []byte `msg:"k"`
	v []byte `msg:"v"`
}

func main() {
	rocksdbPath := flag.String("rocksdb", "", "Path to the target RocksDB database")
	dataPath := flag.String("data", "", "Path to the target RocksDB database")
	table := flag.String("table", "", "table name")
	flag.Parse()

	logger := log.Root()
	logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StderrHandler))

	// Open the target RocksDB database
	logger.Info("Opening target RocksDB database", "path", *rocksdbPath)
	dstDB, err := openRocksDB(*rocksdbPath, logger)
	if err != nil {
		logger.Error("Failed to open RocksDB database", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}
	defer dstDB.Close()

	batch, err := loadDataFromFile(*dataPath)
	if err != nil {
		logger.Error("Failed to load data from file", "path", *dataPath, "error", err)
		os.Exit(1)
	}

	err = dstDB.Update(context.Background(), func(tx kv.RwTx) error {
		c, err := tx.RwCursor(*table)
		if err != nil {
			return err
		}

		for _, item := range batch {
			if err := c.Put(item.k, item.v); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("Failed to put batch to database", "path", dataPath, "error", err)
		os.Exit(1)
	}

	os.Exit(0)
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

func loadDataFromFile(filename string) ([]dataPair, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 使用 msgpack 反序列化数据
	var pairs []dataPair
	dec := msgpack.NewDecoder(file)
	err = dec.Decode(&pairs)
	if err != nil {
		return nil, err
	}

	return pairs, nil
}
