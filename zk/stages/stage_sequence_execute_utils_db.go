package stages

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/eth/stagedsync"
	db2 "github.com/ledgerwatch/erigon/smt/pkg/db"
	smtNs "github.com/ledgerwatch/erigon/smt/pkg/smt"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"github.com/ledgerwatch/log/v3"
)

type stageDb struct {
	ctx   context.Context
	db    kv.RwDB
	dbsmt kv.RwDB

	tx          kv.RwTx
	txsmt       kv.RwTx
	hermezDb    *hermez_db.HermezDb
	eridb       smtNs.DB
	stateReader *state.PlainStateReader
	smt         *smtNs.SMT

	supportAC bool
}

func newStageDb(ctx context.Context, db, dbsmt kv.RwDB, supportAC bool) (sdb *stageDb, err error) {
	var tx kv.RwTx
	if tx, err = db.BeginRw(ctx); err != nil {
		log.Error("failed to start maindb tx", "err", err)
		return nil, err
	}

	sdb = &stageDb{
		supportAC: supportAC,
		ctx:       ctx,
		db:        db,
		dbsmt:     dbsmt,
	}

	if supportAC {
		// Support Async IO, only need to create read only transaction
		var txsmt kv.RwTx = nil
		if dbsmt != nil {
			if txsmt, err = dbsmt.BeginRw(ctx); err != nil {
				log.Error("failed to start smt tx", "err", err)
				return nil, err
			}
		}

		eridb := db2.NewEriCacheDb(sdb.ctx, txsmt, tx)
		sdb.SetTx(tx, txsmt, eridb)
	} else {
		// Support Sync IO，so need to create read write transaction
		var txsmt kv.RwTx = nil
		if dbsmt != nil {
			if txsmt, err = dbsmt.BeginRw(ctx); err != nil {
				log.Error("failed to start smt tx", "err", err)
				return nil, err
			}
		}

		eridb := db2.NewEriDb(txsmt, tx)
		sdb.SetTx(tx, txsmt, eridb)
	}

	return sdb, nil
}

func (sdb *stageDb) SetTx(tx kv.RwTx, txsmt kv.RwTx, eridb smtNs.DB) {
	sdb.tx = tx
	sdb.hermezDb = hermez_db.NewHermezDb(tx)
	if txsmt == nil {
		sdb.eridb = db2.NewEriDb(tx, tx)
	} else {
		sdb.eridb = db2.NewEriDb(txsmt, tx)
	}
	sdb.stateReader = state.NewPlainStateReader(tx)

	sdb.txsmt = txsmt
	sdb.eridb = eridb
	sdb.smt = smtNs.NewSMT(sdb.eridb, false)
}

func (sdb *stageDb) CommitAndStart() (err error) {
	if err = sdb.tx.Commit(); err != nil {
		if !sdb.supportAC && sdb.dbsmt != nil {
			sdb.txsmt.Rollback()
		}
		return err
	}

	tx, err := sdb.db.BeginRw(sdb.ctx)
	if err != nil {
		return err
	}

	if !sdb.supportAC && sdb.dbsmt != nil {
		if err = sdb.txsmt.Commit(); err != nil {
			return err
		}

		txsmt, err := sdb.dbsmt.BeginRw(sdb.ctx)
		if err != nil {
			return err
		}
		eridb := db2.NewEriDb(txsmt, tx)

		sdb.SetTx(tx, txsmt, eridb)
	} else {
		if sdb.dbsmt != nil {
			sdb.SetTx(tx, sdb.txsmt, sdb.eridb)
		} else {
			sdb.SetTx(tx, tx, sdb.eridb)
		}
	}

	return nil
}

func (sdb *stageDb) Commit(s *stagedsync.StageState, flushSmt bool) error {
	if !sdb.supportAC && flushSmt {
		smtCache, deltaCache := sdb.eridb.RetriveAndCleanCache()
		s.SetSmtCache(smtCache, deltaCache)
	}

	err := sdb.tx.Commit()
	if err != nil {
		if !sdb.supportAC && sdb.dbsmt != nil {
			sdb.txsmt.Rollback()
		}
		return err
	}

	if !sdb.supportAC && sdb.dbsmt != nil {
		return sdb.txsmt.Commit()
	} else {
		return nil
	}
}

func (sdb *stageDb) Rollback() {
	sdb.tx.Rollback()
	if sdb.txsmt != nil {
		sdb.txsmt.Rollback()
	}
}
