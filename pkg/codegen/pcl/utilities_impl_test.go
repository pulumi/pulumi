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

package pcl

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedSet(t *testing.T) {
	t.Parallel()

	t.Run("order", func(t *testing.T) {
		t.Parallel()
		s := newOrderedSet[string]()
		s.Add("b")
		s.Add("a")
		s.Add("b")
		s.Add("c")
		assert.Equal(t, []string{
			"b", "a", "c",
		}, slices.Collect(s.Iter()))
	})

	t.Run("deletion", func(t *testing.T) {
		t.Parallel()
		s := newOrderedSet[string]()
		s.Add("a")
		s.Add("b")
		s.Add("c")
		s.Delete("b")
		assert.Equal(t, []string{"a", "c"}, slices.Collect(s.Iter()))
	})

	t.Run("re-insertion", func(t *testing.T) {
		t.Parallel()
		s := newOrderedSet[string]()
		s.Add("a")
		s.Add("b")
		s.Add("c")
		s.Delete("b")
		s.Add("b")
		assert.Equal(t, []string{"a", "c", "b"}, slices.Collect(s.Iter()))
	})
}
