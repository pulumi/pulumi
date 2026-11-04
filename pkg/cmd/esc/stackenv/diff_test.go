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

package stackenv

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diffLines(t *testing.T, published, local string) []string {
	t.Helper()
	changes, err := DiffDefinitions([]byte(published), []byte(local))
	require.NoError(t, err)
	var b strings.Builder
	WriteDefinitionDiff(&b, colors.Never, "org/proj/env", 7, changes)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")[1:]
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "    ")
	}
	return lines
}

func TestDiffDefinitions(t *testing.T) {
	t.Parallel()

	const secret1, secret2 = "{fn::secret: {ciphertext: Y2lwaGVyMQ==}}", "{fn::secret: {ciphertext: Y2lwaGVyMg==}}"
	cases := []struct {
		name, published, local string
		want                   []string
	}{
		{
			"add, remove and change at depth",
			"values:\n  a:\n    b:\n      c: 1\n      d: x\n",
			"values:\n  a:\n    b:\n      c: 2\n      e: [1, {f: true}]\n  g: null\n",
			[]string{`~ values.a.b.c: 1 -> 2`, `- values.a.b.d: "x"`, `+ values.a.b.e: [1,{"f":true}]`, `+ values.g: null`},
		},
		{"null to a value", "values:\n  a: null\n", "values:\n  a: 1\n", []string{`~ values.a: null -> 1`}},
		{"key starting with a digit", "values:\n  2fa: 1\n", "values:\n  2fa: 2\n", []string{`~ values.2fa: 1 -> 2`}},
		{"type change", "values:\n  a: {b: 1}\n", "values:\n  a: [1]\n", []string{`~ values.a: {"b":1} -> [1]`}},
		{
			"list by index", "imports: [proj/a, proj/b, proj/c]\n", "imports: [proj/a, proj/c]\n",
			[]string{`~ imports[1]: "proj/b" -> "proj/c"`, `- imports[2]: "proj/c"`},
		},
		{
			"key order ignored", "values:\n  b: 2\n  a: {d: 4, c: 3}\nimports: [x]\n",
			"imports: [x]\nvalues: {a: {c: 3, d: 4}, b: 2}\n",
			[]string{},
		},
		{"a quoted number is a string", "values:\n  a: 1\n", "values:\n  a: \"1\"\n", []string{`~ values.a: 1 -> "1"`}},
		{
			"numbers as ESC decodes them", "values:\n  a: 0x1F\n  b: .inf\n", "values:\n  a: 31\n  b: 1e3\n",
			[]string{`~ values.b: +Inf -> 1000`},
		},
		{"no HTML escaping", "values:\n  a: x\n", "values:\n  a: \"<a&b>\\n\"\n", []string{`~ values.a: "x" -> "<a&b>\n"`}},
		{
			"$-escaped keys and values as written",
			"values:\n  pulumiConfig:\n    proj:$$a: $${x}\n",
			"values:\n  pulumiConfig:\n    proj:$$a: ${x}\n    \"q\\\"k\": 1\n",
			[]string{`~ values.pulumiConfig["proj:$$a"]: "$${x}" -> "${x}"`, `+ values.pulumiConfig["q\"k"]: 1`},
		},
		{
			"fn:: keys",
			"values:\n  creds: {fn::open::aws-login: {oidc: {roleArn: a}}}\n  j: {fn::join: [\",\", [a, b]]}\n",
			"values:\n  creds: {fn::open::aws-login: {oidc: {roleArn: b}}}\n  j: {fn::join: [\",\", [a, c]]}\n",
			[]string{
				`~ values.creds["fn::open::aws-login"].oidc.roleArn: "a" -> "b"`,
				`~ values.j["fn::join"][1][1]: "b" -> "c"`,
			},
		},
		{"empty published definition", "", "values:\n  a: 1\n", []string{`+ values: {"a":1}`}},
		{"same ciphertext", "values:\n  pw: " + secret1 + "\n", "values:\n  pw: " + secret1 + "\n", []string{}},
		{
			"ciphertext change", "values:\n  pulumiConfig:\n    proj:pw: " + secret1 + "\n",
			"values:\n  pulumiConfig:\n    proj:pw: " + secret2 + "\n",
			[]string{`~ values.pulumiConfig["proj:pw"]: [secret] -> [secret]`},
		},
		{
			"plaintext to secret", "values:\n  pw: hunter2\n  db: {pw: hunter2}\n",
			"values:\n  pw: " + secret1 + "\n  db: " + secret2 + "\n",
			[]string{`~ values.db: [plaintext] -> [secret]`, `~ values.pw: [plaintext] -> [secret]`},
		},
		{
			"long values are cut", "values:\n  a: x\n",
			"values:\n  a: " + strings.Repeat("x", 300) + "\n  b: " + strings.Repeat("é", 200) + "\n",
			[]string{
				`~ values.a: "x" -> "` + strings.Repeat("x", 255) + `... (302 bytes)`,
				`+ values.b: "` + strings.Repeat("é", 127) + `... (402 bytes)`,
			},
		},
		{
			"secret to plaintext", "values:\n  pw: " + secret1 + "\n", "values:\n  pw: hunter2\n",
			[]string{`~ values.pw: [secret] -> [plaintext]`},
		},
		{
			"secret added and removed", "values:\n  old: " + secret1 + "\n",
			"values:\n  new: " + secret2 + "\n  list: [{fn::secret: hunter2}]\n",
			[]string{`+ values.list: ["[secret]"]`, `+ values.new: [secret]`, `- values.old: [secret]`},
		},
		{
			"type change around a nested secret", "values:\n  a: hunter2\n  b: [" + secret1 + "]\n",
			"values:\n  a: {pw: " + secret2 + "}\n  b: hunter2\n",
			[]string{`~ values.a: [plaintext] -> [secret]`, `~ values.b: [secret] -> [plaintext]`},
		},
		// The service rejects both of these definitions.
		{
			"fn::secret with a sibling key", "values:\n  a: {fn::secret: x, b: 1}\n", "values:\n  a: {fn::secret: y, b: 1}\n",
			[]string{`~ values.a["fn::secret"]: "x" -> "y"`},
		},
		{
			"Fn::secret", "values:\n  a: {Fn::secret: x}\n", "values:\n  a: {Fn::secret: y}\n",
			[]string{`~ values.a["Fn::secret"]: "x" -> "y"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := diffLines(t, tc.published, tc.local)
			assert.Equal(t, tc.want, got)
			for _, leaked := range []string{"hunter2", "Y2lwaGVy", "ciphertext"} {
				assert.NotContains(t, strings.Join(got, "\n"), leaked)
			}
		})
	}
}

func TestDiffDefinitionsNoValueChanges(t *testing.T) {
	t.Parallel()

	published := "values:\n  a: x\n  b: |\n    line\n"
	local := "# note\nvalues:\n  a: \"x\" # trailing\n  b: \"line\\n\"\n"
	equal, err := DefinitionsEqual([]byte(local), []byte(published))
	require.NoError(t, err)
	require.False(t, equal)

	changes, err := DiffDefinitions([]byte(published), []byte(local))
	require.NoError(t, err)
	assert.Empty(t, changes)

	var b strings.Builder
	WriteDefinitionDiff(&b, colors.Never, "org/proj/env", 7, changes)
	assert.Equal(t, "Environment org/proj/env: local changes based on revision 7 (no value changes); "+
		"up, refresh and destroy publish them, preview does not\n", b.String())
}

func TestDiffDefinitionsErrors(t *testing.T) {
	t.Parallel()

	_, err := DiffDefinitions([]byte("values: [\n"), []byte("values: {}\n"))
	assert.ErrorContains(t, err, "published definition: ")
	_, err = DiffDefinitions([]byte("values: {}\n"), []byte("a: &x 1\nb: *x\n"))
	assert.ErrorContains(t, err, "local definition: ")
	_, err = DiffDefinitions([]byte("values: {}\n"), []byte("- 1\n"))
	assert.EqualError(t, err, "local definition: the top level is [1], not a mapping")
}

func TestWriteDefinitionDiff(t *testing.T) {
	t.Parallel()

	changes, err := DiffDefinitions([]byte("values:\n  a: 1\n  b: 2\n"), []byte("values:\n  a: 3\n  c: 4\n"))
	require.NoError(t, err)

	const header = "Environment org/proj/env: local changes based on revision 7; " +
		"up, refresh and destroy publish them, preview does not\n"
	cases := []struct {
		color                         colors.Colorization
		update, delete, create, reset string
	}{
		{colors.Never, "", "", "", ""},
		{colors.Raw, colors.SpecUpdate, colors.SpecDelete, colors.SpecCreate, colors.Reset},
		{colors.Always, "\x1b[38;5;3m", "\x1b[38;5;1m", "\x1b[38;5;2m", "\x1b[0m"},
	}
	for _, tc := range cases {
		t.Run(string(tc.color), func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			WriteDefinitionDiff(&b, tc.color, "org/proj/env", 7, changes)
			assert.Equal(t, header+
				"    "+tc.update+"~ values.a: 1 -> 3"+tc.reset+"\n"+
				"    "+tc.delete+"- values.b: 2"+tc.reset+"\n"+
				"    "+tc.create+"+ values.c: 4"+tc.reset+"\n", b.String())
		})
	}
}

func TestWriteDefinitionDiffTerminalSafe(t *testing.T) {
	t.Parallel()

	changes, err := DiffDefinitions([]byte("values: {}\n"),
		[]byte("values:\n  a: \"<{%x%}>\"\n  b: \"<{%\"\n  \"c\\nd\": 1\n  \"e\\ef\\u009b\": 2\n"+
			"  \"g\\u202eh\": 3\n  \"i\\u2028j\": 4\n  k: \"\\u202e\"\n  l: \"\\u2028\"\n  m: \"\\x7f\\u009b\"\n"+
			"  \"n\\U000E0001\": 5\n"))
	require.NoError(t, err)

	var b strings.Builder
	WriteDefinitionDiff(&b, colors.Always, "org/proj/env", 7, changes)
	lines := strings.Split(b.String(), "\n")
	assert.Equal(t, []string{
		"    \x1b[38;5;2m+ values.a: \"\\u003c{%x%}>\"\x1b[0m",
		"    \x1b[38;5;2m+ values.b: \"\\u003c{%\"\x1b[0m",
		"    \x1b[38;5;2m+ values[\"c\\u000ad\"]: 1\x1b[0m",
		"    \x1b[38;5;2m+ values[\"e\\u001bf\\u009b\"]: 2\x1b[0m",
		"    \x1b[38;5;2m+ values[\"g\\u202eh\"]: 3\x1b[0m",
		"    \x1b[38;5;2m+ values[\"i\\u2028j\"]: 4\x1b[0m",
		"    \x1b[38;5;2m+ values.k: \"\\u202e\"\x1b[0m",
		"    \x1b[38;5;2m+ values.l: \"\\u2028\"\x1b[0m",
		"    \x1b[38;5;2m+ values.m: \"\\u007f\\u009b\"\x1b[0m",
		"    \x1b[38;5;2m+ values[\"n\\U000e0001\"]: 5\x1b[0m",
		"",
	}, lines[1:])
}
