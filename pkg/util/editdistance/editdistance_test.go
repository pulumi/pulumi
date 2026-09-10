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

package editdistance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"list", "list", 0},
		{"ab", "ba", 1},       // the minimal adjacent transposition
		{"lsit", "list", 1},   // adjacent transposition
		{"stakc", "stack", 1}, // adjacent transposition
		{"lisst", "list", 1},  // insertion
		{"lit", "list", 1},    // deletion
		{"lost", "list", 1},   // substitution
		{"LIST", "list", 4},   // case-sensitive
		{"", "list", 4},
		{"abcd", "dbca", 2}, // non-adjacent swap: two substitutions, no discount
		{"abc", "cab", 2},   // rotation is not a transposition
		{"up", "rm", 2},
		{"héllo", "hello", 1}, // runes, not bytes
	}
	for _, c := range cases {
		assert.Equal(t, c.want, OSA(c.a, c.b), "OSA(%q, %q)", c.a, c.b)
		assert.Equal(t, c.want, OSA(c.b, c.a), "OSA(%q, %q)", c.b, c.a)
	}
}
