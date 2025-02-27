// Copyright 2023, Pulumi Corporation.
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

package eval

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/ast"
	"github.com/pulumi/esc/internal/util"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"golang.org/x/exp/maps"
)

// A ProviderLoader provides the environment evaluator the capability to load providers.
type ProviderLoader interface {
	// LoadProvider loads the provider with the given name.
	LoadProvider(ctx context.Context, name string) (esc.Provider, error)
	// LoadRotator loads the rotator with the given name.
	LoadRotator(ctx context.Context, name string) (esc.Rotator, error)
}

// An EnvironmentLoader provides the environment evaluator the capability to load imported environment definitions.
type EnvironmentLoader interface {
	// LoadEnvironment loads the definition for the environment with the given name.
	LoadEnvironment(ctx context.Context, name string) ([]byte, Decrypter, error)
}

// LoadYAML decodes a YAML template from an io.Reader.
func LoadYAML(filename string, r io.Reader) (*ast.EnvironmentDecl, syntax.Diagnostics, error) {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	return LoadYAMLBytes(filename, bytes)
}

// LoadYAMLBytes decodes a YAML template from a byte array.
func LoadYAMLBytes(filename string, source []byte) (*ast.EnvironmentDecl, syntax.Diagnostics, error) {
	var diags syntax.Diagnostics

	syn, sdiags := encoding.DecodeYAMLBytes(filename, source, TagDecoder)
	diags.Extend(sdiags...)
	if sdiags.HasErrors() {
		return nil, diags, nil
	}

	t, tdiags := ast.ParseEnvironment(source, syn)
	diags.Extend(tdiags...)
	if tdiags.HasErrors() {
		return nil, diags, nil
	}

	return t, diags, nil
}

// EvalEnvironment evaluates the given environment.
func EvalEnvironment(
	ctx context.Context,
	name string,
	env *ast.EnvironmentDecl,
	decrypter Decrypter,
	providers ProviderLoader,
	environments EnvironmentLoader,
	execContext *esc.ExecContext,
) (*esc.Environment, syntax.Diagnostics) {
	opened, _, diags := evalEnvironment(ctx, false, false, name, env, decrypter, providers, environments, execContext, true, nil)
	return opened, diags
}

// CheckEnvironment symbolically evaluates the given environment. Calls to fn::open are not invoked, and instead
// evaluate to unknown values with appropriate schemata.
func CheckEnvironment(
	ctx context.Context,
	name string,
	env *ast.EnvironmentDecl,
	decrypter Decrypter,
	providers ProviderLoader,
	environments EnvironmentLoader,
	execContext *esc.ExecContext,
	showSecrets bool,
) (*esc.Environment, syntax.Diagnostics) {
	checked, _, diags := evalEnvironment(ctx, true, false, name, env, decrypter, providers, environments, execContext, showSecrets, nil)
	return checked, diags
}

// RotateEnvironment evaluates the given environment and invokes provider rotate methods.
// The updated rotation state is returned with a set of patches to be written back to the environment.
func RotateEnvironment(
	ctx context.Context,
	name string,
	env *ast.EnvironmentDecl,
	decrypter Decrypter,
	providers ProviderLoader,
	environments EnvironmentLoader,
	execContext *esc.ExecContext,
	paths []resource.PropertyPath,
) (*esc.Environment, *RotationResult, syntax.Diagnostics) {
	rotateDocPaths := make(map[string]bool, len(paths))
	for _, path := range paths {
		rotateDocPaths["values."+path.String()] = true
	}
	return evalEnvironment(ctx, false, true, name, env, decrypter, providers, environments, execContext, true, rotateDocPaths)
}

// evalEnvironment evaluates an environment and exports the result of evaluation.
func evalEnvironment(
	ctx context.Context,
	validating bool,
	rotating bool,
	name string,
	env *ast.EnvironmentDecl,
	decrypter Decrypter,
	providers ProviderLoader,
	envs EnvironmentLoader,
	execContext *esc.ExecContext,
	showSecrets bool,
	rotatePaths map[string]bool,
) (*esc.Environment, *RotationResult, syntax.Diagnostics) {
	if env == nil || (len(env.Values.GetEntries()) == 0 && len(env.Imports.GetElements()) == 0) {
		return nil, nil, nil
	}

	ec := newEvalContext(ctx, validating, rotating, name, env, true, decrypter, providers, envs, map[string]*imported{}, execContext, showSecrets, rotatePaths)
	v, diags := ec.evaluate()

	s := schema.Never().Schema()
	if v != nil {
		if v.base != nil {
			s = mergedSchema(v.base.schema, v.schema)
		} else {
			s = v.schema
		}
	}

	executionContext := &esc.EvaluatedExecutionContext{
		Properties: ec.myContext.export(name).Value.(map[string]esc.Value),
		Schema:     ec.myContext.schema,
	}

	return &esc.Environment{
		Exprs:            ec.root.export(name).Object,
		Properties:       v.export(name).Value.(map[string]esc.Value),
		Schema:           s,
		ExecutionContext: executionContext,
	}, &ec.rotationResult, diags
}

type imported struct {
	evaluating bool
	value      *value
}

// An evalContext carries the state necessary to evaluate an environment.
type evalContext struct {
	ctx          context.Context      // the cancellation context for evaluation
	validating   bool                 // true if we are only checking the environment
	rotating     bool                 // true if we are invoking rotators
	showSecrets  bool                 // true if secrets should be decrypted during validation
	name         string               // the name of the environment
	env          *ast.EnvironmentDecl // the root of the environment AST
	isRootEnv    bool                 // true if this environment is the root of evaluation (not an import)
	decrypter    Decrypter            // the decrypter to use for the environment
	providers    ProviderLoader       // the provider loader to use
	environments EnvironmentLoader    // the environment loader to use
	imports      map[string]*imported // the shared set of imported environments
	execContext  *esc.ExecContext     // evaluation context used for interpolation

	myContext *value // evaluated context to be used to interpolate properties
	myImports *value // directly-imported environments
	root      *expr  // the root expression
	base      *value // the base value

	rotateDocPaths map[string]bool // the subset of document paths to invoke rotation for when rotating. if empty, all rotators will be invoked.
	rotationResult RotationResult  // result of secret rotations

	diags syntax.Diagnostics // diagnostics generated during evaluation
}

