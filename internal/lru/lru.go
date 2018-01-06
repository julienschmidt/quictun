package lru

import (
	"sync"
)

type entry struct {
	key   uint64
	value uint32
	next  *entry
}

// LRU is an LRU cache.
// Concurrent access is synchronized.
//
// A map is used as the index.
// The LRU order is tracked in a linked list.
type LRU struct {
	size  int               // max number of entries
	head  *entry            // first entry in LRU order
	cache map[uint64]*entry // mapping of all key-value pairs
	lock  sync.Mutex        // guards the whole struct
}

// NewLRU creates a new LRU cache with the given size
func New(size int) *LRU {
	if size < 2 {
		panic("size must be at least 2")
	}
	return &LRU{
		size:  size,
		cache: make(map[uint64]*entry, size),
	}
}

// Set sets the value for the given key. If an entry for the given key already
// exists, it is overwritten.
func (l *LRU) Set(key uint64, value uint32) (old uint32) {
	l.lock.Lock()
	if ep, ok := l.cache[key]; ok {
		old = ep.value
		ep.value = value
		l.moveToFront(ep)
		l.lock.Unlock()
		return
	}

	// insert new entry for key
	ep := new(entry)
	ep.key = key
	ep.value = value
	ep.next = l.head
	l.head = ep
	l.cache[key] = ep

	if len(l.cache) > l.size {
		l.removeLast()
	}
	l.lock.Unlock()
	return
}

// Get returns the current value for the given key.
// If no value for the given key exists, 0 is returned.
func (l *LRU) Get(key uint64) (value uint32) {
	l.lock.Lock()
	if ep, ok := l.cache[key]; ok {
		value = ep.value
		l.moveToFront(ep)
	}
	l.lock.Unlock()
	return value
}

func (l *LRU) moveToFront(ep *entry) {
	// move entry to front
	if l.head != ep {
		after := ep.next
		cur := l.head
		ep.next = cur
		for cur.next != ep {
			cur = cur.next
		}
		cur.next = after
		l.head = ep
	}
}

func (l *LRU) removeLast() {
	// remove last entry from list
	prev := l.head
	last := prev.next
	for last.next != nil {
		prev = last
		last = last.next
	}
	prev.next = nil

	// remove from cache
	delete(l.cache, last.key)
}
