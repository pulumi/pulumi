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

package display

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_decodeValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		repr     string
		kind     string
		expected any
	}{
		// Negative cases
		{repr: ""},
		{repr: "foo"},
		{repr: "1.0"},
		{repr: "true"},
		{repr: "no"},
		{repr: "-"},
		{repr: "foo: bar"},
		{repr: "[] bar"},
		{repr: "{} bar"},
		{repr: "[] \n not yaml"},
		{repr: "---\n'hello'\n...\n---\ngoodbye\n...\n"},

		// Positive cases
		{
			repr:     "[]",
			kind:     "json",
			expected: []any{},
		},
		{
			repr:     "[\"foo\", \"bar\"]",
			kind:     "json",
			expected: []any{"foo", "bar"},
		},
		{
			repr:     "{}",
			kind:     "json",
			expected: map[string]any{},
		},
		{
			repr:     `{"foo": "bar"}`,
			kind:     "json",
			expected: map[string]any{"foo": "bar"},
		},
		{
			repr:     `  {"with": "whitespace"}  `,
			kind:     "json",
			expected: map[string]any{"with": "whitespace"},
		},
		{
			repr:     "- foo\n- bar",
			kind:     "yaml",
			expected: []any{"foo", "bar"},
		},
		{
			repr:     "foo: bar\nbaz: qux\n",
			kind:     "yaml",
			expected: map[string]any{"foo": "bar", "baz": "qux"},
		},
	}

	for _, c := range cases {
		t.Run(c.repr, func(t *testing.T) {
			t.Parallel()

			var printer propertyPrinter
			actual, kind, ok := printer.decodeValue(c.repr)

			if c.kind == "" {
				assert.Equal(t, resource.PropertyValue{}, actual)
				assert.Equal(t, "", kind)
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				require.Equal(t, c.kind, kind)
				assert.True(t, resource.NewPropertyValue(c.expected).DeepEquals(actual))
			}
		})
	}
}

