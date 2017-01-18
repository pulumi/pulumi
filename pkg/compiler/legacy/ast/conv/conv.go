// Copyright 2016 Marapongo, Inc. All rights reserved.

// A collection of handy AST conversion helpers.
package conv

import (
	"github.com/marapongo/mu/pkg/compiler/legacy/ast"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Simple conversions to the direct underlying contents of literal nodes:

func ToAny(l ast.Literal) (interface{}, bool) {
	if t, ok := l.(ast.AnyLiteral); ok {
		return t.Any(), true
	}
	return nil, false
}

func ToBool(l ast.Literal) (bool, bool) {
	if t, ok := l.(ast.BoolLiteral); ok {
		return t.Bool(), true
	}
	return false, false
}

func ToNumber(l ast.Literal) (float64, bool) {
	if t, ok := l.(ast.NumberLiteral); ok {
		return t.Number(), true
	}
	return 0, false
}

func ToString(l ast.Literal) (string, bool) {
	if t, ok := l.(ast.StringLiteral); ok {
		return t.String(), true
	}
	return "", false
}

func ToService(l ast.Literal) (*ast.ServiceRef, bool) {
	if t, ok := l.(ast.ServiceLiteral); ok {
		return t.Service(), true
	}
	return nil, false
}

// More complex conversions that attempt to pluck out contents of complex data structures:

func ToServiceArray(l ast.Literal) ([]*ast.ServiceRef, bool) {
	if al, ok := l.(ast.ArrayLiteral); ok {
		arr := al.Array()
		typ := al.ElemType()
		// TODO: this isn't 100% correct; it should check for stack types also...
		if typ.Primitive != nil && *typ.Primitive == ast.PrimitiveTypeService {
			as := make([]*ast.ServiceRef, len(arr))
			for i, e := range arr {
				as[i] = e.(ast.ServiceLiteral).Service()
			}
			return as, true
		}
	}
	return nil, false
}

func ToStringArray(l ast.Literal) ([]string, bool) {
	if al, ok := l.(ast.ArrayLiteral); ok {
		arr := al.Array()
		typ := al.ElemType()
		if typ.Primitive != nil && *typ.Primitive == ast.PrimitiveTypeString {
			as := make([]string, len(arr))
			for i, e := range arr {
				as[i] = e.(ast.StringLiteral).String()
			}
			return as, true
		}
	}
	return nil, false
}

func ToStringMap(l ast.Literal) (map[string]interface{}, bool) {
	if ml, ok := l.(ast.MapLiteral); ok {
		kt := ml.KeyType()
		if kt.Primitive != nil && *kt.Primitive == ast.PrimitiveTypeString {
			mm := make(map[string]interface{})
			for i, k := range ml.Keys() {
				ks := k.(ast.StringLiteral).String()
				mm[ks] = ToValue(ml.Values()[i])
			}
			return mm, true
		}
	}
	return nil, false
}

func ToStringStringMap(l ast.Literal) (map[string]string, bool) {
	if ml, ok := l.(ast.MapLiteral); ok {
		kt := ml.KeyType()
		vt := ml.KeyType()
		if kt.Primitive != nil && *kt.Primitive == ast.PrimitiveTypeString &&
			vt.Primitive != nil && *vt.Primitive == ast.PrimitiveTypeString {
			mm := make(map[string]string)
			for i, k := range ml.Keys() {
				ks := k.(ast.StringLiteral).String()
				vs := ml.Values()[i].(ast.StringLiteral).String()
				mm[ks] = vs
			}
			return mm, true
		}
	}
	return nil, false
}

// ToValue is the hammer that transitively converts a Literal to its full-blown Go runtime representation.  Note that it
// does not attempt to strongly type any arrays or maps.  Those will need to be handled by the above helpers.
func ToValue(l ast.Literal) interface{} {
	switch t := l.(type) {
	case ast.AnyLiteral:
		return t.Any()
	case ast.BoolLiteral:
		return t.Bool()
	case ast.NumberLiteral:
		return t.Number()
	case ast.StringLiteral:
		return t.String()
	case ast.ServiceLiteral:
		return t.Service()
	case ast.ArrayLiteral:
		arr := t.Array()
		res := make([]interface{}, len(arr))
		for i, v := range arr {
			res[i] = ToValue(v)
		}
		return res
	case ast.MapLiteral:
		keys := t.Keys()
		vals := t.Values()
		// Map keys must be one of the primitive types.
		keyt := t.KeyType().Primitive
		contract.Assert(keyt != nil)
		switch *keyt {
		case ast.PrimitiveTypeBool:
			m := make(map[bool]interface{})
			for i, k := range keys {
				kb := ToValue(k).(bool)
				m[kb] = ToValue(vals[i])
			}
			return m
		case ast.PrimitiveTypeNumber:
			m := make(map[float64]interface{})
			for i, k := range keys {
				kn := ToValue(k).(float64)
				m[kn] = ToValue(vals[i])
			}
			return m
		case ast.PrimitiveTypeString:
			m := make(map[string]interface{})
			for i, k := range keys {
				ks := ToValue(k).(string)
				m[ks] = ToValue(vals[i])
			}
			return m
		default:
			contract.Failf("Unexpected map key type: %v", keyt)
			return nil
		}
	case ast.SchemaLiteral:
		p := make(map[string]interface{})
		for k, v := range t.Properties() {
			p[k] = ToValue(v)
		}
		return p
	default:
		contract.Failf("Unexpected literal type")
		return nil
	}
}
