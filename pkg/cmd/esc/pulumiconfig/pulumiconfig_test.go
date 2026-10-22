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

package pulumiconfig

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func parse(t *testing.T, text string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(text), &doc))
	return &doc
}

func render(t *testing.T, doc *yaml.Node) string {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	require.NoError(t, enc.Encode(doc))
	require.NoError(t, enc.Close())
	return buf.String()
}

func mustPath(t *testing.T, s string) resource.PropertyPath {
	t.Helper()
	p, err := resource.ParsePropertyPathStrict(s)
	require.NoError(t, err)
	return p
}

func TestSet(t *testing.T) {
	t.Parallel()

	const pc = "values:\n  pulumiConfig:\n"
	const base = "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: x\n"
	const nested = pc + "    proj:a:\n      b: 1\n"
	const list = pc + "    proj:a:\n      - one\n      - two\n"
	const secret = pc + "    proj:a:\n      fn::secret:\n        ciphertext: c\n"

	tests := []struct {
		name, in, key, value, out string
		setErr                    string
		path                      bool
		names                     []string
		typ                       config.Type
	}{
		{
			name: "new key keeps sibling comments", in: base, key: "b", value: "val",
			out: "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: x\n    proj:b: val\n",
		},
		{
			name: "short spelling is edited in place", in: pc + "    a: x\n", key: "a", value: "val",
			names: []string{"proj:a", "a"}, out: pc + "    a: val\n",
		},
		{
			name: "both spellings fail", in: pc + "    a: x\n    proj:a: y\n", key: "a", value: "z",
			names: []string{"proj:a", "a"}, setErr: "pulumiConfig has both proj:a and a",
		},
		{name: "null values placeholder", in: "values:\n", key: "a", value: "x", out: pc + "    proj:a: x\n"},
		{name: "null pulumiConfig placeholder", in: pc, key: "a", value: "x", out: pc + "    proj:a: x\n"},
		{
			name: "no values", in: "imports:\n  - base\n", key: "a", value: "x",
			out: "imports:\n  - base\n" + pc + "    proj:a: x\n",
		},
		{
			name: "flow placeholder gains block style", in: "values:\n  pulumiConfig: {}\n", key: "a", value: "x",
			out: pc + "    proj:a: x\n",
		},
		{
			name: "top-level bool-like string stays a string", in: base, key: "a", value: "true",
			out: "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: \"true\"\n",
		},
		{
			name: "top-level leading zero stays a string", in: base, key: "a", value: "0123",
			out: "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: \"0123\"\n",
		},
		{
			name: "single-segment path stays a string", in: base, key: "a", value: "true", path: true,
			out: "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: \"true\"\n",
		},
		{
			name: "path leaf is coerced to bool", in: nested, key: "a.c", path: true, value: "true",
			out: pc + "    proj:a:\n      b: 1\n      c: true\n",
		},
		{
			name: "path leaf is coerced to int", in: nested, key: "a.b", path: true, value: "42",
			out: pc + "    proj:a:\n      b: 42\n",
		},
		{
			name: "path leaf with leading zero stays a string", in: nested, key: "a.b", path: true, value: "0123",
			out: pc + "    proj:a:\n      b: \"0123\"\n",
		},
		{
			name: "path leaf with type string", in: nested, key: "a.b", path: true, value: "42", typ: config.TypeString,
			out: pc + "    proj:a:\n      b: \"42\"\n",
		},
		{
			name: "path into existing list", in: list, key: "a[1]", path: true, value: "three",
			out: pc + "    proj:a:\n      - one\n      - three\n",
		},
		{
			name: "index past the end of an existing list", in: list, key: "a[3]", path: true, value: "x",
			setErr: "array index out of range",
		},
		{
			name: "sparse new array", in: pc + "    proj:z: 1\n", key: "a[2]", path: true, value: "x",
			out: pc + "    proj:z: 1\n    proj:a:\n      - null\n      - null\n      - x\n",
		},
		{
			name: "sparse array nested in a new map", in: pc + "    proj:z: 1\n", key: "a.b[1]", path: true, value: "x",
			out: pc + "    proj:z: 1\n    proj:a:\n      b:\n        - null\n        - x\n",
		},
		{
			name: "scalar replaced by a container", in: pc + "    proj:a: old\n", key: "a.b", path: true, value: "x",
			out: pc + "    proj:a:\n      b: x\n",
		},
		{
			name: "nested scalar is not replaced", in: nested, key: "a.b.c", path: true, value: "x",
			setErr: `["proj:a"].b.c: expected an array or an object`,
		},
		{
			name: "secret entry replaced by a map", in: secret, key: "a.b", path: true, value: "x",
			out: pc + "    proj:a:\n      b: x\n",
		},
		{
			name: "secret entry replaced by a list", in: secret, key: "a[0]", path: true, value: "x",
			out: pc + "    proj:a:\n      - x\n",
		},
		{
			name: "nested secret is not extended", in: pc + "    proj:a:\n      b:\n        fn::secret: s\n",
			key: "a.b.c", path: true, value: "x", setErr: `["proj:a"].b.c: expected an array or an object`,
		},
		{
			name: "other function calls are not replaced", in: pc + "    proj:a:\n      fn::open: x\n", key: "a.b", path: true,
			value: "z", out: pc + "    proj:a:\n      fn::open: x\n      b: z\n",
		},
		{
			name: "secure leaf overwritten alone", in: pc + "    proj:a:\n      secure: true\n", key: "a.secure", path: true,
			value: "tls", setErr: `maps with the single key "secure" are reserved`,
		},
		{
			name: "secure leaf next to a sibling", in: nested, key: "a.secure", path: true, value: "tls",
			out: pc + "    proj:a:\n      b: 1\n      secure: tls\n",
		},
		{
			name: "secure leaf alone in a new map", in: pc, key: "a.secure", path: true, value: "tls",
			setErr: `maps with the single key "secure" are reserved`,
		},
		{
			name: "secure leaf alone in a replaced entry", in: secret, key: "a.secure", path: true, value: "tls",
			setErr: `maps with the single key "secure" are reserved`,
		},
		{
			name: "secure leaf typed as a string", in: pc, key: "a.secure", path: true, value: "42",
			typ: config.TypeString, setErr: `maps with the single key "secure" are reserved`,
		},
		{
			name: "secure leaf that is not a string", in: pc, key: "a.secure", path: true, value: "true",
			out: pc + "    proj:a:\n      secure: true\n",
		},
		{
			name: "dollar in a value is a literal", in: base, key: "a", value: "${other} and $$",
			out: "# head\nvalues:\n  # keep\n  other: 1 # trailing\n  pulumiConfig:\n    proj:a: $${other} and $$$$\n",
		},
		{
			name: "dollar in the entry name is a literal", in: pc + "    proj:a: x\n", key: "$a", value: "val",
			out: pc + "    proj:a: x\n    proj:$$a: val\n",
		},
		{
			name: "dollar path segment finds the escaped key", in: pc + "    proj:a:\n      $$: old\n", key: `a["$"]`,
			path: true, value: "new", out: pc + "    proj:a:\n      $$: new\n",
		},
		{
			name: "fn path segment is rejected", in: base, key: `a["fn::toJSON"]`, path: true, value: "hello",
			setErr: "keys starting with fn:: are reserved in environments",
		},
		{
			name: "fn entry name is rejected", in: base, key: "fn::x", value: "hello", names: []string{"fn::x"},
			setErr: "keys starting with fn:: are reserved in environments",
		},
		{
			name: "fn path segment in another case is rejected", in: base, key: `a["Fn::Sub"]`, path: true, value: "hello",
			setErr: "keys starting with fn:: are reserved in environments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value := config.NewValue(tt.value)
			if tt.typ != 0 {
				value = config.NewTypedValue(tt.value, tt.typ)
			}
			entry, path := tt.key, resource.PropertyPath(nil)
			if tt.path {
				full := mustPath(t, tt.key)
				entry, path = full[0].(string), full[1:]
			}
			node, err := ValueNode(value, len(path) > 0)
			require.NoError(t, err)

			names := tt.names
			if names == nil {
				names = []string{"proj:" + entry}
			}
			doc := parse(t, tt.in)
			err = Set(doc, names, path, node)
			if tt.setErr != "" {
				require.ErrorContains(t, err, tt.setErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.out, render(t, doc))
		})
	}
}

