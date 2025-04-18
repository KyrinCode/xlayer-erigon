package dbbuilder

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
)

type DatabseType int

const (
	DatabseTypeMdbx DatabseType = iota // default value
	DatabaseTypeRocksDB
)

func ToDatabaseType(s string) DatabseType {
	switch s {
	case "mdbx":
		return DatabseTypeMdbx
	case "rocksdb":
		return DatabaseTypeRocksDB
	default:
		panic(fmt.Sprintf("unknown db type: %s", s))
	}
}

func NewDB(dbType DatabseType, ctx context.Context, opts mdbx.MdbxOpts, tableCfg kv.TableCfg) (kv.RwDB, error) {
	switch dbType {
	case DatabseTypeMdbx:
		return opts.Open(ctx)
	case DatabaseTypeRocksDB:
		return rocksdb.NewRocksDB(opts.GetPath(), opts.GetLogger(), tableCfg, opts.GetLabel(), opts.GetRoTxsLimiter(), opts.IsReadonly(), rocksdb.WriteMethodPut)
	default:
		panic(fmt.Sprintf("unknown db type: %v", dbType))
	}
}
