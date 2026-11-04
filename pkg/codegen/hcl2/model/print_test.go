// Copyright 2020, Pulumi Corporation.
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

package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

func TestPrintNoTokens(t *testing.T) {
	t.Parallel()

	b := &Block{
		Type: "block", Body: &Body{
			Items: []BodyItem{
				&Attribute{
					Name: "attribute",
					Value: &LiteralValueExpression{
						Value: cty.True,
					},
				},
				&Attribute{
					Name: "literal",
					Value: &TemplateExpression{
						Parts: []Expression{
							&LiteralValueExpression{
								Value: cty.StringVal("foo${bar} %{"),
							},
							&LiteralValueExpression{
								Value: cty.StringVal("$"),
							},
							&LiteralValueExpression{
								Value: cty.StringVal("%{"),
							},
						},
					},
				},
			},
		},
	}
	expected := `block {
    attribute = true
    literal = "foo$${bar} %%{$%%{"
}`
	assert.Equal(t, expected, fmt.Sprintf("%v", b))
}

func TestPrettyPrintingNoneType(t *testing.T) {
	t.Parallel()
	pretty := NoneType.Pretty().String()
	assert.Equal(t, "none", pretty)
}

func TestPrettyPrintingDynamicType(t *testing.T) {
	t.Parallel()
	pretty := DynamicType.Pretty().String()
	assert.Equal(t, "dynamic", pretty)
}

func TestPrintGeneratedComments(t *testing.T) {
	t.Parallel()

	literal := func(trailing ...syntax.Trivia) *LiteralValueExpression {
		tokens := syntax.NewLiteralValueTokens(cty.NumberIntVal(1))
		tokens.Value[0].LeadingTrivia = syntax.TriviaList{syntax.NewWhitespace(' ')}
		tokens.Value[0].TrailingTrivia = trailing
		return &LiteralValueExpression{Tokens: tokens, Value: cty.NumberIntVal(1)}
	}
	// attribute builds an attribute with explicit whitespace, in the style of a PCL formatter.
	attribute := func(name string, leading syntax.TriviaList, trailing ...syntax.Trivia) *Attribute {
		tokens := syntax.NewAttributeTokens(name)
		tokens.Name.LeadingTrivia = leading
		return &Attribute{Tokens: tokens, Name: name, Value: literal(trailing...)}
	}
	tab := syntax.NewWhitespace('\t')
	space := syntax.NewWhitespace(' ')

	blockTokens := syntax.NewBlockTokens("resource", "r", "pkg:index:Res")
	blockTokens.Type.LeadingTrivia = syntax.TriviaList{syntax.Comment{Lines: []string{" block", " comment"}}}
	blockTokens.OpenBrace.TrailingTrivia = syntax.TriviaList{syntax.NewWhitespace('\n')}
	blockTokens.CloseBrace.LeadingTrivia = nil
	blockTokens.CloseBrace.TrailingTrivia = syntax.TriviaList{space, syntax.Comment{Lines: []string{" after", " block"}}}

	body := &Body{Items: []BodyItem{
		&Block{
			Tokens: blockTokens,
			Type:   "resource",
			Labels: []string{"r", "pkg:index:Res"},
			Body: &Body{Items: []BodyItem{
				attribute("leading", syntax.TriviaList{tab, syntax.Comment{Lines: []string{" one", " two"}}}),
				attribute("trailing", syntax.TriviaList{tab}, space, syntax.Comment{Lines: []string{" one"}}),
				attribute("moved", syntax.TriviaList{tab}, space, syntax.Comment{Lines: []string{" one", " two"}}),
				&Attribute{Name: "noTokens", Value: literal(space, syntax.Comment{Lines: []string{" one", " two"}})},
				attribute("twoComments", syntax.TriviaList{
					tab, syntax.Comment{Lines: []string{" one"}}, syntax.Comment{Lines: []string{" two", " three"}},
				}),
				attribute("leadingAndMoved", syntax.TriviaList{tab, syntax.Comment{Lines: []string{" one"}}},
					space, syntax.Comment{Lines: []string{" two", " three"}}),
			}},
		},
	}}

	expected := "// block\n" +
		"// comment\n" +
		"// after\n" +
		"// block\n" +
		"resource r \"pkg:index:Res\" {\n" +
		"\t// one\n" +
		"\t// two\n" +
		"\tleading = 1\n" +
		"\ttrailing = 1 // one\n" +
		"\t// one\n" +
		"\t// two\n" +
		"\tmoved = 1\n" +
		"    // one\n" +
		"    // two\n" +
		"    noTokens = 1\n" +
		"\t// one\n" +
		"\t// two\n" +
		"\t// three\n" +
		"\ttwoComments = 1\n" +
		"\t// one\n" +
		"\t// two\n" +
		"\t// three\n" +
		"\tleadingAndMoved = 1\n" +
		"}\n"
	assert.Equal(t, expected, fmt.Sprintf("%v", body))

	parser := syntax.NewParser()
	require.NoError(t, parser.ParseFile(strings.NewReader(expected), "main.pp"))
	assert.Empty(t, parser.Diagnostics)
}
