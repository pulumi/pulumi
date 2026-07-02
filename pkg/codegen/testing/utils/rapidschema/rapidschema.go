// Copyright 2026, Pulumi Corporation.
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

// Package rapidschema provides a rapid (property-based) generator for valid
// Pulumi schemas.
//
// The generator produces fully-formed schemas exercising the full breadth of
// the Pulumi schema language — primitives, arrays, maps, named complex
// objects, enums, unions (with and without discriminators), refs to the
// built-in Archive/Asset/Json/Any types, optional and required properties,
// plain, secret.
package rapidschema

import (
	"fmt"
	"sort"

	"github.com/blang/semver"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

// Package returns a generator for valid Pulumi schemas.
//
// Generated packages always carry resources but never a provider with input
// or output properties, and never any Functions or Config: this generator
// targets the importer round-trip test, which only consumes resources.
// Object and enum types are generated lazily — every declared type is
// referenced from at least one resource property (or transitively from
// another type), and types may reference each other (including cycles via
// self-references).
func Package() *rapid.Generator[*schema.Package] {
	return rapid.Custom(func(t *rapid.T) *schema.Package {
		spec := drawPackageSpec(t)
		var poisonLoader schema.Loader = struct{ schema.Loader }{} // Will panic on usage
		pkg, diags, err := schema.BindSpec(spec, poisonLoader, schema.ValidationOptions{})
		require.NoError(t, err, "rapidschema: BindSpec returned error on generated spec %q: %v", spec.Name, err)
		require.Nil(t, diags, "rapidschema: BindSpec produced diagnostics on generated spec %q: %s",
			spec.Name, diags.Error())
		return pkg
	})
}

// Version returns a generator for semantic version values.
//
// Pre-release and build strings are constrained to the semver identifier
// alphabet ([0-9A-Za-z-]). Pre-release alphanumeric identifiers must contain
// at least one non-digit character (otherwise they would be parsed as
// numeric identifiers and reject leading zeros under strict semver), so we
// require a leading letter or hyphen. The Pulumi metaschema validates
// PackageSpec.Version against the strict semver regex; values produced by
// this generator round-trip through .String() / Parse cleanly.
func Version() *rapid.Generator[semver.Version] {
	pre := rapid.Custom(func(t *rapid.T) (v semver.PRVersion) {
		v.IsNum = rapid.Bool().Draw(t, "isnum")
		if v.IsNum {
			v.VersionNum = rapid.Uint64Range(0, 1000).Draw(t, "versionnum")
		} else {
			v.VersionStr = rapid.StringMatching(`[A-Za-z-][0-9A-Za-z-]*`).Draw(t, "versionstr")
		}
		return v
	})
	return rapid.Custom(func(t *rapid.T) semver.Version {
		return semver.Version{
			Major: rapid.Uint64Range(0, 1000).Draw(t, "major"),
			Minor: rapid.Uint64Range(0, 1000).Draw(t, "minor"),
			Patch: rapid.Uint64Range(0, 1000).Draw(t, "patch"),
			Build: rapid.Map(rapid.SliceOf(rapid.StringMatching(`[0-9A-Za-z-]+`)), normalizeSlize).Draw(t, "build"),
			Pre:   rapid.Map(rapid.SliceOf(pre), normalizeSlize).Draw(t, "pre"),
		}
	})
}

func normalizeSlize[T any, S ~[]T](s S) S {
	if len(s) == 0 {
		return nil
	}
	return s
}

// maxTypeDepth caps recursion through type generation: array/map/union
// members and lazily-introduced object bodies each consume one level.
const maxTypeDepth = 3

// pkgCtx is the running state passed through the generator stack so that
// nested generators can declare new types and refer to existing ones.
type pkgCtx struct {
	name string

	// modules is the (mutable) list of module names that have been picked
	// somewhere in this package. Each new resource/type either reuses one of
	// these or draws a fresh one, so a single package can carry tokens
	// across several modules.
	modules []string

	// objectTokens is the (mutable) list of object-type tokens declared so
	// far. New tokens are appended as the generator decides to introduce a
	// new object type while drawing a property's type.
	objectTokens []string
	// enumTokensByBase groups declared enum tokens by their underlying
	// primitive ("string" / "integer" / "number" / "boolean").
	enumTokensByBase map[string][]string
	// enumBases is the deterministic ordering of bases that currently have
	// at least one declared enum, kept here so map iteration order does not
	// leak into rapid's draw sequence.
	enumBases []string

	// typeDefs accumulates all complex-type bodies (objects + enums) to be
	// emitted in PackageSpec.Types.
	typeDefs map[string]schema.ComplexTypeSpec

	objectCount int
	enumCount   int
}

func drawPackageSpec(t *rapid.T) schema.PackageSpec {
	name := drawPackageName(t, "pkgName")

	emptyProvider := func() *schema.ResourceSpec {
		// Provider is required by the binder; an empty object satisfies it
		// without contributing any properties.
		return &schema.ResourceSpec{ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}}
	}

	// Settle the parameterization before drawing any tokens: extension
	// parameterized packages must namespace their tokens under the base
	// provider rather than their own name, so the token namespace has to be
	// known up front.
	var spec schema.PackageSpec
	tokenNamespace := name
	switch rapid.IntRange(0, 2).Draw(t, "parameterization") {
	case 0:
		spec.Provider = emptyProvider()
	case 1:
		p := drawParameterizationSpec(t, "parameterization")
		spec.Parameterization = &p
		spec.Provider = emptyProvider()
	case 2:
		e := drawExtensionParameterizationSpec(t, "extensionParameterization")
		spec.ExtensionParameterization = &e
		tokenNamespace = e.BaseProvider.Name
	}

	ctx := &pkgCtx{
		name:             tokenNamespace,
		enumTokensByBase: map[string][]string{},
		typeDefs:         map[string]schema.ComplexTypeSpec{},
	}

	nResources := rapid.IntRange(0, 4).Draw(t, "nResources")
	resourceTokens := make([]string, nResources)
	for i := 0; i < nResources; i++ {
		module := drawModule(t, ctx, fmt.Sprintf("res%d:module", i))
		resourceTokens[i] = fmt.Sprintf("%s:%s:Res%d", tokenNamespace, module, i)
	}

	resources := make(map[string]schema.ResourceSpec, nResources)
	for _, tok := range resourceTokens {
		resources[tok] = drawResourceSpec(t, ctx, tok)
	}

	spec.Name = name
	spec.Version = Version().Draw(t, "version").String()
	spec.Types = ctx.typeDefs
	spec.Resources = resources

	return spec
}

