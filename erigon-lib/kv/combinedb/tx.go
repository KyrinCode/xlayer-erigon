package combinedb

import "C"
import (
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv/iter"
	"github.com/ledgerwatch/erigon-lib/kv/order"
	"sync/atomic"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type CombineTx struct {
	mdbxTx    kv.Tx
	rocksdbTx kv.Tx

	logger *combineLogger
}

type CombineRwTx struct {
	*CombineTx
	mdbxTx    kv.RwTx
	rocksdbTx kv.RwTx
}

var txCounter atomic.Uint64

type kvPair struct {
	key   []byte
	value []byte
}

func newCombineTx(dbPrefix string, mdbxTx kv.Tx, rocksdbTx kv.Tx) *CombineTx {
	return &CombineTx{
		mdbxTx:    mdbxTx,
		rocksdbTx: rocksdbTx,
		logger:    newCombinLogger(fmt.Sprintf("%s txid=%d", dbPrefix, txCounter.Add(1))),
	}
}

func newCombineRwTx(dbPrefix string, mdbxTx kv.RwTx, rocksdbTx kv.RwTx) kv.RwTx {
	return &CombineRwTx{
		CombineTx: newCombineTx(dbPrefix, mdbxTx, rocksdbTx),
		mdbxTx:    mdbxTx,
		rocksdbTx: rocksdbTx,
	}
}

func (tx *CombineTx) Has(table string, key []byte) (bool, error) {
	tx.logger.Debugf("Has(%s,%x)", table, key)
	defer tx.logger.Debugf("Has done")

	b1, err1 := tx.mdbxTx.Has(table, key)
	b2, err2 := tx.rocksdbTx.Has(table, key)
	assertError(tx.logger, err1, err2, "Has")

	assertEq(tx.logger, b1, b2, "Has mismatch: mdbx: %v. rocksdb: %v", b1, b2)
	return b1, nil
}

func (tx *CombineTx) GetOne(table string, key []byte) (val []byte, err error) {
	tx.logger.Debugf("GetOne(%s,%x)", table, key)
	defer tx.logger.Debugf("GetOne done")

	v1, err1 := tx.mdbxTx.GetOne(table, key)
	v2, err2 := tx.rocksdbTx.GetOne(table, key)
	assertError(tx.logger, err1, err2, "GetOne")

	assertEq(tx.logger, v1, v2, "GetOne mismatch: mdbx: %x. rocksdb: %x", v1, v2)
	return v1, nil
}

func (tx *CombineTx) ForEach(table string, fromPrefix []byte, walker func(k, v []byte) error) error {
	tx.logger.Debugf("ForEach(%s,%x)", table, fromPrefix)
	defer tx.logger.Debugf("ForEach done")

	mdbxPairs := make([]kvPair, 0)
	err := tx.mdbxTx.ForEach(table, fromPrefix, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForEach error. mdbx err: %v", err)
		return err
	}

	rocksdbPairs := make([]kvPair, 0)
	err = tx.rocksdbTx.ForEach(table, fromPrefix, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForEach error. rocksdb err: %v", err)
		return err
	}

	assertEq(tx.logger, mdbxPairs, rocksdbPairs, "ForEach mismatch: mdbx: %v. rocksdb: %v", mdbxPairs, rocksdbPairs)
	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) ForPrefix(table string, prefix []byte, walker func(k, v []byte) error) error {
	tx.logger.Debugf("ForPrefix(%s,%x)", table, prefix)
	defer tx.logger.Debugf("ForPrefix done")

	mdbxPairs := make([]kvPair, 0)
	err := tx.mdbxTx.ForPrefix(table, prefix, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForPrefix error. mdbx err: %v", err)
		return err
	}

	rocksdbPairs := make([]kvPair, 0)
	err = tx.rocksdbTx.ForPrefix(table, prefix, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForPrefix error. rocksdb err: %v", err)
		return err
	}

	assertEq(tx.logger, mdbxPairs, rocksdbPairs, "ForPrefix mismatch: mdbx: %v. rocksdb %v", mdbxPairs, rocksdbPairs)
	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) ForAmount(table string, prefix []byte, amount uint32, walker func(k, v []byte) error) error {
	tx.logger.Debugf("ForAmount(%s,%x,%d)", table, prefix, amount)
	defer tx.logger.Debugf("ForAmount done")

	mdbxPairs := make([]kvPair, 0)
	err := tx.mdbxTx.ForAmount(table, prefix, amount, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForAmount error. mdbx err: %v", err)
		return err
	}

	rocksdbPairs := make([]kvPair, 0)
	err = tx.rocksdbTx.ForAmount(table, prefix, amount, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})
	if err != nil {
		tx.logger.Errorf("ForAmount error. rocksdb err: %v", err)
		return err
	}

	assertEq(tx.logger, mdbxPairs, rocksdbPairs, "ForAmount mismatch: mdbx: %v. rocksdb: %v", mdbxPairs, rocksdbPairs)
	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) Commit() error {
	tx.logger.Debugf("Commit")
	defer tx.logger.Debugf("Commit done")

	err := tx.mdbxTx.Commit()
	if err != nil {
		tx.logger.Errorf("Commit error. mdbx err: %v", err)
		return err
	}
	err = tx.rocksdbTx.Commit()
	if err != nil {
		tx.logger.Fatalf("Commit error. rocksdb err: %v", err)
	}
	return nil
}

