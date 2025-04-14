package smt

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

// createChangedKeys creates a map for testing
func createChangedKeys(tableKeys map[string][]string) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{})
	for table, keys := range tableKeys {
		result[table] = make(map[string]struct{})
		for _, key := range keys {
			result[table][key] = struct{}{}
		}
	}
	return result
}

// TestSmtDeltaList tests basic operations of SmtDeltaList
func TestSmtDeltaList(t *testing.T) {
	list := NewSmtDeltaList()

	// Test initial state
	if list.Length() != 0 {
		t.Errorf("Expected initial length 0, got %d", list.Length())
	}
	if len(list.GetAllChanges()) != 0 {
		t.Errorf("Expected empty GetAllChanges, got %d snapshots", len(list.GetAllChanges()))
	}
	if len(list.GetDeltaSnapshotUpTo(1)) != 0 {
		t.Errorf("Expected empty GetDeltaSnapshotUpTo(1), got %d snapshots", len(list.GetDeltaSnapshotUpTo(1)))
	}

	// Test Push
	list.Push(1, createChangedKeys(map[string][]string{"table1": {"key1", "key2"}}))
	list.Push(2, createChangedKeys(map[string][]string{"table2": {"key3"}}))
	list.Push(3, createChangedKeys(map[string][]string{"table1": {"key4"}}))

	if list.Length() != 3 {
		t.Errorf("Expected length 3, got %d", list.Length())
	}

	// Test GetAllChanges
	snapshots := list.GetAllChanges()
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}
	expectedHeights := []uint64{3, 2, 1} // Newest to oldest
	for i, snapshot := range snapshots {
		if snapshot.BlockHeight != expectedHeights[i] {
			t.Errorf("Expected snapshot %d height %d, got %d", i, expectedHeights[i], snapshot.BlockHeight)
		}
	}

	// Test GetDeltaSnapshotUpTo
	snapshotsUpTo2 := list.GetDeltaSnapshotUpTo(2)
	if len(snapshotsUpTo2) != 2 {
		t.Errorf("Expected 2 snapshots up to height 2, got %d", len(snapshotsUpTo2))
	}
	expectedUpTo2 := []uint64{2, 1}
	for i, snapshot := range snapshotsUpTo2 {
		if snapshot.BlockHeight != expectedUpTo2[i] {
			t.Errorf("Expected snapshot %d height %d, got %d", i, expectedUpTo2[i], snapshot.BlockHeight)
		}
	}

	// Test DeleteUpTo
	if !list.DeleteUpTo(2) {
		t.Errorf("Expected DeleteUpTo(2) to return true")
	}
	if list.Length() != 1 {
		t.Errorf("Expected length 1 after DeleteUpTo(2), got %d", list.Length())
	}
	snapshotsAfterDelete := list.GetAllChanges()
	if len(snapshotsAfterDelete) != 1 || snapshotsAfterDelete[0].BlockHeight != 3 {
		t.Errorf("Expected one snapshot with height 3, got %v", snapshotsAfterDelete)
	}

	// Test Clear
	list.Clear()
	if list.Length() != 0 {
		t.Errorf("Expected length 0 after Clear, got %d", list.Length())
	}
	if len(list.GetAllChanges()) != 0 {
		t.Errorf("Expected empty GetAllChanges after Clear, got %d", len(list.GetAllChanges()))
	}
}

