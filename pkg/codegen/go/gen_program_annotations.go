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
		for _, arg := range x.Args {
			applyInputAnnotations(arg, isInput)
		}
		switch x.Name {
		// for __convert calls we rely on an opaqueType to be present in the union return type
		case hcl2.IntrinsicConvert:
			switch rt := x.Signature.ReturnType.(type) {
			case *model.UnionType:
				for _, t := range rt.ElementTypes {
					switch t := t.(type) {
					case *model.OpaqueType:
						t.Annotations = append(t.Annotations, hcl2.IntrinsicInput)
					}
				}
			}
		}
	case *model.LiteralValueExpression:
		t := x.Type()
		switch t := t.(type) {
		case *model.OpaqueType:
			t.Annotations = append(t.Annotations, hcl2.IntrinsicInput)
		}
	}

	return x
}
