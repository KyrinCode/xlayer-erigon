package membatchwithdb

import (
	"bytes"
	"github.com/ledgerwatch/erigon-lib/common"

	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/log/v3"
)

// MemoryMutationWithCache extends MemoryMutation with a caching layer
type MemoryMutationWithCache struct {
	*MemoryMutation
	cache map[string]map[string][]byte // Cache for key-value pairs
}

// NewMemoryBatchWithCache creates a MemoryMutation with caching
func NewMemoryBatchWithCache(tx kv.Tx, tmpDir string, logger log.Logger, cache map[string]map[string][]byte) *MemoryMutationWithCache {
	base := NewMemoryBatch(tx, tmpDir, logger)
	if cache == nil {
		cache = make(map[string]map[string][]byte)
	}

	return &MemoryMutationWithCache{
		MemoryMutation: base,
		cache:          cache,
	}
}

// NewMemoryBatchNoSequenceWithCache creates a cached version without sequence initialization
func NewMemoryBatchNoSequenceWithCache(tx kv.Tx, tmpDir string, logger log.Logger, cache map[string]map[string][]byte) *MemoryMutationWithCache {
	base := NewMemoryBatchNoSequence(tx, tmpDir, logger)
	if cache == nil {
		cache = make(map[string]map[string][]byte)
	}

	return &MemoryMutationWithCache{
		MemoryMutation: base,
		cache:          cache,
	}
}

// NewMemoryBatchWithSizeWithCache creates a cached version with custom size
func NewMemoryBatchWithSizeNoSequenceWithCache(tx kv.Tx, tmpDir string, mapSize datasize.ByteSize, cache map[string]map[string][]byte) *MemoryMutationWithCache {
	base := NewMemoryBatchWithSizeNoSequence(tx, tmpDir, mapSize)
	if cache == nil {
		cache = make(map[string]map[string][]byte)
	}

	return &MemoryMutationWithCache{
		MemoryMutation: base,
		cache:          cache,
	}
}

// NewMemoryBatchWithCustomDBWithCache creates a cached version with a custom DB
func NewMemoryBatchWithCustomDBWithCache(tx kv.Tx, db kv.RwDB, uTx kv.RwTx, tmpDir string, cache map[string]map[string][]byte) *MemoryMutationWithCache {
	base := NewMemoryBatchWithCustomDB(tx, db, uTx, tmpDir)
	if cache == nil {
		cache = make(map[string]map[string][]byte)
	}

	return &MemoryMutationWithCache{
		MemoryMutation: base,
		cache:          cache,
	}
}

// GetOne with cache support, returns cached value if available
func (m *MemoryMutationWithCache) GetOne(table string, key []byte) ([]byte, error) {
	if m.isTableCleared(table) || m.isEntryDeleted(table, key) {
		return nil, nil
	}

	keyStr := string(key)
	if keys, ok := m.cache[table]; ok {
		if cachedVal, exists := keys[keyStr]; exists {
			return cachedVal, nil // Directly return cached value
		}
	}

	c, err := m.statelessCursor(table)
	if err != nil {
		return nil, err
	}
	_, v, err := c.SeekExact(key)
	if err == nil && v != nil {
		if _, ok := m.cache[table]; !ok {
			m.cache[table] = make(map[string][]byte)
		}
		// Store a copy of the value in the cache
		m.cache[table][keyStr] = common.Copy(v)
	}
	return v, err
}

// Has with cache support
func (m *MemoryMutationWithCache) Has(table string, key []byte) (bool, error) {
	if m.isTableCleared(table) || m.isEntryDeleted(table, key) {
		return false, nil
	}

	keyStr := string(key)
	if keys, ok := m.cache[table]; ok {
		if _, exists := keys[keyStr]; exists {
			return true, nil
		}
	}

	c, err := m.statelessCursor(table)
	if err != nil {
		return false, err
	}
	k, _, err := c.Seek(key)
	if err != nil {
		return false, err
	}
	exists := bytes.Equal(key, k)
	if exists {
		if _, ok := m.cache[table]; !ok {
			m.cache[table] = make(map[string][]byte)
		}
		// Cache presence only, value will be updated on GetOne
		m.cache[table][keyStr] = nil
	}
	return exists, nil
}

// Put with cache support
func (m *MemoryMutationWithCache) Put(table string, k, v []byte) error {
	err := m.memTx.Put(table, k, v)
	if err != nil {
		return err
	}
	if _, ok := m.cache[table]; !ok {
		m.cache[table] = make(map[string][]byte)
	}
	// Store a copy of the value in the cache
	m.cache[table][string(k)] = common.Copy(v)
	return nil
}

// Append with cache support
func (m *MemoryMutationWithCache) Append(table string, key []byte, value []byte) error {
	err := m.memTx.Append(table, key, value)
	if err != nil {
		return err
	}
	if _, ok := m.cache[table]; !ok {
		m.cache[table] = make(map[string][]byte)
	}
	// Store a copy of the value in the cache
	m.cache[table][string(key)] = common.Copy(value)
	return nil
}

// Delete with cache support
func (m *MemoryMutationWithCache) Delete(table string, k []byte) error {
	err := m.MemoryMutation.Delete(table, k)
	if err != nil {
		return err
	}
	if keys, ok := m.cache[table]; ok {
		delete(keys, string(k))
	}
	return nil
}

// Commit with cache support
func (m *MemoryMutationWithCache) Commit() error {
	err := m.MemoryMutation.Commit()
	if err != nil {
		return err
	}
	m.cache = make(map[string]map[string][]byte)
	return nil
}

// Rollback with cache support
func (m *MemoryMutationWithCache) Rollback() {
	m.MemoryMutation.Rollback()
	m.cache = make(map[string]map[string][]byte)
}

// ClearBucket with cache support
func (m *MemoryMutationWithCache) ClearBucket(bucket string) error {
	err := m.MemoryMutation.ClearBucket(bucket)
	if err != nil {
		return err
	}
	delete(m.cache, bucket)
	return nil
}
