package memrdb

import (
	"errors"
	"sort"

	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/rdb/common"
)

// ascend order map
type OrderedMap struct {
	data map[string]*common.DBValue
	keys []string
}

type OrderedMapIterator struct {
	omap *OrderedMap

	beginPrefix []byte
	endPrefix   []byte

	valid   bool
	current int
	err     error
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		data: make(map[string]*common.DBValue),
		keys: make([]string, 0),
	}
}

func (m *OrderedMap) Put(key []byte, value *common.DBValue) {
	sKey := string(key)

	if _, exists := m.data[sKey]; !exists {
		m.keys = append(m.keys, sKey)
	}
	m.data[sKey] = value
	m.sort()
}

func (m *OrderedMap) Get(key []byte) (*common.DBValue, bool) {
	sKey := string(key)

	val, ok := m.data[sKey]
	return val, ok
}

func (m *OrderedMap) Delete(key []byte) {
	sKey := string(key)

	if _, ok := m.data[sKey]; ok {
		delete(m.data, sKey)
		for i, k := range m.keys {
			if k == sKey {
				m.keys = append(m.keys[:i], m.keys[i+1:]...)
				break
			}
		}
	}
}

func (m *OrderedMap) sort() {
	sort.Slice(m.keys, func(i, j int) bool {
		return m.keys[i] < m.keys[j]
	})
}

func (m *OrderedMap) NewIterator(beginPrefix, endPrefix []byte) *OrderedMapIterator {
	iter := &OrderedMapIterator{
		omap:        m,
		beginPrefix: beginPrefix,
		endPrefix:   endPrefix,
	}
	iter.setInvalid("init")

	return iter
}

func (iter *OrderedMapIterator) Valid() bool {
	return iter.valid
}

func (iter *OrderedMapIterator) SeekToFirst() {
	if iter.beginPrefix == nil {
		if len(iter.omap.keys) == 0 {
			iter.setInvalid("SeekToFirst: no keys")
			return
		}
		iter.setValid(0)
		return
	}

	iter.setInvalid("SeekToFirst: don't find begin prefix")
	beginPrefix := string(iter.beginPrefix)
	for i, key := range iter.omap.keys {
		if key >= beginPrefix {
			iter.setValid(i)
			break
		}
	}
}

func (iter *OrderedMapIterator) SeekToLast() {
	if iter.endPrefix == nil {
		if len(iter.omap.keys) == 0 {
			iter.setInvalid("SeekToLast: no keys")
			return
		}
		iter.setValid(len(iter.omap.keys) - 1)
		return
	}

	iter.setInvalid("SeekToLast: don't find end prefix")
	endPrefix := string(iter.endPrefix)
	for i := len(iter.omap.keys) - 1; i >= 0; i-- {
		key := iter.omap.keys[i]
		if key < endPrefix {
			iter.setValid(i)
			break
		}
	}
}

func (iter *OrderedMapIterator) Next() {
	if !iter.Valid() {
		return
	}

	iter.current++
	if iter.current >= len(iter.omap.keys) {
		iter.setInvalid("Next: no more items")
		return
	}

	curKey := iter.currentKey()
	if curKey >= string(iter.endPrefix) {
		iter.setInvalid("Next: exceed end prefix")
		return
	}
}

func (iter *OrderedMapIterator) Prev() {
	if !iter.Valid() {
		return
	}

	iter.current--
	if iter.current < 0 {
		iter.setInvalid("Prev: no more items")
		return
	}

	curKey := iter.currentKey()
	if curKey < string(iter.beginPrefix) {
		iter.setInvalid("Prev: exceed begin prefix")
		return
	}
}

func (iter *OrderedMapIterator) Seek(key []byte) {
	iter.SeekToFirst()
	if !iter.Valid() {
		return
	}

	seekKey := string(key)
	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		curKey := iter.currentKey()
		if curKey >= seekKey {
			return
		}
	}

	iter.setInvalid("Seek: dont find seek key")
}

func (iter *OrderedMapIterator) Key() []byte {
	if !iter.Valid() {
		return nil
	}

	curKey := iter.currentKey()
	return []byte(curKey)
}

func (iter *OrderedMapIterator) Value() *common.DBValue {
	if !iter.Valid() {
		return nil
	}

	curKey := iter.omap.keys[iter.current]
	return iter.omap.data[curKey]
}

func (iter *OrderedMapIterator) Close() {
	// do nothing
}

func (iter *OrderedMapIterator) Err() error {
	return iter.err
}

func (iter *OrderedMapIterator) currentKey() string {
	return iter.omap.keys[iter.current]
}

func (iter *OrderedMapIterator) setInvalid(err string) {
	iter.valid = false
	iter.current = -1
	iter.err = errors.New(err)
}

func (iter *OrderedMapIterator) setValid(current int) {
	iter.valid = true
	iter.current = current
	iter.err = nil
}
