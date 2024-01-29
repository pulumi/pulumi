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

package ast

import (
	"testing"

	"github.com/pulumi/esc/syntax"
	"github.com/stretchr/testify/assert"
)

func TestInvalidInterpolations(t *testing.T) {
	cases := []struct {
		text  string
		parts []Interpolation
	}{
		{
			text: "${foo",
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "foo"},
						},
					},
				},
			},
		},
		{
			text: "${foo ",
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "foo"},
						},
					},
				},
				{
					Text: " ",
				},
			},
		},
		{
			text: `${foo} ${["baz} bar`,
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "foo"},
						},
					},
				},
				{
					Text: " ",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertySubscript{Index: "baz} bar"},
						},
					},
				},
			},
		},
		{
			text: `missing ${property[} subscript`,
			parts: []Interpolation{
				{
					Text: "missing ",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "property"},
						},
					},
				},
				{
					Text: " subscript",
				},
			},
		},
		{
			text: `${[bar].baz}`,
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertySubscript{Index: 0},
							&PropertyName{Name: "baz"},
						},
					},
				},
			},
		},
		{
			text: `${foo.`,
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "foo"},
						},
					},
				},
			},
		},
		{
			text: `${foo[`,
			parts: []Interpolation{
				{
					Text: "",
					Value: &PropertyAccess{
						Accessors: []PropertyAccessor{
							&PropertyName{Name: "foo"},
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			node := syntax.String(c.text)
			parts, diags := parseInterpolate(node, c.text)
			assert.NotEmpty(t, diags)
			assert.Equal(t, c.parts, parts)
		})
	}
}

func TestEscapeInterpolationWorks(t *testing.T) {
	t.Parallel()
	node := syntax.String("Hello $${world}!")
	parts, diags := parseInterpolate(node, node.Value())
	assert.Empty(t, diags)
	assert.Len(t, parts, 1, "Expected one interpolation part")
	assert.Equal(t, "Hello ${world}!", parts[0].Text)
}
