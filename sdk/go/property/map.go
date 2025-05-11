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

package property

import (
	"slices"
)

// An immutable Map of [Value]s.
type Map struct{ m map[string]Value }

// AsMap converts the [Map] into a native Go map from strings to [Values].
//
// AsMap always returns a non-nil map.
func (m Map) AsMap() map[string]Value {
	// We always return non-nil because it's generally painful to work with nil maps
	// in Go.
	//
	// We return a nil for Array.AsSlice because it is easy to work with nil slices in
	// Go.
	return copyMapNonNil(m.m)
}

// All calls yield for each key value pair in the Map. All iterates in random order, just
// like Go's native maps. For stable iteration order, use [Map.AllStable].
//
// If yield returns false, then the iteration terminates.
//
//	m := property.NewMap(map[]property.Value{
//		"one": property.New(1),
//		"two": property.New(2),
//		"three": property.New(3),
//	})
//
//	m.All(func(k string, v Value) bool {
//		fmt.Printf("Key: %s, value: %s\n", k, v)
//		return true
//	})
//
// With Go 1.23, you can use iterator syntax to access each element:
//
//	for k, v := range arr.All {
//		fmt.Printf("Index: %s, value: %s\n", k, v)
//	}
func (m Map) All(yield func(string, Value) bool) {
	for k, v := range m.m {
		if !yield(k, v) {
			return
		}
	}
}

// AllStable calls yield for each key value pair in the Map in sorted key order.
//
// For usage, see [Map.All].
func (m Map) AllStable(yield func(string, Value) bool) {
	keys := make([]string, 0, len(m.m))
	for k := range m.m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !yield(k, m.m[k]) {
			return
		}
	}
}

// Get retrieves the [Value] associated with key in the [Map]. If key is not in [Map],
// then a [Null] value is returned.
//
// To distinguish between a zero value and no value, use [Map.GetOk].
func (m Map) Get(key string) Value {
	return m.m[key]
}

func (m Map) GetOk(key string) (Value, bool) {
	v, ok := m.m[key]
	return v, ok
}

// The number of elements in the [Map].
func (m Map) Len() int {
	return len(m.m)
}

// Set produces a new map identical to the receiver with key mapped to value.
//
// Set does not mutate it's receiver.
func (m Map) Set(key string, value Value) Map {
	cp := copyMapNonNil(m.m)
	cp[key] = value
	return Map{cp}
}

// Delete produces a new map identical to the receiver with given keys removed.
func (m Map) Delete(keys ...string) Map {
	cp := copyMapMaybeNil(m.m)
	for _, k := range keys {
		delete(cp, k)
	}
	if len(cp) == 0 {
		return Map{}
	}
	return Map{cp}
}

// NewMap creates a new map from m.
func NewMap(m map[string]Value) Map { return Map{copyMapMaybeNil(m)} }

func copyMapMaybeNil(m map[string]Value) map[string]Value {
	if len(m) == 0 {
		return nil
	}
	return copyMapNonNil(m)
}

func copyMapNonNil(m map[string]Value) map[string]Value {
	cp := make(map[string]Value, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
