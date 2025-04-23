package realdb

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/rdb"
	"github.com/linxGnu/grocksdb"
)

type RealRDB struct {
	db *grocksdb.TransactionDB
}

func NewRealRDB(opts *grocksdb.Options, txopts *grocksdb.TransactionDBOptions, dbPath string) (*RealRDB, error) {
	db, err := grocksdb.OpenTransactionDb(opts, txopts, dbPath)
	return &RealRDB{db: db}, err
}

func (db *RealRDB) TransactionBegin(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) rdb.RDBTransaction {
	return newRealRtx(db.db.TransactionBegin(opts, transactionOpts, oldTransaction))

}

func (db *RealRDB) Close() {
	db.db.Close()
	db.db = nil
}
