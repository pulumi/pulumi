// Copyright 2016-2020, Pulumi Corporation.
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

package hcl2

import (
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type packageSchema struct {
	schema    *schema.Package
	resources map[string]*schema.Resource
	functions map[string]*schema.Function
}

type PackageCache struct {
	m sync.RWMutex

	entries map[string]*packageSchema
}

func NewPackageCache() *PackageCache {
	return &PackageCache{
		entries: map[string]*packageSchema{},
	}
}

func (c *PackageCache) getPackageSchema(name string) (*packageSchema, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	schema, ok := c.entries[name]
	return schema, ok
}

// loadPackageSchema loads the schema for a given package by loading the corresponding provider and calling its
// GetSchema method.
//
// TODO: schema and provider versions
func (c *PackageCache) loadPackageSchema(loader schema.Loader, name string) (*packageSchema, error) {
	if s, ok := c.getPackageSchema(name); ok {
		return s, nil
	}

	version := (*semver.Version)(nil)
	pkg, err := loader.LoadPackage(name, version)
	if err != nil {
		return nil, err
	}

	resources := map[string]*schema.Resource{}
	for _, r := range pkg.Resources {
		resources[canonicalizeToken(r.Token, pkg)] = r
	}
	functions := map[string]*schema.Function{}
	for _, f := range pkg.Functions {
		functions[canonicalizeToken(f.Token, pkg)] = f
	}

	schema := &packageSchema{
		schema:    pkg,
		resources: resources,
		functions: functions,
	}

	c.m.Lock()
	defer c.m.Unlock()

	if s, ok := c.entries[name]; ok {
		return s, nil
	}
	c.entries[name] = schema

	return schema, nil
}

// canonicalizeToken converts a Pulumi token into its canonical "pkg:module:member" form.
func canonicalizeToken(tok string, pkg *schema.Package) string {
	_, _, member, _ := DecomposeToken(tok, hcl.Range{})
	return fmt.Sprintf("%s:%s:%s", pkg.Name, pkg.TokenToModule(tok), member)
}

// loadReferencedPackageSchemas loads the schemas for any pacakges referenced by a given node.
func (b *binder) loadReferencedPackageSchemas(n Node) error {
	// TODO: package versions
	packageNames := codegen.StringSet{}

	if r, ok := n.(*Resource); ok {
		token, tokenRange := getResourceToken(r)
		packageName, _, _, _ := DecomposeToken(token, tokenRange)
		if packageName != "pulumi" {
			packageNames.Add(packageName)
		}
	}

	diags := hclsyntax.VisitAll(n.SyntaxNode(), func(node hclsyntax.Node) hcl.Diagnostics {
		call, ok := node.(*hclsyntax.FunctionCallExpr)
		if !ok {
			return nil
		}
		token, tokenRange, ok := getInvokeToken(call)
		if !ok {
			return nil
		}
		packageName, _, _, _ := DecomposeToken(token, tokenRange)
		if packageName != "pulumi" {
			packageNames.Add(packageName)
		}
		return nil
	})
	contract.Assert(len(diags) == 0)

	for _, name := range packageNames.SortedValues() {
		if _, ok := b.referencedPackages[name]; ok {
			continue
		}
		pkg, err := b.options.packageCache.loadPackageSchema(b.options.loader, name)
		if err != nil {
			return err
		}
		b.referencedPackages[name] = pkg.schema
	}
	return nil
}

// schemaTypeToType converts a schema.Type to a model Type.
func (b *binder) schemaTypeToType(src schema.Type) (result model.Type) {
	switch src := src.(type) {
	case *schema.ArrayType:
		return model.NewListType(b.schemaTypeToType(src.ElementType))
	case *schema.MapType:
		return model.NewMapType(b.schemaTypeToType(src.ElementType))
	case *schema.EnumType:
		// TODO(codegen): make this a union of constant types.
		return b.schemaTypeToType(src.ElementType)
	case *schema.ObjectType:
		if t, ok := b.schemaTypes[src]; ok {
			return t
		}

		properties := map[string]model.Type{}
		objType := model.NewObjectType(properties, src)
		b.schemaTypes[src] = objType
		for _, prop := range src.Properties {
			typ := prop.Type
			if b.options.allowMissingProperties {
				typ = &schema.OptionalType{ElementType: typ}
			}

			t := b.schemaTypeToType(typ)
			if prop.ConstValue != nil {
				var value cty.Value
				switch v := prop.ConstValue.(type) {
				case bool:
					value = cty.BoolVal(v)
				case float64:
					value = cty.NumberFloatVal(v)
				case string:
					value = cty.StringVal(v)
				default:
					contract.Failf("unexpected constant type %T", v)
				}
				t = model.NewConstType(t, value)
			}
			properties[prop.Name] = t
		}
		return objType
	case *schema.TokenType:
		t, ok := model.GetOpaqueType(src.Token)
		if !ok {
			tt, err := model.NewOpaqueType(src.Token)
			contract.IgnoreError(err)
			t = tt
		}

		if src.UnderlyingType != nil {
			underlyingType := b.schemaTypeToType(src.UnderlyingType)
			return model.NewUnionType(t, underlyingType)
		}
		return t
	case *schema.InputType:
		elementType := b.schemaTypeToType(src.ElementType)
		resolvedElementType := b.schemaTypeToType(codegen.ResolvedType(src.ElementType))
		return model.NewUnionTypeAnnotated([]model.Type{elementType, model.NewOutputType(resolvedElementType)}, src)
	case *schema.OptionalType:
		elementType := b.schemaTypeToType(src.ElementType)
		return model.NewOptionalType(elementType)
	case *schema.UnionType:
		types := make([]model.Type, len(src.ElementTypes))
		for i, src := range src.ElementTypes {
			types[i] = b.schemaTypeToType(src)
		}
		if src.Discriminator != "" {
			return model.NewUnionTypeAnnotated(types, src)
		}
		return model.NewUnionType(types...)
	default:
		switch src {
		case schema.BoolType:
			return model.BoolType
		case schema.IntType:
			return model.IntType
		case schema.NumberType:
			return model.NumberType
		case schema.StringType:
			return model.StringType
		case schema.ArchiveType:
			return ArchiveType
		case schema.AssetType:
			return AssetType
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return model.DynamicType
		default:
			return model.NoneType
		}
	}
}

var schemaArrayTypes = make(map[schema.Type]*schema.ArrayType)

// GetSchemaForType extracts the schema.Type associated with a model.Type, if any.
//
// The result may be a *schema.UnionType if multiple schema types are associated with the input type.
func GetSchemaForType(t model.Type) (schema.Type, bool) {
	switch t := t.(type) {
	case *model.ListType:
		element, ok := GetSchemaForType(t.ElementType)
		if !ok {
			return nil, false
		}
		if t, ok := schemaArrayTypes[element]; ok {
			return t, true
		}
		schemaArrayTypes[element] = &schema.ArrayType{ElementType: element}
		return schemaArrayTypes[element], true
	case *model.ObjectType:
		if len(t.Annotations) == 0 {
			return nil, false
		}
		for _, a := range t.Annotations {
			if t, ok := a.(schema.Type); ok {
				return t, true
			}
		}
		return nil, false
	case *model.OutputType:
		return GetSchemaForType(t.ElementType)
	case *model.PromiseType:
		return GetSchemaForType(t.ElementType)
	case *model.UnionType:
		for _, a := range t.Annotations {
			switch a := a.(type) {
			case *schema.UnionType:
				return a, true
			case *schema.InputType:
				return a, true
			}
		}
		schemas := codegen.Set{}
		for _, t := range t.ElementTypes {
			if s, ok := GetSchemaForType(t); ok {
				if union, ok := s.(*schema.UnionType); ok {
					for _, s := range union.ElementTypes {
						schemas.Add(s)
					}
				} else {
					schemas.Add(s)
				}
			}
		}
		if len(schemas) == 0 {
			return nil, false
		}
		schemaTypes := make([]schema.Type, 0, len(schemas))
		for t := range schemas {
			schemaTypes = append(schemaTypes, t.(schema.Type))
		}
		if len(schemaTypes) == 1 {
			return schemaTypes[0], true
		}
		return &schema.UnionType{ElementTypes: schemaTypes}, true
	default:
		return nil, false
	}
}

// GetDiscriminatedUnionObjectMapping calculates a map of type names to object types for a given
// union type.
func GetDiscriminatedUnionObjectMapping(t *model.UnionType) map[string]model.Type {
	mapping := map[string]model.Type{}
	for _, t := range t.ElementTypes {
		k, v := getDiscriminatedUnionObjectItem(t)
		mapping[k] = v
	}
	return mapping
}

func getDiscriminatedUnionObjectItem(t model.Type) (string, model.Type) {
	switch t := t.(type) {
	case *model.ListType:
		return getDiscriminatedUnionObjectItem(t.ElementType)
	case *model.ObjectType:
		if schemaType, ok := GetSchemaForType(t); ok {
			if objType, ok := schemaType.(*schema.ObjectType); ok {
				return objType.Token, t
			}
		}
	case *model.OutputType:
		return getDiscriminatedUnionObjectItem(t.ElementType)
	case *model.PromiseType:
		return getDiscriminatedUnionObjectItem(t.ElementType)
	}
	return "", nil
}
