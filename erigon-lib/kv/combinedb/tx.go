package combinedb

import "C"
import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/iter"
	"github.com/ledgerwatch/erigon-lib/kv/order"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
	"github.com/linxGnu/grocksdb"
)

type CombineTx struct {
	mdbxTx    kv.Tx
	rocksdbTx kv.Tx

	latestSnapshotCreator func() *grocksdb.Snapshot

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

func newCombineTx(parentLogger *combineLogger, mdbxTx kv.Tx, rocksdbTx kv.Tx) *CombineTx {
	logger := newCombinLogger(parentLogger.isEnable(), fmt.Sprintf("%s txid=%d", parentLogger.getPrefix(), txCounter.Add(1)))
	logger.Info("create combine tx")
	return &CombineTx{
		mdbxTx:    mdbxTx,
		rocksdbTx: rocksdbTx,
		logger:    logger,
	}
}

func newCombineRwTx(parentLogger *combineLogger, mdbxTx kv.RwTx, rocksdbTx kv.RwTx) kv.RwTx {
	return &CombineRwTx{
		CombineTx: newCombineTx(parentLogger, mdbxTx, rocksdbTx),
		mdbxTx:    mdbxTx,
		rocksdbTx: rocksdbTx,
	}
}

func (tx *CombineTx) Has(table string, key []byte) (bool, error) {
	tx.logger.Infof("Has(table=%s, key=%x)", table, key)
	defer tx.logger.Info("Has done")

	b1, err1 := tx.mdbxTx.Has(table, key)
	b2, err2 := tx.rocksdbTx.Has(table, key)
	if err := assertError(tx.logger, err1, err2, "Has"); err != nil {
		return false, err
	}

	assertEqualF(tx.logger, b1, b2, "Has mismatch: mdbx: %v. rocksdb: %v", b1, b2)
	return b1, nil
}

func (tx *CombineTx) GetOne(table string, key []byte) (val []byte, err error) {
	tx.logger.Infof("GetOne(table=%s, key=%x)", table, key)
	defer tx.logger.Infof("GetOne done. table=%s, key=%x, val=%x, err=%v", table, key, val, err)

	// sometimes the data is commiting and another goroutine is reading the data,
	// but the different database can't commit data at the same time,
	// which will make one get the right value and another get a old value.
	// so we have to try multiple times to make sure data is all commit as much as possible.
	const retryCount = 10
	var mdbxErrs, rockErrs [retryCount]error
	var mdbxValues, rockValues [retryCount][]byte

	for i := 0; i < retryCount; i++ {
		mdbxValues[i], mdbxErrs[i] = tx.mdbxTx.GetOne(table, key)
		rockValues[i], rockErrs[i] = tx.rocksdbTx.GetOne(table, key)

		if (mdbxErrs[i] == nil && rockErrs[i] == nil) && bytes.Equal(mdbxValues[i], rockValues[i]) {
			return mdbxValues[i], nil
		}

		tx.rocksdbTx.(*rocksdb.RocksDbTx).UpdateSnapshot()
		time.Sleep(time.Millisecond * 500)
	}

	var mdbxValuesHex, rockValuesHex [retryCount]string
	for i := 0; i < retryCount; i++ {
		mdbxValuesHex[i] = fmt.Sprintf("%x", mdbxValues[i])
		rockValuesHex[i] = fmt.Sprintf("%x", rockValues[i])
	}
	tx.logger.Fatalf(
		"GetOne(table=%s, key=%x) mismatch. mdbx values:%v. rocks values:%v. mdbx errors:%v. rocks errors:%v",
		table, key, mdbxValuesHex, rockValuesHex, mdbxErrs, rockErrs)
	return nil, nil
}

func (tx *CombineTx) ForEach(table string, fromPrefix []byte, walker func(k, v []byte) error) error {
	tx.logger.Infof("ForEach(table=%s, key=%x)", table, fromPrefix)
	defer tx.logger.Info("ForEach done")

	mdbxPairs := make([]kvPair, 0)
	rocksdbPairs := make([]kvPair, 0)
	err1 := tx.mdbxTx.ForEach(table, fromPrefix, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	err2 := tx.rocksdbTx.ForEach(table, fromPrefix, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})

	if err := assertError(tx.logger, err1, err2, "ForEach"); err != nil {
		return err
	}
	assertEqualF(tx.logger, mdbxPairs, rocksdbPairs, "ForEach mismatch: mdbx: %v. rocksdb: %v", mdbxPairs, rocksdbPairs)

	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) ForPrefix(table string, prefix []byte, walker func(k, v []byte) error) error {
	tx.logger.Infof("ForPrefix(table=%s, key=%x)", table, prefix)
	defer tx.logger.Info("ForPrefix done")

	mdbxPairs := make([]kvPair, 0)
	rocksdbPairs := make([]kvPair, 0)
	err1 := tx.mdbxTx.ForPrefix(table, prefix, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	err2 := tx.rocksdbTx.ForPrefix(table, prefix, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})

	if err := assertError(tx.logger, err1, err2, "ForPrefix"); err != nil {
		return err
	}
	assertEqualF(tx.logger, mdbxPairs, rocksdbPairs, "ForPrefix mismatch: mdbx: %v. rocksdb %v", mdbxPairs, rocksdbPairs)

	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) ForAmount(table string, prefix []byte, amount uint32, walker func(k, v []byte) error) error {
	tx.logger.Infof("ForAmount(table=%s, prefix=%x, amount=%d)", table, prefix, amount)
	defer tx.logger.Info("ForAmount done")

	mdbxPairs := make([]kvPair, 0)
	rocksdbPairs := make([]kvPair, 0)
	err1 := tx.mdbxTx.ForAmount(table, prefix, amount, func(k, v []byte) error {
		mdbxPairs = append(mdbxPairs, kvPair{k, v})
		return nil
	})
	err2 := tx.rocksdbTx.ForAmount(table, prefix, amount, func(k, v []byte) error {
		rocksdbPairs = append(rocksdbPairs, kvPair{k, v})
		return nil
	})
	if err := assertError(tx.logger, err1, err2, "ForAmount"); err != nil {
		return err
	}
	assertEqualF(tx.logger, mdbxPairs, rocksdbPairs, "ForAmount mismatch: mdbx: %v. rocksdb: %v", mdbxPairs, rocksdbPairs)

	for _, pair := range mdbxPairs {
		if err := walker(pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func (tx *CombineTx) Commit() error {
	commitLock.Lock()
	defer commitLock.Unlock()

	tx.logger.Info("Commit")
	defer tx.logger.Info("Commit done")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tx.rocksdbTx.Commit(); err != nil {
			tx.logger.Error("rocksdb tx Commit error", "error", err)
			panic(err)
		}
	}()
	// mdbx tx can't be used in another goroutine(thread)
	if err := tx.mdbxTx.Commit(); err != nil {
		tx.logger.Error("mdbx tx Commit error", "error", err)
		panic(err)
	}

	wg.Wait()
	return nil
}

func (tx *CombineTx) Rollback() {
	tx.logger.Info("Rollback")
	defer tx.logger.Info("Rollback done")

	tx.mdbxTx.Rollback()
	tx.rocksdbTx.Rollback()
}

func (tx *CombineTx) ReadSequence(table string) (uint64, error) {
	tx.logger.Infof("ReadSequence(table=%s)", table)
	defer tx.logger.Info("ReadSequence done")

	s1, err1 := tx.mdbxTx.ReadSequence(table)
	s2, err2 := tx.rocksdbTx.ReadSequence(table)
	if err := assertError(tx.logger, err1, err2, "ReadSequence"); err != nil {
		return 0, err
	}

	assertEqualF(tx.logger, s1, s2, "ReadSequence mismatch: mdbx %v. rocksdb %v", s1, s2)
	return s1, nil
}

func (tx *CombineTx) ListBuckets() ([]string, error) {
	tx.logger.Info("ListBuckets")
	defer tx.logger.Info("ListBuckets done")

	table1, err1 := tx.mdbxTx.ListBuckets()
	table2, err2 := tx.rocksdbTx.ListBuckets()

	if err := assertError(tx.logger, err1, err2, "ListBuckets"); err != nil {
		return nil, err
	}
	assertEqualF(tx.logger, table1, table2, "ListBuckets mismatch: mdbx %v. rocksdb %v", table1, table2)

	return table1, nil
}

func (tx *CombineTx) ViewID() uint64 {
	tx.logger.Info("ViewID")
	defer tx.logger.Info("ViewID done")

	id1 := tx.mdbxTx.ViewID()
	id2 := tx.rocksdbTx.ViewID()

	assertEqualF(tx.logger, id1, id2, "ViewID mismatch: mdbx %v. rocksdb %v", id1, id2)
	return id1
}

func (tx *CombineTx) Cursor(table string) (kv.Cursor, error) {
	tx.logger.Infof("Cursor(table=%s)", table)
	defer tx.logger.Info("Cursor done")

	mdbxCursor, err1 := tx.mdbxTx.Cursor(table)
	rocksdbCursor, err2 := tx.rocksdbTx.Cursor(table)
	if err := assertError(tx.logger, err1, err2, "Cursor"); err != nil {
		return nil, err
	}

	return newCombineCursor(tx.logger, mdbxCursor, rocksdbCursor, table), nil
}

func (tx *CombineTx) CursorDupSort(table string) (kv.CursorDupSort, error) {
	tx.logger.Infof("CursorDupSort table=%s", table)
	defer tx.logger.Info("CursorDupSort done")

	mdbxCursor, err1 := tx.mdbxTx.CursorDupSort(table)
	rocksdbCursor, err2 := tx.rocksdbTx.CursorDupSort(table)
	if err := assertError(tx.logger, err1, err2, "CursorDupSort"); err != nil {
		return nil, err
	}

	return newCombineCursorDupSort(tx.logger, mdbxCursor, rocksdbCursor, table), nil
}

func (tx *CombineTx) DBSize() (uint64, error) {
	tx.logger.Info("DBSize")
	defer tx.logger.Info("DBSize done")

	v1, err1 := tx.mdbxTx.DBSize()
	v2, err2 := tx.rocksdbTx.DBSize()
	if err := assertError(tx.logger, err1, err2, "DBSize"); err != nil {
		return 0, err
	}
	assertEqualF(tx.logger, v1, v2, "DBSize mismatch: mdbx: %v. rocksdb: %v", v1, v2)

	return v1, nil
}

func (tx *CombineTx) Range(table string, fromPrefix, toPrefix []byte) (iter.KV, error) {
	tx.logger.Infof("Range(table=%s,from=%x,to=%x)", table, fromPrefix, toPrefix)
	defer tx.logger.Info("Range done")

	iter1, err1 := tx.mdbxTx.Range(table, fromPrefix, toPrefix)
	iter2, err2 := tx.rocksdbTx.Range(table, fromPrefix, toPrefix)
	if err := assertError(tx.logger, err1, err2, "Range"); err != nil {
		return nil, err
	}

	return newCombineDual(tx.logger, iter1, iter2), nil
}

func (tx *CombineTx) RangeAscend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	tx.logger.Infof("RangeAscend(table=%s,from=%x,to=%x)", table, fromPrefix, toPrefix)
	defer tx.logger.Info("RangeAscend done")

	iter1, err1 := tx.mdbxTx.RangeAscend(table, fromPrefix, toPrefix, limit)
	iter2, err2 := tx.rocksdbTx.RangeAscend(table, fromPrefix, toPrefix, limit)
	if err := assertError(tx.logger, err1, err2, "RangeAscend"); err != nil {
		return nil, err
	}

	return newCombineDual(tx.logger, iter1, iter2), nil
}

func (tx *CombineTx) RangeDescend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	tx.logger.Infof("RangeDescend(table=%s,fromPrefix=%x,toPrefix=%x,limit=%d)", table, fromPrefix, toPrefix, limit)
	defer tx.logger.Info("RangeDescend done")

	iter1, err1 := tx.mdbxTx.RangeDescend(table, fromPrefix, toPrefix, limit)
	iter2, err2 := tx.rocksdbTx.RangeDescend(table, fromPrefix, toPrefix, limit)
	if err := assertError(tx.logger, err1, err2, "RangeDescend"); err != nil {
		return nil, err
	}

	return newCombineDual(tx.logger, iter1, iter2), nil
}

func (tx *CombineTx) Prefix(table string, prefix []byte) (iter.KV, error) {
	tx.logger.Infof("Prefix(table=%s, prefix=%x)", table, prefix)
	defer tx.logger.Info("Prefix done")

	iter1, err1 := tx.mdbxTx.Prefix(table, prefix)
	iter2, err2 := tx.rocksdbTx.Prefix(table, prefix)
	if err := assertError(tx.logger, err1, err2, "Prefix"); err != nil {
		return nil, err
	}

	return newCombineDual(tx.logger, iter1, iter2), nil
}

func (tx *CombineTx) RangeDupSort(table string, key []byte, fromPrefix, toPrefix []byte, asc order.By, limit int) (iter.KV, error) {
	tx.logger.Infof("RangeDupSort(table=%s, key=%x, fromPrefix=%x, toPrefix=%x, asc=%v, limit=%d)", table, key, fromPrefix, toPrefix, asc, limit)
	defer tx.logger.Info("RangeDupSort done")

	iter1, err1 := tx.mdbxTx.RangeDupSort(table, key, fromPrefix, toPrefix, asc, limit)
	iter2, err2 := tx.rocksdbTx.RangeDupSort(table, key, fromPrefix, toPrefix, asc, limit)
	if err := assertError(tx.logger, err1, err2, "RangeDupSort"); err != nil {
		return nil, err
	}

	return newCombineDual(tx.logger, iter1, iter2), nil
}

func (tx *CombineTx) CHandle() unsafe.Pointer {
	tx.logger.Info("CHandle()")
	defer tx.logger.Info("CHandle done")

	panic("CHandle not supported")
}

func (tx *CombineTx) BucketSize(table string) (uint64, error) {
	tx.logger.Infof("BucketSize(table=%s)", table)
	defer tx.logger.Info("BucketSize done")

	v1, err1 := tx.mdbxTx.BucketSize(table)
	v2, err2 := tx.rocksdbTx.BucketSize(table)
	if err := assertError(tx.logger, err1, err2, "BucketSize"); err != nil {
		return 0, err
	}

	assertEqualF(tx.logger, v1, v2, "BucketSize mismatch: mdbx: %d. rocksdb: %d", v1, v2)
	return v1, nil
}

func (tx *CombineRwTx) Put(table string, k, v []byte) error {
	tx.logger.Infof("Put(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Info("Put done")

	err1 := tx.mdbxTx.Put(table, k, v)
	err2 := tx.rocksdbTx.Put(table, k, v)
	return assertError(tx.logger, err1, err2, "Put")
}

func (tx *CombineRwTx) Delete(table string, k []byte) error {
	tx.logger.Infof("Delete(table=%s, k=%x)", table, k)
	defer tx.logger.Info("Delete done")

	// NOTE: don't know why if call mdbx Delete first the `k` will be changed
	err2 := tx.rocksdbTx.Delete(table, k)
	err1 := tx.mdbxTx.Delete(table, k)
	return assertError(tx.logger, err1, err2, "Delete")
}

func (tx *CombineRwTx) IncrementSequence(table string, amount uint64) (uint64, error) {
	tx.logger.Infof("IncrementSequence(table=%s, amount=%d)", table, amount)
	defer tx.logger.Info("IncrementSequence done")

	v1, err1 := tx.mdbxTx.IncrementSequence(table, amount)
	v2, err2 := tx.rocksdbTx.IncrementSequence(table, amount)
	if err := assertError(tx.logger, err1, err2, "IncrementSequence"); err != nil {
		return 0, err
	}

	assertEqualF(tx.logger, v1, v2, "IncrementSequence mismatch: mdbx: %d. rocksdb: %d", v1, v2)
	return v1, nil
}

func (tx *CombineRwTx) Append(table string, k, v []byte) error {
	tx.logger.Infof("Append(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Info("Append done")

	err1 := tx.mdbxTx.Append(table, k, v)
	err2 := tx.rocksdbTx.Append(table, k, v)
	return assertError(tx.logger, err1, err2, "Append")
}

func (tx *CombineRwTx) AppendDup(table string, k, v []byte) error {
	tx.logger.Infof("AppendDup(table=%s, k=%x, v=%x)", table, k, v)
	defer tx.logger.Info("AppendDup done")

	err1 := tx.mdbxTx.AppendDup(table, k, v)
	err2 := tx.rocksdbTx.AppendDup(table, k, v)
	return assertError(tx.logger, err1, err2, "AppendDup")
}

func (tx *CombineRwTx) ListBuckets() ([]string, error) {
	tx.logger.Info("ListBuckets()")
	defer tx.logger.Info("ListBuckets done")

	list1, err1 := tx.mdbxTx.ListBuckets()
	list2, err2 := tx.rocksdbTx.ListBuckets()
	if err := assertError(tx.logger, err1, err2, "ListBuckets"); err != nil {
		return nil, err
	}

	assertEqualF(tx.logger, list1, list2, "ListBuckets mismatch: mdbx: %v. rocksdb: %v", list1, list2)
	return list1, nil
}

func (tx *CombineRwTx) DropBucket(table string) error {
	tx.logger.Infof("DropBucket(table=%s)", table)
	defer tx.logger.Info("DropBucket done")

	err1 := tx.mdbxTx.DropBucket(table)
	err2 := tx.rocksdbTx.DropBucket(table)
	return assertError(tx.logger, err1, err2, "DropBucket")
}

func (tx *CombineRwTx) CreateBucket(table string) error {
	tx.logger.Infof("CreateBucket(table=%s)", table)
	defer tx.logger.Info("CreateBucket done")

	err1 := tx.mdbxTx.CreateBucket(table)
	err2 := tx.rocksdbTx.CreateBucket(table)
	return assertError(tx.logger, err1, err2, "CreateBucket")
}

func (tx *CombineRwTx) ExistsBucket(table string) (bool, error) {
	tx.logger.Infof("ExistsBucket(table=%s)", table)
	defer tx.logger.Info("ExistsBucket done")

	exists1, err1 := tx.mdbxTx.ExistsBucket(table)
	exists2, err2 := tx.rocksdbTx.ExistsBucket(table)
	if err := assertError(tx.logger, err1, err2, "ExistsBucket"); err != nil {
		return false, err
	}

	assertEqualF(tx.logger, exists1, exists2, "ExistsBucket mismatch: mdbx: %v. rocksdb: %v", exists1, exists2)
	return exists1, nil
}

func (tx *CombineRwTx) ClearBucket(table string) error {
	tx.logger.Infof("ClearBucket(table=%s)", table)
	defer tx.logger.Info("ClearBucket done")

	err1 := tx.mdbxTx.ClearBucket(table)
	err2 := tx.rocksdbTx.ClearBucket(table)
	return assertError(tx.logger, err1, err2, "ClearBucket")
}

func (tx *CombineRwTx) RwCursor(table string) (kv.RwCursor, error) {
	tx.logger.Infof("RwCursor(table=%s)", table)
	defer tx.logger.Info("RwCursor done")

	mdbxCursor, err1 := tx.mdbxTx.RwCursor(table)
	rocksdbCursor, err2 := tx.rocksdbTx.RwCursor(table)
	if err := assertError(tx.logger, err1, err2, "RwCursor"); err != nil {
		return nil, err
	}

	return newCombineRwCursor(tx.logger, mdbxCursor, rocksdbCursor, table), nil
}

func (tx *CombineRwTx) RwCursorDupSort(table string) (kv.RwCursorDupSort, error) {
	tx.logger.Infof("RwCursorDupSort table=%s", table)
	defer tx.logger.Info("RwCursorDupSort done")

	mdbxCursor, err1 := tx.mdbxTx.RwCursorDupSort(table)
	rocksdbCursor, err2 := tx.rocksdbTx.RwCursorDupSort(table)
	if err := assertError(tx.logger, err1, err2, "RwCursorDupSort"); err != nil {
		return nil, err
	}

	return newCombineRwCursorDupSort(tx.logger, mdbxCursor, rocksdbCursor, table), nil
}

func (tx *CombineRwTx) CollectMetrics() {
	tx.logger.Info("CollectMetrics")
	defer tx.logger.Info("CollectMetrics done")

	tx.mdbxTx.CollectMetrics()
	tx.rocksdbTx.CollectMetrics()
}

func (tx *CombineRwTx) SpaceDirty() (uint64, uint64, error) {
	tx.logger.Info("SpaceDirty")
	defer tx.logger.Info("SpaceDirty done")

	v1, v2, err := tx.mdbxTx.SpaceDirty()
	tx.logger.Infof("SpaceDirty: %d, %d, err=%v", v1, v2, err)
	// we don't compare the returned value because rocksdb don't support this function
	return v1, v2, err
}
