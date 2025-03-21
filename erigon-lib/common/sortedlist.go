// For X Layer

package common

type OrderedList[T comparable] struct {
	list        map[T]struct{}
	isOrdered   bool
	compareFunc func(a, b T) int
}

func (l *OrderedList[T]) Add(item T) {
	l.isOrdered = false
	l.list[item] = struct{}{}
}

func (l *OrderedList[T]) Sort() {
	//slices.SortFunc(l.list, l.compareFunc)
	//l.isOrdered = true
}

//func (l *OrderedList[T]) containsBinarySearch(item T) bool {
//	upper := len(l.list)
//	lower := 0
//	for lower < upper {
//		mid := (upper + lower) / 2
//		cmp := l.compareFunc(item, l.list[mid])
//		if cmp == 0 {
//			return true
//		}
//		if cmp > 0 {
//			lower = mid + 1
//		} else {
//			upper = mid
//		}
//	}
//	return false
//}
//
//func (l *OrderedList[T]) containsLinear(item T) bool {
//	for _, i := range l.list {
//		if l.compareFunc(i, item) == 0 {
//			return true
//		}
//	}
//	return false
//}

func (l *OrderedList[T]) Contains(item T) bool {
	_, ok := l.list[item]
	return ok
	//if !l.isOrdered {
	//	l.containsLinear(item)
	//}
	//return l.containsBinarySearch(item)
}

func (l *OrderedList[T]) Size() int {
	return len(l.list)
}

func (l *OrderedList[T]) Items() []T {
	ll := make([]T, 0, len(l.list))
	for i := range l.list {
		ll = append(ll, i)
	}
	return ll
}
