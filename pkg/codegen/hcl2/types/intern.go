// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"hash/maphash"
	"maps"
	"runtime"
	"slices"
	"sync"
	"weak"
)

// table interns values by a comparable key. For as long as any reference to the interned value for a key is live,
// intern returns that value for the key, so two interned values are equal exactly when their keys are. The table
// holds its values weakly: an entry is removed once its value is unreachable, and a later intern of the same key
// creates a new value.
//
// Entries are bucketed by the hash of the key, and the cleanup of a value knows only that hash. A key holds strong
// references to other interned values, so a cleanup that held the key would keep them alive for one more collection
// at each level of nesting.
type table[K comparable, V any] struct {
	// newValue builds the value for a key, and keyOf reads the key back from a value.
	newValue func(K) *V
	keyOf    func(*V) K

	mu   sync.Mutex
	seed maphash.Seed
	m    map[uint64][]weak.Pointer[V]
	// peak is the largest number of buckets m has held since it was last reallocated. A Go map never returns the
	// space of deleted entries, so the cleanup reallocates m when it has shrunk well below peak.
	peak int
}

func newTable[K comparable, V any](newValue func(K) *V, keyOf func(*V) K) *table[K, V] {
	return &table[K, V]{newValue: newValue, keyOf: keyOf, seed: maphash.MakeSeed(), m: map[uint64][]weak.Pointer[V]{}}
}

// intern returns the interned value for k.
func (t *table[K, V]) intern(k K) *V {
	h := maphash.Comparable(t.seed, k)
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, wp := range t.m[h] {
		if v := wp.Value(); v != nil && t.keyOf(v) == k {
			return v
		}
	}
	v := t.newValue(k)
	wp := weak.Make(v)
	t.m[h] = append(t.m[h], wp)
	runtime.AddCleanup(v, func(h uint64) {
		t.mu.Lock()
		defer t.mu.Unlock()
		bucket := slices.DeleteFunc(t.m[h], func(p weak.Pointer[V]) bool { return p == wp })
		if len(bucket) == 0 {
			delete(t.m, h)
		} else {
			t.m[h] = bucket
		}
		if len(t.m) < t.peak/4 {
			// maps.Clone keeps the capacity of its source, so copy into a map sized for the live entries.
			m := make(map[uint64][]weak.Pointer[V], len(t.m))
			maps.Copy(m, t.m)
			t.m, t.peak = m, len(m)
		}
	}, h)
	t.peak = max(t.peak, len(t.m))
	return v
}

var (
	nodeTable = newTable(func(n node) *entry { return &entry{node: n} }, func(e *entry) node { return e.node })
	cellTable = newTable(func(c cell) *cell { return &c }, func(c *cell) cell { return *c })
)
