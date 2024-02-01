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

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
	"github.com/stretchr/testify/assert"
)

type SimpleScalar string

func (s SimpleScalar) Range() *hcl.Range {
	return &hcl.Range{
		Start: hcl.Pos{},
		End:   hcl.Pos{Byte: len(s)},
	}
}

func (s SimpleScalar) Path() string {
	return "test"
}

func (s SimpleScalar) ScalarRange(start, end int) *hcl.Range {
	r := s.Range()
	return &hcl.Range{
		Start: hcl.Pos{Byte: r.Start.Byte + start},
		End:   hcl.Pos{Byte: r.Start.Byte + end},
	}
}

func mkInterp(text string, accessors ...PropertyAccessor) Interpolation {
	return Interpolation{
		Text:  text,
		Value: mkAccess(accessors...),
	}
}

func mkAccess(accessors ...PropertyAccessor) *PropertyAccess {
	if len(accessors) == 0 {
		return nil
	}
	return &PropertyAccess{Accessors: accessors}
}

func mkPropertyName(name string, start, end int) *PropertyName {
	return &PropertyName{
		Name: name,
		AccessorRange: &hcl.Range{
			Start: hcl.Pos{Byte: start},
			End:   hcl.Pos{Byte: end},
		},
	}
}

func mkPropertySubscript[T string | int](index T, start, end int) *PropertySubscript {
	return &PropertySubscript{
		Index: index,
		AccessorRange: &hcl.Range{
			Start: hcl.Pos{Byte: start},
			End:   hcl.Pos{Byte: end},
		},
	}
}

func TestInvalidInterpolations(t *testing.T) {
	cases := []struct {
		text  string
		parts []Interpolation
	}{
		{
			text:  "${foo",
			parts: []Interpolation{mkInterp("", mkPropertyName("foo", 2, 5))},
		},
		{
			text: "${foo ",
			parts: []Interpolation{
				mkInterp("", mkPropertyName("foo", 2, 5)),
				mkInterp(" "),
			},
		},
		{
			text: `${foo} ${["baz} bar`,
			parts: []Interpolation{
				mkInterp("", mkPropertyName("foo", 2, 5)),
				mkInterp(" ", mkPropertySubscript("baz} bar", 9, 19)),
			},
		},
		{
			text: `missing ${property[} subscript`,
			parts: []Interpolation{
				mkInterp("missing ", mkPropertyName("property", 10, 18), mkPropertySubscript("", 18, 19)),
				mkInterp(" subscript"),
			},
		},
		{
			text: `${[bar].baz}`,
			parts: []Interpolation{
				mkInterp("", mkPropertySubscript("bar", 2, 7), mkPropertyName("baz", 7, 11)),
			},
		},
		{
			text: `${foo.`,
			parts: []Interpolation{
				mkInterp("", mkPropertyName("foo", 2, 5), mkPropertyName("", 5, 6)),
			},
		},
		{
			text: `${foo[`,
			parts: []Interpolation{
				mkInterp("", mkPropertyName("foo", 2, 5), mkPropertySubscript("", 5, 6)),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			node := syntax.StringSyntax(SimpleScalar(c.text), c.text)
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
