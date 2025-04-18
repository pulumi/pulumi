// Copyright 2016-2025, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrayAppend(t *testing.T) {
	t.Parallel()
	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{
			New("a"),
			New("b"),
			New("c"),
		})

		cp := arr.Append(New("d"), New("e"))

		assert.Equal(t, NewArray([]Value{
			New("a"),
			New("b"),
			New("c"),
		}), arr)

		assert.Equal(t, NewArray([]Value{
			New("a"),
			New("b"),
			New("c"),
			New("d"),
			New("e"),
		}), cp)
	})

	t.Run("appends do not conflict", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{New(1.0)})

		arr2 := arr.Append(New(2.0))
		arr3 := arr.Append(New(3.0))

		assert.Equal(t, NewArray([]Value{
			New(1.0),
		}), arr)

		assert.Equal(t, NewArray([]Value{
			New(1.0),
			New(2.0),
		}), arr2)

		assert.Equal(t, NewArray([]Value{
			New(1.0),
			New(3.0),
		}), arr3)
	})
}

func TestArraySlice(t *testing.T) {
	t.Parallel()

	t.Run("from slice", func(t *testing.T) {
		t.Parallel()
		s := []Value{
			New(1.0),
			New(2.0),
		}

		arr := NewArray(s)

		s[1] = New(3.0) // Ensure that the mutation of s does not allow a mutation of arr.

		assert.Equal(t, NewArray([]Value{
			New(1.0),
			New(2.0),
		}), arr)
	})

	t.Run("to slice", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{
			New(1.0),
			New(2.0),
		})

		s := arr.AsSlice()

		s[1] = New(3.0) // Ensure that the mutation of s does not allow a mutation of arr.

		assert.Equal(t, NewArray([]Value{
			New(1.0),
			New(2.0),
		}), arr)
	})
}

func TestArrayLen(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, NewArray([]Value{
		New(1.0),
		New(2.0),
	}).Len())

	assert.Equal(t, 0, NewArray(nil).Len())
}

func TestArrayIndex(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{
			New(1.0),
			New(2.0),
		})
		assert.Equal(t, New(2.0), arr.Get(1))
	})

	t.Run("negative", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{
			New(1.0),
			New(2.0),
		})
		assert.Panics(t, func() { arr.Get(-1) })
	})

	t.Run("too big", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{
			New(1.0),
			New(2.0),
		})
		assert.Panics(t, func() { arr.Get(2) })
	})
}

func TestArrayAll(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		arr := NewArray(nil)
		arr.All(func(int, Value) bool {
			assert.Fail(t, "An empty array should never call it's iterator")
			return true
		})
	})

	t.Run("single element", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{New("e1")})
		var wasCalled bool
		arr.All(func(i int, v Value) bool {
			if wasCalled {
				assert.Fail(t, "An array with only 1 element should only be called once")
			}
			wasCalled = true
			assert.Equal(t, 0, i)
			assert.Equal(t, New("e1"), v)
			return true
		})
	})

	t.Run("multiple elements", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{New("e1"), New("e2"), New("e3")})
		var callCount int
		arr.All(func(i int, v Value) bool {
			callCount++
			switch callCount {
			case 1:
				assert.Equal(t, 0, i)
				assert.Equal(t, New("e1"), v)
			case 2:
				assert.Equal(t, 1, i)
				assert.Equal(t, New("e2"), v)
			case 3:
				assert.Equal(t, 2, i)
				assert.Equal(t, New("e3"), v)
			default:
				assert.Fail(t, "unexpected call")
			}
			return true
		})
		assert.Equal(t, 3, callCount)
	})

	t.Run("early exit", func(t *testing.T) {
		t.Parallel()
		arr := NewArray([]Value{New("e1"), New("e2")})
		var wasCalled bool
		arr.All(func(i int, v Value) bool {
			if wasCalled {
				assert.Fail(t, "An array with only 1 element should only be called once")
			}
			wasCalled = true
			assert.Equal(t, 0, i)
			assert.Equal(t, New("e1"), v)
			return false
		})
	})
}

func TestArrayEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Array{}, NewArray(nil))
	assert.Equal(t, Array{}, NewArray([]Value{}))

	assert.Equal(t, New(Array{}), New([]Value{}))
	assert.Equal(t, New(Array{}).AsArray(), New([]Value{}).AsArray())
	assert.Equal(t, New(Array{}).AsArray(), Array{})
	assert.Nil(t, Array{}.AsSlice())
}