func newEvalContext(
	ctx context.Context,
	validating bool,
	rotating bool,
	name string,
	env *ast.EnvironmentDecl,
	isRootEnv bool,
	decrypter Decrypter,
	providers ProviderLoader,
	environments EnvironmentLoader,
	imports map[string]*imported,
	execContext *esc.ExecContext,
	showSecrets bool,
	rotateDocPaths map[string]bool,
) *evalContext {
	return &evalContext{
		ctx:            ctx,
		validating:     validating,
		rotating:       rotating,
		showSecrets:    showSecrets,
		name:           name,
		env:            env,
		isRootEnv:      isRootEnv,
		decrypter:      decrypter,
		providers:      providers,
		environments:   environments,
		imports:        imports,
		execContext:    execContext.CopyForEnv(name),
		rotateDocPaths: rotateDocPaths,
	}
}

// decryptSecrets returns true if static secrets should be decrypted.
func (e *evalContext) decryptSecrets() bool {
	return !e.validating || e.showSecrets
}

// error records an evaluation error associated with an expression.
func (e *evalContext) error(expr ast.Expr, summary string) {
	diag := ast.ExprError(expr, summary)
	e.diags.Extend(diag)
}

// errorf is like error, but accepts a format string and arguments (ala fmt.Sprintf)
func (e *evalContext) errorf(expr ast.Expr, format string, a ...any) {
	e.error(expr, fmt.Sprintf(format, a...))
}

func (e *evalContext) accessorError(expr ast.Expr, accessor ast.PropertyAccessor, summary string) {
	diag := ast.AccessorError(expr, accessor, summary)
	e.diags.Extend(diag)
}

func (e *evalContext) accessorErrorf(expr ast.Expr, accessor ast.PropertyAccessor, format string, a ...any) {
	e.accessorError(expr, accessor, fmt.Sprintf(format, a...))
}

type exprNode interface {
	ast.Expr
	comparable
}

// declare creates an expr from an ast.Expr, sets its representation and initial schema, and attaches it to the given
// base. declare is also responsible for recursively declaring child exprs. declare may issue errors for duplicate keys
// in objects.
//
// The mapping of ast.Exprs to exprReprs is:
//
// - {Null, Boolean, Number, String}Expr -> literalExpr
// - InterpolateExpr                     -> interpolateExpr
// - SymbolExpr                          -> symbolExpr
// - FromBase64Expr                      -> fromBase64Expr
// - FromJSONExpr                        -> fromJSONExpr
// - JoinExpr                            -> joinExpr
// - OpenExpr                            -> openExpr
// - SecretExpr                          -> secretExpr
// - ToBase64Expr                        -> toBase64Expr
// - ToJSONExpr                          -> toJSONExpr
// - ArrayExpr                           -> arrayExpr
// - ObjectExpr                          -> objectExpr
//
// This is parameterized on the Expr type to avoid unintentionally creating non-nil ast.Expr values out of nil
// pointers. Parameterization allows us to check for nil pointers explicitly.
func declare[Expr exprNode](e *evalContext, path string, x Expr, base *value) *expr {
	var zero Expr
	if x == zero {
		return newMissingExpr(path, base)
	}

	switch x := any(x).(type) {
	case *ast.NullExpr:
		return newExpr(path, &literalExpr{node: x}, schema.Null().Schema(), base)
	case *ast.BooleanExpr:
		return newExpr(path, &literalExpr{node: x}, schema.Boolean().Const(x.Value).Schema(), base)
	case *ast.NumberExpr:
		return newExpr(path, &literalExpr{node: x}, schema.Number().Const(x.Value).Schema(), base)
	case *ast.StringExpr:
		return newExpr(path, &literalExpr{node: x}, schema.String().Const(x.Value).Schema(), base)
	case *ast.InterpolateExpr:
		parts := make([]interpolation, len(x.Parts))
		for i, p := range x.Parts {
			var value *propertyAccess
			if p.Value != nil {
				accessors := make([]*propertyAccessor, len(p.Value.Accessors))
				for i, a := range p.Value.Accessors {
					accessors[i] = &propertyAccessor{accessor: a}
				}
				value = &propertyAccess{accessors: accessors}
			}
			parts[i] = interpolation{syntax: p, value: value}
		}
		return newExpr(path, &interpolateExpr{node: x, parts: parts}, schema.String().Schema(), base)
	case *ast.SymbolExpr:
		accessors := make([]*propertyAccessor, len(x.Property.Accessors))
		for i, a := range x.Property.Accessors {
			accessors[i] = &propertyAccessor{accessor: a}
		}
		property := &propertyAccess{accessors: accessors}
		return newExpr(path, &symbolExpr{node: x, property: property}, schema.Always().Schema(), base)
	case *ast.FromBase64Expr:
		repr := &fromBase64Expr{node: x, string: declare(e, "", x.String, nil)}
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.FromJSONExpr:
		repr := &fromJSONExpr{node: x, string: declare(e, "", x.String, nil)}
		return newExpr(path, repr, schema.Always(), base)
	case *ast.JoinExpr:
		repr := &joinExpr{
			node:      x,
			delimiter: declare(e, "", x.Delimiter, nil),
			values:    declare(e, "", x.Values, nil),
		}
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.OpenExpr:
		repr := &openExpr{
			node:        x,
			provider:    declare(e, "", x.Provider, nil),
			inputs:      declare(e, "", x.Inputs, nil),
			inputSchema: schema.Always().Schema(),
		}
		return newExpr(path, repr, schema.Always().Schema(), base)
	case *ast.RotateExpr:
		repr := &rotateExpr{
			node:        x,
			provider:    declare(e, "", x.Provider, nil),
			inputs:      declare(e, "", x.Inputs, nil),
			state:       declare(e, "", x.State, nil),
			inputSchema: schema.Always().Schema(),
			stateSchema: schema.Always().Schema(),
		}
		return newExpr(path, repr, schema.Always().Schema(), base)
	case *ast.SecretExpr:
		if x.Plaintext != nil {
			repr := &secretExpr{node: x, plaintext: declare(e, "", x.Plaintext, nil)}
			repr.plaintext.secret = true
			return newExpr(path, repr, schema.String().Schema(), base)
		}
		repr := &secretExpr{node: x, ciphertext: declare(e, "", x.Ciphertext, nil)}
		repr.ciphertext.secret = true
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.ToBase64Expr:
		repr := &toBase64Expr{node: x, value: declare(e, "", x.Value, nil)}
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.ToJSONExpr:
		repr := &toJSONExpr{node: x, value: declare(e, "", x.Value, nil)}
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.ToStringExpr:
		repr := &toStringExpr{node: x, value: declare(e, "", x.Value, nil)}
		return newExpr(path, repr, schema.String().Schema(), base)
	case *ast.ArrayExpr:
		elements := make([]*expr, len(x.Elements))
		for i, x := range x.Elements {
			elements[i] = declare(e, fmt.Sprintf("%v[%d]", path, i), x, nil)
		}
		repr := &arrayExpr{node: x, elements: elements}
		return newExpr(path, repr, schema.Array().Items(schema.Always()).Schema(), base)
	case *ast.ObjectExpr:
		properties := make(map[string]*expr, len(x.Entries))
		for _, entry := range x.Entries {
			k := entry.Key.Value
			if _, ok := properties[k]; ok {
				e.errorf(entry.Key, "duplicate key %q", k)
			} else {
				properties[k] = declare(e, util.JoinKey(path, k), entry.Value, base.property(entry.Key, k))
			}
		}
		repr := &objectExpr{node: x, properties: properties}
		return newExpr(path, repr, schema.Object().AdditionalProperties(schema.Always()).Schema(), base)
	default:
		panic(fmt.Sprintf("fatal: invalid expr type %v", reflect.TypeOf(x)))
	}
}

