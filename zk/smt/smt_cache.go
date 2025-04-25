package smt

import (
	"errors"
	"sync"

	"github.com/benbjohnson/immutable"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

const TableSmt = "HermezSmt"
const TableStats = "HermezSmtStats"
const MetaLastHeight = "lastHeight"

var flushSmtCachePeriod = uint64(50)

type SmtCacheSave struct {
	SmtData     map[string]map[string][]byte
	BlockHeight uint64
}

type SmtCache struct {
	PushedHeap       *Uint64MinHeap
	ConfirmedHeap    *Uint64MinHeap
	LastPushedHeight uint64

	DeltaSnapshotList *SmtDeltaList
	DeltaSnapshotLock sync.RWMutex

	CurrentBatchBlockSnapshotList *SmtCacheList
	CurrentBatchSnapshotLock      sync.RWMutex

	SmtCacheDataCh   chan SmtCacheSave
	ToPushedSmtCache map[string]map[string][]byte

	PrimaryCache        *immutable.Map[string, *immutable.Map[string, []byte]]
	PrimaryCacheHistory map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]]
	PrimaryCacheLock    sync.RWMutex

	LastResetHeight       uint64
	LastResetSmtHeight    uint64
	LastRecordBlockHeight uint64
}

func CreateNewSmtCache() *SmtCache {
	return &SmtCache{
		PushedHeap:       NewUint64MinHeap(),
		ConfirmedHeap:    NewUint64MinHeap(),
		LastPushedHeight: 0,

		DeltaSnapshotList: NewSmtDeltaList(),

		CurrentBatchBlockSnapshotList: NewSmtCacheList(),

		SmtCacheDataCh:   make(chan SmtCacheSave, 1000),
		ToPushedSmtCache: make(map[string]map[string][]byte),

		PrimaryCache:        immutable.NewMap[string, *immutable.Map[string, []byte]](nil),
		PrimaryCacheHistory: make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]]),

		LastResetHeight:       0,
		LastResetSmtHeight:    0,
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
		cache.DeltaSnapshotLock.Lock()
		cache.DeltaSnapshotList.DeleteUpTo(maxConfirmedHeight)
		cache.DeltaSnapshotLock.Unlock()
	}
}

func (cache *SmtCache) GetSmtCache() *immutable.Map[string, *immutable.Map[string, []byte]] {
	cache.PrimaryCacheLock.RLock()
	defer cache.PrimaryCacheLock.RUnlock()

	return cache.PrimaryCache
}

func (cache *SmtCache) CascadeGetCurrentBatchSnapshotCache(blockNumber uint64) *immutable.Map[string, *immutable.Map[string, []byte]] {
	cache.PrimaryCacheLock.RLock()
	if blockNumber == cache.LastRecordBlockHeight {
		cache.PrimaryCacheLock.RUnlock()
		return cache.PrimaryCache
	}
	snapshot, exists := cache.PrimaryCacheHistory[blockNumber]
	cache.PrimaryCacheLock.RUnlock()

	if exists {
		return snapshot
	}

	cache.DeltaSnapshotLock.RLock()
	deltas := cache.DeltaSnapshotList.GetDeltaSnapshotUpTo(blockNumber)
	cache.DeltaSnapshotLock.RUnlock()

	cache.PrimaryCacheLock.RLock()
	defer cache.PrimaryCacheLock.RUnlock()

	tmpSnapShot := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	for _, delta := range deltas {
		for table, keys := range delta.ChangedKeys {
			innerMap, _ := tmpSnapShot.Get(table)
			if innerMap == nil {
				innerMap = immutable.NewMap[string, []byte](nil)
			}

			currentInner, _ := cache.PrimaryCache.Get(table)
			if currentInner == nil {
				continue
			}

			for key := range keys {
				if val, ok := currentInner.Get(key); ok {
					innerMap = innerMap.Set(key, val)
				}
			}

			tmpSnapShot = tmpSnapShot.Set(table, innerMap)
		}
	}

	return tmpSnapShot
}

