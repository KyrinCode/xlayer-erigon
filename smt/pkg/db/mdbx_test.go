package db

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func randomUint64() uint64 {
	var num uint64
	binary.Read(rand.Reader, binary.LittleEndian, &num)
	return num
}

func TestEriDb(t *testing.T) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx, nil)
	err := CreateEriDbBuckets(tx)
	assert.NoError(t, err)

	// Rest of your test code remains the same
	for i := 0; i < 10; i++ {
		key := utils.NodeKey{randomUint64(), randomUint64(), randomUint64(), randomUint64()}
		nodeValue := utils.NodeValue8Raw{randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64()}
		val := utils.NodeValue12Raw{
			Value: nodeValue,
			Flag:  byte(i % 2),
		}
		err = db.Insert(key, val)
		assert.NoError(t, err)
		retrievedValue, err := db.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, val, retrievedValue)

		err = db.InsertAccountValue(key, nodeValue)
		assert.NoError(t, err)
		retrievedAcctVal, err := db.GetAccountValue(key)
		assert.NoError(t, err)
		assert.Equal(t, nodeValue, retrievedAcctVal)
	}

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)
}

func TestEriDbBatch(t *testing.T) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	tx, _ := dbi.BeginRw(context.Background())
	db := NewEriDb(tx, nil)
	err := CreateEriDbBuckets(tx)
	assert.NoError(t, err)

	// The key and value we're going to test
	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}

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
	altValue := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}
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
	assert.Equal(t, utils.NodeValue12Raw{}, val)
}

func BenchmarkEriDb_Get(b *testing.B) {
	dbi, _ := mdbx.NewTemporaryMdbx(context.Background(), b.TempDir())
	tx, _ := dbi.BeginRw(context.Background())

	dbiChain, _ := mdbx.NewTemporaryMdbx(context.Background(), b.TempDir())
	txChain, _ := dbiChain.BeginRw(context.Background())

	db := NewEriDb(tx, txChain)
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
			Flag: byte(1),
		}

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
		if val.Value[4] != big.NewInt(int64(i%numKeys)).Uint64() {
			b.Fatal("unexpected value")
		}
	}
}

func setupTestDB(t *testing.T) (*EriDb, *EriRoDb) {
	dbi, err := mdbx.NewTemporaryMdbx(context.Background(), t.TempDir())
	assert.NoError(t, err)
	tx, err := dbi.BeginRw(context.Background())
	assert.NoError(t, err)
	err = CreateEriDbBuckets(tx)
	assert.NoError(t, err)
	err = tx.Commit()
	assert.NoError(t, err)
	tx, err = dbi.BeginRw(context.Background())
	assert.NoError(t, err)
	db := NewEriDb(tx, nil)
	return db, NewRoEriDb(tx, nil)
}

func TestEriRoDb_GetLastRoot(t *testing.T) {
	db, dbro := setupTestDB(t)

	// Test when data is not present
	root, err := dbro.GetLastRoot()
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(0), root)

	// Test when data is present
	expectedRoot := big.NewInt(12345)
	err = db.SetLastRoot(expectedRoot)
	assert.NoError(t, err)

	root, err = dbro.GetLastRoot()
	assert.NoError(t, err)
	assert.Equal(t, expectedRoot, root)
}

func TestEriRoDb_GetDepth(t *testing.T) {
	db, dbro := setupTestDB(t)

	// Test when data is not present
	depth, err := dbro.GetDepth()
	assert.NoError(t, err)
	assert.Equal(t, uint8(0), depth)

	// Test when data is present
	expectedDepth := uint8(5)
	err = db.SetDepth(expectedDepth)
	assert.NoError(t, err)

	depth, err = dbro.GetDepth()
	assert.NoError(t, err)
	assert.Equal(t, expectedDepth, depth)
}

func TestEriRoDb_Get(t *testing.T) {
	db, dbro := setupTestDB(t)

	key := utils.NodeKey{1, 2, 3, 4}
	expectedValue := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}

	// Test when data is not present
	value, err := dbro.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, utils.NodeValue12Raw{}, value)

	keyBytes := utils.NodeKeyToByteArray(&key)
	valBytes := utils.NodeValue12RawToByteArray(&expectedValue)

	err = db.tx.Put(TableSmt, keyBytes, valBytes)
	assert.NoError(t, err)

	value, err = dbro.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
}

func TestEriRoDb_GetAccountValue(t *testing.T) {
	db, dbro := setupTestDB(t)

	key := utils.NodeKey{1, 2, 3, 4}
	expectedValue := utils.NodeValue8Raw{1, 2, 3, 4, 5, 6, 7, 8}

	// Test when data is not present
	value, err := dbro.GetAccountValue(key)
	assert.NoError(t, err)
	assert.Equal(t, utils.NodeValue8Raw{}, value)

	// Test when data is present
	err = db.InsertAccountValue(key, expectedValue)
	assert.NoError(t, err)
	value, err = dbro.GetAccountValue(key)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
}

func TestEriRoDb_GetKeySource(t *testing.T) {
	db, dbro := setupTestDB(t)

	key := utils.NodeKey{1, 2, 3, 4}
	expectedValue := []byte("source_value")

	// Test when data is not present
	value, err := dbro.GetKeySource(key)
	assert.Error(t, err)
	assert.Nil(t, value)

	// Test when data is present
	err = db.InsertKeySource(key, expectedValue)
	assert.NoError(t, err)
	value, err = dbro.GetKeySource(key)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
}

func TestEriRoDb_GetHashKey(t *testing.T) {
	db, dbro := setupTestDB(t)

	key := utils.NodeKey{1, 2, 3, 4}
	expectedValue := utils.NodeKey{5, 6, 7, 8}

	// Test when data is not present
	value, err := dbro.GetHashKey(key)
	assert.Error(t, err)
	assert.Equal(t, utils.NodeKey{}, value)

	// Test when data is present
	err = db.InsertHashKey(key, expectedValue)
	assert.NoError(t, err)
	value, err = dbro.GetHashKey(key)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
}

/*
func codeToCodeHash(code []byte) ([]byte, error) {
	codeHash := utils.HashContractBytecode(hex.EncodeToString(code))
	codeHashBytes, err := hex.DecodeString(strings.TrimPrefix(codeHash, "0x"))
	if err != nil {
		return nil, err
	}
	return utils.ResizeHashTo32BytesByPrefixingWithZeroes(codeHashBytes), nil
}

// ! This test is commented out because the Code table is not in Smt database
func TestEriRoDb_GetCode(t *testing.T) {
	db, dbro := setupTestDB(t)

	expectedValue := []byte("code_value")
	codeHash, err := codeToCodeHash(expectedValue)
	assert.NoError(t, err)

	// Test when data is not present
	value, err := dbro.GetCode([]byte(codeHash))
	assert.Error(t, err)
	assert.Nil(t, value)

	// Test when data is present
	err = db.AddCode(expectedValue)
	assert.NoError(t, err)
	value, err = dbro.GetCode([]byte(codeHash))
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
}
*/
