package combinedb

import (
	"fmt"
	"sync/atomic"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type CombineCursor struct {
	mdbxCursor    kv.Cursor
	rocksdbCursor kv.Cursor

	logger *combineLogger
}

type CombineRwCursor struct {
	*CombineCursor

	mdbxCursor    kv.RwCursor
	rocksdbCursor kv.RwCursor
}

type CombineCursorDupSort struct {
	*CombineCursor

	mdbxCursor    kv.CursorDupSort
	rocksdbCursor kv.CursorDupSort
}

type CombineRwCursorDupSort struct {
	*CombineCursorDupSort
	*CombineRwCursor

	mdbxCursor    kv.RwCursorDupSort
	rocksdbCursor kv.RwCursorDupSort
}

var cursorCounter atomic.Uint64

func newCombineCursor(txPrefix string, mdbxCursor, rocksdbCursor kv.Cursor) *CombineCursor {
	return &CombineCursor{
		mdbxCursor:    mdbxCursor,
		rocksdbCursor: rocksdbCursor,
		logger:        newCombinLogger(fmt.Sprintf("%s cursorid=%d", txPrefix, cursorCounter.Add(1))),
	}
}
func newCombineRwCursor(txPrefix string, mdbxCursor, rocksdbCursor kv.RwCursor) *CombineRwCursor {
	return &CombineRwCursor{
		CombineCursor: newCombineCursor(txPrefix, mdbxCursor, rocksdbCursor),
		mdbxCursor:    mdbxCursor,
		rocksdbCursor: rocksdbCursor,
	}
}

func newCombineCursorDupSort(txPrefix string, mdbxCursor, rocksdbCursor kv.CursorDupSort) *CombineCursorDupSort {
	return &CombineCursorDupSort{
		CombineCursor: newCombineCursor(txPrefix, mdbxCursor, rocksdbCursor),
		mdbxCursor:    mdbxCursor,
		rocksdbCursor: rocksdbCursor,
	}
}

func newCombineRwCursorDupSort(txPrefix string, mdbxCursor, rocksdbCursor kv.RwCursorDupSort) kv.RwCursorDupSort {
	return &CombineRwCursorDupSort{
		CombineCursorDupSort: newCombineCursorDupSort(txPrefix, mdbxCursor, rocksdbCursor),
		CombineRwCursor:      newCombineRwCursor(txPrefix, mdbxCursor, rocksdbCursor),

		mdbxCursor:    mdbxCursor,
		rocksdbCursor: rocksdbCursor,
	}
}

func (c *CombineCursor) First() ([]byte, []byte, error) {
	c.logger.Debugf("First()")
	defer c.logger.Debugf("First() done")

	k1, v1, err1 := c.mdbxCursor.First()
	k2, v2, err2 := c.rocksdbCursor.First()
	assertError(c.logger, err1, err2, "First")

	assertEq(c.logger, k1, k2, "First key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "First value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) Seek(seek []byte) ([]byte, []byte, error) {
	c.logger.Debugf("Seek()")
	defer c.logger.Debugf("Seek() done")

	k1, v1, err1 := c.mdbxCursor.Seek(seek)
	k2, v2, err2 := c.rocksdbCursor.Seek(seek)
	assertError(c.logger, err1, err2, "Seek")

	assertEq(c.logger, k1, k2, "Seek key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "Seek value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) SeekExact(key []byte) ([]byte, []byte, error) {
	c.logger.Debugf("SeekExact()")
	defer c.logger.Debugf("SeekExact() done")

	k1, v1, err1 := c.mdbxCursor.SeekExact(key)
	k2, v2, err2 := c.rocksdbCursor.SeekExact(key)
	assertError(c.logger, err1, err2, "SeekExact")

	assertEq(c.logger, k1, k2, "SeekExact key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "SeekExact value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) Next() ([]byte, []byte, error) {
	c.logger.Debugf("Next()")
	defer c.logger.Debugf("Next() done")

	k1, v1, err1 := c.mdbxCursor.Next()
	k2, v2, err2 := c.rocksdbCursor.Next()
	assertError(c.logger, err1, err2, "Next")

	assertEq(c.logger, k1, k2, "Next key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "Next value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) Prev() ([]byte, []byte, error) {
	c.logger.Debugf("Prev()")
	defer c.logger.Debugf("Prev() done")

	k1, v1, err1 := c.mdbxCursor.Prev()
	k2, v2, err2 := c.rocksdbCursor.Prev()
	assertError(c.logger, err1, err2, "Prev")

	assertEq(c.logger, k1, k2, "Prev key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "Prev value mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	return k1, v2, nil
}

func (c *CombineCursor) Last() ([]byte, []byte, error) {
	c.logger.Debugf("Last()")
	defer c.logger.Debugf("Last() done")

	k1, v1, err1 := c.mdbxCursor.Last()
	k2, v2, err2 := c.rocksdbCursor.Last()
	assertError(c.logger, err1, err2, "Last")

	assertEq(c.logger, k1, k2, "Last key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "Last value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) Current() ([]byte, []byte, error) {
	c.logger.Debugf("Current()")
	defer c.logger.Debugf("Current() done")

	k1, v1, err1 := c.mdbxCursor.Current()
	k2, v2, err2 := c.rocksdbCursor.Current()
	assertError(c.logger, err1, err2, "Current")

	assertEq(c.logger, k1, k2, "Current key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "Current value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v2, nil
}

func (c *CombineCursor) Count() (uint64, error) {
	c.logger.Debugf("Count()")
	defer c.logger.Debugf("Count() done")

	v1, err1 := c.mdbxCursor.Count()
	v2, err2 := c.rocksdbCursor.Count()
	assertError(c.logger, err1, err2, "Count")

	assertEq(c.logger, v1, v2, "Count mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return v1, nil
}

func (c *CombineCursor) Close() {
	c.logger.Debugf("Close()")
	defer c.logger.Debugf("Close() done")

	c.mdbxCursor.Close()
	c.rocksdbCursor.Close()
}

func (c *CombineRwCursor) Put(k, v []byte) error {
	c.logger.Debugf("Put(k=%x, v=%x)", k, v)
	defer c.logger.Debugf("Put() done")

	err1 := c.mdbxCursor.Put(k, v)
	err2 := c.rocksdbCursor.Put(k, v)
	assertError(c.logger, err1, err2, "Put")

	return nil
}

func (c *CombineRwCursor) Append(k []byte, v []byte) error {
	c.logger.Debugf("Append(k=%x, v=%x)", k, v)
	defer c.logger.Debugf("Append() done")

	err1 := c.mdbxCursor.Append(k, v)
	err2 := c.rocksdbCursor.Append(k, v)
	assertError(c.logger, err1, err2, "Append")

	return nil
}

func (c *CombineRwCursor) Delete(k []byte) error {
	c.logger.Debugf("Delete(k=%x)", k)
	defer c.logger.Debugf("Delete() done")

	err1 := c.mdbxCursor.Delete(k)
	err2 := c.rocksdbCursor.Delete(k)
	assertError(c.logger, err1, err2, "Delete")

	return nil
}

func (c *CombineRwCursor) DeleteCurrent() error {
	c.logger.Debugf("DeleteCurrent()")
	defer c.logger.Debugf("DeleteCurrent() done")

	k1, v1, err1 := c.mdbxCursor.Current()
	k2, v2, err2 := c.rocksdbCursor.Current()
	assertEq(c.logger, k1, v1, "DeleteCurrent key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, k1, v1, "DeleteCurrent value mismatch. mdbx: %x. rocksdb: %x", v1, v2)

	err1 = c.mdbxCursor.DeleteCurrent()
	err2 = c.rocksdbCursor.DeleteCurrent()
	assertError(c.logger, err1, err2, "DeleteCurrent")

	// make sure the current value is still same after delete
	k1, v1, err1 = c.mdbxCursor.Current()
	k2, v2, err2 = c.rocksdbCursor.Current()
	assertEq(c.logger, k1, v1, "DeleteCurrent after delete key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, k1, v1, "DeleteCurrent after delete value mismatch. mdbx: %x. rocksdb: %x", v1, v2)

	return nil
}

func (c *CombineCursorDupSort) SeekBothExact(key, value []byte) ([]byte, []byte, error) {
	c.logger.Debugf("SeekBothExact(key=%x, value=%x)", key, value)
	defer c.logger.Debugf("SeekBothExact() done")

	k1, v1, err1 := c.mdbxCursor.SeekBothExact(key, value)
	k2, v2, err2 := c.rocksdbCursor.SeekBothExact(key, value)
	assertError(c.logger, err1, err2, "SeekBothExact")

	assertEq(c.logger, k1, k2, "SeekBothExact key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "SeekBothExact value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v1, nil
}

func (c *CombineCursorDupSort) SeekBothRange(key, value []byte) ([]byte, error) {
	c.logger.Debugf("SeekBothRange(key=%x, value=%x)", key, value)
	defer c.logger.Debugf("SeekBothRange() done")

	v1, err1 := c.mdbxCursor.SeekBothRange(key, value)
	v2, err2 := c.rocksdbCursor.SeekBothRange(key, value)
	assertError(c.logger, err1, err2, "SeekBothRange")

	assertEq(c.logger, v1, v2, "SeekBothRange mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return v1, nil
}

func (c *CombineCursorDupSort) FirstDup() ([]byte, error) {
	c.logger.Debugf("FirstDup()")
	defer c.logger.Debugf("FirstDup() done")

	v1, err1 := c.mdbxCursor.FirstDup()
	v2, err2 := c.rocksdbCursor.FirstDup()
	assertError(c.logger, err1, err2, "FirstDup")

	assertEq(c.logger, v1, v2, "FirstDup mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return v1, nil
}

func (c *CombineCursorDupSort) NextDup() ([]byte, []byte, error) {
	c.logger.Debugf("NextDup()")
	defer c.logger.Debugf("NextDup() done")

	k1, v1, err1 := c.mdbxCursor.NextDup()
	k2, v2, err2 := c.rocksdbCursor.NextDup()
	assertError(c.logger, err1, err2, "NextDup")

	assertEq(c.logger, k1, k2, "NextDup key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "NextDup value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v1, nil
}

func (c *CombineCursorDupSort) NextNoDup() ([]byte, []byte, error) {
	c.logger.Debugf("NextNoDup()")
	defer c.logger.Debugf("NextNoDup() done")

	k1, v1, err1 := c.mdbxCursor.NextNoDup()
	k2, v2, err2 := c.rocksdbCursor.NextNoDup()
	assertError(c.logger, err1, err2, "NextNoDup")

	assertEq(c.logger, k1, k2, "NextNoDup key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "NextNoDup value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v1, nil
}

func (c *CombineCursorDupSort) PrevDup() ([]byte, []byte, error) {
	c.logger.Debugf("PrevDup()")
	defer c.logger.Debugf("PrevDup() done")

	k1, v1, err1 := c.mdbxCursor.PrevDup()
	k2, v2, err2 := c.rocksdbCursor.PrevDup()
	assertError(c.logger, err1, err2, "PrevDup")

	assertEq(c.logger, k1, k2, "PrevDup key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "PrevDup value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v1, nil
}

func (c *CombineCursorDupSort) PrevNoDup() ([]byte, []byte, error) {
	c.logger.Debugf("PrevNoDup()")
	defer c.logger.Debugf("PrevNoDup() done")

	k1, v1, err1 := c.mdbxCursor.PrevNoDup()
	k2, v2, err2 := c.rocksdbCursor.PrevNoDup()
	assertError(c.logger, err1, err2, "PrevNoDup")

	assertEq(c.logger, k1, k2, "PrevNoDup key mismatch. mdbx: %x. rocksdb: %x", k1, k2)
	assertEq(c.logger, v1, v2, "PrevNoDup value mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return k1, v1, nil
}

func (c *CombineCursorDupSort) LastDup() ([]byte, error) {
	c.logger.Debugf("LastDup()")
	defer c.logger.Debugf("LastDup() done")

	v1, err1 := c.mdbxCursor.LastDup()
	v2, err2 := c.mdbxCursor.LastDup()
	assertError(c.logger, err1, err2, "LastDup")

	assertEq(c.logger, v1, v2, "LastDup mismatch. mdbx: %x. rocksdb: %x", v1, v2)
	return v1, nil
}

func (c *CombineCursorDupSort) CountDuplicates() (uint64, error) {
	c.logger.Debugf("CountDuplicates()")
	defer c.logger.Debugf("CountDuplicates() done")

	v1, err1 := c.mdbxCursor.CountDuplicates()
	v2, err2 := c.rocksdbCursor.CountDuplicates()
	assertError(c.logger, err1, err2, "CountDuplicates")

	assertEq(c.logger, v1, v2, "CountDuplicates mismatch. mdbx: %d. rocksdb: %d", v1, v2)
	return v1, nil
}

func (c *CombineRwCursorDupSort) PutNoDupData(key, value []byte) error {
	c.CombineCursorDupSort.logger.Debugf("PutNoDupData(key=%x, value=%x)", key, value)
	defer c.CombineCursorDupSort.logger.Debugf("PutNoDupData() done")

	err1 := c.mdbxCursor.PutNoDupData(key, value)
	err2 := c.rocksdbCursor.PutNoDupData(key, value)
	assertError(c.CombineCursorDupSort.logger, err1, err2, "PutNoDupData")

	return nil
}

func (c *CombineRwCursorDupSort) DeleteCurrentDuplicates() error {
	c.CombineCursorDupSort.logger.Debugf("DeleteCurrentDuplicates()")
	defer c.CombineCursorDupSort.logger.Debugf("DeleteCurrentDuplicates() done")

	err1 := c.mdbxCursor.DeleteCurrentDuplicates()
	err2 := c.rocksdbCursor.DeleteCurrentDuplicates()
	assertError(c.CombineCursorDupSort.logger, err1, err2, "DeleteCurrentDuplicates")

	return nil
}

func (c *CombineRwCursorDupSort) DeleteExact(k1, k2 []byte) error {
	c.CombineCursorDupSort.logger.Debugf("DeleteExact(key=%x, value=%x)", k1, k2)
	defer c.CombineCursorDupSort.logger.Debugf("DeleteExact() done")

	err1 := c.mdbxCursor.DeleteExact(k1, k2)
	err2 := c.rocksdbCursor.DeleteExact(k1, k2)
	assertError(c.CombineCursorDupSort.logger, err1, err2, "DeleteExact")

	return nil
}

func (c *CombineRwCursorDupSort) AppendDup(key, value []byte) error {
	c.CombineCursorDupSort.logger.Debugf("AppendDup(key=%x, value=%x)", key, value)
	defer c.CombineCursorDupSort.logger.Debugf("AppendDup() done")

	err1 := c.mdbxCursor.AppendDup(key, value)
	err2 := c.rocksdbCursor.AppendDup(key, value)
	assertError(c.CombineCursorDupSort.logger, err1, err2, "AppendDup")

	return nil
}

func (c *CombineRwCursorDupSort) First() ([]byte, []byte, error) {
	return c.CombineCursorDupSort.First()
}

func (c *CombineRwCursorDupSort) Seek(seek []byte) ([]byte, []byte, error) {
	return c.CombineCursorDupSort.Seek(seek)
}

func (c *CombineRwCursorDupSort) SeekExact(key []byte) ([]byte, []byte, error) {
	return c.CombineCursorDupSort.SeekExact(key)
}

func (c *CombineRwCursorDupSort) Next() ([]byte, []byte, error) {
	return c.CombineCursorDupSort.Next()
}

func (c *CombineRwCursorDupSort) Prev() ([]byte, []byte, error) {
	return c.CombineCursorDupSort.Prev()
}

func (c *CombineRwCursorDupSort) Last() ([]byte, []byte, error) {
	return c.CombineCursorDupSort.Last()
}

func (c *CombineRwCursorDupSort) Current() ([]byte, []byte, error) {
	return c.CombineCursorDupSort.Current()
}

func (c *CombineRwCursorDupSort) Count() (uint64, error) {
	return c.CombineCursorDupSort.Count()
}

func (c *CombineRwCursorDupSort) Close() {
	c.CombineCursorDupSort.Close()
}
