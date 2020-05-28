package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
)

type ternaryTemp struct {
	Name         string
	Condition    model.Expression
	TrueResult   model.Expression
	FalseResult  model.Expression
	VariableType model.Type
}

func (tt *ternaryTemp) Type() model.Type {
	return tt.VariableType
}

func (tt *ternaryTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return tt.VariableType.Traverse(traverser)
}

func (tt *ternaryTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type tempAllocator struct {
	temps []*ternaryTemp
}

func (ta *tempAllocator) allocateExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	var temp *ternaryTemp
	switch x := x.(type) {
	case *model.ConditionalExpression:
		cond, _ := ta.allocateExpression(x.Condition)
		t, _ := ta.allocateExpression(x.TrueResult)
		f, _ := ta.allocateExpression(x.FalseResult)
		temp = &ternaryTemp{
			Name:         fmt.Sprintf("tmp%d", len(ta.temps)),
			VariableType: x.Type(),
			Condition:    cond,
			TrueResult:   t,
			FalseResult:  f,
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
	allocator := &tempAllocator{}
	x, diags := model.VisitExpression(x, allocator.allocateExpression, nil)

	return x, allocator.temps, diags

}
