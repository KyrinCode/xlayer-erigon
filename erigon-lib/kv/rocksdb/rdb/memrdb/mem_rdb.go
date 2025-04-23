package memrdb

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/rdb"
	"github.com/linxGnu/grocksdb"
)

type MemoryRDB struct {
	storage *OrderedMap
}

func NewMemoryRDB() *MemoryRDB {
	return &MemoryRDB{
		storage: NewOrderedMap(),
	}
}

func (db *MemoryRDB) Close() {
	// do nothing
}

func (db *MemoryRDB) TransactionBegin(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) rdb.RDBTransaction {
	return NewMemoryRTX(db)
}

func (db *MemoryRDB) GetMemStorage() map[string][]byte {
	return db.storage.data
}

func (db *MemoryRDB) put(key []byte, value []byte) error {
	db.storage.Put(key, value)
	return nil
}

func (db *MemoryRDB) get(key []byte) ([]byte, error) {
	key, ok := db.storage.Get(key)
	if !ok {
		return nil, rdb.ErrKeyNotExist
	}
	return key, nil
}
