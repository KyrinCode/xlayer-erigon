package rocksdb

import (
	"bytes"
	"errors"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/linxGnu/grocksdb"
)

type pairCache struct {
	key   []byte
	value []byte

	err error
}

func makeInvalidPair(err error) pairCache {
	if err == nil {
		err = errors.New("invalid pair")
	}
	return pairCache{nil, nil, err}
}

func makePairCache(k, v []byte, err error) pairCache {
	return pairCache{k, v, err}
}

func (p pairCache) isValid() bool {
	return p.err == nil
}

type iterCache struct {
	key   []byte
	value *DBValueIterator
}

func invalidIterCache() iterCache {
	return iterCache{nil, nil}
}

func (ic iterCache) isValid() bool {
	return ic.value != nil
}

// the order of rocksdb's iterator is inverse with order of mdbx's cursor,
// so we have to wrap a iterator to make the order consist with mdbx.
type iteratorWrapper struct {
	*grocksdb.Iterator

	table string
}

//func (iter *iteratorWrapper) SeekToFirst() {
//	iter.Iterator.Seek(mergeKey(iter.table, []byte{}))
//}

//func (iter *iteratorWrapper) SeekToLast() {
//	iter.Iterator.SeekToFirst()
//}
//
//func (iter *iteratorWrapper) Next() {
//	iter.Iterator.Prev()
//}
//
//func (iter *iteratorWrapper) Prev() {
//	iter.Iterator.Next()
//}

type RocksDbIterator struct {
	tx    *grocksdb.Transaction
	ropts *grocksdb.ReadOptions

	// iterator of rocksdb.
	// when iterate values, must iterate dbvIter first if dbvIter is not null
	it *iteratorWrapper

	table string
	// beginPrefix and endPrefix come from table.
	// beginPrefix is included but endPrefix is excluded.
	beginPrefix []byte
	endPrefix   []byte

	current iterCache
}

func NewRocksDbIterator(tx *grocksdb.Transaction, table string) *RocksDbIterator {
	beginPrefix := mergeKey(table, []byte{})
	endPrefix, _ := kv.NextSubtree(beginPrefix)

	ropts := grocksdb.NewDefaultReadOptions()
	ropts.SetIterateLowerBound(beginPrefix)
	ropts.SetIterateUpperBound(endPrefix)
	it := &iteratorWrapper{Iterator: tx.NewIterator(ropts), table: table}
	it.SeekToFirst()

	current := invalidIterCache()
	if it.Valid() {
		current = createCacheWithPriorFirst(it)
	}

	return &RocksDbIterator{
		tx:    tx,
		ropts: ropts,

		table:       table,
		beginPrefix: beginPrefix,
		endPrefix:   endPrefix,
		it:          it,
		current:     current,
	}
}

func (iter *RocksDbIterator) Close() {
	if iter.it != nil {
		iter.it.Close()
		iter.it = nil
	}
	if iter.ropts != nil {
		iter.ropts.Destroy()
		iter.ropts = nil
	}
}

func (iter *RocksDbIterator) First() ([]byte, []byte, error) {
	iter.it.SeekToFirst()
	if !iter.it.Valid() {
		return nil, nil, iter.it.Err()
	}
	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, _ := iter.current.value.Current()
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) Last() ([]byte, []byte, error) {
	iter.it.SeekToLast()
	if !iter.it.Valid() {
		return nil, nil, ErrInvalidIter
	}
	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, ok := iter.current.value.SeekToLast()
	if !ok {
		panic("must have a value for Last")
	}
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) Current() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if v, ok := iter.current.value.Current(); ok {
			return iter.current.key, v, nil
		}
	}

	return nil, nil, ErrInvalidIter
}

func (iter *RocksDbIterator) NextKey() error {
	iter.it.Next()
	if !iter.it.Valid() {
		iter.current = invalidIterCache()
		return ErrInvalidIter
	}
	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	return nil
}

func (iter *RocksDbIterator) Count() (uint64, error) {
	it := &iteratorWrapper{Iterator: iter.tx.NewIterator(iter.ropts), table: iter.table}
	defer it.Close()

	count := uint64(0)
	for it.SeekToFirst(); it.Valid(); it.Next() {
		count += createCacheWithFirstValueIsCurrent(it).value.Count()
	}

	return count, nil
}

func (iter *RocksDbIterator) Seek(key []byte) ([]byte, []byte, error) {
	return iter.SeekWithValue(key, nil)
}

func (iter *RocksDbIterator) SeekWithValue(key, seekValue []byte) (k []byte, v []byte, err error) {
	defer func() {
		if err != nil {
			iter.current = invalidIterCache()
		}
	}()

	iter.it.Seek(mergeKey(iter.table, key))
	if !iter.it.Valid() {
		return nil, nil, ErrNotFound
	}
	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, err = iter.current.value.Seek(seekValue)
	return iter.current.key, v, err
}

func (iter *RocksDbIterator) SeekExact(key []byte) ([]byte, []byte, error) {
	return iter.SeekExactKeyWithGeValue(key, nil)
}

// seek to exact key and value >= seekV
func (iter *RocksDbIterator) SeekExactKeyWithGeValue(seekK, seekV []byte) (k []byte, v []byte, err error) {
	return iter.seekExactKeyWithValueSeekFunc(seekK, func(valueIter *DBValueIterator) ([]byte, error) {
		return valueIter.Seek(seekV)
	})
}

