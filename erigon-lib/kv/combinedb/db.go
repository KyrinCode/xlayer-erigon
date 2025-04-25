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

func NewCombinDB(ctx context.Context, opts mdbx.MdbxOpts, tableCfg kv.TableCfg, enableLog bool) (db kv.RwDB, err error) {
	dbDir := opts.GetPath()

	opts = opts.Path(path.Join(dbDir, "mdbx"))
	opts.GetLogger().Info("Set mdbx path", "new path", opts.GetPath())
	mdbx, err := opts.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			mdbx.Close()
		}
	}()

	rocksdbDir := path.Join(dbDir, "rocksdb")
	opts.GetLogger().Info("Set rocksdb path", "new path", rocksdbDir)
	rocksdb, err := rocksdb.NewRocksDB(rocksdbDir, opts.GetLogger(), tableCfg, opts.GetLabel(), opts.GetRoTxsLimiter(), opts.IsReadonly(), rocksdb.RealRDB)
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

		logger: newCombinLogger(enableLog, fmt.Sprintf("combinedb dbid=%d", dbCounter.Add(1))),
	}, nil
}

func (db *CombineDB) Close() {
	db.logger.Info("Close")
	defer db.logger.Info("Close done")
	db.mdbx.Close()
	db.mdbx = nil

	db.rocksdb.Close()
	db.rocksdb = nil
}

func (db *CombineDB) ReadOnly() bool {
	db.logger.Info("ReadOnly")
	defer db.logger.Info("ReadOnly done")

	b1 := db.mdbx.ReadOnly()
	b2 := db.rocksdb.ReadOnly()

	assertEqualF(db.logger, b1, b2, "ReadOnly mismatch: mdbx: %v. rocksdb: %v", b1, b2)
	return b1
}

func (db *CombineDB) View(ctx context.Context, f func(tx kv.Tx) error) error {
	db.logger.Info("View")
	defer db.logger.Info("View done")

	return db.mdbx.View(ctx, func(mdbxTx kv.Tx) error {
		return db.rocksdb.View(ctx, func(rocksdbTx kv.Tx) error {
			ctx := newCombineTx(db.logger, mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) BeginRo(ctx context.Context) (kv.Tx, error) {
	db.logger.Info("BeginRo")
	defer db.logger.Info("BeginRo done")

	mdbxTx, err1 := db.mdbx.BeginRo(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRo(ctx)
	assertError(db.logger, err1, err2, "BeginRo")

	return newCombineTx(db.logger, mdbxTx, rocksdbTx), nil
}

func (db *CombineDB) AllTables() kv.TableCfg {
	db.logger.Info("AllTables")
	defer db.logger.Info("AllTables done")

	mdbxTables := db.mdbx.AllTables()
	rocksdbTables := db.rocksdb.AllTables()

	assertEqualF(db.logger, mdbxTables, rocksdbTables, "AllTables mismatch: mdbx table: %v\nrocksdb table: %v", mdbxTables, rocksdbTables)
	return mdbxTables
}

func (db *CombineDB) PageSize() uint64 {
	db.logger.Info("PageSize")
	defer db.logger.Info("PageSize done")

	mdbxPageSize := db.mdbx.PageSize()
	rocksdbPageSize := db.rocksdb.PageSize()

	assertEqualF(db.logger, mdbxPageSize, mdbxPageSize, "PageSize mismatch: mdbx.PageSize=%v\nrocksdb.PageSize=%v", mdbxPageSize, rocksdbPageSize)
	return mdbxPageSize
}

// Pointer to the underlying C environment handle, if applicable (e.g. *C.MDBX_env)
func (db *CombineDB) CHandle() unsafe.Pointer {
	db.logger.Info("CHandle")
	defer db.logger.Info("CHandle done")

	mdbxH := db.mdbx.CHandle()
	rocksdbH := db.rocksdb.CHandle()
	assertEqualF(db.logger, mdbxH, rocksdbH, "CHandle mismatch: mdbx: %v\nrocksdb: %v", mdbxH, rocksdbH)
	return mdbxH
}

func (db *CombineDB) Update(ctx context.Context, f func(tx kv.RwTx) error) error {
	db.logger.Info("Update")
	defer db.logger.Info("Update done")

	return db.mdbx.Update(ctx, func(mdbxTx kv.RwTx) error {
		return db.rocksdb.Update(ctx, func(rocksdbTx kv.RwTx) error {
			ctx := newCombineRwTx(db.logger, mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) UpdateNosync(ctx context.Context, f func(tx kv.RwTx) error) error {
	db.logger.Info("UpdateNosync")
	defer db.logger.Info("UpdateNosync done")

	return db.mdbx.UpdateNosync(ctx, func(mdbxTx kv.RwTx) error {
		return db.rocksdb.Update(ctx, func(rocksdbTx kv.RwTx) error {
			ctx := newCombineRwTx(db.logger, mdbxTx, rocksdbTx)
			return f(ctx)
		})
	})
}

func (db *CombineDB) BeginRw(ctx context.Context) (kv.RwTx, error) {
	db.logger.Info("BeginRw")
	defer db.logger.Info("BeginRw done")

	mdbxTx, err1 := db.mdbx.BeginRw(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRw(ctx)
	assertError(db.logger, err1, err2, "BeginRw")

	return newCombineRwTx(db.logger, mdbxTx, rocksdbTx), nil
}

func (db *CombineDB) BeginRwNosync(ctx context.Context) (kv.RwTx, error) {
	db.logger.Info("BeginRwNosync")
	defer db.logger.Info("BeginRwNosync done")

	mdbxTx, err1 := db.mdbx.BeginRwNosync(ctx)
	rocksdbTx, err2 := db.rocksdb.BeginRwNosync(ctx)
	assertError(db.logger, err1, err2, "BeginRwNosync")

	return newCombineRwTx(db.logger, mdbxTx, rocksdbTx), nil
}
