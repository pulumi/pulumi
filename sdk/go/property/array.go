// Copyright 2016, Pulumi Corporation.
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
	"iter"
	"slices"
)

// An immutable Array of [Value]s.
//
// An Array is not itself a [Value], but it can be cheaply converted into a [Value] with
// [New].
type Array struct{ arr []Value }

// AsSlice copies the [Array] into a slice.
//
// AsSlice always returns a non-nil slice.
func (a Array) AsSlice() []Value {
	if a.arr == nil {
		return []Value{}
	}
	return slices.Clone(a.arr)
}

// All calls yield for each element of the list.
//
// If yield returns false, then the iteration terminates.
//
//	arr := property.NewArray([]property.Value{
//		property.New(1),
//		property.New(2),
//		property.New(3),
//	})
//
//	for i, v := range arr.All() {
//		fmt.Printf("Index: %d, value: %s\n", i, v)
//	}
func (a Array) All() iter.Seq2[int, Value] {
	return slices.All(a.arr)
}

// Get the value from an [Array] at an index.
//
// If idx is negative or if idx is greater then or equal to the length of the [Array],
// then this function will panic.
func (a Array) Get(idx int) Value {
	return a.arr[idx]
}

// The length of the [Array].
func (a Array) Len() int {
	return len(a.arr)
}

// Append a new value to the end of the [Array].
func (a Array) Append(v ...Value) Array {
	// We need to copy a.arr since append may mutate the backing array, which may be
	// shared.
	if len(v) == 0 {
		return a
	}
	return Array{append(copyArray(a.arr), v...)}
}

// NewArray creates a new [Array] from a slice of [Value]s. It is the inverse of
// [Array.AsSlice].
func NewArray(slice []Value) Array {
	return Array{copyArray(slice)}
}

// any reports whether f returns true for any [Value] in the Array or any nested Value.
func (a Array) any(f func(Value) bool) bool {
	for _, v := range a.arr {
		if v.any(f) {
			return true
		}
	}
	return false
}

// HasComputed returns true if any [Value] in the Array, or any nested Value, is computed.
func (a Array) HasComputed() bool { return a.any(Value.IsComputed) }

// HasSecrets returns true if any [Value] in the Array, or any nested Value, is secret.
func (a Array) HasSecrets() bool { return a.any(Value.IsSecret) }

func copyArray[T any](a []T) []T {
	if len(a) == 0 {
		return nil
	}
	return slices.Clone(a)
}