func Test_PrintObject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		object     resource.PropertyMap
		expected   string
		showSecret bool
	}{
		{
			"numbers",
			resource.NewPropertyMapFromMap(map[string]any{
				"int":         1,
				"float":       2.3,
				"large_int":   1234567,
				"large_float": 1234567.1234567,
			}),
			`<{%reset%}>float      : <{%reset%}><{%reset%}>2.3<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>int        : <{%reset%}><{%reset%}>1<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_float: <{%reset%}><{%reset%}>1.2345671234567e+06<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_int  : <{%reset%}><{%reset%}>1234567<{%reset%}><{%reset%}>
<{%reset%}>`,
			false,
		},
		{
			"secret_noshow",
			resource.NewPropertyMapFromMap(map[string]any{
				"secret": resource.NewProperty(&resource.Secret{Element: resource.NewProperty("secrets")}),
				"nested_secret": resource.NewPropertyMapFromMap(map[string]any{
					"super_secret": resource.NewProperty(&resource.Secret{
						Element: resource.NewProperty("super_secret"),
					}),
				}),
			}),
			`<{%reset%}>nested_secret: <{%reset%}><{%reset%}>{
<{%reset%}><{%reset%}>    super_secret: <{%reset%}><{%reset%}>[secret]<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>}<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>secret       : <{%reset%}><{%reset%}>[secret]<{%reset%}><{%reset%}>
<{%reset%}>`,
			false,
		},
		{
			"secrets_show",
			resource.NewPropertyMapFromMap(map[string]any{
				"secret": resource.NewProperty(&resource.Secret{Element: resource.NewProperty("my_secret")}),
				"nested_secret": resource.NewPropertyMapFromMap(map[string]any{
					"super_secret": resource.NewProperty(&resource.Secret{
						Element: resource.NewProperty("my_super_secret"),
					}),
				}),
			}),
			`<{%reset%}>nested_secret: <{%reset%}><{%reset%}>{
<{%reset%}><{%reset%}>    super_secret: <{%reset%}><{%reset%}>"my_super_secret"<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>}<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>secret       : <{%reset%}><{%reset%}>"my_secret"<{%reset%}><{%reset%}>
<{%reset%}>`,
			true,
		},
		{
			"byte_string",
			resource.NewPropertyMapFromMap(map[string]any{
				"bytes": resource.NewProperty("\x00hello \x80\xfe\xff world\xf0\x28"),
			}),
			`<{%reset%}>bytes: <{%reset%}><{%reset%}>b"AGhlbGxvIID+/yB3b3JsZPAo"<{%reset%}><{%reset%}>
<{%reset%}>`,
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			PrintObject(&buf, c.object, false, 0, deploy.OpSame, false, false, false, c.showSecret)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func TestGetResourceOutputsPropertiesString(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name           string
		oldState       engine.StepEventStateMetadata
		newState       engine.StepEventStateMetadata
		showSames      bool
		showSecrets    bool
		truncateOutput bool
		expected       string
	}{
		{
			name: "stack outputs are with showSames = true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"banana": "yummy",
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"banana": "yummy",
				}),
			},
			showSames: true,
			expected: "<{%fg 3%}>    banana: <{%reset%}>" +
				"<{%fg 3%}>\"yummy\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>",
		},
		{
			name: "stack outputs are shown with showSames = false",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"banana": "yummy",
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"banana": "yummy",
				}),
			},
			showSames: false,
			expected: "<{%fg 3%}>    banana: <{%reset%}>" +
				"<{%fg 3%}>\"yummy\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>",
		},
		{
			name: "stack output added",
			oldState: engine.StepEventStateMetadata{
				URN:     "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type:    "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"someProp": "added",
				}),
			},
			expected: "<{%fg 2%}>  + someProp: <{%reset%}>" +
				"<{%fg 2%}>\"added\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>",
		},
		{
			name: "stack output removed",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"someProp": "removed",
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:     "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type:    "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{}),
			},
			expected: "<{%fg 1%}>  - someProp: <{%reset%}>" +
				"<{%fg 1%}>\"removed\"<{%reset%}>" +
				"<{%fg 1%}>\n<{%reset%}>",
		},
		{
			name: "stack output changed",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"someProp": "initial",
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"someProp": "changed",
				}),
			},
			expected: "<{%fg 3%}>  ~ someProp: <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 1%}>initial<{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 3%}> => <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 2%}>changed<{%reset%}>" +
				"<{%fg 3%}>\"\n<{%reset%}>",
		},
		{
			name: "stack output secret with showSecrets = false",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("shhhh")),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("shhhh")),
					"secretObj": resource.MakeSecret(
						resource.NewProperty(resource.NewPropertyMapFromMap(map[string]any{
							"a": resource.NewProperty("1"),
						}))),
				}),
			},
			showSecrets: false,
			expected: "<{%reset%}>" +
				"    secret   : <{%reset%}>" +
				"<{%reset%}>" +
				"[secret]<{%reset%}>" +
				"<{%reset%}>" +
				"\n<{%reset%}>" +
				"<{%fg 2%}>  + secretObj: <{%reset%}>" +
				"<{%fg 2%}>[secret]<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>",
		},
		{
			name: "stack output secret with showSecrets = true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("shhhh")),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("shhhh")),
					"secretObj": resource.MakeSecret(
						resource.NewProperty(resource.NewPropertyMapFromMap(map[string]any{
							"a": resource.NewProperty("1"),
						}))),
				}),
			},
			showSecrets: true,
			expected: "<{%reset%}>" +
				"    secret   : <{%reset%}>" +
				"<{%reset%}>" +
				"\"shhhh\"<{%reset%}>" +
				"<{%reset%}>" +
				"\n<{%reset%}>" +
				"<{%fg 2%}>  + secretObj: <{%reset%}>" +
				"<{%fg 2%}>{\n<{%reset%}" +
				"><{%fg 2%}>      + a: <{%reset%}" +
				"><{%fg 2%}>\"1\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>" +
				"<{%fg 2%}>    }<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>",
		},
		{
			name: "stack output secret changed with showSecrets = true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("initial")),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("changed")),
				}),
			},
			showSecrets: true,
			expected: "<{%fg 3%}>  ~ secret: <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 1%}>initial<{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 3%}> => <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 2%}>changed<{%reset%}>" +
				"<{%fg 3%}>\"\n<{%reset%}>",
		},
		{
			name: "stack output secret changed with showSecrets = false",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("initial")),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("changed")),
				}),
			},
			showSecrets: false,
			expected: "<{%fg 3%}>  ~ secret: <{%reset%}>" +
				"<{%fg 1%}>[secret]<{%reset%}>" +
				"<{%fg 3%}> => <{%reset%}>" +
				"<{%fg 2%}>[secret]<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>",
		},
		{
			name: "stack output secret became plain with showSecrets = true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("initial")),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.NewProperty("changed"),
				}),
			},
			showSecrets: true,
			expected: "<{%fg 3%}>  ~ secret: <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 1%}>initial<{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 3%}> => <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 2%}>changed<{%reset%}>" +
				"<{%fg 3%}>\"\n<{%reset%}>",
		},
		{
			name: "stack output plain became secret with showSecrets = true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.NewProperty("initial"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"secret": resource.MakeSecret(resource.NewProperty("changed")),
				}),
			},
			showSecrets: true,
			expected: "<{%fg 3%}>  ~ secret: <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 1%}>initial<{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 3%}> => <{%reset%}>" +
				"<{%fg 3%}>\"<{%reset%}>" +
				"<{%fg 2%}>changed<{%reset%}>" +
				"<{%fg 3%}>\"\n<{%reset%}>",
		},
		{
			name: "truncates with truncateOutput=true",
			oldState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"truncate_this":        strings.Repeat("a", 300), // max 150 characters
					"dont_truncate_this":   strings.Repeat("a", 150),
					"truncate_this_also":   "1\n2\n3\n4\n5\n6", // max 3 lines
					"dont_truncate_either": "1\n2\n3\n",
				}),
			},
			showSames:      false,
			truncateOutput: true,
			expected: "<{%fg 3%}>    dont_truncate_either: <{%reset%}>" +
				"<{%fg 3%}>\"1\\n2\\n3\\n...\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>" +
				"<{%fg 3%}>    dont_truncate_this  : <{%reset%}>" +
				//nolint:lll
				"<{%fg 3%}>\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>" +
				"<{%fg 3%}>    truncate_this       : <{%reset%}>" +
				//nolint:lll
				"<{%fg 3%}>\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa...\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>" +
				"<{%fg 3%}>    truncate_this_also  : <{%reset%}>" +
				"<{%fg 3%}>\"1\\n2\\n3\\n...\"<{%reset%}>" +
				"<{%fg 3%}>\n<{%reset%}>",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := engine.StepEventMetadata{
				Op:   "update",
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Old:  &tt.oldState,
				New:  &tt.newState,
			}
			s := getResourceOutputsPropertiesString(
				step,
				1,                 /*indent */
				false,             /* planning */
				false,             /* debug */
				false,             /* refresh */
				tt.showSames,      /* showSames */
				tt.showSecrets,    /* showSecrets */
				tt.truncateOutput, /* truncateOutput */
			)
			require.Equal(t, tt.expected, s)
		})
	}
}

