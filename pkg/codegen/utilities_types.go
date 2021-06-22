package codegen

import "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

type Type struct {
	schema.Type

	Plain    bool
	Optional bool
}

func visitTypeClosure(t Type, visitor func(t Type), seen Set) {
	if seen.Has(t) {
		return
	}
	seen.Add(t)

	visitor(t)

	switch st := t.Type.(type) {
	case *schema.ArrayType:
		visitTypeClosure(Type{st.ElementType, t.Plain, t.Optional}, visitor, seen)
	case *schema.MapType:
		visitTypeClosure(Type{st.ElementType, t.Plain, t.Optional}, visitor, seen)
	case *schema.ObjectType:
		visitPropertyTypeClosure(t, st.Properties, visitor, seen)
	case *schema.UnionType:
		for _, e := range st.ElementTypes {
			visitTypeClosure(Type{e, t.Plain, t.Optional}, visitor, seen)
		}
	}
}

func visitPropertyTypeClosure(root Type, properties []*schema.Property, visitor func(t Type), seen Set) {
	for _, p := range properties {
		visitTypeClosure(Type{
			Type:     p.Type,
			Plain:    root.Plain || p.IsPlain,
			Optional: !p.IsRequired,
		}, visitor, seen)
	}
}

func VisitTypeClosure(properties []*schema.Property, visitor func(t Type)) {
	visitPropertyTypeClosure(Type{}, properties, visitor, Set{})
}
