package combinedb

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
	"path"
	"sync/atomic"
	"unsafe"
)

type CombineDB struct {
	mdbx    kv.RwDB
	rocksdb kv.RwDB

	logger *combineLogger
}

var dbCounter atomic.Uint64

func NewCombinDB(ctx context.Context, opts mdbx.MdbxOpts, tableCfg kv.TableCfg) (db kv.RwDB, err error) {
	dbDir := opts.GetPath()

	opts.Path(path.Join(dbDir, "mdbx"))
	mdbx, err := opts.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			mdbx.Close()
		}
	}()

	rocksdb, err := rocksdb.NewRocksDB(path.Join(dbDir, "rocksdb"), opts.GetLogger(), tableCfg, opts.GetLabel(), opts.GetRoTxsLimiter(), opts.IsReadonly(), rocksdb.WriteMethodPut)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			rocksdb.Close()
		}
	}()

	return &CombineDB{
		mdbx:    mdbx,
		rocksdb: rocksdb,

		logger: newCombinLogger(fmt.Sprintf("dbid=%d", dbCounter.Add(1))),
	}, nil
}

func (db *CombineDB) Close() {
	db.logger.Debugf("Close")
	defer db.logger.Debugf("Close done")
	db.mdbx.Close()
	db.mdbx = nil

	db.rocksdb.Close()
	db.rocksdb = nil
}

func (db *CombineDB) ReadOnly() bool {
	db.logger.Debugf("ReadOnly")
	defer db.logger.Debugf("ReadOnly done")

	b1 := db.mdbx.ReadOnly()
	b2 := db.rocksdb.ReadOnly()

	assertEq(db.logger, b1, b2, "ReadOnly mismatch: mdbx: %v. rocksdb: %v", b1, b2)
	return b1
}

func (db *CombineDB) View(ctx context.Context, f func(tx kv.Tx) error) error {
	db.logger.Debugf("View")
	defer db.logger.Debugf("View done")

	return db.mdbx.View(ctx, func(mdbxTx kv.Tx) error {
		return db.rocksdb.View(ctx, func(rocksdbTx kv.Tx) error {
			ctx := newCombineTx(db.logger.getPrefix(), mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) BeginRo(ctx context.Context) (kv.Tx, error) {
	db.logger.Debugf("BeginRo")
	defer db.logger.Debugf("BeginRo done")

	mdbxTx, err1 := db.mdbx.BeginRo(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRo(ctx)
	assertError(db.logger, err1, err2, "BeginRo")

	return newCombineTx(db.logger.getPrefix(), mdbxTx, rocksdbTx), nil
}

func (db *CombineDB) AllTables() kv.TableCfg {
	db.logger.Debugf("AllTables")
	defer db.logger.Debugf("AllTables done")

	mdbxTables := db.mdbx.AllTables()
	rocksdbTables := db.rocksdb.AllTables()

	assertEq(db.logger, mdbxTables, rocksdbTables, "AllTables mismatch: mdbx table: %v\nrocksdb table: %v", mdbxTables, rocksdbTables)
	return mdbxTables
}

func (db *CombineDB) PageSize() uint64 {
	db.logger.Debugf("PageSize")
	defer db.logger.Debugf("PageSize done")

	mdbxPageSize := db.mdbx.PageSize()
	rocksdbPageSize := db.rocksdb.PageSize()

	assertEq(db.logger, mdbxPageSize, mdbxPageSize, "PageSize mismatch: mdbx.PageSize=%v\nrocksdb.PageSize=%v", mdbxPageSize, rocksdbPageSize)
	return mdbxPageSize
}

// Pointer to the underlying C environment handle, if applicable (e.g. *C.MDBX_env)
func (db *CombineDB) CHandle() unsafe.Pointer {
	db.logger.Debugf("CHandle")
	defer db.logger.Debugf("CHandle done")

	mdbxH := db.mdbx.CHandle()
	rocksdbH := db.rocksdb.CHandle()
	assertEq(db.logger, mdbxH, rocksdbH, "CHandle mismatch: mdbx: %v\nrocksdb: %v", mdbxH, rocksdbH)
	return mdbxH
}

func (db *CombineDB) Update(ctx context.Context, f func(tx kv.RwTx) error) error {
	db.logger.Debugf("Update")
	defer db.logger.Debugf("Update done")

	return db.mdbx.Update(ctx, func(mdbxTx kv.RwTx) error {
		return db.rocksdb.Update(ctx, func(rocksdbTx kv.RwTx) error {
			ctx := newCombineRwTx(db.logger.getPrefix(), mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) UpdateNosync(ctx context.Context, f func(tx kv.RwTx) error) error {
	db.logger.Debugf("UpdateNosync")
	defer db.logger.Debugf("UpdateNosync done")

	return db.mdbx.UpdateNosync(ctx, func(mdbxTx kv.RwTx) error {
		return db.rocksdb.Update(ctx, func(rocksdbTx kv.RwTx) error {
			ctx := newCombineRwTx(db.logger.getPrefix(), mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) BeginRw(ctx context.Context) (kv.RwTx, error) {
	db.logger.Debugf("BeginRw")
	defer db.logger.Debugf("BeginRw done")

	mdbxTx, err1 := db.mdbx.BeginRw(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRw(ctx)
	assertError(db.logger, err1, err2, "BeginRw")

	return newCombineRwTx(db.logger.getPrefix(), mdbxTx, rocksdbTx), nil
}

func (db *CombineDB) BeginRwNosync(ctx context.Context) (kv.RwTx, error) {
	db.logger.Debugf("BeginRwNosync")
	defer db.logger.Debugf("BeginRwNosync done")

	mdbxTx, err1 := db.mdbx.BeginRwNosync(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRwNosync(ctx)
	assertError(db.logger, err1, err2, "BeginRwNosync")

	return newCombineRwTx(db.logger.getPrefix(), mdbxTx, rocksdbTx), nil
}