func (e *evalContext) isReserveTopLevelKey(k string) bool {
	switch k {
	case "imports", "context", "environments":
		return true
	default:
		return false
	}
}

// evaluate drives the evaluation of the evalContext's environment.
func (e *evalContext) evaluate() (*value, syntax.Diagnostics) {
	mine := &imported{evaluating: true}
	defer func() { mine.evaluating = false }()
	e.imports[e.name] = mine

	// Evaluate context. We prepare the context values to later evaluate interpolations.
	e.evaluateContext()
	// Evaluate imports. We do this prior to declaration so that we can plumb base values as part of declaration.
	e.evaluateImports()

	// Build the root value. We do this manually b/c the AST uses a declaration rather than an expression for the
	// root.
	properties := make(map[string]*expr, len(e.env.Values.GetEntries()))
	e.root = &expr{
		path: "<" + e.name + ">",
		repr: &objectExpr{
			node:       ast.Object(),
			properties: properties,
		},
		base: e.base,
	}

	// Declare the root value's properties.
	for _, entry := range e.env.Values.GetEntries() {
		key := entry.Key.GetValue()

		if e.isReserveTopLevelKey(key) {
			e.errorf(entry.Key, "%q is a reserved key", key)
		} else if _, ok := properties[key]; ok {
			e.errorf(entry.Key, "duplicate key %q", key)
		} else {
			properties[key] = declare(e, key, entry.Value, e.base.property(entry.Key, key))
		}
	}

	// Evaluate the root value and return.
	v := e.evaluateExpr(e.root, schema.Always())
	return v, e.diags
}

func (e *evalContext) evaluateContext() {
	def := declare(e, "", ast.Symbol(&ast.PropertyName{Name: "context"}), nil)
	e.myContext = unexport(esc.NewValue(e.execContext.Values()), def)
}

// evaluateImports evaluates an environment's imports.
func (e *evalContext) evaluateImports() {
	myImports := map[string]*value{}
	for _, entry := range e.env.Imports.GetElements() {
		// If the import does not have a name, there's nothing we can do. This can happen for environments
		// with parse errors.
		if entry.Environment == nil {
			continue
		}
		name := entry.Environment.Value

		merge := true
		if entry.Meta != nil && entry.Meta.Merge != nil {
			merge = entry.Meta.Merge.Value
		}

		val, ok := e.evaluateImport(entry.Environment, name)
		if !ok {
			continue
		}

		myImports[name] = val
		if merge {
			val = newCopier().copy(val)
			val.merge(e.base)
			e.base = val
		}
	}

	properties := make(schema.SchemaMap, len(myImports))
	for k, v := range myImports {
		properties[k] = v.schema
	}
	s := schema.Record(properties).Schema()

	def := declare(e, "", ast.Symbol(&ast.PropertyName{Name: "imports"}), nil)
	def.schema, def.state = s, exprDone

	val := &value{
		def:    def,
		schema: s,
		repr:   myImports,
	}
	def.value = val

	e.myImports = val
}

