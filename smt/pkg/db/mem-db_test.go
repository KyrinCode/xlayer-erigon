package db

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

func TestMemDb(t *testing.T) {
	db := NewMemDb()

	// The key and value we're going to test
	key := utils.NodeKey{0, 2, 3, 4}
	value := utils.NodeValue12{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(0), big.NewInt(6),
		big.NewInt(7), big.NewInt(8), big.NewInt(1), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	// Testing Insert method
	err := db.Insert(key, value)
	assert.NoError(t, err)

	// Testing Get method
	retrievedValue, err := db.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)

	//retrievedRawValue, err := db.GetRaw(key)
	//fmt.Printf("retrieved raw value: %v\n", retrievedRawValue)
}

func BenchmarkMemDb_Insert(b *testing.B) {
	db := NewMemDb()

	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5), big.NewInt(6),
		big.NewInt(7), big.NewInt(8), big.NewInt(1), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Modify key slightly for each iteration to avoid measuring cache effects
		key[0] = uint64(i)
		value[4] = big.NewInt(int64(i))

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
