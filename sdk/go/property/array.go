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

// An immutable Array of [Value]s.
type Array struct{ arr []Value }

// AsSlice copies the [Array] into a slice.
func (a Array) AsSlice() []Value { return copyArray(a.arr) }

func (a Array) All(yield func(int, Value) bool) {
	for k, v := range a.arr {
		if !yield(k, v) {
			return
		}
	}
}

func (a Array) Get(idx int) Value {
	return a.arr[idx]
}

func (a Array) Len() int {
	return len(a.arr)
}

// Append a new value to the end of the current array.
func (a Array) Append(v Value) Array {
	// We need to copy a.arr since append may mutate the backing array, which may be
	// shared.
	return Array{append(copyArray(a.arr), v)}
}

func NewArray(slice []Value) Array { return Array{copyArray(slice)} }

func copyArray(a []Value) []Value {
	// Perform a shallow copy on v.
	cp := make([]Value, len(a))
	copy(cp, a)
	return cp
}
