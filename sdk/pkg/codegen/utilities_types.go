// Copyright 2021-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

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

func VisitType(schemaType schema.Type, visitor func(t schema.Type)) {
	seen := Set{}
	visitTypeClosure(schemaType, visitor, seen)
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

// MapInnerType applies f to the first non-wrapper type in t.
// MapInnerType does not mutate it's input, and t should not either.
func MapInnerType(t schema.Type, f func(schema.Type) schema.Type) schema.Type {
	switch t := t.(type) {
	case *schema.InputType:
		return &schema.InputType{ElementType: MapInnerType(t.ElementType, f)}
	case *schema.OptionalType:
		return &schema.OptionalType{ElementType: MapInnerType(t.ElementType, f)}
	case *schema.ArrayType:
		return &schema.ArrayType{ElementType: MapInnerType(t.ElementType, f)}
	case *schema.MapType:
		return &schema.MapType{ElementType: MapInnerType(t.ElementType, f)}
	default:
		return f(t)
	}
}

// Applies f to the first non-optional type in t.
// If t is Optional{v} then returns Optional{f(v)}, otherwise f(t) is returned
func MapOptionalType(t schema.Type, f func(schema.Type) schema.Type) schema.Type {
	if opt, ok := t.(*schema.OptionalType); ok {
		return &schema.OptionalType{ElementType: f(opt.ElementType)}
	}
	return f(t)
}

func IsNOptionalInput(t schema.Type) bool {
	for {
		switch typ := t.(type) {
		case *schema.InputType:
			return true
		case *schema.OptionalType:
			t = typ.ElementType
		default:
			return false
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

// If a helper function needs to be invoked to provide default values for a
// plain type. The provided map cannot be reused.
func IsProvideDefaultsFuncRequired(t schema.Type) bool {
	return isProvideDefaultsFuncRequiredHelper(t, map[string]bool{})
}

func isProvideDefaultsFuncRequiredHelper(t schema.Type, seen map[string]bool) bool {
	if seen[t.String()] {
		return false
	}
	seen[t.String()] = true
	t = UnwrapType(t)
	object, ok := t.(*schema.ObjectType)
	if !ok {
		return false
	}
	for _, p := range object.Properties {
		if p.DefaultValue != nil || isProvideDefaultsFuncRequiredHelper(p.Type, seen) {
			return true
		}
	}
	return false
}

// PackageReferences returns a list of packages that are referenced by the given package.
func PackageReferences(pkg *schema.Package) []schema.PackageReference {
	referencedPackages := map[string]schema.PackageReference{}
	visitor := func(t schema.Type) {
		if rt, ok := t.(*schema.ResourceType); ok && rt.Resource != nil {
			referencedPackageName := rt.Resource.PackageReference.Name()
			if referencedPackageName != pkg.Name {
				referencedPackages[referencedPackageName] = rt.Resource.PackageReference
			}
		}

		if objectType, ok := t.(*schema.ObjectType); ok {
			referencedPackageName := objectType.PackageReference.Name()
			if referencedPackageName != pkg.Name {
				referencedPackages[referencedPackageName] = objectType.PackageReference
			}
		}

		if et, ok := t.(*schema.EnumType); ok {
			referencedPackageName := et.PackageReference.Name()
			if referencedPackageName != pkg.Name {
				referencedPackages[referencedPackageName] = et.PackageReference
			}
		}
	}

	for _, resource := range pkg.Resources {
		VisitTypeClosure(resource.InputProperties, visitor)
		VisitTypeClosure(resource.Properties, visitor)
	}

	for _, function := range pkg.Functions {
		if function.Inputs != nil {
			VisitTypeClosure(function.Inputs.Properties, visitor)
		}
		if function.Outputs != nil {
			VisitTypeClosure(function.Outputs.Properties, visitor)
		}
	}

	for _, t := range pkg.Types {
		VisitType(t, visitor)
	}

	output := slice.Prealloc[schema.PackageReference](len(referencedPackages))
	for _, ref := range referencedPackages {
		output = append(output, ref)
	}
	return output
}
