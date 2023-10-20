// Copyright 2023, Pulumi Corporation.
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

package diags

import (
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
		assert.Equal(t, c.expected, editDistance(c.a, c.b))
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
		{[]string{"c", "b", "a"}, "test", []string{"a", "b", "c"}},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, sortByEditDistance(c.words, c.comparedTo), "sortByEditDistance(%v, %v)", c.words, c.comparedTo)
	}
}

func TestDisplayList(t *testing.T) {
	t.Parallel()
	cases := []struct {
		h         []string
		conjuctor string
		expected  string
	}{
		{[]string{}, "and", ""},
		{[]string{"a"}, "and", "a"},
		{[]string{"a", "b"}, "and", "a and b"},
		{[]string{"a", "b"}, "or", "a or b"},
		{[]string{"a", "b"}, "random", "a random b"},
		{[]string{"a", "b", "c"}, "and", "a, b and c"},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, displayList(c.h, c.conjuctor), "displayList(%v, %v)", c.h, c.conjuctor)
	}
}
