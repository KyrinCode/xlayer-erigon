package smt

import (
	"errors"
	"sync"

	"github.com/benbjohnson/immutable"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

var flushSmtCachePeriod = uint64(50)

type SmtCacheSave struct {
	SmtData     map[string]map[string][]byte
	BlockHeight uint64
}

type SmtCache struct {
	PushedHeap       *Uint64MinHeap
	LastPushedHeight uint64
	ConfirmedHeap    *Uint64MinHeap

	SmtCacheSnapshotList *SmtCacheList
	SmtCacheSnapshotLock sync.RWMutex

	CurrentBatchBlockSnapshotList *SmtCacheList
	CurrentBatchSnapshotLock      sync.RWMutex

	SmtCacheDataCh   chan SmtCacheSave
	ToPushedSmtCache map[string]map[string][]byte

	LongLivedSmtCache        *immutable.Map[string, *immutable.Map[string, []byte]]
	LongLivedSmtCacheHistory map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]]
	LongLivedSmtCacheLock    sync.RWMutex

	LastResetHeight       uint64
	LastRecordBlockHeight uint64
}

func CreateNewSmtCache() *SmtCache {
	return &SmtCache{
		PushedHeap:       NewUint64MinHeap(),
		ConfirmedHeap:    NewUint64MinHeap(),
		LastPushedHeight: 0,

		SmtCacheSnapshotList:          NewSmtCacheList(),
		CurrentBatchBlockSnapshotList: NewSmtCacheList(),

		SmtCacheDataCh:   make(chan SmtCacheSave, 1000),
		ToPushedSmtCache: make(map[string]map[string][]byte),

		LongLivedSmtCache:        immutable.NewMap[string, *immutable.Map[string, []byte]](nil),
		LongLivedSmtCacheHistory: make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]]),

		LastResetHeight:       0,
		LastRecordBlockHeight: 0,
	}
}

func (cache *SmtCache) TruncateSmtCacheList(blockHeight uint64) {
	cache.ConfirmedHeap.ThreadSafePush(blockHeight)

	maxConfirmedHeight := uint64(0)
	for {
		confirmHeight, _ := cache.ConfirmedHeap.ThreadSafeTop()
		pushedHeight, _ := cache.PushedHeap.ThreadSafeTop()
		if confirmHeight == pushedHeight && confirmHeight > 0 {
			cache.ConfirmedHeap.ThreadSafePop()
			cache.PushedHeap.ThreadSafePop()
			maxConfirmedHeight = confirmHeight
		} else {
			break
		}
	}

	if maxConfirmedHeight > 0 {
		cache.SmtCacheSnapshotLock.Lock()
		cache.SmtCacheSnapshotList.cascadeDeleteCache(maxConfirmedHeight)
		cache.SmtCacheSnapshotLock.Unlock()
	}
}

func (cache *SmtCache) GetSmtCache() *immutable.Map[string, *immutable.Map[string, []byte]] {
	cache.LongLivedSmtCacheLock.RLock()
	defer cache.LongLivedSmtCacheLock.RUnlock()

	return cache.LongLivedSmtCache
}

func (cache *SmtCache) CascadeGetCurrentBatchSnapshotCache(blockNumber uint64) *immutable.Map[string, *immutable.Map[string, []byte]] {
	cache.LongLivedSmtCacheLock.RLock()
	if snapshot, exists := cache.LongLivedSmtCacheHistory[blockNumber]; exists {
		cache.LongLivedSmtCacheLock.RUnlock()
		return snapshot
	}

	cache.SmtCacheSnapshotLock.RLock()
	defer cache.SmtCacheSnapshotLock.RUnlock()
	cacheData, _ := cache.SmtCacheSnapshotList.cascadeGetCacheShapshot(blockNumber)

	tmpSnapShot := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	if cacheData != nil && len(cacheData) > 0 {
		for table, bucket := range cacheData {
			innerMap := immutable.NewMap[string, []byte](nil)
			for k, v := range bucket {
				innerMap = innerMap.Set(k, v)
			}
			tmpSnapShot = tmpSnapShot.Set(table, innerMap)
		}
	}

	return tmpSnapShot
}

func (cache *SmtCache) SetSmtCache(blockNumber uint64, blockCache map[string]map[string][]byte) {
	cache.SmtCacheSnapshotLock.Lock()
	cache.SmtCacheSnapshotList.Push(blockNumber, blockCache)
	cache.SmtCacheSnapshotLock.Unlock()

	cache.CurrentBatchSnapshotLock.Lock()
	cache.CurrentBatchBlockSnapshotList.Push(blockNumber, blockCache)
	cache.CurrentBatchSnapshotLock.Unlock()

	cache.LongLivedSmtCacheLock.Lock()
	newSnapshot := cache.LongLivedSmtCache

	var wg sync.WaitGroup
	resultChan := make(chan struct {
		table string
		inner *immutable.Map[string, []byte]
	}, len(blockCache))

	// Process each table concurrently
	for table, bucket := range blockCache {
		wg.Add(1)
		go func(table string, bucket map[string][]byte) {
			defer wg.Done()

			// Get or create inner map
			existingInner, _ := newSnapshot.Get(table)
			newInner := existingInner
			if newInner == nil {
				newInner = immutable.NewMap[string, []byte](nil)
			}

			// Update inner map
			for key, value := range bucket {
				newInner = newInner.Set(key, value)
			}

			// Send result
			resultChan <- struct {
				table string
				inner *immutable.Map[string, []byte]
			}{table, newInner}
		}(table, bucket)
	}

	// Collect results in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Update snapshot with results
	for result := range resultChan {
		newSnapshot = newSnapshot.Set(result.table, result.inner)
	}

	cache.LongLivedSmtCacheHistory[blockNumber] = cache.LongLivedSmtCache
	cache.LongLivedSmtCache = newSnapshot
	cache.LastRecordBlockHeight = blockNumber
	cache.LongLivedSmtCacheLock.Unlock()
}

