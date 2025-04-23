package rdb

import (
	"errors"
	"github.com/linxGnu/grocksdb"
)

var ErrKeyNotExist = errors.New("key not exists")

type RDB interface {
	TransactionBegin(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) RDBTransaction
	Close()
}

type RDBTransaction interface {
	Get(opts *grocksdb.ReadOptions, key []byte) ([]byte, error)
	Put(key, value []byte) error
	Delete(key []byte) error
	Commit() error
	Rollback() error
	NewIterator(beginPrefix, endPrefix []byte) RDBIterator
	Destroy()
}

type RDBIterator interface {
	Valid() bool
	SeekToFirst()
	SeekToLast()
	Next()
	Prev()
	Seek(key []byte)
	Key() []byte
	Value() []byte
	Close()
	Err() error
}