// evaluateImport evaluates an imported environment.
//
// Each environment in the import closure is only evaluated once.
func (e *evalContext) evaluateImport(expr ast.Expr, name string) (*value, bool) {
	var val *value
	if imported, ok := e.imports[name]; ok {
		if imported.evaluating {
			e.diags.Extend(syntax.Error(expr.Syntax().Syntax().Range(), fmt.Sprintf("cyclic import of %v", name), expr.Syntax().Syntax().Path()))
			return nil, false
		}
		val = imported.value
	} else {
		bytes, dec, err := e.environments.LoadEnvironment(e.ctx, name)
		if err != nil {
			e.errorf(expr, "%s", err.Error())
			return nil, false
		}

		env, diags, err := LoadYAMLBytes(name, bytes)
		e.diags.Extend(diags...)
		if err != nil {
			e.errorf(expr, "%s", err.Error())
			return nil, false
		}
		if diags.HasErrors() {
			return nil, false
		}

		// we only want to rotate the root environment, so set rotating flag to false when evaluating imports
		imp := newEvalContext(e.ctx, e.validating, false, name, env, false, dec, e.providers, e.environments, e.imports, e.execContext, e.showSecrets, nil)
		v, diags := imp.evaluate()
		e.diags.Extend(diags...)

		val = v
		e.imports[name].value = val
	}
	return val, true
}

// evaluateExpr evaluates an expression. If the expression has already been evaluated, it returns the
// previously-computed result. evaluateExpr is also responsible for updating the expression's schema to that of its
// final, merged value.
func (e *evalContext) evaluateExpr(x *expr, accept *schema.Schema) *value {
	switch x.state {
	case exprDone:
		return x.value
	case exprEvaluating:
		e.errorf(x.repr.syntax(), "cyclic reference to %v", x.path)
		return &value{
			def:     x,
			schema:  schema.Always().Schema(),
			unknown: true,
		}
	default:
		x.state = exprEvaluating
		defer func() {
			x.state = exprDone
		}()
	}

	// terminate evaluation early if necessary
	if val, done := e.evaluateSkippedExpr(x, accept); done {
		return val
	}

	val := (*value)(nil)
	switch repr := x.repr.(type) {
	case *missingExpr:
		val = &value{def: x, schema: x.schema, unknown: true}
	case *literalExpr:
		switch syntax := x.repr.syntax().(type) {
		case *ast.NullExpr:
			val = &value{def: x, schema: x.schema, repr: nil}
		case *ast.BooleanExpr:
			val = &value{def: x, schema: x.schema, repr: syntax.Value}
		case *ast.NumberExpr:
			val = &value{def: x, schema: x.schema, repr: syntax.Value}
		case *ast.StringExpr:
			val = &value{def: x, schema: x.schema, repr: syntax.Value}
		}
	case *interpolateExpr:
		val = e.evaluateInterpolate(x, repr)
	case *symbolExpr:
		val = e.evaluatePropertyAccess(x, repr.property.accessors, accept)
	case *fromBase64Expr:
		val = e.evaluateBuiltinFromBase64(x, repr)
	case *fromJSONExpr:
		val = e.evaluateBuiltinFromJSON(x, repr)
	case *joinExpr:
		val = e.evaluateBuiltinJoin(x, repr)
	case *openExpr:
		val = e.evaluateBuiltinOpen(x, repr)
	case *rotateExpr:
		val = e.evaluateBuiltinRotate(x, repr)
	case *secretExpr:
		val = e.evaluateBuiltinSecret(x, repr)
	case *toBase64Expr:
		val = e.evaluateBuiltinToBase64(x, repr)
	case *toJSONExpr:
		val = e.evaluateBuiltinToJSON(x, repr)
	case *toStringExpr:
		val = e.evaluateBuiltinToString(x, repr)
	case *arrayExpr:
		val = e.evaluateArray(x, repr, accept)
	case *objectExpr:
		val = e.evaluateObject(x, repr, accept)
	default:
		panic(fmt.Sprintf("fatal: invalid expr type %T", repr))
	}

	if accept.IsRotateOnly() {
		val.rotateOnly = true
	}
	if x.secret {
		val.secret = true
	}
	val.merge(x.base)

	x.schema = val.schema
	x.value = val
	return val
}

// evaluateSkippedExpr returns a missing value if it's necessary to stop evaluating this expr early
func (e *evalContext) evaluateSkippedExpr(x *expr, accept *schema.Schema) (*value, bool) {
	// if we're not rotating, rotateOnly inputs are resolved as unknown.
	//
	// however, we also need to make sure the user has permission to access rotateOnly environments when they are editing an environment to
	// avoid privilege escalation from adding a reference to an environment that they don't have access to, but the scheduled rotator does.
	// therefore we will still evaluate rotateOnly imports when validating the root environment.
	//
	// we only do this for the root environment, because only root environments are rotated, and it is permissible for a user to import a
	// rotated environment that transitively uses managing credentials that they don't have access to:
	// allowed: "my-environment" <-imports- "my-iam-user" <-rotateOnly- "privileged-creds" (no access)
	//
	// thus, we want to skip evaluation if in a rotateOnly context and opening the root environment
	// or if this is an imported env
	if skipEval := accept.IsRotateOnly() && (!e.rotating && !e.validating || !e.isRootEnv); skipEval {
		return &value{def: newMissingExpr(x.path, x.base), schema: schema.Always(), unknown: true, rotateOnly: true}, true
	}

	return nil, false
}

// evaluateTypedExpr evaluates an expression and typechecks it against the given schema. Returns false if typechecking
// fails.
func (e *evalContext) evaluateTypedExpr(x *expr, accept *schema.Schema) (*value, bool) {
	v := e.evaluateExpr(x, accept)
	vv := validator{}
	ok := vv.validateValue(v, accept, validationLoc{x: x})
	e.diags.Extend(vv.diags...)
	return v, ok
}

