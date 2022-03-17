package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

type readDirTemp struct {
	Name  string
	Value *model.FunctionCallExpression
}

func (rt *readDirTemp) Type() model.Type {
	return rt.Value.Type()
}

func (rt *readDirTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return rt.Type().Traverse(traverser)
}

func (rt *readDirTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type readDirSpiller struct {
	temps []*readDirTemp
	count int
}

func (rs *readDirSpiller) spillExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	var temp *readDirTemp
	scopeName := ""
	switch x := x.(type) {
	case *model.FunctionCallExpression:
		switch x.Name {
		case "readDir":
			scopeName = fmt.Sprintf("fileNames%d", rs.count)
			temp = &readDirTemp{
				Name:  fmt.Sprintf("files%d", rs.count),
				Value: x,
			}
			rs.temps = append(rs.temps, temp)
			rs.count++
		default:
			return x, nil
		}
	default:
		return x, nil
	}
	return &model.ScopeTraversalExpression{
		RootName:  scopeName,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteReadDir(
	x model.Expression,
	spiller *readDirSpiller,
) (model.Expression, []*readDirTemp, hcl.Diagnostics) {
	spiller.temps = nil
	x, diags := model.VisitExpression(x, spiller.spillExpression, nil)

	return x, spiller.temps, diags

}
