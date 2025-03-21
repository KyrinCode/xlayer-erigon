package smt

import "errors"

type SmtCache struct {
	SmtCacheDataCh        chan SmtCacheToWrite
	FinishedBlockHeightCh chan uint64
	SmtCacheSnapshotList  *SmtCacheList

	DeltaSmtCache     map[string]map[string][]byte
	LongLivedSmtCache map[string]map[string][]byte
	LastResetHeight   uint64
}

type SmtCacheToWrite struct {
	SmtCacheData   map[string]map[string][]byte
	MaxBlockHeight uint64
}

func CreateNewSmtCache() *SmtCache {
	return &SmtCache{
		SmtCacheDataCh:        make(chan SmtCacheToWrite, 1),
		FinishedBlockHeightCh: make(chan uint64, 1000),
		SmtCacheSnapshotList:  NewSmtCacheList(),
		DeltaSmtCache:         make(map[string]map[string][]byte),
		LongLivedSmtCache:     make(map[string]map[string][]byte),
		LastResetHeight:       uint64(0),
	}
}

// TruncateSmtCacheList delete all the block snapshot cache that is lower than the target blockHeight
func (cache *SmtCache) TruncateSmtCacheList(blockHeight uint64) {
	cache.SmtCacheSnapshotList.delCache(blockHeight)
}

func (cache *SmtCache) GetSmtCache() map[string]map[string][]byte {
	return cache.LongLivedSmtCache
}

func (cache *SmtCache) GetSmtSnapshotCache(blockNumber uint64) map[string]map[string][]byte {
	cacheData, _ := cache.SmtCacheSnapshotList.getCacheShapshot(blockNumber, false)
	if cacheData == nil {
		cacheData = map[string]map[string][]byte{}
	}

	return cacheData
}

func (cache *SmtCache) SetSmtCache(blockNumber uint64, longLivedCache, blockCache map[string]map[string][]byte) {
	if cache.SmtCacheSnapshotList == nil {
		cache.SmtCacheSnapshotList = NewSmtCacheList()
	}
	if cache.LongLivedSmtCache == nil {
		cache.LongLivedSmtCache = make(map[string]map[string][]byte)
	}
	if cache.DeltaSmtCache == nil {
		cache.DeltaSmtCache = make(map[string]map[string][]byte)
	}

	cache.SmtCacheSnapshotList.Push(blockNumber, blockCache)

	// merge blockCache into deltaCache
	for table, bucket := range blockCache {
		if existingBucket, exists := cache.DeltaSmtCache[table]; exists {
			if existingBucket == nil {
				existingBucket = make(map[string][]byte)
				cache.DeltaSmtCache[table] = existingBucket
			}
			for k, v := range bucket {
				existingBucket[k] = v
			}
		} else {
			cache.DeltaSmtCache[table] = bucket
		}
	}

	if blockNumber-cache.LastResetHeight > 1000 {
		_, deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(true)
		if deltaSmtCache == nil {
			deltaSmtCache = map[string]map[string][]byte{}
		}

		// Reset LongLivedSmtCache, prevent excessive memory usage.
		cache.LongLivedSmtCache = deltaSmtCache
		cache.LastResetHeight = blockNumber
	} else {
		for table, bucket := range longLivedCache {
			cache.LongLivedSmtCache[table] = bucket
		}
	}
}

func (cache *SmtCache) CachedBlockLen() int {
	return cache.SmtCacheSnapshotList.Length()
}

func (cache *SmtCache) FlushSmtCache() error {
	blockHeight := cache.SmtCacheSnapshotList.MaxBlockHeight()

	cacheData := SmtCacheToWrite{
		cache.DeltaSmtCache,
		blockHeight,
	}

	select {
	case cache.SmtCacheDataCh <- cacheData:
		cache.DeltaSmtCache = map[string]map[string][]byte{}
		return nil
	default:
		return errors.New("failed to flush: channel is full or no receiver")
	}
}
