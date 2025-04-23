package rocksdb

import (
	"bytes"
	"encoding/binary"
	"sort"
)

type DBValueStamp struct {
	index int
	value []byte
}

type DBValue struct {
	values [][]byte
}

func DBValueWithOneValue(v []byte) *DBValue {
	return &DBValue{values: [][]byte{v}}
}

func DeserializeDBValue(buf []byte) *DBValue {
	if buf == nil {
		return &DBValue{values: nil}
	}

	count := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]

	lenSliceSize := count * 4
	lenSlice := buf[:lenSliceSize]
	buf = buf[lenSliceSize:]

	values := make([][]byte, count)
	valueOffset := 0
	for i := 0; i < int(count); i++ {
		l := binary.LittleEndian.Uint32(lenSlice[i*4:])
		values[i] = buf[valueOffset : valueOffset+int(l)]
		valueOffset += int(l)
	}

	return &DBValue{values: values}
}

func (dbv *DBValue) Serialize() []byte {
	// yztodo: make sure values is nil behave like mdbx
	if dbv.values == nil {
		return nil
	}

	bufLen := 4 + 4*len(dbv.values)
	for _, buf := range dbv.values {
		bufLen += len(buf)
	}

	buf := make([]byte, bufLen)
	pos := 0

	count := uint32(len(dbv.values))
	binary.LittleEndian.PutUint32(buf[pos:], count)
	pos += 4

	for _, v := range dbv.values {
		binary.LittleEndian.PutUint32(buf[pos:], uint32(len(v)))
		pos += 4
	}

	for _, v := range dbv.values {
		copy(buf[pos:], v)
		pos += len(v)
	}

	return buf
}

func (dbv *DBValue) First() []byte {
	return newDBValueIterator(dbv).First()
}

func (dbv *DBValue) SortedInsert(v []byte) {
	dbv.values = append(dbv.values, v)
	sort.Slice(dbv.values, func(i, j int) bool {
		return bytes.Compare(dbv.values[i], dbv.values[j]) < 0
	})
}

func (dbv *DBValue) Replace(valueStamp DBValueStamp, newV []byte) {
	if valueStamp.index < 0 || valueStamp.index >= len(dbv.values) {
		panic("index out of range for DBValue.Replace")
	}
	if oldV := dbv.values[valueStamp.index]; !bytes.Equal(oldV, valueStamp.value) {
		panic("value is different with value stamp when Replace")
	}
	dbv.values[valueStamp.index] = newV
}

func (dbv *DBValue) Delete(valueStamp DBValueStamp) {
	if valueStamp.index < 0 || valueStamp.index >= len(dbv.values) {
		panic("index out of range for DBValue.Delete")
	}
	if oldV := dbv.values[valueStamp.index]; !bytes.Equal(oldV, valueStamp.value) {
		panic("value is different with value stamp when Delete")
	}
	dbv.values = append(dbv.values[:valueStamp.index], dbv.values[valueStamp.index+1:]...)
}

func (dbv *DBValue) IsEmpty() bool {
	return len(dbv.values) == 0
}

type DBValueIterator struct {
	dbv     *DBValue
	current int
}

func newDBValueIterator(dbv *DBValue) *DBValueIterator {
	return &DBValueIterator{dbv: dbv, current: -1}
}

func (it *DBValueIterator) First() []byte {
	if len(it.dbv.values) == 0 {
		return nil
	}

	return it.dbv.values[0]
}

func (it *DBValueIterator) Last() []byte {
	if len(it.dbv.values) == 0 {
		return nil
	}

	return it.dbv.values[len(it.dbv.values)-1]
}

func (it *DBValueIterator) Seek(seekV []byte) ([]byte, error) {
	it.SeekToFirst()

	if seekV == nil {
		return it.First(), nil
	}

	for v, ok := it.Current(); ok; v, ok = it.Next() {
		compareResult := bytes.Compare(seekV, v)
		if compareResult <= 0 {
			return v, nil
		}
	}

	return nil, ErrNotFound
}

func (it *DBValueIterator) SeekExact(seekV []byte) ([]byte, error) {
	it.SeekToFirst()

	if seekV == nil {
		return it.First(), nil
	}

	for v, ok := it.Current(); ok; v, ok = it.Next() {
		if bytes.Equal(v, seekV) {
			return v, nil
		}
	}

	return nil, ErrNotFound
}

// Next return false if it iterated past either the first or the last value
func (it *DBValueIterator) Next() ([]byte, bool) {
	if it.current < -1 || it.current >= len(it.dbv.values)-1 {
		return nil, false
	}

	it.current++
	v := it.dbv.values[it.current]
	return v, true
}

func (it *DBValueIterator) Prev() ([]byte, bool) {
	if it.current <= 0 || it.current > len(it.dbv.values) {
		return nil, false
	}

	it.current--
	v := it.dbv.values[it.current]
	return v, true
}

func (it *DBValueIterator) Current() ([]byte, bool) {
	if it.current < 0 || it.current >= len(it.dbv.values) {
		return nil, false
	}

	return it.dbv.values[it.current], true
}

func (it *DBValueIterator) CurrentStamp() DBValueStamp {
	return DBValueStamp{
		index: it.current,
		value: it.dbv.values[it.current],
	}
}

func (it *DBValueIterator) Count() uint64 {
	return uint64(len(it.dbv.values))
}

func (it *DBValueIterator) SeekToFirst() ([]byte, bool) {
	if len(it.dbv.values) == 0 {
		return nil, false
	}

	it.current = 0
	v := it.dbv.values[it.current]
	return v, true
}

func (it *DBValueIterator) SeekToLast() ([]byte, bool) {
	if len(it.dbv.values) == 0 {
		return nil, false
	}

	it.current = len(it.dbv.values)
	if it.current > 0 {
		it.current--
	}

	v := it.dbv.values[it.current]
	return v, true
}

func (it *DBValueIterator) SeekToOverLast() {
	it.current = len(it.dbv.values)
}

func (it *DBValueIterator) mustSeekToValue(value []byte) {
	for i := 0; i < len(it.dbv.values); i++ {
		if bytes.Equal(it.dbv.values[i], value) {
			it.current = i
			return
		}
	}
	panic("didn't find value")

}
