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

package pretty

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type test struct {
	expected  string
	formatter Formatter
}

func testFormatter(t *testing.T, tests []test) {
	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.formatter.String())
		})
	}
}

func TestIndentFormatter(t *testing.T) {
	t.Parallel()
	testFormatter(t, []test{
		{
			">>123",
			indent{
				prefix: ">>",
				inner:  FromString("123"),
			},
		},
		{
			">>123\n>>456",
			indent{
				prefix: ">>",
				inner:  FromString("123\n456"),
			},
		},
	})
}

func TestObjectFormatter(t *testing.T) {
	t.Parallel()

	testFormatter(t, []test{
		{
			"{ fizz: abc, hello: world }",
			Object{
				Properties: map[string]Formatter{
					"fizz":  FromString("abc"),
					"hello": FromString("world"),
				},
			},
		},
		{
			"{\n  fizz: abc,\n  hello: world,\n}",
			Object{
				Properties: map[string]Formatter{
					"fizz":  FromString("abc"),
					"hello": FromString("world"),
				},
			}.Columns(18),
		},
		{
			"{\n  aFoo: bar?,\n  bFizz: \n    buzz,\n}",
			Object{
				Properties: map[string]Formatter{
					"aFoo": Wrap{
						Postfix:         "?",
						PostfixSameline: true,
						Value:           FromString("bar"),
					},
					"bFizz": FromString("buzz"),
				},
			}.Columns(14),
		},
	})
}

func TestWrapFormatter(t *testing.T) {
	t.Parallel()

	testFormatter(t, []test{
		{
			"A(123456)",
			Wrap{
				Prefix:  "A(",
				Postfix: ")",
				Value:   FromString("123456"),
			}.Columns(10),
		},
		{
			"B(\n  123456\n)",
			Wrap{
				Prefix:  "B(",
				Postfix: ")",
				Value:   FromString("123456"),
			}.Columns(9),
		},
		{
			"C({\n  123456\n  123456\n})",
			Wrap{
				Prefix:  "C(",
				Postfix: ")",
				Value:   FromString("{\n  123456\n  123456\n}"),
			}.Columns(6),
		},
		{
			"foo-bar:\n  fizz-buzz,",
			Wrap{
				Prefix:          "foo-bar:",
				Postfix:         ",",
				PostfixSameline: true,
				Value:           FromString("fizz-buzz"),
			}.Columns(8),
		},
	})
}

func TestListFormatter(t *testing.T) {
	t.Parallel()

	commaList := List{
		AdjoinSeparator: true,
		Separator:       ", ",
		Elements: []Formatter{
			FromString("a"),
			FromString("b"),
			FromString("c"),
			FromString("d"),
		},
	}

	barList := List{
		Separator: " | ",
		Elements: []Formatter{
			FromString("a"),
			FromString("b"),
			FromString("c"),
			FromString("d"),
		},
	}

	testFormatter(t, []test{
		{"a, b, c, d", commaList},
		{"a,\nb,\nc,\nd", commaList.Columns(4)},
		{"a | b | c | d", barList},
		{"  a\n| b\n| c\n| d", barList.Columns(4)},
		{`  a
| b
| [
    1,
    2,
    3,
    4
  ]
| c`,
			List{
				Separator: " | ",
				Elements: []Formatter{
					FromString("a"),
					FromString("b"),
					Wrap{
						Prefix:  "[",
						Postfix: "]",
						Value: List{
							AdjoinSeparator: true,
							Separator:       ", ",
							Elements: []Formatter{
								FromString("1"),
								FromString("2"),
								FromString("3"),
								FromString("4"),
							},
						},
					},
					FromString("c"),
				},
			}.Columns(4),
		},
	})
}
