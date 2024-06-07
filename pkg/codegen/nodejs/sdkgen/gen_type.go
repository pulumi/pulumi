package sdkgen

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs/codebase"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func (g *generator) generateType(m *codebase.Module, t schema.Type) (codebase.Type, error) {
	switch t := t.(type) {
	case *schema.ArrayType:
		elemType, err := g.generateType(m, t.ElementType)
		if err != nil {
			return codebase.NeverT, err
		}

		return codebase.ArrayT(elemType), nil
	case *schema.InputType:
		elemType, err := g.generateType(m, t.ElementType)
		if err != nil {
			return codebase.NeverT, err
		}

		return InputT(m, elemType), nil
	case *schema.MapType:
		elemType, err := g.generateType(m, t.ElementType)
		if err != nil {
			return codebase.NeverT, err
		}

		return codebase.RecordT(codebase.StringT, elemType), nil
	case *schema.OptionalType:
		elemType, err := g.generateType(m, t.ElementType)
		if err != nil {
			return codebase.NeverT, err
		}

		return codebase.UnionT(elemType, codebase.UndefinedT), nil
	case *schema.ResourceType:
		syms, err := g.makeResourceSymbols(t.Resource)
		if err != nil {
			return codebase.NeverT, err
		}

		return syms.Type.AsType(), nil
	case *schema.TokenType:
		syms, err := g.makePlainTypeSymbols(t.Token)
		if err != nil {
			return codebase.NeverT, err
		}

		return syms.Type.AsType(), nil
	default:
		switch t {
		case schema.BoolType:
			return codebase.BooleanT, nil
		case schema.IntType, schema.NumberType:
			return codebase.NumberT, nil
		case schema.StringType:
			return codebase.StringT, nil
		default:
			return codebase.NeverT, nil
		}
	}
}
