package codegen

import "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

func visitTypeClosure(t schema.Type, visitor func(t schema.Type), seen Set) {
	if seen.Has(t) {
		return
	}
	seen.Add(t)

	visitor(t)

	switch st := t.(type) {
	case *schema.ArrayType:
		visitTypeClosure(st.ElementType, visitor, seen)
	case *schema.MapType:
		visitTypeClosure(st.ElementType, visitor, seen)
	case *schema.ObjectType:
		for _, p := range st.Properties {
			visitTypeClosure(p.Type, visitor, seen)
		}
	case *schema.UnionType:
		for _, e := range st.ElementTypes {
			visitTypeClosure(e, visitor, seen)
		}
	case *schema.InputType:
		visitTypeClosure(st.ElementType, visitor, seen)
	case *schema.OptionalType:
		visitTypeClosure(st.ElementType, visitor, seen)
	}
}

func VisitTypeClosure(properties []*schema.Property, visitor func(t schema.Type)) {
	seen := Set{}
	for _, p := range properties {
		visitTypeClosure(p.Type, visitor, seen)
	}
}

func SimplifyInputUnion(t schema.Type) schema.Type {
	union, ok := t.(*schema.UnionType)
	if !ok {
		return t
	}

	elements := make([]schema.Type, len(union.ElementTypes))
	for i, et := range union.ElementTypes {
		if input, ok := et.(*schema.InputType); ok {
			elements[i] = input.ElementType
		} else {
			elements[i] = et
		}
	}
	return &schema.UnionType{
		ElementTypes:  elements,
		DefaultType:   union.DefaultType,
		Discriminator: union.Discriminator,
		Mapping:       union.Mapping,
	}
}

// RequiredType unwraps the OptionalType enclosing the Property's type, if any.
func RequiredType(p *schema.Property) schema.Type {
	if optional, ok := p.Type.(*schema.OptionalType); ok {
		return optional.ElementType
	}
	return p.Type
}

// OptionalType wraps the Property's type in an OptionalType if it is not already optional.
func OptionalType(p *schema.Property) schema.Type {
	if optional, ok := p.Type.(*schema.OptionalType); ok {
		return optional
	}
	return &schema.OptionalType{ElementType: p.Type}
}

// UnwrapType removes any outer OptionalTypes and InputTypes from t.
func UnwrapType(t schema.Type) schema.Type {
	for {
		switch typ := t.(type) {
		case *schema.InputType:
			t = typ.ElementType
		case *schema.OptionalType:
			t = typ.ElementType
		default:
			return t
		}
	}
}

func resolvedType(t schema.Type, plainObjects bool) schema.Type {
	switch typ := t.(type) {
	case *schema.InputType:
		return resolvedType(typ.ElementType, plainObjects)
	case *schema.OptionalType:
		e := resolvedType(typ.ElementType, plainObjects)
		if e == typ.ElementType {
			return typ
		}
		return &schema.OptionalType{ElementType: e}
	case *schema.ArrayType:
		e := resolvedType(typ.ElementType, plainObjects)
		if e == typ.ElementType {
			return typ
		}
		return &schema.ArrayType{ElementType: e}
	case *schema.MapType:
		e := resolvedType(typ.ElementType, plainObjects)
		if e == typ.ElementType {
			return typ
		}
		return &schema.MapType{ElementType: e}
	case *schema.ObjectType:
		if !plainObjects || !typ.IsInputShape() {
			return typ
		}
		return typ.PlainShape
	case *schema.UnionType:
		elems, changed := make([]schema.Type, len(typ.ElementTypes)), false
		for i, e := range typ.ElementTypes {
			elems[i] = resolvedType(e, plainObjects)
			changed = changed || elems[i] != e
		}
		if !changed {
			return typ
		}
		return &schema.UnionType{
			ElementTypes:  elems,
			DefaultType:   typ.DefaultType,
			Discriminator: typ.Discriminator,
			Mapping:       typ.Mapping,
		}
	default:
		return t
	}
}

// PlainType deeply removes any InputTypes from t, with the exception of argument structs. Use ResolvedType to
// unwrap argument structs as well.
func PlainType(t schema.Type) schema.Type {
	return resolvedType(t, false)
}

// ResolvedType deeply removes any InputTypes from t.
func ResolvedType(t schema.Type) schema.Type {
	return resolvedType(t, true)
}