// TestSmtDeltaListDeleteUpTo tests DeleteUpTo scenarios
func TestSmtDeltaListDeleteUpTo(t *testing.T) {
	list := NewSmtDeltaList()

	// Test DeleteUpTo on empty list
	if list.DeleteUpTo(1) {
		t.Errorf("Expected DeleteUpTo(1) on empty list to return false")
	}
	if list.Length() != 0 {
		t.Errorf("Expected length 0 after DeleteUpTo on empty list, got %d", list.Length())
	}

	// Populate list
	list.Push(1, createChangedKeys(map[string][]string{"table": {"key1"}}))
	list.Push(2, createChangedKeys(map[string][]string{"table": {"key2"}}))
	list.Push(3, createChangedKeys(map[string][]string{"table": {"key3"}}))
	list.Push(5, createChangedKeys(map[string][]string{"table": {"key5"}}))

	// Test DeleteUpTo middle node
	if !list.DeleteUpTo(3) {
		t.Errorf("Expected DeleteUpTo(3) to return true")
	}
	if list.Length() != 1 {
		t.Errorf("Expected length 1 after DeleteUpTo(3), got %d", list.Length())
	}
	snapshots := list.GetAllChanges()
	if len(snapshots) != 1 || snapshots[0].BlockHeight != 5 {
		t.Errorf("Expected one snapshot with height 5, got %v", snapshots)
	}

	// Repopulate
	list.Clear()
	list.Push(1, createChangedKeys(map[string][]string{"table": {"key1"}}))
	list.Push(2, createChangedKeys(map[string][]string{"table": {"key2"}}))
	list.Push(3, createChangedKeys(map[string][]string{"table": {"key3"}}))

	// Test DeleteUpTo tail
	if !list.DeleteUpTo(1) {
		t.Errorf("Expected DeleteUpTo(1) to return true")
	}
	if list.Length() != 2 {
		t.Errorf("Expected length 2 after DeleteUpTo(1), got %d", list.Length())
	}
	snapshots = list.GetAllChanges()
	expectedHeights := []uint64{3, 2}
	for i, snapshot := range snapshots {
		if snapshot.BlockHeight != expectedHeights[i] {
			t.Errorf("Expected snapshot %d height %d, got %d", i, expectedHeights[i], snapshot.BlockHeight)
		}
	}

	// Test DeleteUpTo head
	if !list.DeleteUpTo(3) {
		t.Errorf("Expected DeleteUpTo(3) to return true")
	}
	if list.Length() != 0 {
		t.Errorf("Expected length 0 after DeleteUpTo(3), got %d", list.Length())
	}
	if list.head != nil || list.tail != nil {
		t.Errorf("Expected head and tail nil after DeleteUpTo(3)")
	}
}

func TestSmtDeltaListBasicOperations(t *testing.T) {
	t.Run("Empty list", func(t *testing.T) {
		list := NewSmtDeltaList()
		assert.Equal(t, 0, list.Length())
		assert.Equal(t, 0, len(list.GetAllChanges()))
		assert.Equal(t, 0, len(list.GetDeltaSnapshotUpTo(1)))
		assert.False(t, list.DeleteUpTo(1))
	})

	t.Run("Single node", func(t *testing.T) {
		list := NewSmtDeltaList()
		keys := map[string]map[string]struct{}{"t1": {"k1": struct{}{}}}

		list.Push(1, keys)
		assert.Equal(t, 1, list.Length())

		snapshots := list.GetAllChanges()
		assert.Len(t, snapshots, 1)
		assert.Equal(t, uint64(1), snapshots[0].BlockHeight)

		assert.True(t, list.DeleteUpTo(1))
		assert.Equal(t, 0, list.Length())
	})

	t.Run("Multiple nodes in-order", func(t *testing.T) {
		list := NewSmtDeltaList()
		for i := 1; i <= 5; i++ {
			list.Push(uint64(i), map[string]map[string]struct{}{
				"table": {string(rune('a' + i)): struct{}{}},
			})
		}

		assert.Equal(t, 5, list.Length())

		t.Run("GetAllChanges order", func(t *testing.T) {
			snapshots := list.GetAllChanges()
			assert.Len(t, snapshots, 5)
			for i := 0; i < 5; i++ {
				assert.Equal(t, uint64(5-i), snapshots[i].BlockHeight)
			}
		})

		t.Run("GetDeltaSnapshotUpTo", func(t *testing.T) {
			snapshots := list.GetDeltaSnapshotUpTo(3)
			assert.Len(t, snapshots, 3)
			assert.Equal(t, uint64(3), snapshots[0].BlockHeight)
			assert.Equal(t, uint64(2), snapshots[1].BlockHeight)
			assert.Equal(t, uint64(1), snapshots[2].BlockHeight)
		})

		t.Run("Delete middle range", func(t *testing.T) {
			assert.True(t, list.DeleteUpTo(3))
			assert.Equal(t, 2, list.Length())

			snapshots := list.GetAllChanges()
			assert.Len(t, snapshots, 2)
			assert.Equal(t, uint64(5), snapshots[0].BlockHeight)
			assert.Equal(t, uint64(4), snapshots[1].BlockHeight)
		})
	})

	t.Run("Out-of-order pushes", func(t *testing.T) {
		list := NewSmtDeltaList()
		list.Push(3, map[string]map[string]struct{}{"t1": {"k1": struct{}{}}})
		list.Push(1, map[string]map[string]struct{}{"t2": {"k2": struct{}{}}})
		list.Push(2, map[string]map[string]struct{}{"t3": {"k3": struct{}{}}})

		assert.Equal(t, 3, list.Length())

		snapshots := list.GetAllChanges()
		assert.Equal(t, uint64(2), snapshots[0].BlockHeight)
		assert.Equal(t, uint64(1), snapshots[1].BlockHeight)
		assert.Equal(t, uint64(3), snapshots[2].BlockHeight)

		assert.True(t, list.DeleteUpTo(2))
		assert.Equal(t, 0, list.Length())
	})
}

