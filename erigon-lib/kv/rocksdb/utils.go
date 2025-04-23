package rocksdb

import (
	"errors"
	"fmt"
	"math"
)

var ErrKeyExist = errors.New("key exists")
var ErrNotFound = errors.New("No matching key/data pair found")
var ErrKeyMismatch = errors.New("given key value is mismatched to the current cursor position")
var ErrValueLeLatest = errors.New("the given value is little or equal to the latest value")
var ErrInvalidIter = errors.New("current iterator is invalid")

func mergeKey(table string, k []byte) []byte {
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

func splitKey(k []byte) (string, []byte) {
	if len(k) < 1 {
		panic("invalid key with length < 1")
	}

	l := k[0]

	table := string(k[1 : l+1])
	k = k[l+1:]
	return table, k
}