// evaluateArray evaluates an array expression.
func (e *evalContext) evaluateArray(x *expr, repr *arrayExpr, accept *schema.Schema) *value {
	v := &value{def: x}

	array, items := make([]*value, len(repr.elements)), make([]schema.Builder, len(repr.elements))
	for i, elem := range repr.elements {
		ev := e.evaluateExpr(elem, accept.Item(i))
		array[i], items[i] = ev, ev.schema
	}

	v.repr, v.schema = array, schema.Tuple(items...).Schema()
	return v
}

// evaluateObject evaluates an object expression.
func (e *evalContext) evaluateObject(x *expr, repr *objectExpr, accept *schema.Schema) *value {
	v := &value{def: x}

	// NOTE: technically, evaluation order of maps is unspecified and the result should be independent of order.
	// However, we always evaluate in lexicographic order so that we can produce predictable diagnostics in the
	// face of cycles.
	keys := maps.Keys(repr.properties)
	sort.Strings(keys)

	object, properties := make(map[string]*value, len(keys)), make(schema.SchemaMap, len(keys))
	for _, k := range keys {
		pv := e.evaluateExpr(repr.properties[k], accept.Property(k))
		object[k], properties[k] = pv, pv.schema
	}

	v.repr, v.schema = object, schema.Record(properties).Schema()
	return v
}

// evaluateInterpolate evaluates a string interpolation expression.
func (e *evalContext) evaluateInterpolate(x *expr, repr *interpolateExpr) *value {
	v := &value{def: x, schema: x.schema}

	var b strings.Builder
	for _, i := range repr.parts {
		b.WriteString(i.syntax.Text)

		if i.value != nil {
			pv := e.evaluatePropertyAccess(x, i.value.accessors, schema.Always())
			s, unknown, secret := pv.toString()
			v.unknown, v.secret = v.containsUnknowns() || unknown, v.containsSecrets() || secret
			if !unknown {
				b.WriteString(s)
			}
		}
	}

	if !v.unknown {
		v.repr = b.String()
	} else {
		v.repr = "[unknown]"
	}
	return v
}

// evaluatePropertyAccess evaluates a property access.
func (e *evalContext) evaluatePropertyAccess(x *expr, accessors []*propertyAccessor, accept *schema.Schema) *value {
	// We make a copy of the resolved value here because evaluateExpr will merge it with its base, which mutates the
	// value. We also stamp over the def with the provided expression in order to maintain proper error reporting.
	v := newCopier().copy(e.evaluateExprAccess(x, accessors, accept))
	v.def = x
	return v
}

// evaluateExprAccess is the primary entrypoint for access evaluation, and begins with the assumption that the receiver
// is an expression. If the receiver is a list, object, or secret  expression, it is _not evaluated_. If the receiver
// is any other type of expression, it is evaluated and the result is passed to evaluateValueAccess. Once all accessors
// have been processed, the resolved expression is evaluated.
func (e *evalContext) evaluateExprAccess(x *expr, accessors []*propertyAccessor, accept *schema.Schema) *value {
	receiver := e.root

	k, ok := e.objectKey(x.repr.syntax(), accessors[0].accessor, false)

	// Check for an imports access.
	if ok && k == "imports" {
		accessors[0].value = e.myImports
		return e.evaluateValueAccess(x.repr.syntax(), e.myImports, accessors[1:])
	}

	// Check for context interpolation.
	if ok && k == "context" {
		accessors[0].value = e.myContext
		return e.evaluateValueAccess(x.repr.syntax(), e.myContext, accessors[1:])
	}

	// Check for inline reference
	if ok && k == "environments" {
		return e.evaluateEnvironmentReferenceAccess(x, accessors, accept)
	}

	for len(accessors) > 0 {
		accessor := accessors[0]
		if receiver == nil {
			e.errorf(x.repr.syntax(), "internal error: no receiver")
			return e.invalidPropertyAccess(x.repr.syntax(), accessors)
		}

		switch repr := receiver.repr.(type) {
		case *arrayExpr:
			index, ok := e.arrayIndex(x.repr.syntax(), accessor.accessor, len(repr.elements))
			if !ok {
				return e.invalidPropertyAccess(x.repr.syntax(), accessors)
			}
			receiver = repr.elements[index]
		case *objectExpr:
			key, ok := e.objectKey(x.repr.syntax(), accessor.accessor, true)
			if !ok {
				return e.invalidPropertyAccess(x.repr.syntax(), accessors)
			}

			// Check for the property in the object itself. If the property does not exist and the value's base is also
			// an object, defer to the base per JSON merge patch semantics. Otherwise, return an "unknown property"
			// error.
			prop, ok := repr.properties[key]
			if !ok {
				if receiver.base.isObject() {
					return e.evaluateValueAccess(x.repr.syntax(), receiver.base, accessors)
				}
				e.accessorErrorf(x.repr.syntax(), accessor.accessor, "unknown property %q", key)
				return e.invalidPropertyAccess(x.repr.syntax(), accessors)
			}
			receiver = prop
		case *secretExpr:
			// Secret expressions are transparent to accessors.
			receiver = repr.plaintext
			continue
		default:
			return e.evaluateValueAccess(x.repr.syntax(), e.evaluateExpr(receiver, accept), accessors)
		}

		// Synthesize a value for the accessor.
		val := &value{
			def:    receiver,
			base:   receiver.base,
			schema: receiver.schema,
		}
		accessor.value, accessors = val, accessors[1:]
	}

	return e.evaluateExpr(receiver, schema.Always())
}

