package smt

import (
	"context"
	"github.com/ledgerwatch/erigon/smt/pkg/db"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

func incrementalRawInsert(tree *SMT, key, val []*big.Int) {
	for i := range key {
		k := utils.ScalarToNodeKey(key[i])
		tree.InsertKA(k, val[i])
	}
}

func TestRawBatchSimpleInsert(t *testing.T) {

	keysRaw := []*big.Int{
		big.NewInt(8),
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(31),
		big.NewInt(31),
		big.NewInt(0),
		big.NewInt(8),
	}
	valuesRaw := []*big.Int{
		big.NewInt(17),
		big.NewInt(18),
		big.NewInt(19),
		big.NewInt(20),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}

	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	smtIncremental := NewSMT(db.NewMemDb(), false)
	smtRawIncremental := NewSMTRaw(db.NewRawMemDb(), false)
	//smtBatch := smt.NewSMT(nil, false)
	//smtBatchNoSave := smt.NewSMT(nil, true)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

		smtRawIncremental.InsertKA(k, valuesRaw[i])
		smtIncremental.InsertKA(k, valuesRaw[i])

	}
	root, _ := smtIncremental.getLastRoot()
	rootRaw, _ := smtRawIncremental.getLastRoot()

	assert.Equal(t, root, rootRaw)
	//smtIncremental.DumpTree()
	//insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	//_, err := smtBatch.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	//assert.NilError(t, err)
	//
	//_, err = smtBatchNoSave.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	//assert.NilError(t, err)
	//
	//fmt.Println()
	//smtBatch.DumpTree()
	//fmt.Println()
	//fmt.Println()
	//fmt.Println()
	//
	//smtIncrementalRootHash, _ := smtIncremental.Db.GetLastRoot()
	//smtBatchRootHash, _ := smtBatch.Db.GetLastRoot()
	//smtBatchNoSaveRootHash, _ := smtBatchNoSave.Db.GetLastRoot()
	//assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtIncrementalRootHash))
	//assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtBatchNoSaveRootHash))
	//
	//assertSmtDbStructure(t, smtBatch, false)
}

func BenchmarkRawSmtIncrementalInsert(b *testing.B) {
	keys := []*big.Int{}
	vals := []*big.Int{}
	for i := 0; i < 1000; i++ {
		rand.Seed(time.Now().UnixNano())
		keys = append(keys, big.NewInt(int64(rand.Intn(10000))))

		rand.Seed(time.Now().UnixNano())
		vals = append(vals, big.NewInt(int64(rand.Intn(10000))))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		smtIncremental := NewSMT(db.NewRawMemDb(), false)
		incrementalRawInsert(smtIncremental, keys, vals)
	}
}

func BenchmarkRawBatchInsert(b *testing.B) {
	keys := []*big.Int{}
	vals := []*big.Int{}
	for i := 0; i < 1000; i++ {
		rand.Seed(time.Now().UnixNano())
		keys = append(keys, big.NewInt(int64(rand.Intn(10000))))

		rand.Seed(time.Now().UnixNano())
		vals = append(vals, big.NewInt(int64(rand.Intn(10000))))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		smtBatch := NewSMT(db.NewRawMemDb(), false)
		batchInsert(smtBatch, keys, vals)
	}
}

func batchInsert(tree *SMT, key, val []*big.Int) {
	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	for i := range key {
		k := utils.ScalarToNodeKey(key[i])
		vArray := utils.ScalarToArrayBig(val[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)
	}
	insertBatchCfg := NewInsertBatchConfig(context.Background(), "", false)
	tree.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
}
