package gen

import (
	"bytes"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/stretchr/testify/assert"
)

type exprTestCase struct {
	hcl2Expr string
	goCode   string
}

func TestLiteralExpression(t *testing.T) {
	cases := []exprTestCase{
		{hcl2Expr: "false", goCode: "false"},
		{hcl2Expr: "true", goCode: "true"},
		{hcl2Expr: "0", goCode: "0"},
		{hcl2Expr: "3.14", goCode: "3.14"},
		{hcl2Expr: "\"foo\"", goCode: `"foo"`},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode)
	}
}

func testGenerateExpression(t *testing.T, hcl2Expr, goCode string) {
	// test program is only for schema info
	g := newTestGenerator(t, "aws-s3-logging.pp")
	var index bytes.Buffer
	expr, _ := model.BindExpressionText(hcl2Expr, nil, hcl.Pos{})
	g.Fgenf(&index, "%v", expr)
	assert.Equal(t, goCode, index.String())
}
