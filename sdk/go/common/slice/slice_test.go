// Copyright 2025, Pulumi Corporation.
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

package slice

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrealloc(t *testing.T) {
	t.Parallel()

	t.Run("panics when capacity is -1", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() { _ = Prealloc[string](-1) })
	})

	t.Run("returns nil when capacity is 0", func(t *testing.T) {
		t.Parallel()

		result := Prealloc[string](0)
		assert.Nil(t, result)
	})

	t.Run("returns empty slice with capacity when capacity > 0", func(t *testing.T) {
		t.Parallel()

		result := Prealloc[string](5)
		assert.NotNil(t, result)
		assert.Equal(t, 0, len(result))
		assert.Equal(t, 5, cap(result))
	})
}

func TestMap(t *testing.T) {
	t.Parallel()

	t.Run("nil map", func(t *testing.T) {
		t.Parallel()

		var input []int
		result := Map(input, func(i int) int {
			assert.Fail(t, "should not be called")
			return i
		})
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()

		input := []int{}
		result := Map(input, func(i int) int {
			assert.Fail(t, "should not be called")
			return i
		})
		assert.Nil(t, result)
	})

	t.Run("maps ints to their squares", func(t *testing.T) {
		t.Parallel()

		input := []int{1, 2, 3}
		expected := []int{1, 4, 9}
		result := Map(input, func(i int) int { return i * i })
		assert.Equal(t, expected, result)
		assert.Equal(t, 3, cap(result))
	})
}

func TestMapError(t *testing.T) {
	t.Parallel()

	t.Run("nil map", func(t *testing.T) {
		t.Parallel()

		var input []int
		result, err := MapError(input, func(i int) (int, error) {
			assert.Fail(t, "should not be called")
			return i, nil
		})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()

		input := []int{}
		result, err := MapError(input, func(i int) (int, error) {
			assert.Fail(t, "should not be called")
			return i, nil
		})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("maps strings to ints", func(t *testing.T) {
		t.Parallel()

		input := []string{"1", "2", "3", "4"}
		expected := []int{1, 2, 3, 4}
		result, err := MapError(input, strconv.Atoi)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
		assert.Equal(t, 4, cap(result))
	})

	t.Run("maps strings to ints error", func(t *testing.T) {
		t.Parallel()

		input := []string{"1", "2", "x", "4"}
		expected := []int{1, 2}
		result, err := MapError(input, strconv.Atoi)
		assert.Error(t, err)
		assert.Equal(t, expected, result)
		assert.Equal(t, 4, cap(result))
	})
}
