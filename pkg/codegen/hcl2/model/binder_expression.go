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

package model

import (
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	_syntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type BindOption func(options *bindOptions)

func AllowMissingVariables(options *bindOptions) {
	options.allowMissingVariables = true
}

type bindOptions struct {
	allowMissingVariables bool
}

type expressionBinder struct {
	options     bindOptions
	anonSymbols map[*hclsyntax.AnonSymbolExpr]Definition
	scope       *Scope
	tokens      _syntax.TokenMap
}

// BindExpression binds an HCL2 expression using the given scope and token map.
func BindExpression(syntax hclsyntax.Node, scope *Scope, tokens _syntax.TokenMap,
	opts ...BindOption) (Expression, hcl.Diagnostics) {

	var options bindOptions
	for _, opt := range opts {
		opt(&options)
	}

	b := &expressionBinder{
		options:     options,
		anonSymbols: map[*hclsyntax.AnonSymbolExpr]Definition{},
		scope:       scope,
		tokens:      tokens,
	}

	return b.bindExpression(syntax)
}

// BindExpressionText parses and binds an HCL2 expression using the given scope.
func BindExpressionText(source string, scope *Scope, initialPos hcl.Pos,
	opts ...BindOption) (Expression, hcl.Diagnostics) {

	syntax, tokens, diagnostics := _syntax.ParseExpression(source, "<anonymous>", initialPos)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return BindExpression(syntax, scope, tokens, opts...)
}

// bindExpression binds a single HCL2 expression.
func (b *expressionBinder) bindExpression(syntax hclsyntax.Node) (Expression, hcl.Diagnostics) {
	switch syntax := syntax.(type) {
	case *hclsyntax.AnonSymbolExpr:
		return b.bindAnonSymbolExpression(syntax)
	case *hclsyntax.BinaryOpExpr:
		return b.bindBinaryOpExpression(syntax)
	case *hclsyntax.ConditionalExpr:
		return b.bindConditionalExpression(syntax)
	case *hclsyntax.ForExpr:
		return b.bindForExpression(syntax)
	case *hclsyntax.FunctionCallExpr:
		return b.bindFunctionCallExpression(syntax)
	case *hclsyntax.IndexExpr:
		return b.bindIndexExpression(syntax)
	case *hclsyntax.LiteralValueExpr:
		return b.bindLiteralValueExpression(syntax)
	case *hclsyntax.ObjectConsExpr:
		return b.bindObjectConsExpression(syntax)
	case *hclsyntax.ObjectConsKeyExpr:
		return b.bindObjectConsKeyExpr(syntax)
	case *hclsyntax.RelativeTraversalExpr:
		return b.bindRelativeTraversalExpression(syntax)
	case *hclsyntax.ScopeTraversalExpr:
		return b.bindScopeTraversalExpression(syntax)
	case *hclsyntax.SplatExpr:
		return b.bindSplatExpression(syntax)
	case *hclsyntax.TemplateExpr:
		return b.bindTemplateExpression(syntax)
	case *hclsyntax.TemplateJoinExpr:
		return b.bindTemplateJoinExpression(syntax)
	case *hclsyntax.TemplateWrapExpr:
		return b.bindTemplateWrapExpression(syntax)
	case *hclsyntax.TupleConsExpr:
		return b.bindTupleConsExpression(syntax)
	case *hclsyntax.UnaryOpExpr:
		return b.bindUnaryOpExpression(syntax)
	default:
		contract.Failf("unexpected expression node of type %T (%v)", syntax, syntax.Range())
		return nil, nil
	}
}

// ctyTypeToType converts a cty.Type to a model Type.
func ctyTypeToType(t cty.Type, optional bool) Type {
	// TODO(pdg): non-primitive types. We simply don't need these yet.
	var result Type
	switch {
	case t.Equals(cty.NilType):
		return NoneType
	case t.Equals(cty.Bool):
		result = BoolType
	case t.Equals(cty.Number):
		result = NumberType
	case t.Equals(cty.String):
		result = StringType
	case t.Equals(cty.DynamicPseudoType):
		result = DynamicType
	case t.IsMapType():
		result = NewMapType(ctyTypeToType(t.ElementType(), false))
	case t.IsListType():
		result = NewListType(ctyTypeToType(t.ElementType(), false))
	case t.IsSetType():
		result = NewSetType(ctyTypeToType(t.ElementType(), false))
	case t.IsObjectType():
		properties := map[string]Type{}
		for key, t := range t.AttributeTypes() {
			properties[key] = ctyTypeToType(t, false)
		}
		result = NewObjectType(properties)
	case t.IsTupleType():
		elements := make([]Type, len(t.TupleElementTypes()))
		for i, t := range t.TupleElementTypes() {
			elements[i] = ctyTypeToType(t, false)
		}
		result = NewTupleType(elements...)
	default:
		contract.Failf("NYI: cty type %v", t.FriendlyName())
		return NoneType
	}
	if optional {
		return NewOptionalType(result)
	}
	return result
}

var typeCapsule = cty.Capsule("type", reflect.TypeOf((*Type)(nil)).Elem())

// encapsulateType wraps the given type in a cty capsule for use in TraverseIndex values.
func encapsulateType(t Type) cty.Value {
	return cty.CapsuleVal(typeCapsule, &t)
}

// MakeTraverser returns an hcl.TraverseIndex with a key of the appropriate type.
func MakeTraverser(t Type) hcl.TraverseIndex {
	return hcl.TraverseIndex{Key: encapsulateType(t)}
}

// getOperationSignature returns the equivalent StaticFunctionSignature for a given Operation. This signature can be
// used for typechecking the operation's arguments.
func getOperationSignature(op *hclsyntax.Operation) StaticFunctionSignature {
	ctyParams := op.Impl.Params()

	sig := StaticFunctionSignature{
		Parameters: make([]Parameter, len(ctyParams)),
	}
	for i, p := range ctyParams {
		sig.Parameters[i] = Parameter{
			Name: p.Name,
			Type: ctyTypeToType(p.Type, p.AllowNull),
		}
	}
	if p := op.Impl.VarParam(); p != nil {
		sig.VarargsParameter = &Parameter{
			Name: p.Name,
			Type: ctyTypeToType(p.Type, p.AllowNull),
		}
	}
	sig.ReturnType = ctyTypeToType(op.Type, false)

	return sig
}

// typecheckArgs typechecks the arguments against a given function signature.
func typecheckArgs(srcRange hcl.Range, signature StaticFunctionSignature, args ...Expression) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	// First typecheck the arguments for positional parameters. It is an error if there are fewer arguments than parameters
	// unless all missing arguments are for parameters with optional types.
	remainingArgs := args
	for _, param := range signature.Parameters {
		if len(remainingArgs) == 0 {
			if !IsOptionalType(param.Type) {
				diagnostics = append(diagnostics, missingRequiredArgument(param, srcRange))
			}
		} else {
			if !InputType(param.Type).ConversionFrom(remainingArgs[0].Type()).Exists() {
				diagnostics = append(diagnostics, ExprNotConvertible(InputType(param.Type), remainingArgs[0]))
			}
			remainingArgs = remainingArgs[1:]
		}
	}

	// Typecheck any remaining arguments against the varargs parameter. It is an error if there is no varargs parameter.
	if len(remainingArgs) > 0 {
		varargs := signature.VarargsParameter
		if varargs == nil {
			diagnostics = append(diagnostics, extraArguments(len(signature.Parameters), len(args), srcRange))
		} else {
			for _, arg := range remainingArgs {
				if !InputType(varargs.Type).ConversionFrom(arg.Type()).Exists() {
					diagnostics = append(diagnostics, ExprNotConvertible(InputType(varargs.Type), arg))
				}
			}
		}
	}

	return diagnostics
}

