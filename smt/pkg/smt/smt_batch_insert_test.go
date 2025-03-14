package smt_test

import (
	"context"
	"fmt"
	db "github.com/ledgerwatch/erigon/smt/pkg/db"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/smt/pkg/smt"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"gotest.tools/v3/assert"
)

func TestBatchInsertEmptyTree(t *testing.T) {
	dbiSmt, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txSmt, _ := dbiSmt.BeginRw(context.Background())

	dbiChain, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txChain, _ := dbiChain.BeginRw(context.Background())
	database := db.NewEriDb(txSmt, txChain)
	err := db.CreateEriDbBuckets(txSmt)
	assert.NilError(t, err)

	keysRaw := []*big.Int{
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(31),
	}
	valuesRaw := []*big.Int{
		big.NewInt(18),
		big.NewInt(19),
		big.NewInt(20),
	}

	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	smtBatch := smt.NewSMT(database, false)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

	}
	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	_, err = smtBatch.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	assert.NilError(t, err)
	smtRoot, _ := smtBatch.Db.GetLastRoot()
	fmt.Printf("root is: %s \n", utils.ConvertBigIntToHex(smtRoot))
	assert.Equal(t, utils.ConvertBigIntToHex(smtRoot), "0xcde0f53a0e2e3e6e7a232442c5f01d6910552f9bae25b75a7dda346536f03c80")
}

func TestBatchInsertNoneEmptyTree(t *testing.T) {
	dbiSmt, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txSmt, _ := dbiSmt.BeginRw(context.Background())

	dbiChain, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txChain, _ := dbiChain.BeginRw(context.Background())
	database := db.NewEriDb(txSmt, txChain)
	err := db.CreateEriDbBuckets(txSmt)
	assert.NilError(t, err)

	keysRaw := []*big.Int{
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(31),
	}
	valuesRaw := []*big.Int{
		big.NewInt(18),
		big.NewInt(19),
		big.NewInt(20),
	}

	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	smtBatch := smt.NewSMT(database, false)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

	}
	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	_, err = smtBatch.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	assert.NilError(t, err)
	smtRoot, _ := smtBatch.Db.GetLastRoot()
	fmt.Printf("root is: %s \n", utils.ConvertBigIntToHex(smtRoot))
	assert.Equal(t, utils.ConvertBigIntToHex(smtRoot), "0xcde0f53a0e2e3e6e7a232442c5f01d6910552f9bae25b75a7dda346536f03c80")

	keysRaw2 := []*big.Int{
		big.NewInt(2),
		big.NewInt(1),
	}
	valuesRaw2 := []*big.Int{
		big.NewInt(21),
		big.NewInt(0),
	}
	keyPointers2 := []*utils.NodeKey{}
	valuePointers2 := []*utils.NodeValue8{}

	for i := range keysRaw2 {
		k := utils.ScalarToNodeKey(keysRaw2[i])
		vArray := utils.ScalarToArrayBig(valuesRaw2[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers2 = append(keyPointers2, &k)
		valuePointers2 = append(valuePointers2, v)

	}
	_, err = smtBatch.InsertBatch(insertBatchCfg, keyPointers2, valuePointers2, nil, nil)
	assert.NilError(t, err)
	smtRoot2, _ := smtBatch.Db.GetLastRoot()
	fmt.Printf("root is: %s \n", utils.ConvertBigIntToHex(smtRoot2))
	assert.Equal(t, utils.ConvertBigIntToHex(smtRoot2), "0xddf60f1805f940c8b6e6dbec357a89ab8ef82378ad46d86e8a62792564f6f4f1")

}

func TestBatchSimpleInsert(t *testing.T) {

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

	smtIncremental := smt.NewSMT(nil, false)
	smtBatch := smt.NewSMT(nil, false)
	//smtBatchNoSave := smt.NewSMT(nil, true)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

		smtIncremental.InsertKA(k, valuesRaw[i])
	}
	smtIncremental.DumpTree()
	fmt.Println("root", smtIncremental.LastRoot())
	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	_, err := smtBatch.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	smtBatch.DumpTree()
	fmt.Println("batch root", smtBatch.LastRoot())
	assert.NilError(t, err)

	//_, err = smtBatchNoSave.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	//assert.NilError(t, err)
	//
	fmt.Println()
	smtBatch.DumpTree()

	smtIncrementalRootHash, _ := smtIncremental.Db.GetLastRoot()
	smtBatchRootHash, _ := smtBatch.Db.GetLastRoot()
	//smtBatchNoSaveRootHash, _ := smtBatchNoSave.Db.GetLastRoot()
	assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtIncrementalRootHash))
	//assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtBatchNoSaveRootHash))
	//
	//assertSmtDbStructure(t, smtBatch, false)
}

