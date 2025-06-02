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

package pcl

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type packageSchema struct {
	schema schema.PackageReference

	// These maps map from canonical tokens to actual tokens.
	//
	// Both maps take `nil` to mean uninitialized.
	resourceTokenMap map[string]string
	functionTokenMap map[string]string
}

type packageOpts struct {
	version           string
	pluginDownloadURL string
}

// Lookup a PCL invoke token in a schema.
func LookupFunction(pkg schema.PackageReference, token string) (*schema.Function, bool, error) {
	s, _, ok, err := newPackageSchema(pkg).LookupFunction(token)
	return s, ok, err
}

// Lookup a PCL resource token in a schema.
func LookupResource(pkg schema.PackageReference, token string) (*schema.Resource, bool, error) {
	r, _, ok, err := newPackageSchema(pkg).LookupResource(token)
	return r, ok, err
}

func (ps *packageSchema) LookupFunction(token string) (*schema.Function, string, bool, error) {
	contract.Assertf(ps != nil, "packageSchema must not be nil")

	if ps.functionTokenMap == nil {
		ps.initFunctionMap()
	}

	schemaToken, ok := ps.functionTokenMap[token]
	if !ok {
		token = canonicalizeToken(token, ps.schema)
		schemaToken, ok = ps.functionTokenMap[token]
		if !ok {
			return nil, "", false, nil
		}
	}

	fn, ok, err := ps.schema.Functions().Get(schemaToken)
	return fn, token, ok, err
}

func (ps *packageSchema) LookupResource(token string) (*schema.Resource, string, bool, error) {
	contract.Assertf(ps != nil, "packageSchema must not be nil")

	if ps.resourceTokenMap == nil {
		ps.initResourceMap()
	}

	schemaToken, ok := ps.resourceTokenMap[token]
	if !ok {
		token = canonicalizeToken(token, ps.schema)
		schemaToken, ok = ps.resourceTokenMap[token]
		if !ok {
			return nil, "", false, nil
		}
	}

	res, ok, err := ps.schema.Resources().Get(schemaToken)
	return res, token, ok, err
}

func (ps *packageSchema) initFunctionMap() {
	functionTokenMap := map[string]string{}
	for it := ps.schema.Functions().Range(); it.Next(); {
		functionTokenMap[canonicalizeToken(it.Token(), ps.schema)] = it.Token()
	}
	ps.functionTokenMap = functionTokenMap
}

func (ps *packageSchema) initResourceMap() {
	resourceTokenMap := map[string]string{}
	for it := ps.schema.Resources().Range(); it.Next(); {
		resourceTokenMap[canonicalizeToken(it.Token(), ps.schema)] = it.Token()
	}
	ps.resourceTokenMap = resourceTokenMap
}

func newPackageSchema(pkg schema.PackageReference) *packageSchema {
	return &packageSchema{schema: pkg}
}

type PackageInfo struct {
	name    string
	version string
}

type PackageCache struct {
	m sync.RWMutex

	// cache by (name, version)
	entries map[PackageInfo]*packageSchema
}

func NewPackageCache() *PackageCache {
	return &PackageCache{
		entries: map[PackageInfo]*packageSchema{},
	}
}

func (c *PackageCache) getPackageSchema(pkg PackageInfo) (*packageSchema, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	schema, ok := c.entries[pkg]
	return schema, ok
}

// loadPackageSchema loads the schema for a given package by loading the corresponding provider and calling its
// GetSchema method.
// If a version is passed in, the cache will be bypassed and the package will be reloaded.
func (c *PackageCache) loadPackageSchema(
	ctx context.Context, loader schema.Loader,
	name, version, pluginDownloadURL string,
) (*packageSchema, error) {
	pkgInfo := PackageInfo{
		name:    name,
		version: version,
	}
	if s, ok := c.getPackageSchema(pkgInfo); ok {
		return s, nil
	}

	var versionSemver *semver.Version
	if v, err := semver.Make(version); err == nil {
		versionSemver = &v
	}

	pkg, err := schema.LoadPackageReferenceV2(ctx, loader, &schema.PackageDescriptor{
		Name:        name,
		Version:     versionSemver,
		DownloadURL: pluginDownloadURL,
	})
	if err != nil {
		return nil, err
	}

	schema := newPackageSchema(pkg)

	c.m.Lock()
	defer c.m.Unlock()

	c.entries[pkgInfo] = schema

	return schema, nil
}