// bindAnonSymbolExpression binds an anonymous symbol expression. These expressions should only occur in the context of
// splat expressions, and are used represent the receiver of the expression following the splat. It is an error for
// an anonymous symbol expression to occur outside this context.
func (b *expressionBinder) bindAnonSymbolExpression(syntax *hclsyntax.AnonSymbolExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	lv, ok := b.anonSymbols[syntax]
	if !ok {
		diagnostics = append(diagnostics, internalError(syntax.Range(), "undefined anonymous symbol"))
		return &ErrorExpression{Syntax: syntax, exprType: DynamicType}, diagnostics
	}

	traversal := hcl.Traversal{hcl.TraverseRoot{Name: "", SrcRange: syntax.SrcRange}}
	return &ScopeTraversalExpression{
		Syntax: &hclsyntax.ScopeTraversalExpr{
			Traversal: traversal,
			SrcRange:  syntax.SrcRange,
		},
		Tokens:    _syntax.NewScopeTraversalTokens(traversal),
		RootName:  "",
		Parts:     []Traversable{lv},
		Traversal: traversal,
	}, diagnostics
}

// bindBinaryOpExpression binds a binary operator expression. If the operands to the binary operator contain eventuals,
// the result of the binary operator is eventual.
func (b *expressionBinder) bindBinaryOpExpression(syntax *hclsyntax.BinaryOpExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	// Bind the operands.
	leftOperand, leftDiags := b.bindExpression(syntax.LHS)
	diagnostics = append(diagnostics, leftDiags...)

	rightOperand, rightDiags := b.bindExpression(syntax.RHS)
	diagnostics = append(diagnostics, rightDiags...)

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.BinaryOpTokens)
	if tokens == nil {
		tokens = _syntax.NewBinaryOpTokens(syntax.Op)
	}
	expr := &BinaryOpExpression{
		Syntax:       syntax,
		Tokens:       tokens,
		LeftOperand:  leftOperand,
		Operation:    syntax.Op,
		RightOperand: rightOperand,
	}

	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindConditionalExpression binds a conditional expression. The condition expression must be of type bool. The type of
