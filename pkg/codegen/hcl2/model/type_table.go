package model

type typeTable struct {
	table []bucket // len is zero or a power of two
	len   int
	head  *entry  // insertion order doubly-linked list; may be nil
	tail  **entry // address of nil link at end of list (perhaps &head)
}

const bucketSize = 8

type bucket struct {
	entries [bucketSize]entry
	next    *bucket
}

type entry struct {
	hash  uint32 // nonzero => in use
	key   Type
	value interface{}
	next  *entry  // insertion order doubly-linked list; may be nil
	prev  **entry // address of link to this entry (perhaps &head)
}

func (tt *typeTable) init(size int) {
	if size < 0 {
		panic("size < 0")
	}
	nb := 1
	for overloaded(size, nb) {
		nb = nb << 1
	}
	tt.table = make([]bucket, nb)
	tt.tail = &tt.head
}

func (tt *typeTable) insert(k Type, v interface{}) error {
	if tt.table == nil {
		tt.init(1)
	}
	h := k.hash(nil)
	if h == 0 {
		h = 1 // zero is reserved
	}

retry:
	var insert *entry

	// Inspect each bucket in the bucket list.
	p := &tt.table[h&(uint32(len(tt.table)-1))]
	for {
		for i := range p.entries {
			e := &p.entries[i]
			if e.hash != h {
				if e.hash == 0 {
					// Found empty entry; make a note.
					insert = e
				}
				continue
			}
			if k.Equals(e.key) {
				// Key already present; update value.
				e.value = v
				return nil
			}
		}
		if p.next == nil {
			break
		}
		p = p.next
	}

	// Key not found. p points to the last bucket.

	// Check the load factor.
	if overloaded(tt.len, len(tt.table)) {
		tt.grow()
		goto retry
	}

	if insert == nil {
		// No space in existing buckets. Add a new one to the bucket list.
		b := &bucket{}
		p.next = b
		insert = &b.entries[0]
	}

	// Insert key/value pair.
	insert.hash = h
	insert.key = k
	insert.value = v

	// Append entry to doubly-linked list.
	insert.prev = tt.tail
	*tt.tail = insert
	tt.tail = &insert.next

	tt.len++

	return nil
}

func overloaded(elems, buckets int) bool {
	const loadFactor = 6.5 // just a guess
	return elems >= bucketSize && float64(elems) >= loadFactor*float64(buckets)
}

func (tt *typeTable) grow() {
	tt.table = make([]bucket, len(tt.table)<<1)
	oldhead := tt.head
	tt.head = nil
	tt.tail = &tt.head
	tt.len = 0
	for e := oldhead; e != nil; e = e.next {
		tt.insert(e.key, e.value)
	}
}

func (tt *typeTable) lookup(k Type, seen map[Type]struct{}) (v interface{}, found bool) {
	h := k.hash(nil)
	if h == 0 {
		h = 1 // zero is reserved
	}
	if tt.table == nil {
		return nil, false // empty
	}

	// Inspect each bucket in the bucket list.
	for p := &tt.table[h&(uint32(len(tt.table)-1))]; p != nil; p = p.next {
		for i := range p.entries {
			e := &p.entries[i]
			if e.hash == h {
				if k.equals(e.key, seen) {
					return e.value, true // found
				}
			}
		}
	}
	return nil, false // not found
}
