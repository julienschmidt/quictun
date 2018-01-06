package lru

import (
	"fmt"
	"testing"
)

func printList(l *LRU) {
	cur := l.head
	i := 0
	for cur != nil {
		fmt.Println(i, cur.key, cur.value)
		cur = cur.next
		i++
	}
}

func TestLRU(t *testing.T) {
	lru := NewLRU(2)

	if n := len(lru.cache); n != 0 {
		t.Fatalf("cache should be empty, actually has %d elements", n)
	}

	// insert first value
	old := lru.Set(1337, 42)
	if old != 0 {
		t.Fatalf("old value for new key is %d, should be 0", old)
	}

	if n := len(lru.cache); n != 1 {
		t.Fatalf("cache should have 1 element, actually has %d elements", n)
	}

	// overwrite existing value
	old = lru.Set(1337, 43)
	if old != 42 {
		t.Fatalf("old value for existing key is %d, should be 42", old)
	}

	if n := len(lru.cache); n != 1 {
		t.Fatalf("cache should have 1 element, actually has %d elements", n)
	}

	// insert second value
	old = lru.Set(1338, 42)
	if old != 0 {
		t.Fatalf("old value for new key is %d, should be 0", old)
	}

	if n := len(lru.cache); n != 2 {
		t.Fatalf("cache should have 2 elements, actually has %d elements", n)
	}

	if head := lru.head.key; head != 1338 {
		t.Fatalf("newly inserted element is not head, key of head is %d", head)
	}

	// access the older value
	if v1 := lru.Get(1337); v1 != 43 {
		t.Fatalf("value of the first entry changed, should be 43, is %d", v1)
	}

	if head := lru.head.key; head != 1337 {
		t.Fatalf("accessed element is not head, key of head is %d", head)
	}

	// overwrite existing value
	old = lru.Set(1337, 42)
	if old != 43 {
		t.Fatalf("old value for existing key is %d, should be 43", old)
	}

	if n := len(lru.cache); n != 2 {
		t.Fatalf("cache should have 2 elements, actually has %d elements", n)
	}

	//printList(lru)

	// insert third value, removing the second
	old = lru.Set(1339, 7)
	if old != 0 {
		t.Fatalf("old value for new key is %d, should be 0", old)
	}

	if n := len(lru.cache); n != 2 {
		t.Fatalf("cache should have 2 elements, actually has %d elements", n)
	}

	if head := lru.head.key; head != 1339 {
		t.Fatalf("newly inserted element is not head, key of head is %d", head)
	}

	//printList(lru)
}