func TestValueNodeObject(t *testing.T) {
	t.Parallel()

	node, err := ValueNode(config.NewObjectValue(`{"b":["${x}",1]}`), false)
	require.NoError(t, err)
	out, err := yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "b:\n    - $${x}\n    - 1\n", string(out))

	_, err = ValueNode(config.NewObjectValue(`{"fn::toJSON":"x"}`), false)
	require.ErrorContains(t, err, "keys starting with fn:: are reserved in environments")

	_, err = ValueNode(config.NewObjectValue(`{"b":{"fn::open":"x"}}`), false)
	require.ErrorContains(t, err, "keys starting with fn:: are reserved in environments")

	_, err = ValueNode(config.NewObjectValue(`{"Fn::GetAtt":["a","b"]}`), false)
	require.ErrorIs(t, err, errReservedName)

	_, err = ValueNode(config.NewObjectValue(`{"fn::open":"x","b":1}`), false)
	require.ErrorIs(t, err, errReservedName)

	node, err = ValueNode(config.NewObjectValue(`{"Fn::GetAtt":["a","b"],"c":1}`), false)
	require.NoError(t, err)
	out, err = yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "Fn::GetAtt:\n    - a\n    - b\nc: 1\n", string(out))

	_, err = ValueNode(config.NewSecureObjectValue(`{"b":"x"}`), false)
	require.ErrorContains(t, err, "secret object values are not supported")

	for _, object := range []string{`{"secure":"x"}`, `{"b":{"secure":"x"}}`, `{"b":[{"secure":"x"}]}`} {
		_, err = ValueNode(config.NewObjectValue(object), false)
		require.ErrorIs(t, err, errSecureReprReserved, object)
	}
	_, err = ValueNode(config.NewObjectValue(`{"secure":"x"}`), true)
	require.ErrorIs(t, err, errSecureReprReserved)

	node, err = ValueNode(config.NewObjectValue(`{"b":{"secure":1},"c":{"secure":"x","d":1}}`), false)
	require.NoError(t, err)
	out, err = yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "b:\n    secure: 1\nc:\n    d: 1\n    secure: x\n", string(out))
}

