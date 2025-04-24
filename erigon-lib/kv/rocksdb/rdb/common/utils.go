package common

import (
	"fmt"
	"github.com/linxGnu/grocksdb"
	"math"
)

func MoveSliceToBytes(s *grocksdb.Slice) []byte {
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

func MergeKey(table string, k []byte) []byte {
	l := len(table)
	if l > math.MaxUint8 {
		panic(fmt.Sprintf("too large table len: [%d]%s", l, table))
	}
	lb := byte(l)

	buf := make([]byte, 0, 1+l+len(k))
	buf = append(buf, lb)
	buf = append(buf, []byte(table)...)
	buf = append(buf, k...)

	return buf
}

func SplitKey(k []byte) (string, []byte) {
	if len(k) < 1 {
		panic("invalid key with length < 1")
	}

	l := k[0]

	table := string(k[1 : l+1])
	k = k[l+1:]
	return table, k
}