func (c *PackageCache) loadPackageSchemaFromDescriptor(
	loader schema.Loader,
	descriptor *schema.PackageDescriptor,
) (*packageSchema, error) {
	version := ""

	descriptorVersion := descriptor.PackageVersion()
	if descriptorVersion != nil {
		version = descriptorVersion.String()
	}

	pkgInfo := PackageInfo{
		name:    descriptor.PackageName(),
		version: version,
	}

	if s, ok := c.getPackageSchema(pkgInfo); ok {
		return s, nil
	}

	pkg, err := schema.LoadPackageReferenceV2(context.TODO(), loader, descriptor)
	if err != nil {
		return nil, err
	}

	schema := newPackageSchema(pkg)

	c.m.Lock()
	defer c.m.Unlock()

	c.entries[pkgInfo] = schema

	return schema, nil
}

// canonicalizeToken converts a Pulumi token into its canonical "pkg:module:member" form.
func canonicalizeToken(tok string, pkg schema.PackageReference) string {
	_, _, member, _ := DecomposeToken(tok, hcl.Range{})
	return fmt.Sprintf("%s:%s:%s", pkg.Name(), pkg.TokenToModule(tok), member)
}

// getPkgOpts gets the package options from an unbound resource node.
func (b *binder) getPkgOpts(node *Resource) packageOpts {
	node.VariableType = model.NewObjectType(map[string]model.Type{
		"id":  model.NewOutputType(model.StringType),
		"urn": model.NewOutputType(model.StringType),
	})
	var rangeKey, rangeValue model.Type
	for _, block := range node.syntax.Body.Blocks {
		if block.Type == "options" {
			if rng, hasRange := block.Body.Attributes["range"]; hasRange {
				expr, _ := model.BindExpression(rng.Expr, b.root, b.tokens, b.options.modelOptions()...)
				typ := model.ResolveOutputs(expr.Type())
				strict := !b.options.skipRangeTypecheck
				rk, rv, _ := model.GetCollectionTypes(typ, rng.Range(), strict)
				rangeKey, rangeValue = rk, rv
			}
		}
	}

	scopes := newResourceScopes(b.root, node, rangeKey, rangeValue)

	block, _ := model.BindBlock(node.syntax, scopes, b.tokens, b.options.modelOptions()...)

	var options *model.Block
	for _, item := range block.Body.Items {
		if item, ok := item.(*model.Block); ok && item.Type == "options" {
			options = item
			break
		}
	}

	pkgOpts := packageOpts{}
	// Typecheck the options block.
	if options != nil {
		resourceOptions := &ResourceOptions{}
		for _, item := range options.Body.Items {
			switch item := item.(type) {
			case *model.Attribute:
				switch item.Name {
				case "version":
					pkgOpts.version = modelExprToString(&item.Value)
				case "pluginDownloadURL":
					pkgOpts.pluginDownloadURL = modelExprToString(&item.Value)
				}
			}
		}
		node.Options = resourceOptions
	}

	return pkgOpts
}

