package realdb

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/rdb"
	"github.com/linxGnu/grocksdb"
)

type RealRtx struct {
	tx *grocksdb.Transaction
}

func newRealRtx(tx *grocksdb.Transaction) *RealRtx {
	return &RealRtx{tx: tx}
}

func (rtx *RealRtx) Get(opts *grocksdb.ReadOptions, key []byte) ([]byte, error) {
	s, err := rtx.tx.Get(opts, key)
	if err != nil {
		return nil, err
	}
	if !s.Exists() {
		return nil, rdb.ErrKeyNotExist
	}

	return moveSliceToBytes(s), nil
}

func (rtx *RealRtx) Put(key, value []byte) error {
	return rtx.tx.Put(key, value)
}

func (rtx *RealRtx) Delete(key []byte) error {
	return rtx.tx.Delete(key)
}

func (rtx *RealRtx) Commit() error {
	return rtx.tx.Commit()
}

func (rtx *RealRtx) Rollback() error {
	return rtx.tx.Rollback()
}

func (rtx *RealRtx) NewIterator(beginPrefix, endPrefix []byte) rdb.RDBIterator {
	ropts := grocksdb.NewDefaultReadOptions()
	ropts.SetIterateLowerBound(beginPrefix)
	ropts.SetIterateUpperBound(endPrefix)

	return newRealIterator(ropts, rtx.tx.NewIterator(ropts))
}

func (rtx *RealRtx) Destroy() {
	rtx.tx.Destroy()
}

func moveSliceToBytes(s *grocksdb.Slice) []byte {
	defer s.Free()
	if !s.Exists() {
		return nil
	}
	if len(s.Data()) == 0 {
		return nil
	}

	v := make([]byte, len(s.Data()))
	copy(v, s.Data())
	return v
}
