package gen

import (
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
)

func applyInputAnnotations(x model.Expression, isInput bool) model.Expression {
	if !isInput {
		return x
	}

	switch x := x.(type) {
	case *model.FunctionCallExpression:
		switch x.Name {
		case hcl2.IntrinsicConvert:
			x.Args[0] = applyInputAnnotations(x.Args[0], isInput)
		}
	case *model.LiteralValueExpression:
		t := x.Type()
		switch t := t.(type) {
		case *model.OpaqueType:
			t.Annotations = append(t.Annotations, hcl2.IntrinsicInput)
			x.SetType(t)
		}
	}

	return x
}
