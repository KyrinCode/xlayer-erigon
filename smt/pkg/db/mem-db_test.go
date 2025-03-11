package db

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

func TestMemDb(t *testing.T) {
	db := NewMemDb()

	for i := 0; i < 10; i++ {
		key := utils.NodeKey{randomUint64(), randomUint64(), randomUint64(), randomUint64()}
		nodeValue := utils.NodeValue8Raw{randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64(), randomUint64()}
		val := utils.NodeValue12Raw{
			Value: nodeValue,
			Flag:  byte(i % 2),
		}
		err := db.Insert(key, val)
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

}

func BenchmarkMemDb_Insert(b *testing.B) {
	db := NewMemDb()

	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Modify key slightly for each iteration to avoid measuring cache effects
		key[0] = uint64(i)
		value.Value[4] = uint64(i)

		err := db.Insert(key, value)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemDb_Get(b *testing.B) {
	// Setup
	db := NewMemDb()
	numKeys := 1000

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
