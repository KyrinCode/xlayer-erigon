package combinedb

import (
	"fmt"
	"sync/atomic"

	"github.com/ledgerwatch/erigon-lib/kv/iter"
)

type CombineDual struct {
	mdbxDual    iter.KV
	rocksdbDual iter.KV

	logger *combineLogger
}

var dualCounter atomic.Uint64

func newCombineDual(prefix string, mdbxDual iter.KV, rocksdbDual iter.KV) iter.KV {
	return &CombineDual{
		mdbxDual:    mdbxDual,
		rocksdbDual: rocksdbDual,
		logger:      newCombinLogger(fmt.Sprintf("%s dualid=%d", prefix, dualCounter.Add(1))),
	}
}

func (d *CombineDual) Next() ([]byte, []byte, error) {
	d.logger.Debugf("Next")
	defer d.logger.Debugf("Next done")

	k1, v1, err1 := d.mdbxDual.Next()
	k2, v2, err2 := d.rocksdbDual.Next()
	assertError(d.logger, err1, err2, "Next")
	assertEq(d.logger, k1, k2, "Next key mismatch. mdbx=%x. rocksdb=%x.", k1, k2)
	assertEq(d.logger, v1, v2, "Next value mismatch. mdbx=%x. rocksdb=%s", v1, v2)

	return k1, v1, nil
}

func (d *CombineDual) HasNext() bool {
	d.logger.Debugf("HasNext")
	defer d.logger.Debugf("HasNext done")

	b1 := d.mdbxDual.HasNext()
	b2 := d.rocksdbDual.HasNext()
	assertEq(d.logger, b1, b2, "HasNext mismatch. mdbx=%v. rocksdb=%v", b1, b2)

	return b1
}
