package smt

import "sync"

type SmtDeltaSnapshot struct {
	BlockHeight uint64
	ChangedKeys map[string]map[string]struct{} // table -> key -> exists

	next *SmtDeltaSnapshot // Points to older block
	prev *SmtDeltaSnapshot // Points to newer block
}

type SmtDeltaList struct {
	head   *SmtDeltaSnapshot
	tail   *SmtDeltaSnapshot
	length int
	mutex  sync.RWMutex
}

func NewSmtDeltaList() *SmtDeltaList {
	return &SmtDeltaList{
		head:   nil,
		tail:   nil,
		length: 0,
	}
}

// Push adds a new snapshot to the head (newest block).
func (list *SmtDeltaList) Push(blockHeight uint64, changedKeys map[string]map[string]struct{}) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	newNode := &SmtDeltaSnapshot{
		BlockHeight: blockHeight,
		ChangedKeys: changedKeys,
		next:        list.head,
		prev:        nil,
	}

	if list.head != nil {
		list.head.prev = newNode // Link the old head to the new node
	} else {
		list.tail = newNode // If list was empty, set tail to the new node
	}

	list.head = newNode
	list.length++ // Increment length when adding a node
}

// findNodeUnsafe finds a node by BlockHeight, starting from head.
func (list *SmtDeltaList) findNodeUnsafe(blockHeight uint64) *SmtDeltaSnapshot {
	current := list.head
	for current != nil {
		if current.BlockHeight == blockHeight {
			return current
		}
		current = current.next
	}
	return nil
}

// DeleteUpTo removes all snapshots with BlockHeight <= targetHeight.
func (list *SmtDeltaList) DeleteUpTo(targetHeight uint64) bool {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	// Find the node with targetHeight
	targetNode := list.findNodeUnsafe(targetHeight)
	if targetNode == nil {
		return false
	}

	// If deleting the head, clear everything up to the tail
	if targetNode == list.head {
		list.head = nil
		list.tail = nil
		list.length = 0
		return true
	}

	cur := list.head
	list.length = 0
	for cur != targetNode {
		list.length++
		cur = cur.next
	}

	// Truncate the list from the node to the tail
	list.tail = targetNode.prev
	targetNode.prev.next = nil
	targetNode.prev = nil

	return true
}

// GetAllChanges returns all snapshots from newest to oldest.
func (list *SmtDeltaList) GetAllChanges() []*SmtDeltaSnapshot {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	// Collect all nodes from head
	snapshots := make([]*SmtDeltaSnapshot, 0, list.length)
	for node := list.head; node != nil; node = node.next {
		snapshots = append(snapshots, node)
	}

	return snapshots
}

// GetDeltaSnapshotUpTo returns snapshots with BlockHeight <= targetHeight, from newest to oldest.
func (list *SmtDeltaList) GetDeltaSnapshotUpTo(targetHeight uint64) []*SmtDeltaSnapshot {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	snapshots := make([]*SmtDeltaSnapshot, 0, list.length)

	// Find the node with targetHeight
	targetNode := list.findNodeUnsafe(targetHeight)
	if targetNode == nil {
		return snapshots
	}

	for node := targetNode; node != nil; node = node.next {
		snapshots = append(snapshots, node)
	}

	return snapshots
}

// Length returns the number of snapshots in the list.
func (list *SmtDeltaList) Length() int {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.length
}

// Clear removes all snapshots from the list.
func (list *SmtDeltaList) Clear() {
	list.mutex.Lock()
	defer list.mutex.Unlock()
	list.head = nil
	list.tail = nil
	list.length = 0
}