// randomText returns lines newline-separated lines of width pseudo-random lowercase characters.
func randomText(seed int64, width int, lines int) string {
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // Deterministic test data.
	var sb strings.Builder
	for l := range lines {
		if l > 0 {
			sb.WriteByte('\n')
		}
		for range width {
			sb.WriteByte(byte('a' + r.Intn(26)))
		}
	}
	return sb.String()
}

// renderStringDiff renders a diff of a changed string property, failing the test if it takes too long.
func renderStringDiff(t *testing.T, old, new string, truncateOutput bool) string {
	t.Helper()

	diff := resource.NewPropertyMapFromMap(map[string]any{"value": old}).
		Diff(resource.NewPropertyMapFromMap(map[string]any{"value": new}))
	require.NotNil(t, diff)

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		PrintObjectDiff(&buf, *diff, nil, false, 1, false, truncateOutput, false, false, nil)
		done <- buf.String()
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(30 * time.Second):
		t.Fatal("timed out rendering a text diff")
		return ""
	}
}

// A formatting-only change deep inside a large JSON value must stay visible in the diff.
func TestTextDiffReformattedJSONShowsChange(t *testing.T) {
	t.Parallel()

	values := make([]int, 2000)
	for i := range values {
		values[i] = i
	}
	b, err := json.MarshalIndent(values, "", "  ")
	require.NoError(t, err)
	old := string(b)
	new := strings.Replace(old, "  1000,", "  1000, ", 1)

	for _, truncateOutput := range []bool{true, false} {
		out := renderStringDiff(t, old, new, truncateOutput)
		assert.Contains(t, out, `1000,<{%reset%}><{%fg 2%}> <{%reset%}>`, "truncateOutput=%v", truncateOutput)
	}
}

