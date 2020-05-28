package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
)

type ternaryTemp struct {
	Name  string
	Value *model.ConditionalExpression
}

func (tt *ternaryTemp) Type() model.Type {
	return tt.Value.Type()
}

func (tt *ternaryTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return tt.Type().Traverse(traverser)
}

func (tt *ternaryTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type tempSpiller struct {
	temps []*ternaryTemp
}

func (ta *tempSpiller) allocateExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	var temp *ternaryTemp
	switch x := x.(type) {
	case *model.ConditionalExpression:
		x.Condition, _ = ta.allocateExpression(x.Condition)
		x.TrueResult, _ = ta.allocateExpression(x.TrueResult)
		x.FalseResult, _ = ta.allocateExpression(x.FalseResult)

		temp = &ternaryTemp{
			Name:  fmt.Sprintf("tmp%d", len(ta.temps)),
			Value: x,
		}
		ta.temps = append(ta.temps, temp)
	default:
		return x, nil
	}
	return &model.ScopeTraversalExpression{
		RootName:  temp.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteTernaries(x model.Expression) (model.Expression, []*ternaryTemp, hcl.Diagnostics) {
	allocator := &tempSpiller{}
	x, diags := model.VisitExpression(x, allocator.allocateExpression, nil)

	return x, allocator.temps, diags

}