func (cache *SmtCache) SetSmtCache(blockNumber uint64, blockCache map[string]map[string][]byte) {
	// Push to CurrentBatchBlockSnapshotList
	cache.CurrentBatchSnapshotLock.Lock()
	cache.CurrentBatchBlockSnapshotList.Push(blockNumber, blockCache)
	cache.CurrentBatchSnapshotLock.Unlock()

	changedKeys := make(map[string]map[string]struct{})

	// Update PrimaryCache
	cache.PrimaryCacheLock.Lock()
	newSnapshot := cache.PrimaryCache

	var wg sync.WaitGroup
	resultChan := make(chan struct {
		table      string
		inner      *immutable.Map[string, []byte]
		changeKeys map[string]struct{}
	}, len(blockCache))

	// Process each table concurrently
	for table, bucket := range blockCache {
		wg.Add(1)
		go func(table string, bucket map[string][]byte) {
			defer wg.Done()

			existingInner, _ := newSnapshot.Get(table)
			newInner := existingInner
			if newInner == nil {
				newInner = immutable.NewMap[string, []byte](nil)
			}

			changeKeys := make(map[string]struct{}, len(bucket))
			for key, value := range bucket {
				newInner = newInner.Set(key, value)
				changeKeys[key] = struct{}{}
			}

			resultChan <- struct {
				table      string
				inner      *immutable.Map[string, []byte]
				changeKeys map[string]struct{}
			}{table, newInner, changeKeys}
		}(table, bucket)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		newSnapshot = newSnapshot.Set(result.table, result.inner)
		changedKeys[result.table] = result.changeKeys
	}

	cache.PrimaryCacheHistory[blockNumber-1] = cache.PrimaryCache
	cache.PrimaryCache = newSnapshot
	cache.LastRecordBlockHeight = blockNumber
	cache.PrimaryCacheLock.Unlock()

	// Push to DeltaSnapshotList
	cache.DeltaSnapshotLock.Lock()
	cache.DeltaSnapshotList.Push(blockNumber, changedKeys)
	cache.DeltaSnapshotLock.Unlock()

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

	height, err := utils.ConvertBytesToUint64(cache.ToPushedSmtCache[TableStats][MetaLastHeight])
	if err != nil {
		return err
	}

	if height-cache.LastResetHeight > 10*flushSmtCachePeriod {
		cache.resetPrimaryCache(height, height-cache.LastResetSmtHeight >= 100*flushSmtCachePeriod)
	} else {
		cache.PrimaryCacheLock.Lock()
		cache.PrimaryCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
		cache.PrimaryCacheLock.Unlock()
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

func (cache *SmtCache) resetPrimaryCache(currentHeight uint64, resetSmt bool) {
	cache.DeltaSnapshotLock.RLock()
	allDeltas := cache.DeltaSnapshotList.GetAllChanges()
	cache.DeltaSnapshotLock.RUnlock()

	cache.PrimaryCacheLock.Lock()
	defer cache.PrimaryCacheLock.Unlock()

	newCache := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	for _, delta := range allDeltas {
		for table, keys := range delta.ChangedKeys {
			//if (table == TableSmt || table == TableStats) && !resetSmt {
			//	continue
			//}
			if table == TableStats && !resetSmt {
				continue
			}

			innerMap, _ := newCache.Get(table)
			if innerMap == nil {
				innerMap = immutable.NewMap[string, []byte](nil)
			}

			currentInner, _ := cache.PrimaryCache.Get(table)
			if currentInner == nil {
				continue
			}

			for key := range keys {
				if val, ok := currentInner.Get(key); ok {
					innerMap = innerMap.Set(key, val)
				}
			}

			newCache = newCache.Set(table, innerMap)
		}
	}

	if !resetSmt {
		//curSmtInner, _ := cache.PrimaryCache.Get(TableSmt)
		//if curSmtInner != nil {
		//	newCache = newCache.Set(TableSmt, curSmtInner)
		//}

		curStatusInner, _ := cache.PrimaryCache.Get(TableStats)
		if curStatusInner != nil {
			newCache = newCache.Set(TableStats, curStatusInner)
		}
	}

	cache.PrimaryCache = newCache
	cache.LastResetHeight = currentHeight
	if resetSmt {
		cache.LastResetSmtHeight = currentHeight
	}

	cache.PrimaryCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
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

	cache.DeltaSnapshotLock.Lock()
	for _, blockNumber := range deleteBlockList {
		cache.DeltaSnapshotList.DeleteUpTo(blockNumber)
	}
	allDeltas := cache.DeltaSnapshotList.GetAllChanges()
	cache.DeltaSnapshotLock.RUnlock()

	cache.PrimaryCacheLock.Lock()
	defer cache.PrimaryCacheLock.Unlock()

	newCache := immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	for _, delta := range allDeltas {
		for table, keys := range delta.ChangedKeys {
			innerMap, _ := newCache.Get(table)
			if innerMap == nil {
				innerMap = immutable.NewMap[string, []byte](nil)
			}

			currentInner, _ := cache.PrimaryCache.Get(table)
			if currentInner == nil {
				continue
			}

			for key := range keys {
				if val, ok := currentInner.Get(key); ok {
					innerMap = innerMap.Set(key, val)
				}
			}

			newCache = newCache.Set(table, innerMap)
		}
	}
	cache.PrimaryCache = newCache
	cache.PrimaryCacheHistory = make(map[uint64]*immutable.Map[string, *immutable.Map[string, []byte]])
	cache.LastResetHeight = resetBlockHeight - 1
	cache.LastResetSmtHeight = resetBlockHeight - 1
	cache.LastRecordBlockHeight = resetBlockHeight - 1
}
