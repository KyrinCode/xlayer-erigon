package rocksdb

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/common/dbg"
	"runtime"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/log/v3"
	"github.com/linxGnu/grocksdb"
	"golang.org/x/sync/semaphore"
)

type RocksDB struct {
	rdb      *grocksdb.TransactionDB
	lruCache *grocksdb.Cache

	closeGuard *CloseGuard

	readOnly  bool // todo: not used
	tablesCfg kv.TableCfg
	label     kv.Label // marker to distinct db instances - one process may open many databases. for example to collect metrics of only 1 database

	readTxLimiter  *semaphore.Weighted
	writeTxLimiter *semaphore.Weighted
	logger         log.Logger
	leakDetector   *dbg.LeakDetector
}

func NewRocksDB(dbPath string, logger log.Logger, tablesCfg kv.TableCfg, label kv.Label, readTxLimiter, writeTxLimiter *semaphore.Weighted, readOnly bool) (kv.RwDB, error) {
	if readTxLimiter == nil {
		targetSemCount := int64(runtime.GOMAXPROCS(-1)) - 1
		readTxLimiter = semaphore.NewWeighted(targetSemCount) // 1 less than max to allow unlocking to happen
	}
	if writeTxLimiter == nil {
		targetSemCount := int64(runtime.GOMAXPROCS(-1)) - 1
		writeTxLimiter = semaphore.NewWeighted(targetSemCount) // 1 less than max to allow unlocking to happen
	}

	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	// defer bbto.Destroy()
	lruCache := grocksdb.NewLRUCache(3 << 30)
	bbto.SetBlockCache(lruCache)

	opts := grocksdb.NewDefaultOptions()
	// defer opts.Destroy()
	opts.SetBlockBasedTableFactory(bbto)
	opts.SetCreateIfMissing(true)
	// opts.SetDisableAutoCompactions(true)
	// 1. 写缓冲相关
	opts.SetWriteBufferSize(64 * 1024 * 1024) // 单个 memtable 64MB
	opts.SetMaxWriteBufferNumber(6)           // 最多 6 个缓冲 memtable
	opts.SetMinWriteBufferNumberToMerge(3)    // 合并 memtable 的阈值
	// 2. compaction stall 相关
	opts.SetLevel0FileNumCompactionTrigger(10) // L0 到 10 就触发 compaction
	opts.SetLevel0SlowdownWritesTrigger(20)
	opts.SetLevel0StopWritesTrigger(40)
	// opts.EnableStatistics()

	txopts := grocksdb.NewDefaultTransactionDBOptions()
	// defer txopts.Destroy()
	rdb, err := grocksdb.OpenTransactionDb(opts, txopts, dbPath)
	if err != nil {
		return nil, err
	}

	return &RocksDB{
		rdb:            rdb,
		lruCache:       lruCache,
		closeGuard:     newCloseGuard(),
		readOnly:       readOnly,
		tablesCfg:      tablesCfg,
		label:          label,
		readTxLimiter:  readTxLimiter,
		writeTxLimiter: writeTxLimiter,
		logger:         logger,
	}, nil
}

// impl Closer interface
func (db *RocksDB) Close() {
	firstClose := db.closeGuard.close()
	if firstClose {
		db.rdb.Close()
		db.rdb = nil

		db.lruCache.Destroy()
		db.lruCache = nil
	}
}

// impl Ro interface
func (db *RocksDB) ReadOnly() bool {
	return db.readOnly
}

func (db *RocksDB) View(ctx context.Context, f func(tx kv.Tx) error) error {
	tx, err := db.BeginRo(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	return f(tx)
}

func (db *RocksDB) BeginRo(ctx context.Context) (tx kv.Tx, err error) {
	return db.beginTx(ctx, db.readTxLimiter)
}

func (db *RocksDB) AllTables() kv.TableCfg {
	return db.tablesCfg
}

func (db *RocksDB) PageSize() uint64 {
	// note: no avaliable call to PageSize for now
	panic("not supported")
}

func (db *RocksDB) CHandle() unsafe.Pointer {
	// note: not support for RocksDB
	panic("not supported")
}

// impl RwDB interface
func (db *RocksDB) Update(ctx context.Context, f func(tx kv.RwTx) error) error {
	tx, err := db.BeginRw(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = f(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *RocksDB) UpdateNosync(ctx context.Context, f func(tx kv.RwTx) error) error {
	tx, err := db.BeginRwNosync(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = f(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *RocksDB) BeginRw(ctx context.Context) (tx kv.RwTx, err error) {
	return db.beginTx(ctx, db.writeTxLimiter)
}

func (db *RocksDB) BeginRwNosync(ctx context.Context) (kv.RwTx, error) {
	// note: yztodo: there isn't a real `no sync` in rocksdb
	return db.BeginRw(ctx)
}

func (db *RocksDB) beginTx(ctx context.Context, txLimiter *semaphore.Weighted) (tx kv.RwTx, err error) {
	// don't try to acquire if the context is already done
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// otherwise carry on
	}

	// will return nil err if context is cancelled (may appear to acquire the semaphore)
	if semErr := txLimiter.Acquire(ctx, 1); semErr != nil {
		return nil, fmt.Errorf("rocksdb.BeginTx: tx limiter error %w", semErr)
	}
	defer func() {
		if tx == nil {
			txLimiter.Release(1)
		}
	}()

	if !db.closeGuard.reference() {
		return nil, fmt.Errorf("db closed")
	}
	defer func() {
		if tx == nil {
			db.closeGuard.deReference()
		}
	}()

	id := db.leakDetector.Add()
	// todo: yztodo: not a real read only tx yet
	return newRocksDbTx(db, ctx, func() {
		db.closeGuard.deReference()
		txLimiter.Release(1)
		db.leakDetector.Del(id)
	})
}