func (iter *RocksDbIterator) SeekExactKeyAndValue(seekK, seekV []byte) (k []byte, v []byte, err error) {
	// we call SeekExactKeyWithGeValue here because SeekExactKeyAndValue must
	// keep `current` consist with `SeekExactKeyWithGeValue`, even the value is mismatch.
	k, v, err = iter.SeekExactKeyWithGeValue(seekK, seekV)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(v, seekV) {
		return nil, nil, ErrNotFound
	}
	return k, v, err
}

func (iter *RocksDbIterator) Next() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if nextV, valid := iter.current.value.Next(); valid {
			return iter.current.key, nextV, nil
		}
	}

	iter.it.Next()
	if !iter.it.Valid() {
		// in rocksdb, if an Iterator get invalid by calling Next or Prev,
		// it can't be used again( e.g. can't be iterated to the contrary direction)
		// Now we got an invalid iterator, maybe because by calling Prev previously,
		// so we need to create it again and try call Next again.
		iter.reCreateIterator()
		iter.it.Seek(mergeKey(iter.table, iter.current.key))
		iter.it.Next()
		if !iter.it.Valid() {
			return nil, nil, ErrInvalidIter
		}
	}

	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, _ := iter.current.value.Current()
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) Prev() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if nextV, valid := iter.current.value.Prev(); valid {
			return iter.current.key, nextV, nil
		}
	}

	iter.it.Prev()
	if !iter.it.Valid() {
		// in rocksdb, if an Iterator get invalid by calling Next or Prev,
		// it can't be used again( e.g. can't be iterated to the contrary direction)
		// Now we got an invalid iterator, maybe because by calling Next previously,
		// so we need to create it again and try call Prev again.
		iter.reCreateIterator()
		iter.it.Seek(mergeKey(iter.table, iter.current.key))
		iter.it.Prev()
		if !iter.it.Valid() {
			return nil, nil, ErrInvalidIter
		}
	}

	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, _ := iter.current.value.SeekToLast()
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) LastDup() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if v, ok := iter.current.value.SeekToLast(); ok {
			return iter.current.key, v, nil
		}
	}

	iter.it.Next()
	if !iter.it.Valid() {
		return nil, nil, ErrInvalidIter
	}

	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, _ := iter.current.value.SeekToLast()
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) FirstDup() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if v, ok := iter.current.value.SeekToFirst(); ok {
			return iter.current.key, v, nil
		}
	}

	iter.it.Next()
	if !iter.it.Valid() {
		return nil, nil, ErrInvalidIter
	}

	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	v, _ := iter.current.value.SeekToFirst()
	return iter.current.key, v, nil
}

func (iter *RocksDbIterator) NextDup() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if nextV, valid := iter.current.value.Next(); valid {
			return iter.current.key, nextV, nil
		}
	}
	return nil, nil, ErrNotFound
}

func (iter *RocksDbIterator) PrevDup() ([]byte, []byte, error) {
	if iter.current.isValid() {
		if nextV, valid := iter.current.value.Prev(); valid {
			return iter.current.key, nextV, nil
		}
	}
	return nil, nil, ErrNotFound
}

func (iter *RocksDbIterator) seekExactKeyWithValueSeekFunc(key []byte, valueSeekFunc func(valueIter *DBValueIterator) ([]byte, error)) (k []byte, v []byte, err error) {
	iter.it.Seek(mergeKey(iter.table, key))
	if !iter.it.Valid() {
		iter.invalidCurrent()
		return nil, nil, ErrNotFound
	}
	iter.current = createCacheWithFirstValueIsCurrent(iter.it)

	sk := iter.it.Key()
	defer sk.Free()
	if !bytes.Equal(sk.Data(), mergeKey(iter.table, key)) {
		return nil, nil, ErrNotFound
	}

	v, err = valueSeekFunc(iter.current.value)
	if err != nil {
		iter.invalidCurrent()
	}
	return iter.current.key, v, err
}

func (iter *RocksDbIterator) currentKeyAndValueStamp() ([]byte, DBValueStamp, error) {
	if !iter.current.isValid() {
		return nil, DBValueStamp{}, ErrInvalidIter
	}
	return iter.current.key, iter.current.value.CurrentStamp(), nil
}

func (iter *RocksDbIterator) mustSeekToKeyValue(key, value []byte) {
	iter.it.Seek(mergeKey(iter.table, key))
	if !iter.it.Valid() {
		panic("seek to key must success")
	}

	iter.current = createCacheWithFirstValueIsCurrent(iter.it)
	iter.current.value.mustSeekToValue(value)
}

func (iter *RocksDbIterator) invalidCurrent() {
	iter.current = invalidIterCache()
}

func (iter *RocksDbIterator) reCreateIterator() {
	if iter.it != nil {
		iter.it.Close()
	}

	iter.it = &iteratorWrapper{Iterator: iter.tx.NewIterator(iter.ropts)}
}

func createCacheWithFirstValueIsCurrent(it *iteratorWrapper) iterCache {
	cache := createCacheWithPriorFirst(it)

	// the current of DBValueIterator is invalid when it is created,
	// so we should call Next to make the current get valid (which is the first one)
	cache.value.Next()
	_, ok := cache.value.Current()
	if !ok {
		panic("invalid DBValue")
	}
	return cache
}

func createCacheWithPriorFirst(it *iteratorWrapper) iterCache {
	_, key := splitKey(moveSliceToBytes(it.Key()))
	valueIter := newDBValueIterator(DeserializeDBValue(moveSliceToBytes(it.Value())))
	return iterCache{
		key:   key,
		value: valueIter,
	}
}
