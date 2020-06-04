package gen

import (
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
)

func applyInputAnnotations(x model.Expression, isInput bool) model.Expression {
	if !isInput {
		return x
	}
	return modifyInputAnnotations(x, applyInput)
}

func stripInputAnnotations(x model.Expression) model.Expression {
	return modifyInputAnnotations(x, stripInput)
}

func stripInput(annotations []interface{}) []interface{} {
	var stripped []interface{}
	for _, a := range annotations {
		if a != hcl2.IntrinsicInput {
			stripped = append(stripped, a)
		}
	}
	return stripped
}

func applyInput(annotations []interface{}) []interface{} {
	return append(annotations, hcl2.IntrinsicInput)
}

func modifyInputAnnotations(
	x model.Expression,
	modf func([]interface{}) []interface{},
) model.Expression {
	switch x := x.(type) {
	case *model.AnonymousFunctionExpression:
		switch rt := x.Signature.ReturnType.(type) {
		case *model.OpaqueType:
			rt.Annotations = modf(rt.Annotations)
		}
	case *model.FunctionCallExpression:
		for i, arg := range x.Args {
			x.Args[i] = modifyInputAnnotations(arg, modf)
		}
		switch x.Name {
		// for __convert calls we rely on an opaqueType to be present in the union return type
		case hcl2.IntrinsicConvert:
			switch rt := x.Signature.ReturnType.(type) {
			case *model.UnionType:
				for _, t := range rt.ElementTypes {
					switch t := t.(type) {
					case *model.OpaqueType:
						t.Annotations = modf(t.Annotations)
					}
				}
			}
		}
	case *model.TemplateExpression:
		if len(x.Parts) == 1 {
			if lit, ok := x.Parts[0].(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
				x.Parts[0] = modifyInputAnnotations(x.Parts[0], modf)
			}
		}
	case *model.LiteralValueExpression:
		t := x.Type()
		switch t := t.(type) {
		case *model.OpaqueType:
			t.Annotations = modf(t.Annotations)
		}
	case *model.ObjectConsExpression:
		for _, item := range x.Items {
			item.Key = modifyInputAnnotations(item.Key, modf)
			item.Value = modifyInputAnnotations(item.Value, modf)
		}
	case *model.TupleConsExpression:
		for i, item := range x.Expressions {
			x.Expressions[i] = modifyInputAnnotations(item, modf)
		}
	}

	return x
}
