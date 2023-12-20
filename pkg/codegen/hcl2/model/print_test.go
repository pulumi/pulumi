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
