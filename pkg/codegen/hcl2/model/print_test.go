// Copyright 2020-2024, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
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
