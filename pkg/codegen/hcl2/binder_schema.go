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
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type packageSchema struct {
	schema    *schema.Package
	resources map[string]*schema.Resource
	functions map[string]*schema.Function
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
		if err := b.loadPackageSchema(name); err != nil {
			return err
		}
	}
	return nil
}

// loadPackageSchema loads the schema for a given package by loading the corresponding provider and calling its
// GetSchema method.
//
// TODO: schema and provider versions
func (b *binder) loadPackageSchema(name string) error {
	if _, ok := b.packageSchemas[name]; ok {
		return nil
	}

	provider, err := b.host.Provider(tokens.Package(name), nil)
	if err != nil {
		return err
	}

	schemaBytes, err := provider.GetSchema(0)
	if err != nil {
		return err
	}

	var spec schema.PackageSpec
	if err := json.Unmarshal(schemaBytes, &spec); err != nil {
		return err
	}

	pkg, err := schema.ImportSpec(spec)
	if err != nil {
		return err
	}

	resources := map[string]*schema.Resource{}
	for _, r := range pkg.Resources {
		resources[canonicalizeToken(r.Token, pkg)] = r
	}
	functions := map[string]*schema.Function{}
	for _, f := range pkg.Functions {
		functions[canonicalizeToken(f.Token, pkg)] = f
	}

	b.packageSchemas[name] = &packageSchema{
		schema:    pkg,
		resources: resources,
		functions: functions,
	}
	return nil
}

// schemaTypeToType converts a schema.Type to a model Type.
func schemaTypeToType(src schema.Type) model.Type {
	switch src := src.(type) {
	case *schema.ArrayType:
		return model.NewListType(schemaTypeToType(src.ElementType))
	case *schema.MapType:
		return model.NewMapType(schemaTypeToType(src.ElementType))
	case *schema.ObjectType:
		properties := map[string]model.Type{}
		for _, prop := range src.Properties {
			t := schemaTypeToType(prop.Type)
			if !prop.IsRequired {
				t = model.NewOptionalType(t)
			}
			properties[prop.Name] = t
		}
		return model.NewObjectType(properties)
	case *schema.TokenType:
		t, ok := model.GetOpaqueType(src.Token)
		if !ok {
			tt, err := model.NewOpaqueType(src.Token)
			contract.IgnoreError(err)
			t = tt
		}

		if src.UnderlyingType != nil {
			underlyingType := schemaTypeToType(src.UnderlyingType)
			return model.NewUnionType(t, underlyingType)
		}
		return t
	case *schema.UnionType:
		types := make([]model.Type, len(src.ElementTypes))
		for i, src := range src.ElementTypes {
			types[i] = schemaTypeToType(src)
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
		case schema.AnyType:
			return model.DynamicType
		default:
			return model.NoneType
		}
	}
}