// evaluateEnvironmentReferenceAccess performs an inline import of an environment.
// The accessor is of the form ["environments", $project, $env, ...], which is transformed into an import name in the form "$project/$env"
func (e *evalContext) evaluateEnvironmentReferenceAccess(x *expr, accessors []*propertyAccessor, accept *schema.Schema) *value {
	// desugar accessor path into import name
	if len(accessors) < 3 {
		// need at least the first three elements to create an import name
		return e.invalidPropertyAccess(x.repr.syntax(), accessors)
	}
	projName, projOk := e.objectKey(x.repr.syntax(), accessors[1].accessor, true)
	envName, envOk := e.objectKey(x.repr.syntax(), accessors[2].accessor, true)
	if !projOk || !envOk {
		return e.invalidPropertyAccess(x.repr.syntax(), accessors)
	}
	qualifiedName := fmt.Sprintf("%s/%s", projName, envName)

	importedValue, ok := e.evaluateImport(x.repr.syntax(), qualifiedName)
	if !ok {
		// failed to import, treat as missing
		importedValue = &value{def: newMissingExpr("", nil), schema: schema.Always(), unknown: true}
	}

	// construct a synthetic object literal of the reference which the accessors can traverse
	environmentsValue := &value{
		def: x,
		repr: map[string]*value{
			envName: importedValue,
		},
		schema: schema.Record(schema.SchemaMap{envName: importedValue.schema}).Schema(),
	}
	projectsValue := &value{
		def: x,
		repr: map[string]*value{
			projName: environmentsValue,
		},
		schema: schema.Record(schema.SchemaMap{projName: environmentsValue.schema}).Schema(),
	}
	referenceValue := &value{
		def: x,
		repr: map[string]*value{
			"environments": projectsValue,
		},
		schema: schema.Record(schema.SchemaMap{"environments": projectsValue.schema}).Schema(),
	}

	referenceValue.rotateOnly = true
	return e.evaluateValueAccess(x.repr.syntax(), referenceValue, accessors)
}

// evaluateValueAccess evaluates a list of accessors relative to a value receiver.
func (e *evalContext) evaluateValueAccess(syntax ast.Expr, receiver *value, accessors []*propertyAccessor) *value {
	for len(accessors) > 0 {
		accessor := accessors[0]

		if receiver.unknown {
			return e.evaluateUnknownAccess(syntax, receiver.schema, accessors)
		}

		switch repr := receiver.repr.(type) {
		case []*value:
			index, ok := e.arrayIndex(syntax, accessor.accessor, len(repr))
			if !ok {
				return e.invalidPropertyAccess(syntax, accessors)
			}
			receiver = repr[index]
		case map[string]*value:
			key, ok := e.objectKey(syntax, accessor.accessor, true)
			if !ok {
				return e.invalidPropertyAccess(syntax, accessors)
			}

			// Check for the property in the object itself. If the property does not exist and the value's base is also
			// an object, defer to the base per JSON merge patch semantics. Otherwise, return an "unknown property"
			// error.
			prop, ok := repr[key]
			if !ok {
				if receiver.base.isObject() {
					return e.evaluateValueAccess(syntax, receiver.base, accessors)
				}
				e.accessorErrorf(syntax, accessor.accessor, "unknown property %q", key)
				return e.invalidPropertyAccess(syntax, accessors)
			}
			receiver = prop
		default:
			e.accessorError(syntax, accessor.accessor, "receiver must be an array or an object")
			return e.invalidPropertyAccess(syntax, accessors)
		}

		accessor.value, accessors = receiver, accessors[1:]
	}

	return receiver
}

// evaluateValueAccess evaluates a list of accessors relative to an unknown value receiver. Unknown values are
// synthesized for each receiver.
func (e *evalContext) evaluateUnknownAccess(syntax ast.Expr, receiver *schema.Schema, accessors []*propertyAccessor) *value {
	var val *value
	for len(accessors) > 0 {
		accessor := accessors[0]

		if !receiver.Always {
			switch receiver.Type {
			case "array":
				n := -1
				if receiver.Items.Never {
					n = len(receiver.PrefixItems)
				}
				index, ok := e.arrayIndex(syntax, accessor.accessor, n)
				if !ok {
					return e.invalidPropertyAccess(syntax, accessors)
				}
				receiver = receiver.Item(index)
			case "object":
				key, ok := e.objectKey(syntax, accessor.accessor, true)
				if !ok {
					return e.invalidPropertyAccess(syntax, accessors)
				}
				receiver = receiver.Property(key)
			default:
				e.accessorError(syntax, accessor.accessor, "receiver must be an array or an object")
				return e.invalidPropertyAccess(syntax, accessors)
			}
		}

		val = &value{
			def: &expr{
				repr:  &literalExpr{node: syntax},
				state: exprDone,
			},
			schema:  receiver,
			unknown: true,
		}

		accessor.value, accessors = val, accessors[1:]
	}
	return val
}

// invalidPropertyAccess resolves each accessor to an unknown value.
func (e *evalContext) invalidPropertyAccess(syntax ast.Expr, accessors []*propertyAccessor) *value {
	for _, accessor := range accessors {
		accessor.value = &value{
			def: &expr{
				repr:  &literalExpr{node: syntax},
				state: exprDone,
			},
			schema:  schema.Always().Schema(),
			unknown: true,
		}
	}
	return accessors[len(accessors)-1].value
}

// arrayIndex extracts an array index from an accessor. If the accessor is not an integer or is out of bounds,
// arrayIndex generates an appropriate error and returns false.
func (e *evalContext) arrayIndex(expr ast.Expr, accessor ast.PropertyAccessor, len int) (int, bool) {
	sub, ok := accessor.(*ast.PropertySubscript)
	if !ok {
		e.accessorError(expr, accessor, "cannot access an array element using a property name")
		return 0, false
	}
	index, ok := sub.Index.(int)
	if !ok {
		e.accessorError(expr, accessor, "cannot access an array element using a property name")
		return 0, false
	}
	if index < 0 {
		e.accessorError(expr, accessor, "array indices must not be negative")
		return 0, false
	}
	if len >= 0 && index >= len {
		e.accessorErrorf(expr, accessor, "array index %v out-of-bounds for array of length %v", index, len)
		return 0, false
	}
	return index, true
}

