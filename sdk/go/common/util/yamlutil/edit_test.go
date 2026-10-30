// Copyright 2016, Pulumi Corporation.
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
	"gopkg.in/yaml.v3"
)

func assertYamlEdit(t *testing.T, original string, edited any, expected string) {
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

func TestEditEmptyFlowCollectionGainsContent(t *testing.T) {
	t.Parallel()

	assertYamlEdit(t, "list: [] # c\nlistFoo: [{bar: x}]\n", Foo{
		List:    []string{"a"},
		ListFoo: []*Foo{{Bar: "x"}, {Bar: "z"}},
	}, "list: [a] # c\nlistFoo: [{bar: x}, {bar: z}]\n")

	assertYamlEdit(t, "- [] # c\n- x\n", []any{[]any{"a"}, "x"}, "- [a] # c\n- x\n")

	assertYamlEdit(t, "values: {config: {} # c\n}\n", map[string]any{
		"values": map[string]any{"config": map[string]any{"a": "b"}},
	}, "values: {config: {a: b} # c\n}\n")

	assertYamlEdit(t, "x: !!map {}\n", map[string]any{"x": map[string]any{"a": "b"}}, "x: !!map {a: b}\n")

	assertYamlEdit(t, "values: {pulumiConfig: {}}\n", map[string]any{
		"values": map[string]any{"pulumiConfig": map[string]any{"foo": "bar"}},
	}, "values: {pulumiConfig: {foo: bar}}\n")

	assertYamlEdit(t, "values:\n  pulumiConfig: {}\n", map[string]any{
		"values": map[string]any{"pulumiConfig": map[string]any{"foo": "bar"}},
	}, "values:\n  pulumiConfig:\n    foo: bar\n")

	assertYamlEdit(t, "config: {} # keep me\nother: 1\n", map[string]any{
		"config": map[string]any{"a": "b"},
		"other":  1,
	}, "config: # keep me\n  a: b\nother: 1\n")

	assertYamlEdit(t, "config: {} # c\nb: 2 # d\n", map[string]any{
		"config": map[string]any{"x": "v"},
		"b":      2,
	}, "config: # c\n  x: v\nb: 2 # d\n")

	assertYamlEdit(t, "values:\n  pulumiConfig: {} # c\n  # foot\nother: 1\n", map[string]any{
		"values": map[string]any{"pulumiConfig": map[string]any{"x": "v"}},
		"other":  1,
	}, "values:\n  pulumiConfig: # c\n    x: v\n  # foot\nother: 1\n")

	assertYamlEdit(t, "rules:\n  - name: a\n    cfg: {} # c\n", map[string]any{
		"rules": []any{map[string]any{"name": "a", "cfg": map[string]any{"k": "v"}}},
	}, "rules:\n  - name: a\n    cfg: # c\n      k: v\n")

	assertYamlEdit(t, "values:\n  pulumiConfig: &pc {} # c\n", map[string]any{
		"values": map[string]any{"pulumiConfig": map[string]any{"a": "b"}},
	}, "values:\n  pulumiConfig: &pc {a: b} # c\n")

	assertYamlEdit(t, "a: &e {}\nb: 1\n", map[string]any{
		"a": map[string]any{"x": "v"},
		"b": 1,
	}, "a: &e\n  x: v\nb: 1\n")

	assertYamlEdit(t, "- {} # c\n- x\n", []any{map[string]any{"a": "b"}, "x"}, "- {a: b} # c\n- x\n")

	flowEnvironment := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "imports"},
		{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "e"}}},
	}}
	assertYamlEdit(t, "environment: {} # c\nother: 1\n", map[string]any{
		"environment": flowEnvironment,
		"other":       1,
	}, "environment: {imports: [e]} # c\nother: 1\n")
}

func TestEditCollectionEmptiedUnderCommentedKey(t *testing.T) {
	t.Parallel()

	assertYamlEdit(t, "config: # keep me\n  a: b\nother: 1\n", map[string]any{
		"config": map[string]any{},
		"other":  1,
	}, "config: {} # keep me\nother: 1\n")

	assertYamlEdit(t, "list: # c\n  - a\nother: 1\n", map[string]any{
		"list":  []any{},
		"other": 1,
	}, "list: [] # c\nother: 1\n")

	assertYamlEdit(t, "values: # v\n  pulumiConfig: # p\n    a: b\n", map[string]any{
		"values": map[string]any{"pulumiConfig": map[string]any{}},
	}, "values: # v\n  pulumiConfig: {} # p\n")

	assertYamlEdit(t, "aws: # c\n  region: x\nb: 1\n", map[string]any{
		"aws": []any{},
		"b":   1,
	}, "aws: [] # c\nb: 1\n")

	assertYamlEdit(t, "other: 1\nconfig: # c\n  a: b\n\n# footer\n", map[string]any{
		"config": map[string]any{},
		"other":  1,
	}, "other: 1\nconfig: {} # c\n\n# footer\n")
}

func TestEditSetThenRemoveRestoresOriginal(t *testing.T) {
	t.Parallel()

	stack := func(pulumiConfig string) string {
		return "environment:\n  imports:\n    - acme/base # org defaults\n  values:\n    # program config\n" +
			"    pulumiConfig:" + pulumiConfig
	}
	edited := func(pulumiConfig map[string]any) map[string]any {
		return map[string]any{"environment": map[string]any{
			"imports": []any{"acme/base"},
			"values":  map[string]any{"pulumiConfig": pulumiConfig},
		}}
	}
	original := stack(" {} # set with pulumi config set\n")
	withValues := stack(" # set with pulumi config set\n      app:pw:\n        fn::secret:\n" +
		"          ciphertext: abc\n      aws:region: us-west-2\n")
	set := edited(map[string]any{
		"aws:region": "us-west-2",
		"app:pw":     map[string]any{"fn::secret": map[string]any{"ciphertext": "abc"}},
	})

	assertYamlEdit(t, original, set, withValues)
	assertYamlEdit(t, withValues, edited(map[string]any{}), original)
}