// loadReferencedPackageSchemas loads the schemas for any packages referenced by a given node.
func (b *binder) loadReferencedPackageSchemas(ctx context.Context, n Node) error {
	var pkgOpts packageOpts
	packageNames := codegen.StringSet{}

	if r, ok := n.(*Resource); ok {
		token, tokenRange := getResourceToken(r)
		packageName, mod, name, _ := DecomposeToken(token, tokenRange)
		if mod == "providers" {
			packageNames.Add(name)
		} else {
			packageNames.Add(packageName)
		}
		pkgOpts = b.getPkgOpts(r)
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
		packageName, mod, name, _ := DecomposeToken(token, tokenRange)
		if packageName != pulumiPackage {
			packageNames.Add(packageName)
		} else if mod == "providers" {
			packageNames.Add(name)
		}
		return nil
	})
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)

	for _, name := range packageNames.SortedValues() {
		if _, ok := b.referencedPackages[name]; ok && pkgOpts.version == "" || name == "" {
			continue
		}

		var pkg *packageSchema
		var err error
		if packageDescriptor, ok := b.packageDescriptors[name]; ok {
			pkg, err = b.options.packageCache.loadPackageSchemaFromDescriptor(b.options.loader, packageDescriptor)
		} else {
			pkg, err = b.options.packageCache.loadPackageSchema(
				ctx, b.options.loader,
				name, pkgOpts.version, pkgOpts.pluginDownloadURL,
			)
		}
		if err != nil {
			if b.options.skipResourceTypecheck || b.options.skipInvokeTypecheck {
				continue
			}
			return err
		}
		b.referencedPackages[name] = pkg.schema
	}
	return nil
}

func buildEnumValue(v interface{}) cty.Value {
	switch v := v.(type) {
	case string:
		return cty.StringVal(v)
	case bool:
		return cty.BoolVal(v)
	case int:
		return cty.NumberIntVal(int64(v))
	case int32:
		return cty.NumberIntVal(int64(v))
	case int64:
		return cty.NumberIntVal(v)
	case float64:
		return cty.NumberFloatVal(v)
	default:
		contract.Failf("Found unexpected constant type %T: %[1]v", v)
		return cty.NilVal
	}
}

// A marker struct to ensure type safety when retrieving the type from an
// annotated `model.EnumType`.
type enumSchemaType struct {
	Type *schema.EnumType
}

// schemaTypeToType converts a schema.Type to a model Type.
func (b *binder) schemaTypeToType(src schema.Type) model.Type {
	switch src := src.(type) {
	case *schema.ArrayType:
		return model.NewListType(b.schemaTypeToType(src.ElementType))
	case *schema.MapType:
		return model.NewMapType(b.schemaTypeToType(src.ElementType))
	case *schema.EnumType:
		values := []cty.Value{}
		elType := b.schemaTypeToType(src.ElementType)
		for _, el := range src.Elements {
			values = append(values, buildEnumValue(el.Value))
		}
		return model.NewEnumType(src.Token, elType, values, enumSchemaType{src})
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

			properties[prop.Name] = b.schemaTypeToTypeOrConst(typ, prop)
		}
		return objType
	case *schema.TokenType:
		t := model.NewOpaqueType(src.Token)

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
	case *schema.ResourceType:
		if t, ok := b.schemaTypes[src]; ok {
			return t
		}

		properties := map[string]model.Type{}
		objType := model.NewObjectType(properties, src)
		b.schemaTypes[src] = objType
		for _, prop := range src.Resource.Properties {
			typ := prop.Type
			if !prop.IsRequired() {
				typ = &schema.OptionalType{ElementType: typ}
			}

			properties[prop.Name] = b.schemaTypeToTypeOrConst(typ, prop)
		}
		return objType
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
			// Generated SDK code accepts assets or archives when schema.AssetType is
			// specified. In an effort to keep PCL type checking in sync with our
			// generated SDKs, we match the SDKs behavior when translating schema types to
			// PCL types.
			return AssetOrArchiveType
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return model.DynamicType
		default:
			return model.NoneType
		}
	}
}