// objectKey extracts an object key from an accessor. If the accessor is not a string, objectKey generates an
// appropriate error and returns false.
func (e *evalContext) objectKey(expr ast.Expr, accessor ast.PropertyAccessor, must bool) (string, bool) {
	switch a := accessor.(type) {
	case *ast.PropertyName:
		return a.Name, true
	case *ast.PropertySubscript:
		s, ok := a.Index.(string)
		if !ok {
			if must {
				e.accessorError(expr, accessor, "cannot access an object property using an integer index")
			}
			return "", false
		}
		return s, true
	default:
		panic(fmt.Errorf("unexpected accessor type %T", accessor))
	}
}

// evaluateBuiltinSecret evaluates a call to the fn::secret builtin. Plaintext secrets evaluate to the
// plaintext value. Ciphertext secrets evaluate to unknown during validation and to their plaintext
// during evaluation.
func (e *evalContext) evaluateBuiltinSecret(x *expr, repr *secretExpr) *value {
	if repr.plaintext != nil {
		return e.evaluateExpr(repr.plaintext, schema.String().Schema())
	}

	v := &value{def: x, schema: x.schema, secret: true}

	ciphertext, err := decodeCiphertext(repr.node.Ciphertext.Value)
	if err != nil {
		e.errorf(repr.syntax(), "invalid ciphertext: %v", err)
		v.unknown = true
		return v
	}
	if !e.decryptSecrets() {
		v.unknown = true
		return v
	}

	plaintext, err := e.decrypter.Decrypt(e.ctx, ciphertext)
	if err != nil {
		e.errorf(repr.syntax(), "decrypting: %v", err)
		v.unknown = true
		return v
	}

	v.repr = string(plaintext)
	return v
}

// evaluateBuiltinOpen evaluates a call to the fn::open builtin. This involves loading the provider, fetching its
// schemata, evaluating the inputs, and when not validating, opening the provider with the given inputs. During
// validation, the result is an unknown value with the output schema.
func (e *evalContext) evaluateBuiltinOpen(x *expr, repr *openExpr) *value {
	v := &value{def: x}

	// Can happen if there are parse errors.
	if repr.node.Provider == nil {
		v.schema = schema.Always()
		v.unknown = true
		return v
	}

	provider, err := e.providers.LoadProvider(e.ctx, repr.node.Provider.GetValue())
	if err != nil {
		e.errorf(repr.syntax(), "%v", err)
	} else {
		inputSchema, outputSchema := provider.Schema()
		if err := inputSchema.Compile(); err != nil {
			e.errorf(repr.syntax(), "internal error: invalid input schema (%v)", err)
		} else {
			repr.inputSchema = inputSchema
		}
		if err := outputSchema.Compile(); err != nil {
			e.errorf(repr.syntax(), "internal error: invalid schema (%v)", err)
		} else {
			x.schema = outputSchema
		}

	}
	v.schema = x.schema

	inputs, ok := e.evaluateTypedExpr(repr.inputs, repr.inputSchema)
	if !ok || inputs.containsUnknowns() || e.validating || err != nil {
		v.unknown = true
		return v
	}

	output, err := provider.Open(e.ctx, inputs.export("").Value.(map[string]esc.Value), e.execContext)
	if err != nil {
		e.errorf(repr.syntax(), "%s", err.Error())
		v.unknown = true
		return v
	}
	return unexport(output, x)
}

// evaluateBuiltinOpen evaluates a call to the fn::rotate builtin.
func (e *evalContext) evaluateBuiltinRotate(x *expr, repr *rotateExpr) *value {
	v := &value{def: x}

	// Can happen if there are parse errors.
	if repr.node.Provider == nil {
		v.schema = schema.Always()
		v.unknown = true
		return v
	}

	rotator, err := e.providers.LoadRotator(e.ctx, repr.node.Provider.GetValue())
	if err != nil {
		e.errorf(repr.syntax(), "%v", err)
	} else {
		inputSchema, stateSchema, outputSchema := rotator.Schema()
		stateSchema = schema.OneOf(stateSchema, schema.Null())
		if err := inputSchema.Compile(); err != nil {
			e.errorf(repr.syntax(), "internal error: invalid input schema (%v)", err)
		} else {
			repr.inputSchema = inputSchema
		}
		if err := stateSchema.Compile(); err != nil {
			e.errorf(repr.syntax(), "internal error: invalid state schema (%v)", err)
		} else {
			repr.stateSchema = stateSchema
		}
		if err := outputSchema.Compile(); err != nil {
			e.errorf(repr.syntax(), "internal error: invalid schema (%v)", err)
		} else {
			x.schema = outputSchema
		}
	}
	v.schema = x.schema

	docPath := x.repr.syntax().Syntax().Syntax().Path()

	inputs, inputsOK := e.evaluateTypedExpr(repr.inputs, repr.inputSchema)
	state, stateOK := e.evaluateTypedExpr(repr.state, repr.stateSchema)
	if !inputsOK || inputs.containsObservableUnknowns(e.rotating) || !stateOK || state.containsUnknowns() || e.validating || err != nil {
		if e.shouldRotate(docPath) {
			e.rotationResult = append(e.rotationResult, &Rotation{
				Path:   docPath,
				Status: RotationNotEvaluated,
			})
		}

		v.unknown = true
		return v
	}

	// if rotating, invoke prior to open
	if e.shouldRotate(docPath) {
		newState, err := rotator.Rotate(
			e.ctx,
			inputs.export("").Value.(map[string]esc.Value),
			asObjectOrNil(state.export("").Value),
			e.execContext,
		)
		if err != nil {
			diag := ast.ExprError(repr.syntax(), err.Error())
			e.rotationResult = append(e.rotationResult, &Rotation{
				Path:   docPath,
				Status: RotationFailed,
				Diags:  []*syntax.Diagnostic{diag},
			})

			e.errorf(repr.syntax(), "rotate: %s", err.Error())
			v.unknown = true
			return v
		}

		e.rotationResult = append(e.rotationResult, &Rotation{
			Path:   docPath,
			Status: RotationSucceeded,
			Patch: &Patch{
				// rotation output is written back to the fn's `state` input
				DocPath:     util.JoinKey(docPath, repr.node.Name().GetValue()) + ".state",
				Replacement: newState,
			},
		})

		// todo: validate newState conforms to state schema

		// pass the updated state to open, as if it were already persisted
		state = unexport(newState, x)
	}

	output, err := rotator.Open(
		e.ctx,
		inputs.export("").Value.(map[string]esc.Value),
		asObjectOrNil(state.export("").Value),
		e.execContext,
	)
	if err != nil {
		e.errorf(repr.syntax(), "%s", err.Error())
		v.unknown = true
		return v
	}
	return unexport(output, x)
}

