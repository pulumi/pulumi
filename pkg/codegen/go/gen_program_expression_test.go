package gen

import (
	"bytes"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/stretchr/testify/assert"
)

type exprTestCase struct {
	hcl2Expr string
	goCode   string
}

type environment map[string]interface{}

func (e environment) scope() *model.Scope {
	s := model.NewRootScope(syntax.None)
	for name, typeOrFunction := range e {
		switch typeOrFunction := typeOrFunction.(type) {
		case *model.Function:
			s.DefineFunction(name, typeOrFunction)
		case model.Type:
			s.Define(name, &model.Variable{Name: name, VariableType: typeOrFunction})
		}
	}
	return s
}

func TestLiteralExpression(t *testing.T) {
	cases := []exprTestCase{
		{hcl2Expr: "false", goCode: "false"},
		{hcl2Expr: "true", goCode: "true"},
		{hcl2Expr: "0", goCode: "0"},
		{hcl2Expr: "3.14", goCode: "3.14"},
		{hcl2Expr: "\"foo\"", goCode: "\"foo\""},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, nil)
	}
}

func TestBinaryOpExpression(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": model.BoolType,
		"b": model.BoolType,
		"c": model.NumberType,
		"d": model.NumberType,
	})
	scope := env.scope()

	cases := []exprTestCase{
		{hcl2Expr: "0 == 0", goCode: "0 == 0"},
		{hcl2Expr: "0 != 0", goCode: "0 != 0"},
		{hcl2Expr: "0 < 0", goCode: "0 < 0"},
		{hcl2Expr: "0 > 0", goCode: "0 > 0"},
		{hcl2Expr: "0 <= 0", goCode: "0 <= 0"},
		{hcl2Expr: "0 >= 0", goCode: "0 >= 0"},
		{hcl2Expr: "0 + 0", goCode: "0 + 0"},
		{hcl2Expr: "0 * 0", goCode: "0 * 0"},
		{hcl2Expr: "0 / 0", goCode: "0 / 0"},
		{hcl2Expr: "0 % 0", goCode: "0 % 0"},
		{hcl2Expr: "false && false", goCode: "false && false"},
		{hcl2Expr: "false || false", goCode: "false || false"},
		{hcl2Expr: "a == true", goCode: "a == true"},
		{hcl2Expr: "b == true", goCode: "b == true"},
		{hcl2Expr: "c + 0", goCode: "c + 0"},
		{hcl2Expr: "d + 0", goCode: "d + 0"},
		{hcl2Expr: "a && true", goCode: "a && true"},
		{hcl2Expr: "b && true", goCode: "b && true"},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope)
	}
}

func TestUnaryOpExrepssion(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": model.NumberType,
		"b": model.BoolType,
	})
	scope := env.scope()

	cases := []exprTestCase{
		{hcl2Expr: "-1", goCode: "-1"},
		{hcl2Expr: "!true", goCode: "!true"},
		{hcl2Expr: "-a", goCode: "-a"},
		{hcl2Expr: "!b", goCode: "!b"},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope)
	}
}

func testGenerateExpression(t *testing.T, hcl2Expr, goCode string, scope *model.Scope) {
	t.Run(hcl2Expr, func(t *testing.T) {
		// test program is only for schema info
		g := newTestGenerator(t, "aws-s3-logging.pp")
		var index bytes.Buffer
		expr, _ := model.BindExpressionText(hcl2Expr, scope, hcl.Pos{})
		g.Fgenf(&index, "%v", expr)
		assert.Equal(t, goCode, index.String())
	})
}