func (b *binder) schemaTypeToTypeOrConst(typ schema.Type, prop *schema.Property) model.Type {
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

	return t
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
		return model.GetObjectTypeAnnotation[schema.Type](t)
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
		schemaTypes := slice.Prealloc[schema.Type](len(schemas))
		for t := range schemas {
			schemaTypes = append(schemaTypes, t.(schema.Type))
		}
		if len(schemaTypes) == 1 {
			return schemaTypes[0], true
		}
		return &schema.UnionType{ElementTypes: schemaTypes}, true
	case *model.EnumType:
		for _, t := range t.Annotations {
			if t, ok := t.(enumSchemaType); ok {
				contract.Assertf(t.Type != nil, "enum schema type must not be nil")
				return t.Type, true
			}
		}
		return nil, false
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

// EnumMember returns the name of the member that matches the given `value`. If
// no member if found, (nil, true) returned. If the query is nonsensical, either
// because no schema is associated with the EnumMember or if the type of value
// mismatches the type of the schema, (nil, false) is returned.
func EnumMember(t *model.EnumType, value cty.Value) (*schema.Enum, bool) {
	srcBase, ok := GetSchemaForType(t)
	if !ok {
		return nil, false
	}
	src := srcBase.(*schema.EnumType)

	switch {
	case t.Type.Equals(model.StringType):
		s := value.AsString()
		for _, el := range src.Elements {
			v := el.Value.(string)
			if v == s {
				return el, true
			}
		}
		return nil, true
	case t.Type.Equals(model.NumberType):
		f, _ := value.AsBigFloat().Float64()
		for _, el := range src.Elements {
			if el.Value.(float64) == f {
				return el, true
			}
		}
		return nil, true
	case t.Type.Equals(model.IntType):
		f, _ := value.AsBigFloat().Int64()
		for _, el := range src.Elements {
			valueInt64, ok := el.Value.(int64)
			if ok && valueInt64 == f {
				return el, true
			}

			valueInt32, ok := el.Value.(int32)
			if ok && int64(valueInt32) == f {
				return el, true
			}
		}
		return nil, true
	default:
		return nil, false
	}
}

// GenEnum is a helper function when generating an enum.
// Given an enum, and instructions on what to do when you find a known value,
// and an unknown value, return a function that will generate an the given enum
// from the given expression.
//
// This function should probably live in the `codegen` namespace, but cannot
// because of import cycles.
func GenEnum(
	t *model.EnumType,
	from model.Expression,
	safeEnum func(member *schema.Enum),
	unsafeEnum func(from model.Expression),
) *hcl.Diagnostic {
	known := cty.NilVal
	if from, ok := from.(*model.TemplateExpression); ok && len(from.Parts) == 1 {
		if from, ok := from.Parts[0].(*model.LiteralValueExpression); ok {
			known = from.Value
		}
	}
	if from, ok := from.(*model.LiteralValueExpression); ok {
		known = from.Value
	}
	if known != cty.NilVal {
		// If the value is known, but we can't find a member, we should have
		// indicated a conversion is impossible when type checking.
		member, ok := EnumMember(t, known)
		contract.Assertf(ok,
			"We have determined %s is a safe enum, which we define as "+
				"being able to calculate a member for", t)
		if member != nil {
			safeEnum(member)
		} else {
			unsafeEnum(from)
			knownVal := strings.Split(strings.Split(known.GoString(), "(")[1], ")")[0]
			diag := &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("%v is not a valid value of the enum \"%v\"", knownVal, t.Token),
			}
			if members := enumMemberValues(t); len(members) > 0 {
				diag.Detail = fmt.Sprintf("Valid members are %v", listToString(members))
			}
			return diag
		}
	} else {
		unsafeEnum(from)
	}
	return nil
}

func enumMemberValues(t *model.EnumType) []interface{} {
	srcBase, ok := GetSchemaForType(t)
	if !ok {
		return nil
	}
	src := srcBase.(*schema.EnumType)
	members := make([]interface{}, len(src.Elements))
	for i, el := range src.Elements {
		members[i] = el.Value
	}
	return members
}

func listToString(l []interface{}) string {
	vals := ""
	for i, v := range l {
		if i == 0 {
			vals = fmt.Sprintf("\"%v\"", v)
		} else {
			vals = fmt.Sprintf("%s, \"%v\"", vals, v)
		}
	}
	return vals
}
