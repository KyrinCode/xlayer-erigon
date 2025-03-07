package common

import (
	"crypto/rand"
	"testing"
)

func BenchmarkOrderedList_Contains(b *testing.B) {
	size := 1000
	list := OrderedList[Address]{
		list:        make([]Address, 0, size),
		isOrdered:   false,
		compareFunc: CompareAddressess,
	}

	set := make(map[Address]struct{})
	dataSet := make(map[Address]struct{})

	for i := 0; i < size; i++ {
		var b [20]byte
		_, err := rand.Read(b[:])
		if err != nil {
			panic(err)
		}
		set[b] = struct{}{}
		list.Add(b)
		dataSet[b] = struct{}{}
	}
	for i := 0; i < size; i++ {
		var b [20]byte
		_, err := rand.Read(b[:])
		if err != nil {
			panic(err)
		}
		dataSet[b] = struct{}{}
	}
	list.Sort()

	b.ResetTimer()

	var ok bool

	b.Run("linear", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for k := range dataSet {
				ok = list.containsLinear(k)
			}
		}
	})

	b.Run("binary", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for k := range dataSet {
				ok = list.containsBinarySearch(k)
			}
		}
	})

	b.Run("set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for k := range dataSet {
				_, ok = set[k]
			}
		}
	})

	_ = ok
}
