package smt

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/smt/pkg/db"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
	"math/big"
	"testing"
)

func incrementalRawInsert(tree *SMT, key, val []*big.Int) {
	for i := range key {
		k := utils.ScalarToNodeKey(key[i])
		tree.InsertKA(k, val[i])
	}
}

func TestRawBatchSimpleInsert(t *testing.T) {

	dbDir := "/Users/yangweitao/data/xlayer/test_mdbx_old"
	fmt.Println("dbDir", dbDir)

	logger := log.New() // Creates a default logger
	// Open a permanent database
	opts := mdbx.NewMDBX(logger).Path(dbDir)
	isMem := opts.GetInMem()
	fmt.Println("isMem", isMem)

	dbi, _ := opts.Open(context.Background())

	tx, _ := dbi.BeginRw(context.Background())
	localDb := db.NewEriDb(tx)
	_ = db.CreateEriDbBuckets(tx)

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

	smtIncremental := NewSMT(localDb, false)
	//smtBatch := smt.NewSMT(nil, false)
	//smtBatchNoSave := smt.NewSMT(nil, true)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

		smtIncremental.InsertKA(k, valuesRaw[i])

	}
	root, _ := smtIncremental.getLastRoot()
	fmt.Println("root", root)
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

//func BenchmarkRawSmtIncrementalInsert(b *testing.B) {
//	keys := []*big.Int{}
//	vals := []*big.Int{}
//	for i := 0; i < 1000; i++ {
//		rand.Seed(time.Now().UnixNano())
//		keys = append(keys, big.NewInt(int64(rand.Intn(10000))))
//
//		rand.Seed(time.Now().UnixNano())
//		vals = append(vals, big.NewInt(int64(rand.Intn(10000))))
//	}
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		smtIncremental := NewSMT(db.NewRawMemDb(), false)
//		incrementalRawInsert(smtIncremental, keys, vals)
//	}
//}

//func BenchmarkRawBatchInsert(b *testing.B) {
//	keys := []*big.Int{}
//	vals := []*big.Int{}
//	for i := 0; i < 1000; i++ {
//		rand.Seed(time.Now().UnixNano())
//		keys = append(keys, big.NewInt(int64(rand.Intn(10000))))
//
//		rand.Seed(time.Now().UnixNano())
//		vals = append(vals, big.NewInt(int64(rand.Intn(10000))))
//	}
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		smtBatch := NewSMT(db.NewRawMemDb(), false)
//		batchInsert(smtBatch, keys, vals)
//	}
//}

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