func TestSmtDeltaListEdgeCases(t *testing.T) {
	t.Run("Delete non-existent height", func(t *testing.T) {
		list := NewSmtDeltaList()
		list.Push(1, map[string]map[string]struct{}{"t1": {"k1": struct{}{}}})
		assert.False(t, list.DeleteUpTo(2))
		assert.Equal(t, 1, list.Length())
	})

	t.Run("Delete exact height", func(t *testing.T) {
		list := NewSmtDeltaList()
		list.Push(1, map[string]map[string]struct{}{"t1": {"k1": struct{}{}}})
		list.Push(2, map[string]map[string]struct{}{"t2": {"k2": struct{}{}}})

		assert.True(t, list.DeleteUpTo(2))
		assert.Equal(t, 0, list.Length())
	})

	t.Run("Delete subset", func(t *testing.T) {
		list := NewSmtDeltaList()
		for i := 1; i <= 10; i++ {
			list.Push(uint64(i), map[string]map[string]struct{}{
				"table": {string(rune('a' + i)): struct{}{}},
			})
		}

		assert.True(t, list.DeleteUpTo(5))
		assert.Equal(t, 5, list.Length())

		snapshots := list.GetAllChanges()
		for i := 0; i < 5; i++ {
			assert.Equal(t, uint64(10-i), snapshots[i].BlockHeight)
		}
	})
}

func TestSmtDeltaListConcurrency(t *testing.T) {
	list := NewSmtDeltaList()
	const numRoutines = 100
	var wg sync.WaitGroup

	// Concurrent pushes
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func(i int) {
			defer wg.Done()
			list.Push(uint64(i+1), map[string]map[string]struct{}{
				"table": {string(rune('a' + i)): struct{}{}},
			})
		}(i)
	}
	wg.Wait()

	// Verify all pushed
	assert.Equal(t, numRoutines, list.Length())

	// Concurrent reads
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			defer wg.Done()
			snapshots := list.GetAllChanges()
			assert.True(t, len(snapshots) >= 0)
		}()
	}
	wg.Wait()

	// Concurrent delete and read
	wg.Add(2)
	go func() {
		defer wg.Done()
		list.DeleteUpTo(uint64(numRoutines / 2))
	}()
	go func() {
		defer wg.Done()
		snapshots := list.GetDeltaSnapshotUpTo(uint64(numRoutines))
		assert.True(t, len(snapshots) >= 0)
	}()
	wg.Wait()
}

func TestFindNodeUnsafe(t *testing.T) {
	list := NewSmtDeltaList()
	for i := 1; i <= 5; i++ {
		list.Push(uint64(i), map[string]map[string]struct{}{
			"table": {string(rune('a' + i)): struct{}{}},
		})
	}

	// Access without lock for test purpose
	list.mutex.Lock()
	assert.Nil(t, list.findNodeUnsafe(0))    // Not exist
	assert.NotNil(t, list.findNodeUnsafe(3)) // Middle
	assert.NotNil(t, list.findNodeUnsafe(5)) // Head
	assert.NotNil(t, list.findNodeUnsafe(1)) // Tail
	list.mutex.Unlock()
}