func TestBatchSimpleInsertNoRemove(t *testing.T) {

	dbiSmt, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txSmt, _ := dbiSmt.BeginRw(context.Background())

	dbiChain, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	txChain, _ := dbiChain.BeginRw(context.Background())
	database := db.NewEriDb(txSmt, txChain)
	err := db.CreateEriDbBuckets(txSmt)
	assert.NilError(t, err)

	keysRaw := []*big.Int{
		big.NewInt(8),
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(31),
	}
	valuesRaw := []*big.Int{
		big.NewInt(17),
		big.NewInt(18),
		big.NewInt(19),
		big.NewInt(20),
	}

	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	smtIncremental := smt.NewSMT(database, false)
	// smtBatch := smt.NewSMT(nil, false)
	// smtBatchNoSave := smt.NewSMT(nil, true)

	for i := range keysRaw {
		k := utils.ScalarToNodeKey(keysRaw[i])
		vArray := utils.ScalarToArrayBig(valuesRaw[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)

		smtIncremental.InsertKA(k, valuesRaw[i])
	}

	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	// _, err := smtBatch.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
	// smtBatch.DumpTree()
	// assert.NilError(t, err)

	keysRaw2 := []*big.Int{
		big.NewInt(2),
	}
	valuesRaw2 := []*big.Int{
		big.NewInt(0),
	}

	keyPointers2 := []*utils.NodeKey{}
	valuePointers2 := []*utils.NodeValue8{}
	for i := range keysRaw2 {
		k := utils.ScalarToNodeKey(keysRaw2[i])
		vArray := utils.ScalarToArrayBig(valuesRaw2[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers2 = append(keyPointers2, &k)
		valuePointers2 = append(valuePointers2, v)
		// smtIncremental.InsertKA(k, valuesRaw2[i])
	}
	smtIncremental.InsertBatch(insertBatchCfg, keyPointers2, valuePointers2, nil, nil)

	// smtBatch.DumpTree()

	// fmt.Println()
	// smtBatch.DumpTree()
	// fmt.Println()
	// fmt.Println()
	// fmt.Println()

	// smtIncrementalRootHash, _ := smtIncremental.Db.GetLastRoot()
	// smtBatchRootHash, _ := smtBatch.Db.GetLastRoot()
	// smtBatchNoSaveRootHash, _ := smtBatchNoSave.Db.GetLastRoot()
	// assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtIncrementalRootHash))
	// assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtBatchNoSaveRootHash))

	// assertSmtDbStructure(t, smtBatch, false)
}

func TestBatchRawInsert(t *testing.T) {
	keysForBatch := []*utils.NodeKey{}
	valuesForBatch := []*utils.NodeValue8{}

	keysForIncremental := []utils.NodeKey{}
	valuesForIncremental := []utils.NodeValue8{}

	smtIncremental := smt.NewSMT(nil, false)
	smtBatch := smt.NewSMT(nil, false)

	rand.Seed(1)
	size := 1 << 10
	for i := 0; i < size; i++ {
		rawKey := big.NewInt(rand.Int63())
		rawValue := big.NewInt(rand.Int63())

		k := utils.ScalarToNodeKey(rawKey)
		vArray := utils.ScalarToArrayBig(rawValue)
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keysForBatch = append(keysForBatch, &k)
		valuesForBatch = append(valuesForBatch, v)

		keysForIncremental = append(keysForIncremental, k)
		valuesForIncremental = append(valuesForIncremental, *v)

	}

	startTime := time.Now()
	for i := range keysForIncremental {
		smtIncremental.Insert(keysForIncremental[i], valuesForIncremental[i])
	}
	t.Logf("Incremental insert %d values in %v\n", len(keysForIncremental), time.Since(startTime))

	startTime = time.Now()

	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", true)
	_, err := smtBatch.InsertBatch(insertBatchCfg, keysForBatch, valuesForBatch, nil, nil)
	assert.NilError(t, err)
	t.Logf("Batch insert %d values in %v\n", len(keysForBatch), time.Since(startTime))

	smtIncrementalRootHash, _ := smtIncremental.Db.GetLastRoot()
	smtBatchRootHash, _ := smtBatch.Db.GetLastRoot()
	assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtIncrementalRootHash))

	assertSmtDbStructure(t, smtBatch, false)

	// DELETE
	keysForBatchDelete := []*utils.NodeKey{}
	valuesForBatchDelete := []*utils.NodeValue8{}

	keysForIncrementalDelete := []utils.NodeKey{}
	valuesForIncrementalDelete := []utils.NodeValue8{}

	sizeToDelete := 1 << 14
	for i := 0; i < sizeToDelete; i++ {
		rawValue := big.NewInt(0)
		vArray := utils.ScalarToArrayBig(rawValue)
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		deleteIndex := rand.Intn(size)

		keyForBatchDelete := keysForBatch[deleteIndex]
		keyForIncrementalDelete := keysForIncremental[deleteIndex]

		keysForBatchDelete = append(keysForBatchDelete, keyForBatchDelete)
		valuesForBatchDelete = append(valuesForBatchDelete, v)

		keysForIncrementalDelete = append(keysForIncrementalDelete, keyForIncrementalDelete)
		valuesForIncrementalDelete = append(valuesForIncrementalDelete, *v)
	}

	startTime = time.Now()
	for i := range keysForIncrementalDelete {
		smtIncremental.Insert(keysForIncrementalDelete[i], valuesForIncrementalDelete[i])
	}
	t.Logf("Incremental delete %d values in %v\n", len(keysForIncrementalDelete), time.Since(startTime))

	startTime = time.Now()

	_, err = smtBatch.InsertBatch(insertBatchCfg, keysForBatchDelete, valuesForBatchDelete, nil, nil)
	assert.NilError(t, err)
	t.Logf("Batch delete %d values in %v\n", len(keysForBatchDelete), time.Since(startTime))

	assertSmtDbStructure(t, smtBatch, false)
}

func BenchmarkIncrementalInsert(b *testing.B) {
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
		smtIncremental := smt.NewSMT(nil, false)
		incrementalInsert(smtIncremental, keys, vals)
	}
}

