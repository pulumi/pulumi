package gen

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

type jsonSpiller struct {
}

func (g *generator) rewriteToJSON(x model.Expression) (model.Expression, []*spillTemp, hcl.Diagnostics) {
	return g.rewriteSpills(x, func(x model.Expression) (string, model.Expression, bool) {
		if call, ok := x.(*model.FunctionCallExpression); ok && call.Name == "toJSON" {
			return "json", x, true
		}
		return "", nil, false
	})
}