func (tx *CombineTx) Rollback() {
	tx.logger.Debugf("Rollback")
	defer tx.logger.Debugf("Rollback done")

	tx.mdbxTx.Rollback()
	tx.rocksdbTx.Rollback()
}

func (tx *CombineTx) ReadSequence(table string) (uint64, error) {
	tx.logger.Debugf("ReadSequence(table=%s)", table)
	defer tx.logger.Debugf("ReadSequence done")

	s1, err1 := tx.mdbxTx.ReadSequence(table)
	s2, err2 := tx.rocksdbTx.ReadSequence(table)
	assertError(tx.logger, err1, err2, "ReadSequence")

	assertEq(tx.logger, s1, s2, "ReadSequence mismatch: mdbx %v. rocksdb %v", s1, s2)
	return s1, nil
}

func (tx *CombineTx) ListBuckets() ([]string, error) {
	tx.logger.Debugf("ListBuckets")
	defer tx.logger.Debugf("ListBuckets done")

	table1, err1 := tx.mdbxTx.ListBuckets()
	table2, err2 := tx.rocksdbTx.ListBuckets()
	assertError(tx.logger, err1, err2, "ListBuckets")

	assertEq(tx.logger, table1, table2, "ListBuckets mismatch: mdbx %v. rocksdb %v", table1, table2)
	return table1, nil
}

func (tx *CombineTx) ViewID() uint64 {
	tx.logger.Debugf("ViewID")
	defer tx.logger.Debugf("ViewID done")

	id1 := tx.mdbxTx.ViewID()
	id2 := tx.rocksdbTx.ViewID()

	assertEq(tx.logger, id1, id2, "ViewID mismatch: mdbx %v. rocksdb %v", id1, id2)
	return id1
}

func (tx *CombineTx) Cursor(table string) (kv.Cursor, error) {
	tx.logger.Debugf("Cursor(table=%s)", table)
	defer tx.logger.Debugf("Cursor done")

	mdbxCursor, err1 := tx.mdbxTx.Cursor(table)
	rocksdbCursor, err2 := tx.rocksdbTx.Cursor(table)
	assertError(tx.logger, err1, err2, "Cursor")

	return newCombineCursor(tx.logger.getPrefix(), mdbxCursor, rocksdbCursor), nil
}

func (tx *CombineTx) CursorDupSort(table string) (kv.CursorDupSort, error) {
	tx.logger.Debugf("CursorDupSort table=%s", table)
	defer tx.logger.Debugf("CursorDupSort done")

	mdbxCursor, err1 := tx.mdbxTx.CursorDupSort(table)
	rocksdbCursor, err2 := tx.rocksdbTx.CursorDupSort(table)
	assertError(tx.logger, err1, err2, "CursorDupSort")

	return newCombineCursorDupSort(tx.logger.getPrefix(), mdbxCursor, rocksdbCursor), nil
}

func (tx *CombineTx) DBSize() (uint64, error) {
	tx.logger.Debugf("DBSize")
	defer tx.logger.Debugf("DBSize done")

	v1, err1 := tx.mdbxTx.DBSize()
	v2, err2 := tx.rocksdbTx.DBSize()
	assertError(tx.logger, err1, err2, "DBSize")

	assertEq(tx.logger, v1, v2, "DBSize mismatch: mdbx: %v. rocksdb: %v", v1, v2)
	return v1, nil
}

func (tx *CombineTx) Range(table string, fromPrefix, toPrefix []byte) (iter.KV, error) {
	tx.logger.Debugf("Range(table=%s,from=%x,to=%x)", table, fromPrefix, toPrefix)
	defer tx.logger.Debugf("Range done")

	iter1, err1 := tx.mdbxTx.Range(table, fromPrefix, toPrefix)
	iter2, err2 := tx.rocksdbTx.Range(table, fromPrefix, toPrefix)
	assertError(tx.logger, err1, err2, "Range")

	return newCombineDual(tx.logger.getPrefix(), iter1, iter2), nil
}