// drawParameterizationSpec produces a fully-formed ParameterizationSpec.
// BasePlugin.Name follows the package-name format; Version is a strict
// semver via Version(); Parameter is arbitrary bytes (often empty).
func drawParameterizationSpec(t *rapid.T, label string) schema.ParameterizationSpec {
	return schema.ParameterizationSpec{
		BaseProvider: schema.BaseProviderSpec{
			Name:    drawPackageName(t, label+":baseProvider:name"),
			Version: Version().Draw(t, label+":baseProvider:version").String(),
		},
		Parameter: rapid.SliceOfN(rapid.Byte(), 0, 32).Draw(t, label+":parameter"),
	}
}

// drawExtensionParameterizationSpec produces a fully-formed
// ExtensionParameterizationSpec. The base provider is sometimes itself a
// parameterization of a plugin. An extension rides on the base provider, so the
// package must not also declare a provider of its own.
func drawExtensionParameterizationSpec(t *rapid.T, label string) schema.ExtensionParameterizationSpec {
	base := schema.BaseProviderRefSpec{
		Name:    drawPackageName(t, label+":baseProvider:name"),
		Version: Version().Draw(t, label+":baseProvider:version").String(),
	}
	if rapid.Bool().Draw(t, label+":baseProvider:hasParameterization") {
		base.Parameterization = &schema.BaseProviderParameterizationSpec{
			BasePlugin: schema.BaseProviderSpec{
				Name:    drawPackageName(t, label+":baseProvider:basePlugin:name"),
				Version: Version().Draw(t, label+":baseProvider:basePlugin:version").String(),
			},
			Parameter: rapid.SliceOfN(rapid.Byte(), 0, 32).Draw(t, label+":baseProvider:basePlugin:parameter"),
		}
	}
	return schema.ExtensionParameterizationSpec{
		BaseProvider: base,
		Parameter:    rapid.SliceOfN(rapid.Byte(), 0, 32).Draw(t, label+":parameter"),
	}
}

// drawModule returns either a freshly-drawn module name (recorded for future
// reuse) or one of the modules already used in this package. This lets a
// single package's tokens span multiple modules while also exercising shared
// modules between resources and types.
func drawModule(t *rapid.T, ctx *pkgCtx, label string) string {
	if len(ctx.modules) > 0 && rapid.Bool().Draw(t, label+":reuse") {
		return rapid.SampledFrom(ctx.modules).Draw(t, label+":existing")
	}
	m := drawModuleName(t, label+":new")
	ctx.modules = append(ctx.modules, m)
	return m
}

var primitiveTypeNames = []string{"boolean", "integer", "number", "string"}

