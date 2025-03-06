package db

import (
	"context"
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestEriDb(t *testing.T) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx)
	err := CreateEriDbBuckets(tx)
	assert.NoError(t, err)

	// Rest of your test code remains the same
	for i := 0; i < 10; i++ {
		key := utils.NodeKey{uint64(i), uint64(i), uint64(i), uint64(i)}
		nodeValue := utils.NodeValue8Raw{uint64(i), uint64(i), uint64(i), uint64(i), uint64(i), uint64(i), uint64(i), uint64(i)}
		val := utils.NodeValue12Raw{
			Value: nodeValue,
			Flag:  byte(i % 2),
		}
		err = db.InsertRaw(key, val)
		assert.NoError(t, err)
		retrievedValue, err := db.GetRaw(key)
		assert.NoError(t, err)
		assert.Equal(t, val, retrievedValue)
	}

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)
}

func TestEriDbBatch(t *testing.T) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx)
	err := CreateEriDbBuckets(tx)
	assert.NoError(t, err)

	// The key and value we're going to test
	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5), big.NewInt(6),
		big.NewInt(7), big.NewInt(8), big.NewInt(9), big.NewInt(10), big.NewInt(11), big.NewInt(12)}

	quit := make(chan struct{})

	// Start a new batch
	db.OpenBatch(quit)

	// Inserting a key-value pair within a batch
	err = db.Insert(key, value)
	assert.NoError(t, err)

	// Commit the batch
	err = db.CommitBatch()
	assert.NoError(t, err)

	// Testing Get method after committing the batch
	retrievedValue, err := db.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)

	// Start another batch
	db.OpenBatch(quit)

	// Inserting a different key-value pair within a batch
	altKey := utils.NodeKey{5, 6, 7, 8}
	altValue := utils.NodeValue12{big.NewInt(13), big.NewInt(14), big.NewInt(15), big.NewInt(16), big.NewInt(17), big.NewInt(18),
		big.NewInt(19), big.NewInt(20), big.NewInt(21), big.NewInt(22), big.NewInt(23), big.NewInt(24)}

	err = db.Insert(altKey, altValue)
	assert.NoError(t, err)

	// Testing Get method before rollback or commit, expecting no value for the altKey
	altValRes, err := db.Get(altKey)
	assert.NoError(t, err)
	assert.Equal(t, altValue, altValRes)

	// Rollback the batch
	db.RollbackBatch()

	// Testing Get method after rollback, expecting no value for the altKey
	val, err := db.Get(altKey)
	assert.NoError(t, err)
	assert.Equal(t, utils.NodeValue12{}, val)
}

func BenchmarkEriDb_Get(b *testing.B) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), b.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx)
	err := CreateEriDbBuckets(tx)
	assert.NoError(b, err)

	numKeys := 100000

	keys := make([]utils.NodeKey, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = utils.NodeKey{uint64(i), 2, 3, 4}
		value := utils.NodeValue12{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(int64(i)), big.NewInt(6),
			big.NewInt(7), big.NewInt(8), big.NewInt(1), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

		if err := db.Insert(keys[i], value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		key := utils.NodeKey{uint64(i % numKeys), 2, 3, 4}
		val, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}

		// Verify to ensure compiler doesn't optimize away
		if val[4].Uint64() != big.NewInt(int64(i%numKeys)).Uint64() {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkEriDb_RawGet(b *testing.B) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), b.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx)
	err := CreateEriDbBuckets(tx)
	assert.NoError(b, err)

	numKeys := 100000

	keys := make([]utils.NodeKey, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = utils.NodeKey{uint64(i), 2, 3, 4}
		value := utils.NodeValue12Raw{
			Value: utils.NodeValue8Raw{
				1, 2, 3, 4, uint64(i), 6, 7, 8,
			},
			Flag: byte(i % 2),
		}

		if err := db.InsertRaw(keys[i], value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		key := utils.NodeKey{uint64(i % numKeys), 2, 3, 4}
		val, err := db.GetRaw(key)
		if err != nil {
			b.Fatal(err)
		}

		if val.Value[4] != uint64(i%numKeys) {
			b.Fatal("unexpected value")
		}
	}
}
