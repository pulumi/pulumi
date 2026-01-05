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

func TestMapSet(t *testing.T) {
	t.Parallel()
	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"a": New("alpha"),
			"b": New("beta"),
			"c": New("gamma"),
		})

		cp := m.Set("d", New("delta")).Set("e", New("epsilon"))

		assert.Equal(t, NewMap(map[string]Value{
			"a": New("alpha"),
			"b": New("beta"),
			"c": New("gamma"),
		}), m)

		assert.Equal(t, NewMap(map[string]Value{
			"a": New("alpha"),
			"b": New("beta"),
			"c": New("gamma"),
			"d": New("delta"),
			"e": New("epsilon"),
		}), cp)
	})

	t.Run("sets do not conflict", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{"key1": New(1.0)})

		m2 := m.Set("key2", New(2.0))
		m3 := m.Set("key3", New(3.0))

		assert.Equal(t, NewMap(map[string]Value{
			"key1": New(1.0),
		}), m)

		assert.Equal(t, NewMap(map[string]Value{
			"key1": New(1.0),
			"key2": New(2.0),
		}), m2)

		assert.Equal(t, NewMap(map[string]Value{
			"key1": New(1.0),
			"key3": New(3.0),
		}), m3)
	})
}

func TestMapDelete(t *testing.T) {
	t.Parallel()
	m := NewMap(map[string]Value{
		"a": New("alpha"),
		"b": New("beta"),
		"c": New("gamma"),
	})

	assert.Equal(t, NewMap(map[string]Value{
		"a": New("alpha"),
		"b": New("beta"),
	}), m.Delete("c"))

	assert.Equal(t, NewMap(map[string]Value{
		"a": New("alpha"),
		"b": New("beta"),
	}), m.Delete("c", "d"))

	assert.Equal(t, m, m.Delete("d"))

	assert.Equal(t, Map{}, m.Delete("a", "b", "c"))

	assert.Equal(t, 3, m.Len())
}

func TestMapLen(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 3, NewMap(map[string]Value{
		"one":   New(1.0),
		"two":   New(2.0),
		"three": New(3.0),
	}).Len())

	assert.Equal(t, 0, NewMap(nil).Len())
}

func TestMapGet(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key1": New(1.0),
			"key2": New(2.0),
		})
		assert.Equal(t, New(2.0), m.Get("key2"))
	})

	t.Run("non-existent", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key1": New(1.0),
		})
		assert.Equal(t, New(Null), m.Get("key2")) // Assuming 'New(nil)' represents a null value.
	})
}

func TestMapAll(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		m := NewMap(nil)
		m.All(func(string, Value) bool {
			assert.Fail(t, "An empty map should never call its iterator")
			return true
		})
	})

	t.Run("single element", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{"key1": New("v1")})
		var wasCalled bool
		m.All(func(k string, v Value) bool {
			if wasCalled {
				assert.Fail(t, "A map with only 1 element should only be called once")
			}
			wasCalled = true
			assert.Equal(t, "key1", k)
			assert.Equal(t, New("v1"), v)
			return true
		})
	})

	t.Run("multiple elements", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{"key1": New("v1"), "key2": New("v2"), "key3": New("v3")})
		var callCount int
		m.All(func(k string, v Value) bool {
			callCount++
			switch k {
			case "key1":
				assert.Equal(t, New("v1"), v)
			case "key2":
				assert.Equal(t, New("v2"), v)
			case "key3":
				assert.Equal(t, New("v3"), v)
			default:
				assert.Fail(t, "unexpected call")
			}
			return true
		})
		assert.Equal(t, 3, callCount)
	})

	t.Run("early exit", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{"key1": New("v1"), "key2": New("v2")})
		var wasCalled bool
		m.All(func(k string, v Value) bool {
			if wasCalled {
				assert.Fail(t, "A map iteration with early exit should only be called once")
			}
			wasCalled = true
			switch k {
			case "key1":
				assert.Equal(t, New("v1"), v)
			case "key2":
				assert.Equal(t, New("v2"), v)
			default:
				assert.Fail(t, "unexpected call")
			}

			return false
		})
	})
}

func TestMapAllStable(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		m := NewMap(nil)
		m.AllStable(func(string, Value) bool {
			assert.Fail(t, "An empty map should never call its iterator")
			return true
		})
	})

	t.Run("single element", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{"key1": New("v1")})
		var wasCalled bool
		m.AllStable(func(k string, v Value) bool {
			if wasCalled {
				assert.Fail(t, "A map with only 1 element should only be called once")
			}
			wasCalled = true
			assert.Equal(t, "key1", k)
			assert.Equal(t, New("v1"), v)
			return true
		})
	})

	t.Run("multiple elements", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key3": New("v3"),
			"key1": New("v1"),
			"key2": New("v2"),
		})

		expectedKeys := []string{"key1", "key2", "key3"}
		var index int
		m.AllStable(func(k string, v Value) bool {
			if index >= len(expectedKeys) {
				assert.Fail(t, "unexpected call")
			}
			assert.Equal(t, expectedKeys[index], k)
			assert.Equal(t, New("v"+string(expectedKeys[index][3])), v) // Matches "v1", "v2", "v3"
			index++
			return true
		})
		assert.Equal(t, len(expectedKeys), index)
	})

	t.Run("early exit", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key1": New("v1"),
			"key2": New("v2"),
		})
		var callCount int
		m.AllStable(func(k string, v Value) bool {
			callCount++
			if k == "key1" {
				assert.Equal(t, New("v1"), v)
				return false
			}
			assert.Fail(t, "The iteration should have been terminated early")
			return true
		})
		assert.Equal(t, 1, callCount)
	})
}

func TestMapGetOk(t *testing.T) {
	t.Parallel()

	t.Run("existing key", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key1": New(1.0),
			"key2": New(2.0),
		})

		value, ok := m.GetOk("key1")
		assert.True(t, ok, "Expected key to be present")
		assert.Equal(t, New(1.0), value)
	})

	t.Run("non-existent key", func(t *testing.T) {
		t.Parallel()
		m := NewMap(map[string]Value{
			"key1": New(1.0),
		})

		value, ok := m.GetOk("key3")
		assert.False(t, ok, "Expected key to be absent")
		assert.Equal(t, New(Null), value) // Assuming 'New(nil)' represents a null or zero value
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		m := NewMap(nil)

		value, ok := m.GetOk("key1")
		assert.False(t, ok, "Expected key not to be found in an empty map")
		assert.Equal(t, New(Null), value) // Assuming 'New(nil)' represents a null or zero value
	})
}

func TestMapEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Map{}, NewMap(nil))
	assert.Equal(t, Map{}, NewMap(map[string]Value{}))

	assert.Equal(t, New(Map{}), New(map[string]Value{}))
	assert.Equal(t, New(Map{}).AsMap(), New(map[string]Value{}).AsMap())
	assert.Equal(t, New(Map{}).AsMap(), Map{})
	assert.Equal(t, Map{}.AsMap(), map[string]Value{})
}
