// Copyright 2016-2022, Pulumi Corporation.
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

package yamlutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertYamlEdit(t *testing.T, original string, edited interface{}, expected string) {
	t.Helper()

	actualValue, err := Edit([]byte(original), edited)
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(actualValue)))
}

type Foo struct {
	Foo     int      `yaml:"foo,omitempty"`
	Bar     string   `yaml:"bar,omitempty"`
	Baz     string   `yaml:"baz,omitempty"`
	Quux    int      `yaml:"quux,omitempty"`
	List    []string `yaml:"list,omitempty"`
	ListFoo []*Foo   `yaml:"listFoo,omitempty"`
}

func TestEdit(t *testing.T) {
	// Covers 100% of the happy path statements
	t.Parallel()

	assertYamlEdit(t, `
# header
foo: ["an illegal list"] # not a valid node in the foo Struct, tests coverage of unlike kinds
bar: baz # test2
list: ["1","2",
  "3" # test3
] #test4
listFoo:
  - bar: "bar1"
  # test5
  - bar: "bar2"  # nestedComment1
    list: ["a", "b", c] # nestedComment2
    # trailer
  - bar: "bar3"

# footer
`, Foo{
		Foo:  1,
		Baz:  "quux",
		List: []string{"1", "two", "pi", "e*2"},
		ListFoo: []*Foo{
			{Bar: "barOne"},
			{Bar: "barTwo", List: []string{"a", "bee", "cee"}},
		},
	}, `
# header
foo: 1
list: ["1", "two", "pi", # test3
  e*2] #test4
listFoo:
  - bar: "barOne"
  # test5
  - bar: "barTwo" # nestedComment1
    list: ["a", "bee", cee] # nestedComment2
    # trailer
baz: quux

# footer
`)
}

func TestEditEmpty(t *testing.T) {
	// Covers 100% of the happy path statements
	t.Parallel()

	assertYamlEdit(t, ``, Foo{
		Foo:  1,
		Baz:  "quux",
		List: []string{"1", "two", "pi", "e*2"},
		ListFoo: []*Foo{
			{Bar: "barOne"},
			{Bar: "barTwo", List: []string{"a", "bee", "cee"}},
		},
	}, `
foo: 1
baz: quux
list:
  - "1"
  - two
  - pi
  - e*2
listFoo:
  - bar: barOne
  - bar: barTwo
    list:
      - a
      - bee
      - cee
`)
}