func TestValueNodeTyping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value    config.Value
		nested   bool
		expected string
	}{
		{config.NewValue("42"), false, "\"42\"\n"},
		{config.NewValue("42"), true, "42\n"},
		{config.NewValue("true"), false, "\"true\"\n"},
		{config.NewValue("true"), true, "true\n"},
		{config.NewValue("042"), true, "\"042\"\n"},
		{config.NewValue("18446744073709551615"), true, "18446744073709551615\n"},
		{config.NewTypedValue("42", config.TypeString), true, "\"42\"\n"},
		{config.NewTypedValue("42", config.TypeInt), false, "42\n"},
		{config.NewValue("${x}"), true, "$${x}\n"},
	}
	for _, tt := range tests {
		node, err := ValueNode(tt.value, tt.nested)
		require.NoError(t, err)
		out, err := yaml.Marshal(&node)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, string(out), "%v nested=%v", tt.value, tt.nested)
	}
}

func TestSecretNode(t *testing.T) {
	t.Parallel()

	node := secretNode("abc")
	out, err := yaml.Marshal(&node)
	require.NoError(t, err)
	assert.Equal(t, "fn::secret:\n    ciphertext: abc\n", string(out))

	viaValue, err := ValueNode(config.NewSecureValue("abc"), false)
	require.NoError(t, err)
	assert.Equal(t, node, viaValue)
}

