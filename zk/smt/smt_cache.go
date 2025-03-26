package smt

import (
	"errors"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"sync"
)

type SmtCache struct {
	PushedHeap       *Uint64MinHeap
	LastPushedHeight uint64
	ConfirmedHeap    *Uint64MinHeap

	SmtCacheDataCh       chan map[string]map[string][]byte
	SmtCacheSnapshotList *SmtCacheList
	SmtCacheSnapshotLock sync.RWMutex // Added lock for SmtCacheSnapshotList

	PreBatchSnapshotImage         map[string]map[string][]byte
	PreBatchImageLastUpdateHeight uint64
	PreBatchImageLock             sync.RWMutex

	CurrentBatchBlockSnapshotList *SmtCacheList
	CurrentBatchSnapshotLock      sync.RWMutex // Added lock for CurrentBatchBlockSnapshotList
	//CurrentBatchDeltaSmtCache     map[string]map[string][]byte

	DeltaSmtCache     map[string]map[string][]byte
	LongLivedSmtCache map[string]map[string][]byte
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
		//CurrentBatchDeltaSmtCache:     make(map[string]map[string][]byte),
		PreBatchSnapshotImage:         make(map[string]map[string][]byte),
		PreBatchImageLastUpdateHeight: 0,
	}
}

func (cache *SmtCache) TruncateSmtCacheList(blockHeight uint64) {
	cache.ConfirmedHeap.ThreadSafePush(blockHeight)

	truncateHeight := uint64(0)
	for {
		confirmHeight, _ := cache.ConfirmedHeap.ThreadSafeTop()
		pushedHeight, _ := cache.PushedHeap.ThreadSafeTop()
		if confirmHeight == pushedHeight && confirmHeight > 0 {
			cache.ConfirmedHeap.ThreadSafePop()
			cache.PushedHeap.ThreadSafePop()

			cache.SmtCacheSnapshotLock.Lock()
			cache.SmtCacheSnapshotList.cascadeDeleteCache(confirmHeight)
			cache.SmtCacheSnapshotLock.Unlock()
			truncateHeight = confirmHeight
		} else {
			break
		}
	}

	if truncateHeight > 0 {
		if truncateHeight-cache.PreBatchImageLastUpdateHeight > 20 {
			cache.SmtCacheSnapshotLock.RLock()
			deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(false)
			cache.SmtCacheSnapshotLock.RUnlock()

			if deltaSmtCache == nil {
				deltaSmtCache = map[string]map[string][]byte{}
			}
			cache.PreBatchImageLock.Lock()
			cache.PreBatchSnapshotImage = deltaSmtCache
			cache.PreBatchImageLastUpdateHeight = truncateHeight
			cache.PreBatchImageLock.Unlock()
		}
	}
}

func (cache *SmtCache) GetSmtCache() map[string]map[string][]byte {
	return cache.LongLivedSmtCache // Thread 3 only, no lock needed
}

func (cache *SmtCache) CascadeGetCurrentBatchSnapshotCache(blockNumber uint64) map[string]map[string][]byte {
	cache.CurrentBatchSnapshotLock.RLock()
	cacheData, ok := cache.CurrentBatchBlockSnapshotList.cascadeGetCacheShapshot(blockNumber, false)
	cache.CurrentBatchSnapshotLock.RUnlock()

	if !ok {
		cache.SmtCacheSnapshotLock.RLock()
		cacheData, _ = cache.SmtCacheSnapshotList.cascadeGetCacheShapshot(blockNumber, false)
		cache.SmtCacheSnapshotLock.RUnlock()
		return cacheData
	}

	cache.PreBatchImageLock.RLock()
	defer cache.PreBatchImageLock.RUnlock()

	result := make(map[string]map[string][]byte, len(cache.PreBatchSnapshotImage))
	for table, bucket := range cache.PreBatchSnapshotImage {
		if _, exist := result[table]; !exist {
			result[table] = make(map[string][]byte, len(bucket)*2)
		}
		for k, v := range bucket {
			result[table][k] = v
		}
	}

	for table, bucket := range cacheData {
		if _, exist := result[table]; !exist {
			result[table] = make(map[string][]byte, len(bucket))
		}
		for k, v := range bucket {
			result[table][k] = v
		}
	}

	return result
}