func (cache *SmtCache) FlushSmtCache(batchPush, grace bool) error {
	cache.CurrentBatchSnapshotLock.RLock()
	currentBatchImage, _ := cache.CurrentBatchBlockSnapshotList.getAllCacheShapshot()
	cache.CurrentBatchSnapshotLock.RUnlock()

	for table, bucket := range currentBatchImage {
		if _, exists := cache.ToPushedSmtCache[table]; !exists {
			cache.ToPushedSmtCache[table] = make(map[string][]byte, len(bucket))
		}
		for key, value := range bucket {
			cache.ToPushedSmtCache[table][key] = value
		}
	}

	cache.CurrentBatchSnapshotLock.Lock()
	cache.CurrentBatchBlockSnapshotList = NewSmtCacheList()
	cache.CurrentBatchSnapshotLock.Unlock()

	height, err := utils.ConvertBytesToUint64(cache.ToPushedSmtCache["HermezSmtStats"]["lastHeight"])
	if err != nil {
		return err
	}

	if height-cache.LastResetHeight > 10*flushSmtCachePeriod {
		cache.SmtCacheSnapshotLock.RLock()
		tmpSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot()
		cache.SmtCacheSnapshotLock.RUnlock()

		cache.LongLivedSmtCacheLock.Lock()
		newSnapshot := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
		if tmpSmtCache != nil && len(tmpSmtCache) > 0 {
			for table, bucket := range tmpSmtCache {
				innerMap := immutable.NewMap[string, []byte](nil)
				for k, v := range bucket {
					innerMap = innerMap.Set(k, v)
				}
				newSnapshot = newSnapshot.Set(table, innerMap)
			}
		}
		cache.LongLivedSmtCache = newSnapshot
		cache.LongLivedSmtCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
		cache.LastResetHeight = height
		cache.LongLivedSmtCacheLock.Unlock()
	} else {
		cache.LongLivedSmtCacheLock.Lock()
		cache.LongLivedSmtCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
		cache.LongLivedSmtCacheLock.Unlock()
	}

	if batchPush && (height-cache.LastPushedHeight < flushSmtCachePeriod) && !grace {
		return nil
	}

	data := SmtCacheSave{
		SmtData:     cache.ToPushedSmtCache,
		BlockHeight: height,
	}

	select {
	case cache.SmtCacheDataCh <- data:
		cache.PushedHeap.ThreadSafePush(height)
		cache.LastPushedHeight = height
		cache.ToPushedSmtCache = make(map[string]map[string][]byte)
		return nil
	default:
		return errors.New("failed to flush: channel is full or no receiver")
	}
}

func (cache *SmtCache) ResetCurrentBatch(resetBlockHeight uint64) {
	if resetBlockHeight > cache.LastRecordBlockHeight {
		return
	}

	deleteBlockList := make([]uint64, 0, resetBlockHeight-cache.LastRecordBlockHeight+1)
	for i := resetBlockHeight; i <= cache.LastRecordBlockHeight; i++ {
		deleteBlockList = append(deleteBlockList, i)
	}

	cache.CurrentBatchSnapshotLock.Lock()
	for _, blockNumber := range deleteBlockList {
		cache.CurrentBatchBlockSnapshotList.deleteTargetCache(blockNumber)
	}
	cache.CurrentBatchSnapshotLock.Unlock()

	cache.SmtCacheSnapshotLock.Lock()
	for _, blockNumber := range deleteBlockList {
		cache.SmtCacheSnapshotList.deleteTargetCache(blockNumber)
	}
	tmpSmtCache, _ := cache.SmtCacheSnapshotList.getAllCacheShapshot()
	cache.SmtCacheSnapshotLock.Unlock()

	cache.LongLivedSmtCacheLock.Lock()
	newSnapshot := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	if tmpSmtCache != nil && len(tmpSmtCache) > 0 {
		for table, bucket := range tmpSmtCache {
			innerMap := immutable.NewMap[string, []byte](nil)
			for k, v := range bucket {
				innerMap = innerMap.Set(k, v)
			}
			newSnapshot = newSnapshot.Set(table, innerMap)
		}
	}
	cache.LongLivedSmtCache = newSnapshot
	cache.LongLivedSmtCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
	cache.LastResetHeight = resetBlockHeight - 1
	cache.LastRecordBlockHeight = resetBlockHeight - 1
	cache.LongLivedSmtCacheLock.Unlock()
}
