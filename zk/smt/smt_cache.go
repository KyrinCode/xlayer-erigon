package smt

import (
	"errors"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
	"sync"
)

type SmtCache struct {
	PushedHeap       *Uint64MinHeap
	LastPushedHeight uint64
	ConfirmedHeap    *Uint64MinHeap

	SmtCacheDataCh       chan map[string]map[string][]byte
	SmtCacheSnapshotList *SmtCacheList

	PreBatchSnapshotImage         map[string]map[string][]byte
	PreBatchImageLastUpdateHeight uint64
	PreBatchImageLock             sync.RWMutex
	CurrentBatchBlockSnapshotList *SmtCacheList

	DeltaSmtCache     map[string]map[string][]byte // data wait to push, may contain several batch data
	LongLivedSmtCache map[string]map[string][]byte // only used by SpawnSequencingStage, so donot need lock to protect it
	LastResetHeight   uint64
}

func CreateNewSmtCache() *SmtCache {
	return &SmtCache{
		PushedHeap:       NewUint64MinHeap(),
		ConfirmedHeap:    NewUint64MinHeap(),
		LastPushedHeight: 0,

		SmtCacheDataCh:       make(chan map[string]map[string][]byte, 1),
		SmtCacheSnapshotList: NewSmtCacheList(),
		DeltaSmtCache:        make(map[string]map[string][]byte),
		LongLivedSmtCache:    make(map[string]map[string][]byte),
		LastResetHeight:      uint64(0),

		CurrentBatchBlockSnapshotList: NewSmtCacheList(),
		PreBatchSnapshotImage:         make(map[string]map[string][]byte),
		PreBatchImageLastUpdateHeight: 0,
	}
}

// TruncateSmtCacheList delete all the block snapshot cache that is lower than the target blockHeight
func (cache *SmtCache) TruncateSmtCacheList(blockHeight uint64) {
	cache.ConfirmedHeap.Push(blockHeight)

	truncateHeight := uint64(0)
	for {
		confirmHeight, _ := cache.ConfirmedHeap.ThreadSafeTop()
		pushedHeight, _ := cache.PushedHeap.ThreadSafeTop()
		if confirmHeight == pushedHeight && confirmHeight > 0 {
			log.Debug("TruncateSmtCacheList", "confirmHeight", confirmHeight)

			cache.ConfirmedHeap.ThreadSafePop()
			cache.PushedHeap.ThreadSafePop()
			cache.SmtCacheSnapshotList.cascadeDeleteCache(confirmHeight)
			truncateHeight = confirmHeight
		} else {
			break
		}
	}

	if truncateHeight > 0 {
		if truncateHeight-cache.PreBatchImageLastUpdateHeight > 20 {
			cache.PreBatchImageLock.Lock()
			defer cache.PreBatchImageLock.Unlock()

			deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(false)
			if deltaSmtCache == nil {
				deltaSmtCache = map[string]map[string][]byte{}
			}
			cache.PreBatchSnapshotImage = deltaSmtCache

			cache.PreBatchImageLastUpdateHeight = truncateHeight
		}

	}
}

func (cache *SmtCache) GetSmtCache() map[string]map[string][]byte {
	return cache.LongLivedSmtCache
}

func (cache *SmtCache) CascadeGetCurrentBatchSnapshotCache(blockNumber uint64) map[string]map[string][]byte {
	cacheData, ok := cache.CurrentBatchBlockSnapshotList.cascadeGetCacheShapshot(blockNumber, false)
	if !ok {
		cacheData, _ = cache.SmtCacheSnapshotList.cascadeGetCacheShapshot(blockNumber, false)
		return cacheData
	}

	cache.PreBatchImageLock.RLock()
	defer cache.PreBatchImageLock.RUnlock()

	// merge preBatchSnapShotImage into current data
	for table, bucket := range cache.PreBatchSnapshotImage {
		if _, exists := cacheData[table]; !exists {
			cacheData[table] = make(map[string][]byte)
		}

		for key, value := range bucket {
			// If the key already exists, keep the earliest value (parent priority)
			if _, exists := cacheData[table][key]; !exists {
				cacheData[table][key] = value // replace the old value with the latest one
			}
		}
	}

	return cacheData
}

// SetSmtCache set smt cache every block
func (cache *SmtCache) SetSmtCache(blockNumber uint64, longLivedCache, blockCache map[string]map[string][]byte) {
	cache.SmtCacheSnapshotList.Push(blockNumber, blockCache)
	cache.CurrentBatchBlockSnapshotList.Push(blockNumber, blockCache)

	// merge blockCache into deltaCache
	for table, bucket := range blockCache {
		if existingBucket, exists := cache.DeltaSmtCache[table]; exists {
			for k, v := range bucket {
				existingBucket[k] = v
			}
		} else {
			cache.DeltaSmtCache[table] = bucket
		}
	}

	if blockNumber-cache.LastResetHeight > 100 {
		deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(true)
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

// FlushSmtCache flush cache every batch!
func (cache *SmtCache) FlushSmtCache(batchPush bool) error {
	height, err := utils.ConvertBytesToUint64(cache.DeltaSmtCache["HermezSmtStats"]["lastHeight"])
	if err != nil {
		return err
	}

	cache.PreBatchImageLock.Lock()
	// 1. merge current batch cache image to PreBatchSnapshotImage
	currentBatchImage, _ := cache.CurrentBatchBlockSnapshotList.getAllCacheShapshot(false)
	for table, bucket := range currentBatchImage {
		if _, exists := cache.PreBatchSnapshotImage[table]; !exists {
			cache.PreBatchSnapshotImage[table] = make(map[string][]byte)
		}

		for key, value := range bucket {
			cache.PreBatchSnapshotImage[table][key] = value // replace the old value with the latest one
		}
	}
	cache.PreBatchImageLock.Unlock()

	// 2. clean current batch cache image
	cache.CurrentBatchBlockSnapshotList = NewSmtCacheList()

	// 3. check and push cached smt data to smt
	if batchPush && (height-cache.LastPushedHeight < 20) {
		return nil
	}

	cache.PushedHeap.Push(height)
	cache.LastPushedHeight = height

	select {
	case cache.SmtCacheDataCh <- cache.DeltaSmtCache:
		cache.DeltaSmtCache = map[string]map[string][]byte{}
		return nil
	default:
		return errors.New("failed to flush: channel is full or no receiver")
	}
}