func (tx *CombineTx) RangeAscend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	tx.logger.Debugf("RangeAscend(table=%s,from=%x,to=%x)", table, fromPrefix, toPrefix)
	defer tx.logger.Debugf("RangeAscend done")

	iter1, err1 := tx.mdbxTx.RangeAscend(table, fromPrefix, toPrefix, limit)
	iter2, err2 := tx.rocksdbTx.RangeAscend(table, fromPrefix, toPrefix, limit)
	assertError(tx.logger, err1, err2, "RangeAscend")

	return newCombineDual(tx.logger.getPrefix(), iter1, iter2), nil
}

func (tx *CombineTx) RangeDescend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	tx.logger.Debugf("RangeDescend(table=%s,fromPrefix=%x,toPrefix=%x,limit=%d)", table, fromPrefix, toPrefix, limit)
	defer tx.logger.Debugf("RangeDescend done")

	iter1, err1 := tx.mdbxTx.RangeDescend(table, fromPrefix, toPrefix, limit)
	iter2, err2 := tx.rocksdbTx.RangeDescend(table, fromPrefix, toPrefix, limit)
	assertError(tx.logger, err1, err2, "RangeDescend")

	return newCombineDual(tx.logger.getPrefix(), iter1, iter2), nil
}

func (tx *CombineTx) Prefix(table string, prefix []byte) (iter.KV, error) {
	tx.logger.Debugf("Prefix(table=%s, prefix=%x)", table, prefix)
	defer tx.logger.Debugf("Prefix done")

	iter1, err1 := tx.mdbxTx.Prefix(table, prefix)
	iter2, err2 := tx.rocksdbTx.Prefix(table, prefix)
	assertError(tx.logger, err1, err2, "Prefix")

	return newCombineDual(tx.logger.getPrefix(), iter1, iter2), nil
}

func (tx *CombineTx) RangeDupSort(table string, key []byte, fromPrefix, toPrefix []byte, asc order.By, limit int) (iter.KV, error) {
	tx.logger.Debugf("RangeDupSort(table=%s, key=%x, fromPrefix=%x, toPrefix=%x, asc=%v, limit=%d)", table, key, fromPrefix, toPrefix, asc, limit)
	defer tx.logger.Debugf("RangeDupSort done")

	iter1, err1 := tx.mdbxTx.RangeDupSort(table, key, fromPrefix, toPrefix, asc, limit)
	iter2, err2 := tx.rocksdbTx.RangeDupSort(table, key, fromPrefix, toPrefix, asc, limit)
	assertError(tx.logger, err1, err2, "RangeDupSort")

	return newCombineDual(tx.logger.getPrefix(), iter1, iter2), nil
}

func (tx *CombineTx) CHandle() unsafe.Pointer {
	tx.logger.Debugf("CHandle()")
	defer tx.logger.Debugf("CHandle done")

	panic("CHandle not supported")
}

func (tx *CombineTx) BucketSize(table string) (uint64, error) {
	tx.logger.Debugf("BucketSize(table=%s)", table)
	defer tx.logger.Debugf("BucketSize done")

	v1, err1 := tx.mdbxTx.BucketSize(table)
	v2, err2 := tx.rocksdbTx.BucketSize(table)
	assertError(tx.logger, err1, err2, "BucketSize")

	assertEq(tx.logger, v1, v2, "BucketSize mismatch: mdbx: %d. rocksdb: %d", v1, v2)
	return v1, nil
}

