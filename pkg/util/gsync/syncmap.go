// Copyright 2016-2024, Pulumi Corporation.
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

// Generic sync package
package gsync

import (
	"sync"
)

// Map is like a Go map[K]V but is safe for concurrent use by multiple goroutines without additional
// locking or coordination. Loads, stores, and deletes run in amortized constant time.
type Map[K comparable, V any] struct {
	m sync.Map
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(k K, v V) {
	m.m.Store(k, v)
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(k K) {
	m.m.Delete(k)
}

// Load returns the value stored in the map for a key, or zero if no value is present. The ok result indicates whether
// value was found in the map
func (m *Map[K, V]) Load(k K) (value V, ok bool) {
	var s interface{}
	s, ok = m.m.Load(k)
	if ok {
		value = s.(V)
	}
	return
}

// LoadOrStore returns the existing value for the key if present. Otherwise, it stores and returns the given value. The
// loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(k K, v V) (value V, ok bool) {
	var s any
	s, ok = m.m.LoadOrStore(k, v)
	value = s.(V)
	return
}

// Range calls f sequentially for each key and value present in the map. If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's contents: no key will be visited more
// than once, but if the value for any key is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not block other methods on the receiver; even f
// itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns false after a constant number of calls.
func (m *Map[K, V]) Range(callback func(key K, value V) bool) {
	m.m.Range(func(k, v interface{}) bool {
		return callback(k.(K), v.(V))
	})
}
