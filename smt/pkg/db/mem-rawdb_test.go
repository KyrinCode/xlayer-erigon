package db

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

func TestRawMemDb(t *testing.T) {
	db := NewRawMemDb()

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

}

func TestRawMemDbRawMethod(t *testing.T) {
	db := NewMemDb()

	// The key and value we're going to test
	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}

	// Testing Insert method
	err := db.Insert(key, value)
	assert.NoError(t, err)

	// Testing Get method
	retrievedValue, err := db.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)

}

func BenchmarkRawMemDb_InsertRaw(b *testing.B) {
	db := NewRawMemDb()

	key := utils.NodeKey{1, 2, 3, 4}
	value := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}
	b.ResetTimer()

	// Run b.N iterations
	for i := 0; i < b.N; i++ {
		// Modify key slightly for each iteration to avoid measuring cache effects
		key[0] = uint64(i)
		value.Value[4] = uint64(i)

		err := db.InsertRaw(key, value)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRawMemDb_InsertRaw_Parallel(b *testing.B) {
	// Setup
	db := NewRawMemDb()

	value := utils.NodeValue12Raw{
		Value: utils.NodeValue8Raw{
			1, 2, 3, 4, 5, 6, 7, 8,
		},
		Flag: byte(1),
	}
	// Reset timer before the parallel operations
	b.ResetTimer()

	// Run parallel benchmark
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own key to avoid conflicts
		key := utils.NodeKey{1, 2, 3, 4}

		counter := uint64(0)
		for pb.Next() {
			// Modify key to avoid conflicts between iterations
			key[0] = counter
			value.Value[4] = uint64(counter)
			counter++

			err := db.InsertRaw(key, value)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRawMemDb_GetRaw(b *testing.B) {
	// Setup
	db := NewRawMemDb()
	numKeys := 1000

	keys := make([]utils.NodeKey, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = utils.NodeKey{uint64(i), 2, 3, 4}
		value := utils.NodeValue12Raw{
			Value: utils.NodeValue8Raw{
				1, 2, 3, 4, 5, 6, 7, 8,
			},
			Flag: byte(1),
		}
		value.Value[4] = uint64(i)
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
		// Verify to ensure compiler doesn't optimize away
		if val.Value[4] != uint64(i%numKeys) {
			b.Fatal("unexpected value")
		}
	}
}