func (tx *CombineRwTx) Put(table string, k, v []byte) error {
	tx.logger.Debugf("Put(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Debugf("Put done")

	err1 := tx.mdbxTx.Put(table, k, v)
	err2 := tx.rocksdbTx.Put(table, k, v)
	assertError(tx.logger, err1, err2, "Put")

	return nil
}

func (tx *CombineRwTx) Delete(table string, k []byte) error {
	tx.logger.Debugf("Delete(table=%s, k=%x)", table, k)
	defer tx.logger.Debugf("Delete done")

	err1 := tx.mdbxTx.Delete(table, k)
	err2 := tx.rocksdbTx.Delete(table, k)
	assertError(tx.logger, err1, err2, "Delete")

	return nil
}

func (tx *CombineRwTx) IncrementSequence(table string, amount uint64) (uint64, error) {
	tx.logger.Debugf("IncrementSequence(table=%s, amount=%d)", table, amount)
	defer tx.logger.Debugf("IncrementSequence done")

	v1, err1 := tx.mdbxTx.IncrementSequence(table, amount)
	v2, err2 := tx.rocksdbTx.IncrementSequence(table, amount)
	assertError(tx.logger, err1, err2, "IncrementSequence")

	assertEq(tx.logger, v1, v2, "IncrementSequence mismatch: mdbx: %d. rocksdb: %d", v1, v2)
	return v1, nil
}

func (tx *CombineRwTx) Append(table string, k, v []byte) error {
	tx.logger.Debugf("Append(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Debugf("Append done")

	err1 := tx.mdbxTx.Append(table, k, v)
	err2 := tx.rocksdbTx.Append(table, k, v)
	assertError(tx.logger, err1, err2, "Append")

	return nil
}

func (tx *CombineRwTx) AppendDup(table string, k, v []byte) error {
	tx.logger.Debugf("AppendDup(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Debugf("AppendDup done")

	err1 := tx.mdbxTx.AppendDup(table, k, v)
	err2 := tx.rocksdbTx.AppendDup(table, k, v)
	assertError(tx.logger, err1, err2, "AppendDup")

	return nil
}

func (tx *CombineRwTx) ListBuckets() ([]string, error) {
	tx.logger.Debugf("ListBuckets()")
	defer tx.logger.Debugf("ListBuckets done")

	list1, err1 := tx.mdbxTx.ListBuckets()
	list2, err2 := tx.rocksdbTx.ListBuckets()
	assertError(tx.logger, err1, err2, "ListBuckets")

	assertEq(tx.logger, list1, list2, "ListBuckets mismatch: mdbx: %v. rocksdb: %v", list1, list2)
	return list1, nil
}

func (tx *CombineRwTx) DropBucket(table string) error {
	tx.logger.Debugf("DropBucket(table=%s)", table)
	defer tx.logger.Debugf("DropBucket done")

	err1 := tx.mdbxTx.DropBucket(table)
	err2 := tx.rocksdbTx.DropBucket(table)
	assertError(tx.logger, err1, err2, "DropBucket")

	return nil
}

func (tx *CombineRwTx) CreateBucket(table string) error {
	tx.logger.Debugf("CreateBucket(table=%s)", table)
	defer tx.logger.Debugf("CreateBucket done")

	err1 := tx.mdbxTx.CreateBucket(table)
	err2 := tx.rocksdbTx.CreateBucket(table)
	assertError(tx.logger, err1, err2, "CreateBucket")

	return nil
}

func (tx *CombineRwTx) ExistsBucket(table string) (bool, error) {
	tx.logger.Debugf("ExistsBucket(table=%s)", table)
	defer tx.logger.Debugf("ExistsBucket done")

	exists1, err1 := tx.mdbxTx.ExistsBucket(table)
	exists2, err2 := tx.rocksdbTx.ExistsBucket(table)
	assertError(tx.logger, err1, err2, "ExistsBucket")

	assertEq(tx.logger, exists1, exists2, "ExistsBucket mismatch: mdbx: %v. rocksdb: %v", exists1, exists2)
	return exists1, nil
}

func (tx *CombineRwTx) ClearBucket(table string) error {
	tx.logger.Debugf("ClearBucket(table=%s)", table)
	defer tx.logger.Debugf("ClearBucket done")

	err1 := tx.mdbxTx.ClearBucket(table)
	err2 := tx.rocksdbTx.ClearBucket(table)
	assertError(tx.logger, err1, err2, "ClearBucket")

	return nil
}

func (tx *CombineRwTx) RwCursor(table string) (kv.RwCursor, error) {
	tx.logger.Debugf("RwCursor(table=%s)", table)
	defer tx.logger.Debugf("RwCursor done")

	mdbxCursor, err1 := tx.mdbxTx.RwCursor(table)
	rocksdbCursor, err2 := tx.rocksdbTx.RwCursor(table)
	assertError(tx.logger, err1, err2, "RwCursor")

	return newCombineRwCursor(tx.logger.getPrefix(), mdbxCursor, rocksdbCursor), nil
}

func (tx *CombineRwTx) RwCursorDupSort(table string) (kv.RwCursorDupSort, error) {
	tx.logger.Debugf("RwCursorDupSort table=%s", table)
	defer tx.logger.Debugf("RwCursorDupSort done")

	mdbxCursor, err1 := tx.mdbxTx.RwCursorDupSort(table)
	rocksdbCursor, err2 := tx.rocksdbTx.RwCursorDupSort(table)
	assertError(tx.logger, err1, err2, "RwCursorDupSort")

	return newCombineRwCursorDupSort(tx.logger.getPrefix(), mdbxCursor, rocksdbCursor), nil
}

func (tx *CombineRwTx) CollectMetrics() {
	tx.logger.Debugf("CollectMetrics")
	defer tx.logger.Debugf("CollectMetrics done")

	tx.mdbxTx.CollectMetrics()
	tx.rocksdbTx.CollectMetrics()
}

func (tx *CombineRwTx) SpaceDirty() (uint64, uint64, error) {
	tx.logger.Debugf("SpaceDirty")
	defer tx.logger.Debugf("SpaceDirty done")

	v1, v2, err := tx.mdbxTx.SpaceDirty()
	tx.logger.Debugf("SpaceDirty: %d, %d, err=%v", v1, v2, err)
	// we don't compare the returned value because rocksdb don't support this function
	return v1, v2, err
}
