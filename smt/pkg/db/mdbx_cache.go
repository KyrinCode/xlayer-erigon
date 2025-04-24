package db

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/benbjohnson/immutable"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/membatch"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
)

type EriCacheDb struct {
	kvTx        kv.Tx
	cacheTx     SmtDbTx
	kvTxChainDB kv.RwTx
	*EriRoDb
}

func NewEriCacheDb(ctx context.Context, txsmt kv.Tx, txcdb kv.RwTx) *EriCacheDb {
	batch := membatch.NewHashCacheBatch(txsmt, ctx.Done(), "./tempdb-cache", log.New())
	defer func() {
		batch.Close()
	}()

	return &EriCacheDb{
		cacheTx:     batch,
		kvTx:        txsmt,
		kvTxChainDB: txcdb,
		EriRoDb:     NewRoEriDb(batch, txcdb),
	}
}

func (m *EriCacheDb) OpenBatch(quitCh <-chan struct{}) {}

func (m *EriCacheDb) SetCache(smtCachedMapValue *immutable.Map[string, *immutable.Map[string, []byte]]) {
	if smtCachedMapValue == nil {
		smtCachedMapValue = immutable.NewMap[string, *immutable.Map[string, []byte]](nil)
	}

	mapCache, ok := m.cacheTx.(*membatch.MapmutationWithDoubleCache)
	if !ok {
		return // don't roll back a kvRw tx
	}

	mapCache.SetCache(smtCachedMapValue)
}

func (m *EriCacheDb) RetriveAndCleanCache() map[string]map[string][]byte {
	mapCache, ok := m.cacheTx.(*membatch.MapmutationWithDoubleCache)
	if !ok {
		return nil // don't roll back a kvRw tx
	}

	return mapCache.RetrieveAndCleanSmtCache(HermezSmtTables)
}

func (m *EriCacheDb) CommitBatch() error {
	batch, ok := m.cacheTx.(kv.PendingMutations)
	if !ok {
		return nil // don't roll back a kvRw tx
	}
	batch.Close()

	m.cacheTx = batch
	m.kvTxRoSMT = batch
	return nil
}

func (m *EriCacheDb) RollbackBatch() {
	batch, ok := m.cacheTx.(kv.PendingMutations)
	if !ok {
		return // don't roll back a kvRw tx
	}
	batch.Close()

	m.cacheTx = batch
	m.kvTxRoSMT = batch
}

func (m *EriCacheDb) SetLastRoot(r *big.Int) error {
	v := utils.ConvertBigIntToHex(r)
	return m.cacheTx.Put(TableStats, []byte(MetaLastRoot), []byte(v))
}

func (m *EriCacheDb) SetLastHeight(blockHeight uint64) error {
	v := utils.ConvertUint64ToBytes(blockHeight)
	return m.cacheTx.Put(TableStats, []byte(MetaLastHeight), []byte(v))
}

func (m *EriCacheDb) SetDepth(depth uint8) error {
	return m.cacheTx.Put(TableStats, []byte(MetaDepth), []byte{depth})
}

func (m *EriCacheDb) Insert(key utils.NodeKey, value utils.NodeValue12) error {
	k := utils.ArrayToHex(key[:])

	vConc := utils.ArrayToScalarBig(value[:])
	v := utils.ArrayToHex(vConc.Bits())

	return m.cacheTx.Put(TableSmt, utils.UnsafeStringToBytes(k), utils.UnsafeStringToBytes(v))
}

func (m *EriCacheDb) Delete(key string) error {
	return m.cacheTx.Delete(TableSmt, []byte(key))
}

func (m *EriCacheDb) DeleteByNodeKey(key utils.NodeKey) error {
	k := utils.ArrayToHex(key[:])
	return m.cacheTx.Delete(TableSmt, utils.UnsafeStringToBytes(k))
}

func (m *EriCacheDb) InsertAccountValue(key utils.NodeKey, value utils.NodeValue8) error {
	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	vals := make([]*big.Int, 8)
	copy(vals, value[:]) // Replace the loop with the copy function

	vConc := utils.ArrayToScalarBig(vals)
	v := utils.ConvertBigIntToHex(vConc)

	return m.cacheTx.Put(TableAccountValues, []byte(k), []byte(v))
}

func (m *EriCacheDb) InsertKeySource(key utils.NodeKey, value []byte) error {
	keyConc := utils.ArrayToScalar(key[:])

	return m.cacheTx.Put(TableMetadata, keyConc.Bytes(), value)
}

func (m *EriCacheDb) DeleteKeySource(key utils.NodeKey) error {
	keyConc := utils.ArrayToScalar(key[:])

	return m.cacheTx.Delete(TableMetadata, keyConc.Bytes())
}

func (m *EriCacheDb) InsertHashKey(key utils.NodeKey, value utils.NodeKey) error {
	keyConc := utils.ArrayToScalar(key[:])

	valConc := utils.ArrayToScalar(value[:])

	return m.cacheTx.Put(TableHashKey, keyConc.Bytes(), valConc.Bytes())
}

func (m *EriCacheDb) DeleteHashKey(key utils.NodeKey) error {
	keyConc := utils.ArrayToScalar(key[:])
	return m.cacheTx.Delete(TableHashKey, keyConc.Bytes())
}

func (m *EriCacheDb) AddCode(code []byte) error {
	codeHash := utils.HashContractBytecode(hex.EncodeToString(code))

	codeHashBytes, err := hex.DecodeString(strings.TrimPrefix(codeHash, "0x"))
	if err != nil {
		return err
	}

	codeHashBytes = utils.ResizeHashTo32BytesByPrefixingWithZeroes(codeHashBytes)

	return m.kvTxChainDB.Put(kv.Code, codeHashBytes, code)
}