// shouldRotate returns true if the rotator at this path should be invoked.
func (e *evalContext) shouldRotate(docPath string) bool {
	if !e.rotating {
		return false
	}
	if len(e.rotateDocPaths) == 0 {
		// we're rotating the full environment
		return true
	}
	return e.rotateDocPaths[docPath]
}

// cast to map[string]esc.Value, or nil
func asObjectOrNil(v any) map[string]esc.Value {
	cast, _ := v.(map[string]esc.Value)
	return cast
}

// evaluateBuiltinJoin evaluates a call to the fn::join builtin.
func (e *evalContext) evaluateBuiltinJoin(x *expr, repr *joinExpr) *value {
	v := &value{def: x, schema: x.schema}

	delim, delimOk := e.evaluateTypedExpr(repr.delimiter, schema.String().Schema())
	vs, vsOk := e.evaluateTypedExpr(repr.values, schema.Array().Items(schema.String()).Schema())
	if !delimOk || !vsOk {
		v.unknown = true
		return v
	}

	v.combine(delim, vs)
	if !v.unknown {
		values := make([]string, len(vs.repr.([]*value)))
		for i, v := range vs.repr.([]*value) {
			values[i] = v.repr.(string)
		}
		v.repr = strings.Join(values, delim.repr.(string))
	}
	return v
}

// evaluateBuiltinFromBase64 evaluates a call from the fn::fromBase64 builtin.
func (e *evalContext) evaluateBuiltinFromBase64(x *expr, repr *fromBase64Expr) *value {
	v := &value{def: x, schema: x.schema}

	str, ok := e.evaluateTypedExpr(repr.string, schema.String().Schema())
	if !ok {
		v.unknown = true
		return v
	}

	v.combine(str)
	if !v.unknown {
		b, err := base64.StdEncoding.DecodeString(str.repr.(string))
		if err != nil {
			e.errorf(repr.syntax(), "decoding base64 string: %v", err)
			v.unknown = true
			return v
		}
		v.repr = string(b)
	}
	return v
}

// evaluateBuiltinFromJSON evaluates a call from the fn::fromJSON builtin.
func (e *evalContext) evaluateBuiltinFromJSON(x *expr, repr *fromJSONExpr) *value {
	v := &value{def: x, schema: x.schema}

	str, ok := e.evaluateTypedExpr(repr.string, schema.String().Schema())
	if !ok {
		v.unknown = true
		return v
	}

	v.combine(str)
	if !v.unknown {
		dec := json.NewDecoder(strings.NewReader(str.repr.(string)))
		dec.UseNumber()

		var jv any
		if err := dec.Decode(&jv); err != nil {
			e.errorf(repr.syntax(), "decoding JSON string: %v", err)
			v.unknown = true
			return v
		}

		ev, err := esc.FromJSON(jv, v.secret)
		if err != nil {
			e.errorf(repr.syntax(), "internal error: decoding JSON value: %v", err)
			v.unknown = true
			return v
		}

		return unexport(ev, x)
	}
	return v
}

// evaluateBuiltinToBase64 evaluates a call to the fn::toBase64 builtin.
func (e *evalContext) evaluateBuiltinToBase64(x *expr, repr *toBase64Expr) *value {
	v := &value{def: x, schema: x.schema}

	str, ok := e.evaluateTypedExpr(repr.value, schema.String().Schema())
	if !ok {
		v.unknown = true
		return v
	}

	v.combine(str)
	if !v.unknown {
		v.repr = base64.StdEncoding.EncodeToString([]byte(str.repr.(string)))
	}
	return v
}

// evaluateBuiltinToJSON evaluates a call to the fn::toJSON builtin.
func (e *evalContext) evaluateBuiltinToJSON(x *expr, repr *toJSONExpr) *value {
	v := &value{def: x, schema: x.schema}

	value := e.evaluateExpr(repr.value, schema.Always())

	v.combine(value)
	if !v.unknown {
		b, err := json.Marshal(value.export("").ToJSON(false))
		if err != nil {
			e.errorf(repr.syntax(), "failed to encode JSON: %v", err)
			v.unknown = true
			return v
		}
		v.repr = string(b)
	}
	return v
}

// evaluateBuiltinToString evaluates a call to the fn::toString builtin.
func (e *evalContext) evaluateBuiltinToString(x *expr, repr *toStringExpr) *value {
	v := &value{def: x, schema: x.schema}

	value := e.evaluateExpr(repr.value, schema.Always())

	s, unknown, secret := value.toString()
	v.unknown, v.secret = unknown, secret
	if !unknown {
		v.repr = s
	}
	return v
}