func (cache *SmtCache) SetSmtCache(blockNumber uint64, longLivedCache, blockCache map[string]map[string][]byte) {
	cache.SmtCacheSnapshotLock.Lock()
	cache.SmtCacheSnapshotList.Push(blockNumber, blockCache)
	cache.SmtCacheSnapshotLock.Unlock()

	cache.CurrentBatchSnapshotLock.Lock()
	cache.CurrentBatchBlockSnapshotList.Push(blockNumber, blockCache)
	cache.CurrentBatchSnapshotLock.Unlock()

	// Merge blockCache into deltaCache (Thread 3 only, no lock needed)
	for table, bucket := range blockCache {
		if _, exists := cache.DeltaSmtCache[table]; !exists { // TODO: replace DeltaSmtCache with CurrentBatchDeltaSmtCache
			cache.DeltaSmtCache[table] = make(map[string][]byte, len(bucket))
		}

		for k, v := range bucket {
			cache.DeltaSmtCache[table][k] = v
		}
	}

	for table, bucket := range longLivedCache {
		cache.LongLivedSmtCache[table] = bucket
	}
}

func (cache *SmtCache) FlushSmtCache(batchPush bool) error {
	// 1. merge current batch cache image to PreBatchSnapshotImage
	cache.CurrentBatchSnapshotLock.Lock()
	currentBatchImage, _ := cache.CurrentBatchBlockSnapshotList.getAllCacheShapshot(false)
	cache.CurrentBatchSnapshotLock.Unlock()

	cache.PreBatchImageLock.Lock()
	for table, bucket := range currentBatchImage {
		if _, exists := cache.PreBatchSnapshotImage[table]; !exists {
			cache.PreBatchSnapshotImage[table] = make(map[string][]byte, len(bucket))
		}
		for key, value := range bucket {
			cache.PreBatchSnapshotImage[table][key] = value // replace the old value with the latest one
		}
	}
	cache.PreBatchImageLock.Unlock()

	// 2. TODO:  merge current batch delta smt to DeltaSmtCache
	//for table, bucket := range cache.CurrentBatchDeltaSmtCache {
	//	if _, exists := cache.DeltaSmtCache[table]; !exists {
	//		cache.DeltaSmtCache[table] = make(map[string][]byte)
	//	}
	//
	//	for key, value := range bucket {
	//		cache.DeltaSmtCache[table][key] = value // replace the old value with the latest one
	//	}
	//}

	// 3. clean current batch cache image
	cache.CurrentBatchSnapshotLock.Lock()
	cache.CurrentBatchBlockSnapshotList = NewSmtCacheList()
	cache.CurrentBatchSnapshotLock.Unlock()

	//cache.CurrentBatchDeltaSmtCache = map[string]map[string][]byte{}

	height, err := utils.ConvertBytesToUint64(cache.DeltaSmtCache["HermezSmtStats"]["lastHeight"])
	if err != nil {
		return err
	}

	if height-cache.LastResetHeight > 100 {
		cache.SmtCacheSnapshotLock.RLock()
		deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(true)
		cache.SmtCacheSnapshotLock.RUnlock()

		if deltaSmtCache == nil {
			deltaSmtCache = map[string]map[string][]byte{}
		}

		// Reset LongLivedSmtCache, prevent excessive memory usage.
		cache.LongLivedSmtCache = deltaSmtCache
		cache.LastResetHeight = height
	}

	if batchPush && (height-cache.LastPushedHeight < 20) {
		return nil
	}

	select {
	case cache.SmtCacheDataCh <- cache.DeltaSmtCache:
		cache.PushedHeap.ThreadSafePush(height)
		cache.LastPushedHeight = height

		cache.DeltaSmtCache = map[string]map[string][]byte{}
		return nil
	default:
		return errors.New("failed to flush: channel is full or no receiver")
	}
}

func (cache *SmtCache) ResetCurrentBatch(lastBlockHeight uint64) {
	// 1. clean CurrentBatchBlockSnapshotList and CurrentBatchDeltaSmtCache
	cache.CurrentBatchSnapshotLock.Lock()
	currentBatchBlockList := cache.CurrentBatchBlockSnapshotList.getBlockList()
	cache.CurrentBatchBlockSnapshotList = NewSmtCacheList()
	cache.CurrentBatchSnapshotLock.Unlock()

	//cache.CurrentBatchDeltaSmtCache = map[string]map[string][]byte{}

	// 2. delete all block cache in current batch
	cache.SmtCacheSnapshotLock.Lock()
	for _, blockNumber := range currentBatchBlockList {
		cache.SmtCacheSnapshotList.deleteTargetCache(blockNumber)
	}

	// 3. reset longLive cache
	deltaSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot(true)
	cache.SmtCacheSnapshotLock.Unlock()

	if deltaSmtCache == nil {
		deltaSmtCache = map[string]map[string][]byte{}
	}

	cache.LongLivedSmtCache = deltaSmtCache
	cache.LastResetHeight = lastBlockHeight
}