// drawPackageName produces a lowercase package name. The first character
// must be a letter; subsequent characters may also include digits and
// hyphens. The binder forbids "pulumi" as a property keyword but accepts it
// as a package name; we still exclude it for clarity.
func drawPackageName(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-z][a-z0-9-]{1,7}`).
		Filter(func(s string) bool { return s != "pulumi" }).
		Draw(t, label)
}

// drawModuleName produces a module string. Modules can be a single segment or
// nested via "/", but the binder warns if a module name is rooted at
// "index/...". We pick from a regex that emits "index" alone or any
// non-"index"-rooted slash-delimited path of up to three segments.
func drawModuleName(t *rapid.T, label string) string {
	return rapid.StringMatching(`(index|[a-z]{2,4}(/[a-z]{2,4}){0,2})`).Draw(t, label)
}

// drawPropertyName returns an identifier-like name that is safe to use in
// every property context the binder validates. Names start with a lowercase
// letter; subsequent characters may also include digits, underscores, and
// hyphens. The reserved set is the union of all context-specific reserved
// names: "pulumi" (any property), "urn" (resource property), "id"
// (non-component resource property and state input), "version" (provider
// input property).
func drawPropertyName(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-z][a-zA-Z0-9_-]{0,9}`).
		Filter(func(s string) bool {
			switch s {
			case "pulumi", "urn", "id", "version":
				return false
			}
			return true
		}).
		Draw(t, label)
}

// addNewObject reserves a fresh object token (in a freshly-drawn or reused
// module), registers it as visible to later draws (so the body may
// reference back to it for self-recursion or mutual recursion with
// subsequently-created types), draws the body, and records the body in
// typeDefs.
func (c *pkgCtx) addNewObject(t *rapid.T, label string, depth int) string {
	module := drawModule(t, c, label+":module")
	token := fmt.Sprintf("%s:%s:Obj%d", c.name, module, c.objectCount)
	c.objectCount++
	c.objectTokens = append(c.objectTokens, token)

	properties, required := drawProperties(t, c, label+":props", depth-1)
	c.typeDefs[token] = schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}
	return token
}

// addNewEnum reserves a fresh enum token (in a freshly-drawn or reused
// module), draws the value list, and records the body in typeDefs.
func (c *pkgCtx) addNewEnum(t *rapid.T, label string) string {
	base := rapid.SampledFrom(primitiveTypeNames).Draw(t, label+":base")
	module := drawModule(t, c, label+":module")
	token := fmt.Sprintf("%s:%s:Enum%d", c.name, module, c.enumCount)
	c.enumCount++

	if _, exists := c.enumTokensByBase[base]; !exists {
		c.enumBases = append(c.enumBases, base)
		sort.Strings(c.enumBases)
	}
	c.enumTokensByBase[base] = append(c.enumTokensByBase[base], token)

	c.typeDefs[token] = drawEnumBody(t, base, label+":body")
	return token
}

// drawEnumBody produces the body of an enum type: 1..N distinct values drawn
// from the primitive generator for the given base. The boolean base is
// capped at 2 values to fit its domain.
func drawEnumBody(t *rapid.T, base, label string) schema.ComplexTypeSpec {
	maxN := 4
	if base == "boolean" {
		maxN = 2
	}
	n := rapid.IntRange(1, maxN).Draw(t, label+":n")

	values := rapid.SliceOfNDistinct(primitiveValueGen(base), n, n, primitiveValueKey).
		Draw(t, label+":values")
	names := rapid.SliceOfNDistinct(
		rapid.StringMatching(`[A-Z][a-zA-Z0-9]{0,8}`),
		n, n,
		rapid.ID[string],
	).Draw(t, label+":names")

	specs := make([]schema.EnumValueSpec, n)
	for i := 0; i < n; i++ {
		specs[i] = schema.EnumValueSpec{Name: names[i], Value: values[i]}
	}
	return schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{Type: base},
		Enum:           specs,
	}
}

