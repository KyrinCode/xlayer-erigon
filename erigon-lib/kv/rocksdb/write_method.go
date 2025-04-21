package rocksdb

import (
	"fmt"
	"github.com/linxGnu/grocksdb"
)

type WriteMethod int

const (
	WriteMethodPut WriteMethod = iota
	WriteMethodDiscardHistory
)

func (wm WriteMethod) config(opts *grocksdb.Options) {
	switch wm {
	case WriteMethodPut: // do nothing
	case WriteMethodDiscardHistory:
		opts.SetMergeOperator(&discardHistory{})
	default:
		panic(fmt.Sprintf("invalid WriteMethod: %v", wm))
	}
}

func (wm WriteMethod) txWrite(tx *grocksdb.Transaction, k, v []byte) error {
	switch wm {
	case WriteMethodPut:
		return tx.Put(k, v)
	case WriteMethodDiscardHistory:
		return tx.Merge(k, v)
	default:
		panic(fmt.Sprintf("invalid WriteMethod: %v", wm))
	}
}

type discardHistory struct{}

func (m *discardHistory) FullMerge(key, existingValue []byte, operands [][]byte) ([]byte, bool) {
	if len(operands) == 0 {
		return existingValue, true
	}
	return operands[len(operands)-1], true
}

// The name of the MergeOperator.
func (m *discardHistory) Name() string {
	return "XlayerMerge_DiscardHistory"
}
