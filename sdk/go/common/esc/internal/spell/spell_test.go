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

package spell

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b     string
		expected int
	}{
		{"vpcId", "cpcId", 1},
		{"vpcId", "foo", 5},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, levenshtein(c.a, c.b, nil))
	}
}

func TestNearest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		words      []string
		comparedTo string
		expected   string
	}{
		{[]string{}, "test", ""},
		{[]string{"", "", ""}, "test", ""},
		{[]string{"test", "test2"}, "test", "test"},
		{[]string{"test2", "test"}, "test", "test"},
		{[]string{"test2", "test", "test2"}, "test", "test"},
		{[]string{"c", "b", "a"}, "test", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, Nearest(c.comparedTo, slices.Values(c.words)))
	}
}

func TestSortByEditDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		words      []string
		comparedTo string
		expected   []string
	}{
		{[]string{}, "test", []string{}},
		{[]string{"", "", ""}, "test", []string{"", "", ""}},
		{[]string{"test", "test2"}, "test", []string{"test", "test2"}},
		{[]string{"test2", "test"}, "test", []string{"test", "test2"}},
		{[]string{"test2", "test", "test2"}, "test", []string{"test", "test2", "test2"}},
		{[]string{"c", "b", "a"}, "test", []string{"c", "b", "a"}},
	}
	for _, c := range cases {
		words := make([]string, len(c.words))
		copy(words, c.words)
		SortByEditDistance(c.comparedTo, words)

		assert.Equalf(t, c.expected, words, "SortByEditDistance(%v, %v)", c.words, c.comparedTo)
	}
}