func TestRemove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		names   []string
		path    string
		removed bool
		out     string
		err     string
	}{
		{
			name:    "entry keeps sibling comments",
			in:      "values:\n  pulumiConfig:\n    # a\n    proj:a: x\n    # b\n    proj:b: val\n",
			names:   []string{"proj:a"},
			removed: true,
			out:     "values:\n  pulumiConfig:\n    # b\n    proj:b: val\n",
		},
		{
			name:    "every spelling is removed",
			in:      "values:\n  pulumiConfig:\n    a: x\n    proj:a: y\n    proj:b: z\n",
			names:   []string{"proj:a", "a"},
			removed: true,
			out:     "values:\n  pulumiConfig:\n    proj:b: z\n",
		},
		{
			name:    "list element",
			in:      "values:\n  pulumiConfig:\n    proj:a:\n      - one\n      - two\n",
			names:   []string{"proj:a"},
			path:    "[0]",
			removed: true,
			out:     "values:\n  pulumiConfig:\n    proj:a:\n      - two\n",
		},
		{
			name:    "map member",
			in:      "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      c: 2\n",
			names:   []string{"proj:a"},
			path:    "b",
			removed: true,
			out:     "values:\n  pulumiConfig:\n    proj:a:\n      c: 2\n",
		},
		{
			name:  "map member leaving a lone secure string",
			in:    "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      secure: tls\n",
			names: []string{"proj:a"},
			path:  "b",
			out:   "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      secure: tls\n",
			err:   `maps with the single key "secure" are reserved`,
		},
		{
			name:  "a failing spelling leaves the other spelling untouched",
			in:    "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      c: 2\n    a:\n      b: 1\n      secure: tls\n",
			names: []string{"proj:a", "a"},
			path:  "b",
			out:   "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      c: 2\n    a:\n      b: 1\n      secure: tls\n",
			err:   `maps with the single key "secure" are reserved`,
		},
		{
			name:    "map member leaving a lone secure bool",
			in:      "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n      secure: true\n",
			names:   []string{"proj:a"},
			path:    "b",
			removed: true,
			out:     "values:\n  pulumiConfig:\n    proj:a:\n      secure: true\n",
		},
		{
			name:  "missing parent is a no-op",
			in:    "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n",
			names: []string{"proj:a"},
			path:  "x.y",
			out:   "values:\n  pulumiConfig:\n    proj:a:\n      b: 1\n",
		},
		{
			name:  "missing entry is a no-op",
			in:    "values:\n  pulumiConfig:\n    proj:a: x\n",
			names: []string{"proj:z"},
			out:   "values:\n  pulumiConfig:\n    proj:a: x\n",
		},
		{
			name:  "no values is a no-op",
			in:    "imports:\n  - base\n",
			names: []string{"proj:a"},
			out:   "imports:\n  - base\n",
		},
		{
			name:  "null placeholder is a no-op",
			in:    "values:\n  pulumiConfig:\n",
			names: []string{"proj:a"},
			out:   "values:\n  pulumiConfig:\n",
		},
		{
			name:    "escaped path segment",
			in:      "values:\n  pulumiConfig:\n    proj:a:\n      $$: 1\n      c: 2\n",
			names:   []string{"proj:a"},
			path:    `["$"]`,
			removed: true,
			out:     "values:\n  pulumiConfig:\n    proj:a:\n      c: 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc := parse(t, tt.in)
			removed, err := Remove(doc, tt.names, mustPath(t, tt.path))
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.removed, removed)
			assert.Equal(t, tt.out, render(t, doc))
		})
	}

	_, err := Remove(parse(t, "values:\n  pulumiConfig:\n    proj:a: x\n"), []string{"proj:a"}, mustPath(t, `["fn::x"]`))
	require.ErrorIs(t, err, errReservedName)
}

func TestFind(t *testing.T) {
	t.Parallel()

	doc := parse(t, "values:\n  pulumiConfig:\n    a: 1\n    proj:$$b: 2\n")

	name, ok := Find(doc, []string{"proj:a", "a"})
	assert.True(t, ok)
	assert.Equal(t, "a", name)

	name, ok = Find(doc, []string{"proj:$b"})
	assert.True(t, ok)
	assert.Equal(t, "proj:$b", name)

	_, ok = Find(doc, []string{"proj:c"})
	assert.False(t, ok)

	_, ok = Find(parse(t, "imports: [x]\n"), []string{"a"})
	assert.False(t, ok)
}

func TestImportsDefinition(t *testing.T) {
	t.Parallel()

	out, ok := ImportsDefinition(parse(t, "imports:\n  - a/b\n  - c\nvalues:\n  pulumiConfig:\n    x: 1\n"))
	require.True(t, ok)
	assert.Equal(t, "imports:\n    - a/b\n    - c\n", string(out))

	_, ok = ImportsDefinition(parse(t, "values:\n  pulumiConfig:\n    x: 1\n"))
	assert.False(t, ok)

	_, ok = ImportsDefinition(parse(t, "imports: []\n"))
	assert.False(t, ok)
}