func BenchmarkBatchInsert(b *testing.B) {
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
		smtBatch := smt.NewSMT(nil, false)
		batchInsert(smtBatch, keys, vals)
	}
}

func BenchmarkBatchInsertNoSave(b *testing.B) {
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
		smtBatch := smt.NewSMT(nil, true)
		batchInsert(smtBatch, keys, vals)
	}
}

func TestBatchSimpleInsert2(t *testing.T) {
	keys := []*big.Int{}
	vals := []*big.Int{}
	for i := 0; i < 2; i++ {
		rand.Seed(time.Now().UnixNano())
		keys = append(keys, big.NewInt(int64(rand.Intn(10000))))

		rand.Seed(time.Now().UnixNano())
		vals = append(vals, big.NewInt(int64(rand.Intn(10000))))
	}

	smtIncremental := smt.NewSMT(nil, false)
	incrementalInsert(smtIncremental, keys, vals)

	smtBatch := smt.NewSMT(nil, false)
	fmt.Printf("Batch insert keys %v values %v \n", keys, vals)
	batchInsert(smtBatch, keys, vals)

	smtBatchNoSave := smt.NewSMT(nil, false)
	batchInsert(smtBatchNoSave, keys, vals)

	smtIncrementalRootHash, _ := smtIncremental.Db.GetLastRoot()
	smtBatchRootHash, _ := smtBatch.Db.GetLastRoot()
	//smtBatchNoSaveRootHash, _ := smtBatchNoSave.Db.GetLastRoot()

	assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtIncrementalRootHash))
	//assert.Equal(t, utils.ConvertBigIntToHex(smtBatchRootHash), utils.ConvertBigIntToHex(smtBatchNoSaveRootHash))
}

func incrementalInsert(tree *smt.SMT, key, val []*big.Int) {
	for i := range key {
		k := utils.ScalarToNodeKey(key[i])
		tree.InsertKA(k, val[i])
	}
}

func batchInsert(tree *smt.SMT, key, val []*big.Int) {
	keyPointers := []*utils.NodeKey{}
	valuePointers := []*utils.NodeValue8{}

	for i := range key {
		k := utils.ScalarToNodeKey(key[i])
		vArray := utils.ScalarToArrayBig(val[i])
		v, _ := utils.NodeValue8FromBigIntArray(vArray)

		keyPointers = append(keyPointers, &k)
		valuePointers = append(valuePointers, v)
	}
	insertBatchCfg := smt.NewInsertBatchConfig(context.Background(), "", false)
	tree.InsertBatch(insertBatchCfg, keyPointers, valuePointers, nil, nil)
}
