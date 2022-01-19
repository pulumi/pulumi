package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

type jsonTemp struct {
	Name  string
	Value *model.FunctionCallExpression
}

func (jt *jsonTemp) Type() model.Type {
	return jt.Value.Type()
}

func (jt *jsonTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return jt.Type().Traverse(traverser)
}

func (jt *jsonTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type jsonSpiller struct {
	temps []*jsonTemp
	count int
}

func (js *jsonSpiller) spillExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	var temp *jsonTemp
	switch x := x.(type) {
	case *model.FunctionCallExpression:
		switch x.Name {
		case "toJSON":
			temp = &jsonTemp{
				Name:  fmt.Sprintf("json%d", js.count),
				Value: x,
			}
			js.temps = append(js.temps, temp)
			js.count++
		default:
			return x, nil
		}
	default:
		return x, nil
	}
	return &model.ScopeTraversalExpression{
		RootName:  temp.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteToJSON(x model.Expression) (model.Expression, []*spillTemp, hcl.Diagnostics) {
	return g.rewriteSpills(x, func(x model.Expression) (string, model.Expression, bool) {
		if call, ok := x.(*model.FunctionCallExpression); ok && call.Name == "toJSON" {
			return "json", x, true
		}
		return "", nil, false
	})
}