func TestTextDiffLimits(t *testing.T) {
	t.Parallel()

	const marker = "detailed diff omitted"

	bigOld := randomText(1, 1<<20, 1)
	bigNew := randomText(2, 1<<20, 1)
	smallEdit := bigOld[:1<<19] + "!" + bigOld[1<<19+1:]
	linesOld := randomText(1, 20, 10_000)
	linesNew := randomText(2, 20, 10_000)
	scattered := strings.Split(linesOld, "\n")
	for i := 0; i < len(scattered); i += 200 {
		scattered[i] = "changed-" + scattered[i]
	}
	values := make([]int, 20_000)
	for i := range values {
		values[i] = i
	}
	minified, err := json.Marshal(values)
	require.NoError(t, err)
	indented, err := json.MarshalIndent(values, "", "  ")
	require.NoError(t, err)
	repeatedLines := strings.Repeat("same\n", 50_000)

	tests := []struct {
		name        string
		old, new    string
		untruncated bool
		contains    []string
		notContains []string
		minLen      int // Set for values that are still diffed, which prints them in full.
		maxLen      int // Set for values that are replaced or whose printed lines are truncated.
	}{
		{
			name: "different single lines are replaced", old: bigOld, new: bigNew,
			contains: []string{bigOld[:150] + "...", bigNew[:150] + "...", marker, "spans 1048576 characters"},
			maxLen:   4096,
		},
		{
			name: "different single lines are replaced in full", old: bigOld, new: bigNew, untruncated: true,
			contains: []string{bigOld, bigNew, marker}, maxLen: 4 * (len(bigOld) + len(bigNew)),
		},
		{
			name: "small edit in a large single line is diffed", old: bigOld, new: smallEdit,
			contains: []string{"<{%fg 2%}>!"}, notContains: []string{marker}, minLen: len(bigOld),
		},
		{
			name: "scattered edits in a large multi-line value are diffed", old: linesOld,
			new:      strings.Join(scattered, "\n"),
			contains: []string{"changed-" + scattered[0][len("changed-"):]}, notContains: []string{marker}, maxLen: 32768,
		},
		{
			name: "rewritten multi-line value is replaced", old: linesOld, new: linesNew,
			contains: []string{marker, "spans 10000 lines", " => "}, maxLen: 4096,
		},
		{
			name: "rewritten multi-line value is shown as a line diff", old: linesOld, new: linesNew, untruncated: true,
			contains: []string{
				"- " + strings.SplitN(linesOld, "\n", 2)[0], "+ " + strings.SplitN(linesNew, "\n", 2)[0],
				marker, "spans 10000 lines",
			},
			notContains: []string{" => "}, minLen: len(linesOld) + len(linesNew),
		},
		{
			name: "lines appended after a final newline are diffed", old: "header\n",
			new: "header\n" + strings.Repeat("new\n", 10_000) + "last", notContains: []string{marker},
		},
		{
			name: "lines removed after a final newline are diffed", old: "header\n" + strings.Repeat("new\n", 10_000) + "last",
			new: "header\n", notContains: []string{marker},
		},
		{
			// The values decode to the same JSON, so the text is replaced and must be quoted only once.
			name: "reformatted JSON is replaced", old: string(minified), new: string(indented),
			contains: []string{`"[0,1,2,3`, marker}, notContains: []string{`\"[`}, maxLen: 4096,
		},
		{
			name: "repeated characters with different ends", old: "X" + strings.Repeat("A", 200_000) + "Y",
			new: "Z" + strings.Repeat("A", 200_000) + "W", untruncated: true,
		},
		{
			name: "repeated lines with different ends", old: "x\n" + repeatedLines + "y",
			new: "z\n" + repeatedLines + "w", untruncated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := renderStringDiff(t, tt.old, tt.new, !tt.untruncated)
			for _, s := range tt.contains {
				assert.Contains(t, out, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, out, s)
			}
			if tt.minLen > 0 {
				assert.Greater(t, len(out), tt.minLen)
			}
			if tt.maxLen > 0 {
				assert.Less(t, len(out), tt.maxLen)
			}
		})
	}
}

func TestEditDistanceWithin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"equal", "hello", "hello", true},
		{"empty", "", "hello", true},
		{"insert", "hello", "helllo", true},
		{"substitute", "hello", "jello", true},
		{"unrelated", randomText(1, 100_000, 1), randomText(2, 100_000, 1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, editDistanceWithin([]rune(tt.a), []rune(tt.b)))
		})
	}
}