// primitiveValueGen returns a generator producing values of the given Pulumi
// primitive type, boxed as `any` for use in EnumValueSpec.Value (which is
// `any`). Floats are bounded so JSON round-tripping is well-defined and
// NaN/Inf are excluded.
func primitiveValueGen(base string) *rapid.Generator[any] {
	switch base {
	case "string":
		return rapid.Map(
			rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,8}`),
			func(s string) any { return s },
		)
	case "integer":
		return rapid.Map(rapid.IntRange(-1000, 1000), func(i int) any { return i })
	case "number":
		return rapid.Map(rapid.Float64Range(-1000, 1000), func(f float64) any { return f })
	case "boolean":
		return rapid.Map(rapid.Bool(), func(b bool) any { return b })
	}
	panic(fmt.Sprintf("rapidschema: unknown primitive base %q", base))
}

// primitiveValueKey is the keyFn used by SliceOfNDistinct to deduplicate
// generated enum values. Including the type prefix makes the key
// unambiguous across primitive bases (in practice all values within a single
// enum share a base, but the prefix makes the function safe to compose).
func primitiveValueKey(v any) string {
	return fmt.Sprintf("%T:%v", v, v)
}

// drawResourceSpec produces a ResourceSpec for the given token.
func drawResourceSpec(t *rapid.T, ctx *pkgCtx, label string) schema.ResourceSpec {
	outputProps, outputRequired := drawProperties(t, ctx, label+":out", maxTypeDepth)
	inputProps, inputRequired := drawProperties(t, ctx, label+":in", maxTypeDepth)

	spec := schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type:       "object",
			Properties: outputProps,
			Required:   outputRequired,
		},
		InputProperties: inputProps,
		RequiredInputs:  inputRequired,
	}

	if rapid.Bool().Draw(t, label+":haveStateInputs") {
		stateProps, stateRequired := drawProperties(t, ctx, label+":state", maxTypeDepth)
		spec.StateInputs = &schema.ObjectTypeSpec{
			Type:       "object",
			Properties: stateProps,
			Required:   stateRequired,
		}
	}

	return spec
}

// drawProperties returns a property map plus a sorted required list.
// Property names are drawn via rapid (filtered against reserved keywords)
// and deduplicated within the map.
func drawProperties(
	t *rapid.T, ctx *pkgCtx, label string, depth int,
) (map[string]schema.PropertySpec, []string) {
	n := rapid.IntRange(0, 5).Draw(t, label+":n")
	if n == 0 {
		return nil, nil
	}
	names := rapid.SliceOfNDistinct(
		rapid.Custom(func(t *rapid.T) string { return drawPropertyName(t, "name") }),
		n, n,
		rapid.ID[string],
	).Draw(t, label+":names")

	props := make(map[string]schema.PropertySpec, n)
	var required []string
	for _, name := range names {
		propLabel := fmt.Sprintf("%s:%s", label, name)
		spec := drawPropertySpec(t, ctx, propLabel, depth)
		props[name] = spec
		// A required property whose top-level type forces another object of
		// some type to be present (a direct ObjectType reference, or a union
		// of such refs with no escape branch) would describe an
		// infinite-size value.
		if isSafeForRequired(spec.TypeSpec, ctx) && rapid.Bool().Draw(t, propLabel+":required") {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return props, required
}

// isSafeForRequired reports whether spec admits at least one value that
// terminates without forcing another object instance of the same kind to
// exist. Arrays/Maps/Optionals always admit an empty/null value, primitives
// terminate naturally, enum and built-in refs don't recurse. ObjectType
// refs are unsafe (their value forces a nested object). Unions are safe iff
// at least one branch is safe.
func isSafeForRequired(spec schema.TypeSpec, ctx *pkgCtx) bool {
	if len(spec.OneOf) > 0 {
		for _, m := range spec.OneOf {
			if isSafeForRequired(m, ctx) {
				return true
			}
		}
		return false
	}
	return !isDirectObjectRef(spec, ctx)
}

// isDirectObjectRef reports whether spec resolves at its top level to an
// ObjectType already declared in ctx. Array/Map/Union members and primitives
// (including pulumi.json#/Archive etc.) all return false.
func isDirectObjectRef(spec schema.TypeSpec, ctx *pkgCtx) bool {
	const prefix = "#/types/"
	if len(spec.Ref) <= len(prefix) || spec.Ref[:len(prefix)] != prefix {
		return false
	}
	tok := spec.Ref[len(prefix):]
	for _, ot := range ctx.objectTokens {
		if ot == tok {
			return true
		}
	}
	return false
}

func drawPropertySpec(t *rapid.T, ctx *pkgCtx, label string, depth int) schema.PropertySpec {
	typ := drawTypeSpec(t, ctx, label, depth)
	spec := schema.PropertySpec{TypeSpec: typ}
	if rapid.Bool().Draw(t, label+":secret") {
		spec.Secret = true
	}
	if rapid.Bool().Draw(t, label+":replaceOnChanges") {
		spec.ReplaceOnChanges = true
	}
	return spec
}

// drawTypeSpec produces a TypeSpec valid against the Pulumi metaschema. It
// picks exactly one of: primitive, builtin ref, $ref to a declared type
// (lazily creating one when allowed), array, map, or union — the metaschema
// rejects mixed shapes. depth caps recursion through array/map/union members
// and through new-object/new-enum bodies.
func drawTypeSpec(t *rapid.T, ctx *pkgCtx, label string, depth int) schema.TypeSpec {
	kinds := []string{"primitive", "archive", "asset", "json", "any"}
	canRecurse := depth > 0
	canReuseObj := len(ctx.objectTokens) > 0
	canReuseEnum := len(ctx.enumBases) > 0
	if canReuseObj || canRecurse {
		kinds = append(kinds, "objectRef")
	}
	if canReuseEnum || canRecurse {
		kinds = append(kinds, "enumRef")
	}
	if canRecurse {
		kinds = append(kinds, "array", "map", "union")
	}
	kind := rapid.SampledFrom(kinds).Draw(t, label+":kind")

	var spec schema.TypeSpec
	switch kind {
	case "primitive":
		spec = schema.TypeSpec{Type: rapid.SampledFrom(primitiveTypeNames).Draw(t, label+":prim")}
	case "archive":
		spec = schema.TypeSpec{Ref: "pulumi.json#/Archive"}
	case "asset":
		spec = schema.TypeSpec{Ref: "pulumi.json#/Asset"}
	case "json":
		spec = schema.TypeSpec{Ref: "pulumi.json#/Json"}
	case "any":
		spec = schema.TypeSpec{Ref: "pulumi.json#/Any"}
	case "objectRef":
		spec = schema.TypeSpec{Ref: "#/types/" + drawObjectToken(t, ctx, label, canReuseObj, canRecurse, depth)}
	case "enumRef":
		spec = schema.TypeSpec{Ref: "#/types/" + drawEnumToken(t, ctx, label, canReuseEnum, canRecurse)}
	case "array":
		items := drawTypeSpec(t, ctx, label+":item", depth-1)
		spec = schema.TypeSpec{Type: "array", Items: &items}
	case "map":
		addl := drawTypeSpec(t, ctx, label+":addl", depth-1)
		spec = schema.TypeSpec{Type: "object", AdditionalProperties: &addl}
	case "union":
		spec = drawUnionTypeSpec(t, ctx, label, depth)
	default:
		panic(fmt.Sprintf("rapidschema: unhandled type kind %q", kind))
	}

	if rapid.Bool().Draw(t, label+":plain") {
		spec.Plain = true
	}
	return spec
}

func drawObjectToken(
	t *rapid.T, ctx *pkgCtx, label string, canReuse, canCreate bool, depth int,
) string {
	if canReuse && (!canCreate || rapid.Bool().Draw(t, label+":reuseObj")) {
		return rapid.SampledFrom(ctx.objectTokens).Draw(t, label+":existingObj")
	}
	return ctx.addNewObject(t, label+":newObj", depth)
}

func drawEnumToken(
	t *rapid.T, ctx *pkgCtx, label string, canReuse, canCreate bool,
) string {
	if canReuse && (!canCreate || rapid.Bool().Draw(t, label+":reuseEnum")) {
		base := rapid.SampledFrom(ctx.enumBases).Draw(t, label+":enumBase")
		return rapid.SampledFrom(ctx.enumTokensByBase[base]).Draw(t, label+":existingEnum")
	}
	return ctx.addNewEnum(t, label+":newEnum")
}

func drawUnionTypeSpec(t *rapid.T, ctx *pkgCtx, label string, depth int) schema.TypeSpec {
	n := rapid.IntRange(2, 4).Draw(t, label+":unionLen")
	members := make([]schema.TypeSpec, n)
	for i := 0; i < n; i++ {
		members[i] = drawTypeSpec(t, ctx, fmt.Sprintf("%s:m%d", label, i), depth-1)
	}
	spec := schema.TypeSpec{OneOf: members}

	if rapid.Bool().Draw(t, label+":haveType") {
		spec.Type = rapid.SampledFrom(primitiveTypeNames).Draw(t, label+":unionType")
	}

	if rapid.Bool().Draw(t, label+":haveDiscriminator") {
		disc := &schema.DiscriminatorSpec{
			PropertyName: drawPropertyName(t, label+":discProp"),
		}
		if rapid.Bool().Draw(t, label+":haveMapping") && len(ctx.objectTokens) > 0 {
			mapping := make(map[string]string)
			count := rapid.IntRange(1, len(ctx.objectTokens)).Draw(t, label+":mappingLen")
			for i := 0; i < count; i++ {
				mapping[fmt.Sprintf("v%d", i)] = "#/types/" + ctx.objectTokens[i]
			}
			disc.Mapping = mapping
		}
		spec.Discriminator = disc
	}
	return spec
}
