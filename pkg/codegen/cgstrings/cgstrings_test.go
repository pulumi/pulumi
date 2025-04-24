// Copyright 2022-2024, Pulumi Corporation.
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

package cgstrings

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCamel(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal("", Camel(""))
	assert.Equal("plugh", Camel("plugh"))
	assert.Equal("waldoThudFred", Camel("WaldoThudFred"))
	assert.Equal("graultBaz", Camel("Grault-Baz"))
	assert.Equal("graultBaz", Camel("grault-baz"))
	assert.Equal("graultBaz", Camel("graultBaz"))
	assert.Equal("grault_Baz", Camel("Grault_Baz"))
	assert.Equal("graultBaz", Camel("Grault-baz"))
}

func TestUnhyphenate(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		input, expected string
	}{
		{"", ""},
		{"waldo", "waldo"},
		{"waldo-thud-fred", "waldoThudFred"},
		{"waldo-Thud-Fred", "waldoThudFred"},
		{"waldo-Thud-Fred-", "waldoThudFred"},
		{"-waldo-Thud-Fred", "WaldoThudFred"},
		{"waldoThudFred", "waldoThudFred"},
		{"WaldoThudFred", "WaldoThudFred"},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("Subtest:%q", tc.input), func(t *testing.T) {
			t.Parallel()
			assert := assert.New(t)
			assert.Equal(tc.expected, Unhyphenate(tc.input))
		})
	}
}
