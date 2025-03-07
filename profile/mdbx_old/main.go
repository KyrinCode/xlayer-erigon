package main

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	db2 "github.com/ledgerwatch/erigon/smt/pkg/db"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
	"math/big"
	"os"
	"runtime/pprof"
	"time"
)

func main() {

	// Create CPU profile file
	f, err := os.Create("cpu.old.prof")
	if err != nil {
		fmt.Println("Could not create CPU profile:", err)
		return
	}
	defer f.Close()

	dbDir := "/Users/yangweitao/data/xlayer/test_mdbx_old"
	fmt.Println("dbDir", dbDir)

	logger := log.New() // Creates a default logger
	// Open a permanent database
	opts := mdbx.NewMDBX(logger).Path(dbDir)
	isMem := opts.GetInMem()
	fmt.Println("isMem", isMem)

	dbi, err := opts.Open(context.Background())

	tx, _ := dbi.BeginRw(context.Background())
	db := db2.NewEriDb(tx)
	err = db2.CreateEriDbBuckets(tx)
	if err != nil {
		panic(err)
	}

	numKeys := 1000000

	keys := make([]utils.NodeKey, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = utils.NodeKey{uint64(i), 2, 3, 4}
		value := utils.NodeValue12{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(int64(i)), big.NewInt(6),
			big.NewInt(7), big.NewInt(8), big.NewInt(1), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

		if err := db.Insert(keys[i], value); err != nil {
			panic(err)
		}
	}

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Println("Could not start CPU profiling:", err)
		return
	}
	defer pprof.StopCPUProfile() // Stop profiling at the end

	for i := 0; i < numKeys; i++ {
		key := utils.NodeKey{uint64(i % numKeys), 2, 3, 4}
		val, err := db.Get(key)
		if err != nil {
			panic(err)
		}

		// Verify to ensure compiler doesn't optimize away
		if val[4].Uint64() != big.NewInt(int64(i%numKeys)).Uint64() {
			panic("unexpected value")
		}
	}

	time.Sleep(2 * time.Second)
}