// the expression is unify(true result, false result). If the type of the condition is eventual, the type of the
// expression is eventual.
func (b *expressionBinder) bindConditionalExpression(syntax *hclsyntax.ConditionalExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	// Bind the operands.
	condition, conditionDiags := b.bindExpression(syntax.Condition)
	diagnostics = append(diagnostics, conditionDiags...)

	trueResult, trueDiags := b.bindExpression(syntax.TrueResult)
	diagnostics = append(diagnostics, trueDiags...)

	falseResult, falseDiags := b.bindExpression(syntax.FalseResult)
	diagnostics = append(diagnostics, falseDiags...)

	expr := &ConditionalExpression{
		Syntax:      syntax,
		Tokens:      b.tokens.ForNode(syntax),
		Condition:   condition,
		TrueResult:  trueResult,
		FalseResult: falseResult,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindForExpression binds a for expression. The value being iterated must be an list, map, or object.  The type of
// the result is an list unless a key expression is present, in which case it is a map. Key types must be strings.
// The element type of the result is the type of the value expression. If the type of the value being iterated is
// optional or eventual, the type of the result is optional or eventual. If the type of the key expression or
// condition expression is eventual, the result is also eventual.
func (b *expressionBinder) bindForExpression(syntax *hclsyntax.ForExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	collection, collectionDiags := b.bindExpression(syntax.CollExpr)
	diagnostics = append(diagnostics, collectionDiags...)

	// Poke through any eventual and optional types that may wrap the collection type.
	collectionType := unwrapIterableSourceType(collection.Type())

	// TODO(pdg): handle union types.
	keyType, valueType, kvDiags := GetCollectionTypes(collectionType, syntax.CollExpr.Range())
	diagnostics = append(diagnostics, kvDiags...)

	// Push a scope for the key and value variables and define these vars.
	b.scope = b.scope.Push(syntax)
	defer func() { b.scope = b.scope.Pop() }()

	var keyVariable *Variable
	if syntax.KeyVar != "" {
		keyVariable = &Variable{Name: syntax.KeyVar, VariableType: keyType}
		ok := b.scope.Define(syntax.KeyVar, keyVariable)
		contract.Assert(ok)
	}
	valueVariable := &Variable{Name: syntax.ValVar, VariableType: valueType}
	if ok := b.scope.Define(syntax.ValVar, valueVariable); !ok {
		diagnostics = append(diagnostics, nameAlreadyDefined(syntax.ValVar, syntax.Range()))
	}

	var key Expression
	if syntax.KeyExpr != nil {
		keyExpr, keyDiags := b.bindExpression(syntax.KeyExpr)
		key, diagnostics = keyExpr, append(diagnostics, keyDiags...)
	}

	value, valueDiags := b.bindExpression(syntax.ValExpr)
	diagnostics = append(diagnostics, valueDiags...)

	var condition Expression
	if syntax.CondExpr != nil {
		condExpr, conditionDiags := b.bindExpression(syntax.CondExpr)
		condition, diagnostics = condExpr, append(diagnostics, conditionDiags...)
	}

	tokens := b.tokens.ForNode(syntax)
	if tokens == nil {
		tokens = _syntax.NewForTokens(syntax.KeyVar, syntax.ValVar, syntax.KeyExpr != nil, syntax.Group,
			syntax.CondExpr != nil)
	}
	expr := &ForExpression{
		Syntax:        syntax,
		Tokens:        tokens,
		KeyVariable:   keyVariable,
		ValueVariable: valueVariable,
		Collection:    collection,
		Key:           key,
		Value:         value,
		Condition:     condition,
		Group:         syntax.Group,
	}
	typecheckDiags := expr.typecheck(false, false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindFunctionCallExpression binds a function call expression. The name of the function is bound using the current
// scope chain. An argument to a function must be assignable to the type of the corresponding parameter. It is not an
// error to omit arguments for trailing positional parameters if those parameters are optional. If any of the
// parameters to the function are eventual, the result type of the function is also eventual.
func (b *expressionBinder) bindFunctionCallExpression(
	syntax *hclsyntax.FunctionCallExpr) (Expression, hcl.Diagnostics) {

	var diagnostics hcl.Diagnostics

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.FunctionCallTokens)
	if tokens == nil {
		tokens = _syntax.NewFunctionCallTokens(syntax.Name, len(syntax.Args))
	}

	// Bind the function's arguments.
	args := make([]Expression, len(syntax.Args))
	for i, syntax := range syntax.Args {
		arg, argDiagnostics := b.bindExpression(syntax)
		args[i], diagnostics = arg, append(diagnostics, argDiagnostics...)
	}

	// Attempt to bind the name of the function to its definition.
	function, hasFunction := b.scope.BindFunctionReference(syntax.Name)
	if !hasFunction {
		diagnostics = append(diagnostics, unknownFunction(syntax.Name, syntax.NameRange))

		return &FunctionCallExpression{
			Syntax: syntax,
			Tokens: tokens,
			Name:   syntax.Name,
			Signature: StaticFunctionSignature{
				VarargsParameter: &Parameter{Name: "args", Type: DynamicType},
				ReturnType:       DynamicType,
			},
			Args: args,
		}, diagnostics
	}

	// Compute the function's signature.
	signature, sigDiags := function.GetSignature(args)
	diagnostics = append(diagnostics, sigDiags...)

	expr := &FunctionCallExpression{
		Syntax:    syntax,
		Tokens:    tokens,
		Name:      syntax.Name,
		Signature: signature,
		Args:      args,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindIndexExpression binds an index expression. The value being indexed must be an list, map, or object.
//
// - If the value is an list, the result type is the type of the list's elements, and the index must be assignable to
//   number (TODO(pdg): require integer indices?)
// - If the value is a map, the result type is the type of the map's values, and the index must be assignable to
//   string
// - If the value is an object, the result type is any, and the index must be assignable to a string
//
// If either the value being indexed or the index is eventual, result is eventual.
func (b *expressionBinder) bindIndexExpression(syntax *hclsyntax.IndexExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	collection, collectionDiags := b.bindExpression(syntax.Collection)
	diagnostics = append(diagnostics, collectionDiags...)

	key, keyDiags := b.bindExpression(syntax.Key)
	diagnostics = append(diagnostics, keyDiags...)

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.IndexTokens)
	if tokens == nil {
		tokens = _syntax.NewIndexTokens()
	}
	expr := &IndexExpression{
		Syntax:     syntax,
		Tokens:     tokens,
		Collection: collection,
		Key:        key,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindLiteralValueExpression binds a literal value expression. The value must be a boolean, integer, number, or
// string.
func (b *expressionBinder) bindLiteralValueExpression(
	syntax *hclsyntax.LiteralValueExpr) (Expression, hcl.Diagnostics) {

	var diagnostics hcl.Diagnostics

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.LiteralValueTokens)
	if tokens == nil {
		tokens = _syntax.NewLiteralValueTokens(syntax.Val)
	}
	expr := &LiteralValueExpression{
		Syntax: syntax,
		Tokens: tokens,
		Value:  syntax.Val,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindObjectConsExpression binds an object construction expression. The expression's keys must be strings. If any of
// the keys are not literal values, the result type is map(U), where U is the unified type of the property types.
// Otherwise, the result type is an object type that maps each key to the type of its value. If any of the keys is
// eventual, the result is eventual.
func (b *expressionBinder) bindObjectConsExpression(syntax *hclsyntax.ObjectConsExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	items := make([]ObjectConsItem, len(syntax.Items))
	for i, item := range syntax.Items {
		keyExpr, keyDiags := b.bindExpression(item.KeyExpr)
		diagnostics = append(diagnostics, keyDiags...)

		valExpr, valDiags := b.bindExpression(item.ValueExpr)
		diagnostics = append(diagnostics, valDiags...)

		items[i] = ObjectConsItem{Key: keyExpr, Value: valExpr}
	}

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.ObjectConsTokens)
	if tokens == nil {
		tokens = _syntax.NewObjectConsTokens(len(syntax.Items))
	}
	expr := &ObjectConsExpression{
		Syntax: syntax,
		Tokens: tokens,
		Items:  items,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindObjectConsKeyExpr binds an object construction key.
func (b *expressionBinder) bindObjectConsKeyExpr(syntax *hclsyntax.ObjectConsKeyExpr) (Expression, hcl.Diagnostics) {
	if !syntax.ForceNonLiteral {
		if name := hcl.ExprAsKeyword(syntax); name != "" {
			expr, diags := b.bindExpression(&hclsyntax.LiteralValueExpr{
				Val:      cty.StringVal(name),
				SrcRange: syntax.Range(),
			})
			lit := expr.(*LiteralValueExpression)
			lit.Tokens, _ = b.tokens.ForNode(syntax).(*_syntax.LiteralValueTokens)
			if lit.Tokens == nil {
				lit.Tokens = _syntax.NewLiteralValueTokens(cty.StringVal(name))
			}
			return lit, diags
		}
	}
	return b.bindExpression(syntax.Wrapped)
}

// bindRelativeTraversalExpression binds a relative traversal expression. Each step of the traversal must be a legal
// step with respect to its receiver. The root receiver is the type of the source expression.
func (b *expressionBinder) bindRelativeTraversalExpression(
	syntax *hclsyntax.RelativeTraversalExpr) (Expression, hcl.Diagnostics) {

	source, diagnostics := b.bindExpression(syntax.Source)

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.RelativeTraversalTokens)
	if tokens == nil {
		tokens = _syntax.NewRelativeTraversalTokens(syntax.Traversal)
	}

	// This occurs with splat expressions.
	if source, ok := source.(*ScopeTraversalExpression); ok {
		source.Syntax.Traversal = append(source.Syntax.Traversal, syntax.Traversal...)
		source.Syntax.SrcRange = hcl.RangeBetween(source.Syntax.SrcRange, syntax.SrcRange)
		source.Tokens.Traversal = append(source.Tokens.Traversal, tokens.Traversal...)

		source.Traversal = source.Syntax.Traversal
		return source, source.Typecheck(false)
	}

	expr := &RelativeTraversalExpression{
		Syntax:    syntax,
		Tokens:    tokens,
		Source:    source,
		Traversal: syntax.Traversal,
	}
	typecheckDiags := expr.typecheck(false, b.options.allowMissingVariables)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindScopeTraversalExpression binds a scope traversal expression. Each step of the traversal must be a legal
// step with respect to its receiver. The root receiver is the definition in the current scope referred to by the root
// name.
func (b *expressionBinder) bindScopeTraversalExpression(
	syntax *hclsyntax.ScopeTraversalExpr) (Expression, hcl.Diagnostics) {

	var diagnostics hcl.Diagnostics

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.ScopeTraversalTokens)
	if tokens == nil {
		tokens = _syntax.NewScopeTraversalTokens(syntax.Traversal)
	}

	rootName := syntax.Traversal.RootName()
	def, ok := b.scope.BindReference(rootName)
	if !ok {
		parts := make([]Traversable, len(syntax.Traversal))
		for i := range parts {
			parts[i] = DynamicType
		}

		if !b.options.allowMissingVariables {
			diagnostics = hcl.Diagnostics{
				undefinedVariable(rootName, syntax.Traversal.SimpleSplit().Abs.SourceRange()),
			}
		}
		return &ScopeTraversalExpression{
			Syntax:    syntax,
			Tokens:    tokens,
			Parts:     parts,
			RootName:  syntax.Traversal.RootName(),
			Traversal: syntax.Traversal,
		}, diagnostics
	}

	expr := &ScopeTraversalExpression{
		Syntax:    syntax,
		Tokens:    tokens,
		Parts:     []Traversable{def},
		RootName:  syntax.Traversal.RootName(),
		Traversal: syntax.Traversal,
	}
	typecheckDiags := expr.typecheck(false, b.options.allowMissingVariables)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindSplatExpression binds a splat expression. If the type being splatted is an list or any, the type of the
// iteration variable is the element type of the list or any, respectively. Otherwise, the type of the iteration
// variable is the type being splatted: in this case, the splat expression implicitly constructs a single-element
// tuple. The type of the result is an list whose elements have the type of the each expression. If the value being
// splatted is eventual or optional, the result type is eventual or optional.
func (b *expressionBinder) bindSplatExpression(syntax *hclsyntax.SplatExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	source, sourceDiags := b.bindExpression(syntax.Source)
	diagnostics = append(diagnostics, sourceDiags...)

	item := &SplatVariable{}
	b.anonSymbols[syntax.Item] = item

	source, item.VariableType = splatItemType(source, syntax)

	each, eachDiags := b.bindExpression(syntax.Each)
	diagnostics = append(diagnostics, eachDiags...)

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.SplatTokens)
	if tokens == nil {
		tokens = _syntax.NewSplatTokens(false)
	}
	expr := &SplatExpression{
		Syntax: syntax,
		Tokens: tokens,
		Source: source,
		Each:   each,
		Item:   item,
	}
	typecheckDiags := expr.typecheck(false, false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindTemplateExpression binds a template expression. The result is always a string. If any of the parts of the
// expression are eventual, the result is eventual.
func (b *expressionBinder) bindTemplateExpression(syntax *hclsyntax.TemplateExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics
	parts := make([]Expression, len(syntax.Parts))
	for i, syntax := range syntax.Parts {
		part, partDiags := b.bindExpression(syntax)
		parts[i], diagnostics = part, append(diagnostics, partDiags...)
	}

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.TemplateTokens)
	if tokens == nil {
		tokens = _syntax.NewTemplateTokens()
	}
	expr := &TemplateExpression{
		Syntax: syntax,
		Tokens: tokens,
		Parts:  parts,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindTemplateJoinExpression binds a template join expression. If any of the parts of the expression are eventual,
// the result is eventual.
func (b *expressionBinder) bindTemplateJoinExpression(
	syntax *hclsyntax.TemplateJoinExpr) (Expression, hcl.Diagnostics) {

	tuple, diagnostics := b.bindExpression(syntax.Tuple)

	expr := &TemplateJoinExpression{
		Syntax: syntax,
		Tuple:  tuple,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindTemplateWrapExpression binds a template wrap expression.
func (b *expressionBinder) bindTemplateWrapExpression(
	syntax *hclsyntax.TemplateWrapExpr) (Expression, hcl.Diagnostics) {

	wrapped, diagnostics := b.bindExpression(syntax.Wrapped)
	if tokens, hasTokens := b.tokens.ForNode(syntax).(*_syntax.TemplateTokens); hasTokens {
		wrapped.SetLeadingTrivia(append(wrapped.GetLeadingTrivia(), tokens.Open.LeadingTrivia...))
		wrapped.SetTrailingTrivia(append(wrapped.GetTrailingTrivia(), tokens.Close.TrailingTrivia...))
	}
	return wrapped, diagnostics
}

// bindTupleConsExpression binds a tuple construction expression. The result is a tuple(T_0, ..., T_N).
func (b *expressionBinder) bindTupleConsExpression(syntax *hclsyntax.TupleConsExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	exprs := make([]Expression, len(syntax.Exprs))
	for i, syntax := range syntax.Exprs {
		expr, exprDiags := b.bindExpression(syntax)
		exprs[i], diagnostics = expr, append(diagnostics, exprDiags...)
	}

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.TupleConsTokens)
	if tokens == nil {
		tokens = _syntax.NewTupleConsTokens(len(syntax.Exprs))
	}
	expr := &TupleConsExpression{
		Syntax:      syntax,
		Tokens:      tokens,
		Expressions: exprs,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}

// bindUnaryOpExpression binds a unary operator expression. If the operand to the unary operator contains eventuals,
// the result of the unary operator is eventual.
func (b *expressionBinder) bindUnaryOpExpression(syntax *hclsyntax.UnaryOpExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	// Bind the operand.
	operand, operandDiags := b.bindExpression(syntax.Val)
	diagnostics = append(diagnostics, operandDiags...)

	tokens, _ := b.tokens.ForNode(syntax).(*_syntax.UnaryOpTokens)
	if tokens == nil {
		tokens = _syntax.NewUnaryOpTokens(syntax.Op)
	}
	expr := &UnaryOpExpression{
		Syntax:    syntax,
		Tokens:    tokens,
		Operation: syntax.Op,
		Operand:   operand,
	}
	typecheckDiags := expr.Typecheck(false)
	diagnostics = append(diagnostics, typecheckDiags...)

	return expr, diagnostics
}
