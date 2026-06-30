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
