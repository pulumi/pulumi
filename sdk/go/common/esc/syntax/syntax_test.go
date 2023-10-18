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

package syntax

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		node     Node
		expected string
	}{
		{String("foo"), "foo"},
		{Null(), "null"},
		{Number(3.14159), "3.14159"},
		{Number(3), "3"},
		{Array(String("e1"), Number(2), Array(), Null()), "[ e1, 2, [ ], null ]"},
		{
			Object(ObjectProperty(String("fizz"), String("buzz")), ObjectProperty(String("empty"), Object())),
			"{ fizz: buzz, empty: { } }",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, c.node.String())
		})
	}
}
