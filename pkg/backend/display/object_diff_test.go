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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_decodeValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		repr     string
		kind     string
		expected property.Value
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
			expected: property.New([]property.Value{}),
		},
		{
			repr: "[\"foo\", \"bar\"]",
			kind: "json",
			expected: property.New([]property.Value{
				property.New("foo"),
				property.New("bar"),
			}),
		},
		{
			repr:     "{}",
			kind:     "json",
			expected: property.New(map[string]property.Value{}),
		},
		{
			repr: `{"foo": "bar"}`,
			kind: "json",
			expected: property.New(map[string]property.Value{
				"foo": property.New("bar"),
			}),
		},
		{
			repr: `  {"with": "whitespace"}  `,
			kind: "json",
			expected: property.New(map[string]property.Value{
				"with": property.New("whitespace"),
			}),
		},
		{
			repr: "- foo\n- bar",
			kind: "yaml",
			expected: property.New([]property.Value{
				property.New("foo"),
				property.New("bar"),
			}),
		},
		{
			repr: "foo: bar\nbaz: qux\n",
			kind: "yaml",
			expected: property.New(map[string]property.Value{
				"foo": property.New("bar"),
				"baz": property.New("qux"),
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.repr, func(t *testing.T) {
			t.Parallel()

			var printer propertyPrinter
			actual, kind, ok := printer.decodeValue(c.repr)

			if c.kind == "" {
				assert.Equal(t, property.Value{}, actual)
				assert.Equal(t, "", kind)
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				require.Equal(t, c.kind, kind)
				assert.True(t, c.expected.Equals(actual))
			}
		})
	}
}

func Test_PrintObject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		object     property.Map
		expected   string
		showSecret bool
	}{
		{
			"numbers",
			property.NewMap(map[string]property.Value{
				"int":         property.New(1.0),
				"float":       property.New(2.3),
				"large_int":   property.New(1234567.0),
				"large_float": property.New(1234567.1234567),
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
			property.NewMap(map[string]property.Value{
				"secret": property.New("secrets").WithSecret(true),
				"nested_secret": property.New(map[string]property.Value{
					"super_secret": property.New("super_secret").WithSecret(true),
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
			property.NewMap(map[string]property.Value{
				"secret": property.New("my_secret").WithSecret(true),
				"nested_secret": property.New(map[string]property.Value{
					"super_secret": property.New("my_super_secret").WithSecret(true),
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
			property.NewMap(map[string]property.Value{
				"bytes": property.New("\x00hello \x80\xfe\xff world\xf0\x28"),
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
			PrintObject(&buf, c.object,
				false, 0, deploy.OpSame, false, false, false, c.showSecret)
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
				Outputs: property.NewMap(map[string]property.Value{
					"banana": property.New("yummy"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"banana": property.New("yummy"),
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
				Outputs: property.NewMap(map[string]property.Value{
					"banana": property.New("yummy"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"banana": property.New("yummy"),
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
				Outputs: property.NewMap(map[string]property.Value{}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"someProp": property.New("added"),
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
				Outputs: property.NewMap(map[string]property.Value{
					"someProp": property.New("removed"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:     "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type:    "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{}),
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
				Outputs: property.NewMap(map[string]property.Value{
					"someProp": property.New("initial"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"someProp": property.New("changed"),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("shhhh").WithSecret(true),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("shhhh").WithSecret(true),
					"secretObj": property.New(map[string]property.Value{
						"a": property.New("1"),
					}).WithSecret(true),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("shhhh").WithSecret(true),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("shhhh").WithSecret(true),
					"secretObj": property.New(map[string]property.Value{
						"a": property.New("1"),
					}).WithSecret(true),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("initial").WithSecret(true),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("changed").WithSecret(true),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("initial").WithSecret(true),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("changed").WithSecret(true),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("initial").WithSecret(true),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("changed"),
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
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("initial"),
				}),
			},
			newState: engine.StepEventStateMetadata{
				URN:  "urn:pulumi:test::stack::pulumi:pulumi:Stack::test-stack",
				Type: "pulumi:pulumi:Stack",
				Outputs: property.NewMap(map[string]property.Value{
					"secret": property.New("changed").WithSecret(true),
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
				Outputs: property.NewMap(map[string]property.Value{
					"truncate_this":        property.New(strings.Repeat("a", 300)), // max 150 characters
					"dont_truncate_this":   property.New(strings.Repeat("a", 150)),
					"truncate_this_also":   property.New("1\n2\n3\n4\n5\n6"), // max 3 lines
					"dont_truncate_either": property.New("1\n2\n3\n"),
				}),
			},
			showSames:      false,
			truncateOutput: true,
			expected: "<{%fg 2%}>  + dont_truncate_either: <{%reset%}>" +
				"<{%fg 2%}>\"1\\n2\\n3\\n...\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>" +
				"<{%fg 2%}>  + dont_truncate_this  : <{%reset%}>" +
				//nolint:lll
				"<{%fg 2%}>\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>" +
				"<{%fg 2%}>  + truncate_this       : <{%reset%}>" +
				//nolint:lll
				"<{%fg 2%}>\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa...\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>" +
				"<{%fg 2%}>  + truncate_this_also  : <{%reset%}>" +
				"<{%fg 2%}>\"1\\n2\\n3\\n...\"<{%reset%}>" +
				"<{%fg 2%}>\n<{%reset%}>",
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
