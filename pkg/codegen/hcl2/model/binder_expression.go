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
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type expressionBinder struct {
	anonSymbols map[*hclsyntax.AnonSymbolExpr]Definition
	scope       *Scope
}

// BindExpression binds an HCL2 expression using the given scope and token map.
func BindExpression(syntax hclsyntax.Node, scope *Scope, tokens syntax.TokenMap) (Expression, hcl.Diagnostics) {
	b := &expressionBinder{
		anonSymbols: map[*hclsyntax.AnonSymbolExpr]Definition{},
		scope:       scope,
	}

	return b.bindExpression(syntax)
}

// BindExpressionText parses and binds an HCL2 expression using the given scope.
func BindExpressionText(source string, scope *Scope) (Expression, hcl.Diagnostics) {
	syntax, tokens, diagnostics := syntax.ParseExpression(source, "<anonymous>", hcl.InitialPos)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return BindExpression(syntax, scope, tokens)
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
			Type: InputType(ctyTypeToType(p.Type, p.AllowNull)),
		}
	}
	if p := op.Impl.VarParam(); p != nil {
		sig.VarargsParameter = &Parameter{
			Name: p.Name,
			Type: InputType(ctyTypeToType(p.Type, p.AllowNull)),
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
			if !param.Type.ConversionFrom(remainingArgs[0].Type()).Exists() {
				diagnostics = append(diagnostics, ExprNotConvertible(param.Type, remainingArgs[0]))
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
				if !varargs.Type.ConversionFrom(arg.Type()).Exists() {
					diagnostics = append(diagnostics, ExprNotConvertible(varargs.Type, arg))
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

	return &ScopeTraversalExpression{
		Syntax: &hclsyntax.ScopeTraversalExpr{
			Traversal: hcl.Traversal{hcl.TraverseRoot{Name: "<anonymous>", SrcRange: syntax.SrcRange}},
			SrcRange:  syntax.SrcRange,
		},
		Parts: []Traversable{lv},
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

	// Compute the signature for the operator and typecheck the arguments.
	signature := getOperationSignature(syntax.Op)
	contract.Assert(len(signature.Parameters) == 2)

	typecheckDiags := typecheckArgs(syntax.Range(), signature, leftOperand, rightOperand)
	diagnostics = append(diagnostics, typecheckDiags...)

	return &BinaryOpExpression{
		Syntax:       syntax,
		LeftOperand:  leftOperand,
		RightOperand: rightOperand,
		exprType:     liftOperationType(signature.ReturnType, leftOperand, rightOperand),
	}, diagnostics
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

	// Compute the type of the result.
	resultType, _ := UnifyTypes(trueResult.Type(), falseResult.Type())

	// Typecheck the condition expression.
	signature := StaticFunctionSignature{Parameters: []Parameter{{Name: "condition", Type: InputType(BoolType)}}}
	typecheckDiags := typecheckArgs(syntax.Range(), signature, condition)
	diagnostics = append(diagnostics, typecheckDiags...)

	return &ConditionalExpression{
		Syntax:      syntax,
		Condition:   condition,
		TrueResult:  trueResult,
		FalseResult: falseResult,
		exprType:    liftOperationType(resultType, condition),
	}, diagnostics
}

// unwrapIterableSourceType removes any eventual types that wrap a type intended for iteration.
func unwrapIterableSourceType(t Type) Type {
	// TODO(pdg): unions
	for {
		switch tt := t.(type) {
		case *OutputType:
			t = tt.ElementType
		case *PromiseType:
			t = tt.ElementType
		default:
			return t
		}
	}
}

// wrapIterableSourceType adds optional or eventual types to a type intended for iteration per the structure of the
// source type.
func wrapIterableResultType(sourceType, iterableType Type) Type {
	// TODO(pdg): unions
	for {
		switch t := sourceType.(type) {
		case *OutputType:
			sourceType, iterableType = t.ElementType, NewOutputType(iterableType)
		case *PromiseType:
			sourceType, iterableType = t.ElementType, NewPromiseType(iterableType)
		default:
			return iterableType
		}
	}
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

	var keyType, valueType Type
	switch collectionType := collectionType.(type) {
	case *ListType:
		keyType, valueType = NumberType, collectionType.ElementType
	case *MapType:
		keyType, valueType = StringType, collectionType.ElementType
	case *TupleType:
		keyType = NumberType
		valueType, _ = UnifyTypes(collectionType.ElementTypes...)
	case *ObjectType:
		keyType = StringType

		types := make([]Type, 0, len(collectionType.Properties))
		for _, t := range collectionType.Properties {
			types = append(types, t)
		}
		valueType, _ = UnifyTypes(types...)
	default:
		// If the collection is a dynamic type, treat it as an iterable(dynamic, dynamic). Otherwise, issue an error.
		if collectionType != DynamicType {
			diagnostics = append(diagnostics, unsupportedCollectionType(collectionType, syntax.CollExpr.Range()))
		}
		keyType, valueType = DynamicType, DynamicType
	}

	// Push a scope for the key and value variables and define these vars.
	b.scope = b.scope.Push(syntax)
	defer func() { b.scope = b.scope.Pop() }()
	if syntax.KeyVar != "" {
		ok := b.scope.Define(syntax.KeyVar, &Variable{Name: syntax.KeyVar, VariableType: keyType})
		contract.Assert(ok)
	}
	if ok := b.scope.Define(syntax.ValVar, &Variable{Name: syntax.ValVar, VariableType: valueType}); !ok {
		diagnostics = append(diagnostics, nameAlreadyDefined(syntax.ValVar, syntax.Range()))
	}

	var key Expression
	if syntax.KeyExpr != nil {
		keyExpr, keyDiags := b.bindExpression(syntax.KeyExpr)
		key, diagnostics = keyExpr, append(diagnostics, keyDiags...)

		// A key expression is only present when producing a map. Key types must therefore be strings.
		if !InputType(StringType).ConversionFrom(key.Type()).Exists() {
			diagnostics = append(diagnostics, ExprNotConvertible(InputType(StringType), key))
		}
	}

	value, valueDiags := b.bindExpression(syntax.ValExpr)
	diagnostics = append(diagnostics, valueDiags...)

	var condition Expression
	if syntax.CondExpr != nil {
		condExpr, conditionDiags := b.bindExpression(syntax.CondExpr)
		condition, diagnostics = condExpr, append(diagnostics, conditionDiags...)

		if !InputType(BoolType).ConversionFrom(condition.Type()).Exists() {
			diagnostics = append(diagnostics, ExprNotConvertible(InputType(BoolType), condition))
		}
	}

	// If there is a key expression, we are producing a map. Otherwise, we are producing an list. In either case, wrap
	// the result type in the same set of eventuals and optionals present in the collection type.
	var resultType Type
	if key != nil {
		valueType := value.Type()
		if syntax.Group {
			valueType = NewListType(valueType)
		}
		resultType = wrapIterableResultType(collection.Type(), NewMapType(valueType))
	} else {
		resultType = wrapIterableResultType(collection.Type(), NewListType(value.Type()))
	}

	// If either the key expression or the condition expression is eventual, the result is eventual: each of these
	// values is required to determine which items are present in the result.
	var liftArgs []Expression
	if key != nil {
		liftArgs = append(liftArgs, key)
	}
	if condition != nil {
		liftArgs = append(liftArgs, condition)
	}

	return &ForExpression{
		Syntax:     syntax,
		Collection: collection,
		Key:        key,
		Value:      value,
		exprType:   liftOperationType(resultType, liftArgs...),
	}, diagnostics
}

// bindFunctionCallExpression binds a function call expression. The name of the function is bound using the current
// scope chain. An argument to a function must be assignable to the type of the corresponding parameter. It is not an
// error to omit arguments for trailing positional parameters if those parameters are optional. If any of the
// parameters to the function are eventual, the result type of the function is also eventual.
func (b *expressionBinder) bindFunctionCallExpression(
	syntax *hclsyntax.FunctionCallExpr) (Expression, hcl.Diagnostics) {

	var diagnostics hcl.Diagnostics

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

	// Wrap the function's parameter types in input types s.t. they accept eventual arguments.
	for i := range signature.Parameters {
		signature.Parameters[i].Type = InputType(signature.Parameters[i].Type)
	}
	if signature.VarargsParameter != nil {
		signature.VarargsParameter.Type = InputType(signature.VarargsParameter.Type)
	}

	// Typecheck the function's arguments.
	typecheckDiags := typecheckArgs(syntax.Range(), signature, args...)
	diagnostics = append(diagnostics, typecheckDiags...)

	signature.ReturnType = liftOperationType(signature.ReturnType, args...)

	return &FunctionCallExpression{
		Syntax:    syntax,
		Name:      syntax.Name,
		Signature: signature,
		Args:      args,
	}, diagnostics
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

	part, partDiags := collection.Type().Traverse(hcl.TraverseIndex{
		Key:      encapsulateType(key.Type()),
		SrcRange: syntax.Key.Range(),
	})

	diagnostics = append(diagnostics, partDiags...)

	return &IndexExpression{
		Syntax:     syntax,
		Collection: collection,
		Key:        key,
		exprType:   liftOperationType(part.(Type), collection, key),
	}, diagnostics
}

// bindLiteralValueExpression binds a literal value expression. The value must be a boolean, integer, number, or
// string.
func (b *expressionBinder) bindLiteralValueExpression(
	syntax *hclsyntax.LiteralValueExpr) (Expression, hcl.Diagnostics) {

	v, typ, diagnostics := syntax.Val, NoneType, hcl.Diagnostics(nil)
	if !v.IsNull() {
		typ = ctyTypeToType(v.Type(), false)
	}

	switch {
	case typ == NoneType || typ == StringType || typ == IntType || typ == NumberType || typ == BoolType:
		// OK
	default:
		typ, diagnostics = DynamicType, hcl.Diagnostics{unsupportedLiteralValue(syntax)}
	}

	return &LiteralValueExpression{
		Syntax:   syntax,
		Value:    v,
		exprType: typ,
	}, diagnostics
}

// bindObjectConsExpression binds an object construction expression. The expression's keys must be strings. If any of
// the keys are not literal values, the result type is map(U), where U is the unified type of the property types.
// Otherwise, the result type is an object type that maps each key to the type of its value. If any of the keys is
// eventual, the result is eventual.
func (b *expressionBinder) bindObjectConsExpression(syntax *hclsyntax.ObjectConsExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	keys := make([]Expression, len(syntax.Items))
	items := make([]ObjectConsItem, len(syntax.Items))
	for i, item := range syntax.Items {
		keyExpr, keyDiags := b.bindExpression(item.KeyExpr)
		diagnostics = append(diagnostics, keyDiags...)
		keys[i] = keyExpr

		if !InputType(StringType).ConversionFrom(keyExpr.Type()).Exists() {
			diagnostics = append(diagnostics, objectKeysMustBeStrings(keyExpr))
		}

		valExpr, valDiags := b.bindExpression(item.ValueExpr)
		diagnostics = append(diagnostics, valDiags...)

		items[i] = ObjectConsItem{Key: keyExpr, Value: valExpr}
	}

	// Attempt to build an object type out of the result. If there are any attribute names that come from variables,
	// type the result as map(unify(propertyTypes)).
	properties, isMapType, types := map[string]Type{}, false, []Type{}
	for _, item := range items {
		types = append(types, item.Value.Type())

		keyLit, ok := item.Key.(*LiteralValueExpression)
		if ok {
			key, err := convert.Convert(keyLit.Value, cty.String)
			if err == nil {
				properties[key.AsString()] = item.Value.Type()
				continue
			}
		}
		isMapType = true
	}
	var typ Type
	if isMapType {
		elementType, _ := UnifyTypes(types...)
		typ = NewMapType(elementType)
	} else {
		typ = NewObjectType(properties)
	}

	return &ObjectConsExpression{
		Syntax:   syntax,
		Items:    items,
		exprType: liftOperationType(typ, keys...),
	}, diagnostics
}

// bindObjectConsKeyExpr binds an object construction key.
func (b *expressionBinder) bindObjectConsKeyExpr(syntax *hclsyntax.ObjectConsKeyExpr) (Expression, hcl.Diagnostics) {
	if !syntax.ForceNonLiteral {
		if name := hcl.ExprAsKeyword(syntax); name != "" {
			return b.bindExpression(&hclsyntax.LiteralValueExpr{
				Val:      cty.StringVal(name),
				SrcRange: syntax.Range(),
			})
		}
	}
	return b.bindExpression(syntax.Wrapped)
}

// bindRelativeTraversalExpression binds a relative traversal expression. Each step of the traversal must be a legal
// step with respect to its receiver. The root receiver is the type of the source expression.
func (b *expressionBinder) bindRelativeTraversalExpression(
	syntax *hclsyntax.RelativeTraversalExpr) (Expression, hcl.Diagnostics) {

	source, diagnostics := b.bindExpression(syntax.Source)

	parts, partDiags := b.bindTraversalParts(source.Type(), syntax.Traversal)
	diagnostics = append(diagnostics, partDiags...)

	return &RelativeTraversalExpression{
		Syntax: syntax,
		Source: source,
		Parts:  parts,
	}, diagnostics
}

// bindScopeTraversalExpression binds a scope traversal expression. Each step of the traversal must be a legal
// step with respect to its receiver. The root receiver is the definition in the current scope referred to by the root
// name.
func (b *expressionBinder) bindScopeTraversalExpression(
	syntax *hclsyntax.ScopeTraversalExpr) (Expression, hcl.Diagnostics) {

	def, ok := b.scope.BindReference(syntax.Traversal.RootName())
	if !ok {
		parts := make([]Traversable, len(syntax.Traversal))
		for i := range parts {
			parts[i] = DynamicType
		}
		return &ScopeTraversalExpression{
			Syntax: syntax,
			Parts:  parts,
		}, hcl.Diagnostics{undefinedVariable(syntax.Traversal.SimpleSplit().Abs.SourceRange())}
	}

	parts, diagnostics := b.bindTraversalParts(def, syntax.Traversal.SimpleSplit().Rel)
	return &ScopeTraversalExpression{
		Syntax: syntax,
		Parts:  parts,
	}, diagnostics
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

	sourceType := unwrapIterableSourceType(source.Type())
	elementType := sourceType
	if arr, isList := sourceType.(*ListType); isList {
		elementType = arr.ElementType
	} else if sourceType != DynamicType {
		source = &TupleConsExpression{
			Syntax: &hclsyntax.TupleConsExpr{
				Exprs:     []hclsyntax.Expression{syntax.Source},
				SrcRange:  syntax.Source.Range(),
				OpenRange: syntax.Source.StartRange(),
			},
			Expressions: []Expression{source},
			exprType:    NewListType(source.Type()),
		}
	}

	item := &Variable{
		Name:         "<anonymous>",
		VariableType: elementType,
	}
	b.anonSymbols[syntax.Item] = item

	each, eachDiags := b.bindExpression(syntax.Each)
	diagnostics = append(diagnostics, eachDiags...)

	return &SplatExpression{
		Syntax:   syntax,
		Source:   source,
		Each:     each,
		Item:     item,
		exprType: wrapIterableResultType(source.Type(), NewListType(each.Type())),
	}, diagnostics
}

// bindTemplateExpression binds a template expression. The result is always a string. If any of the parts of the
// expression are eventual, the result is eventual.
func (b *expressionBinder) bindTemplateExpression(syntax *hclsyntax.TemplateExpr) (Expression, hcl.Diagnostics) {
	if syntax.IsStringLiteral() {
		return b.bindExpression(syntax.Parts[0])
	}

	var diagnostics hcl.Diagnostics
	parts := make([]Expression, len(syntax.Parts))
	for i, syntax := range syntax.Parts {
		part, partDiags := b.bindExpression(syntax)
		parts[i], diagnostics = part, append(diagnostics, partDiags...)
	}

	return &TemplateExpression{
		Syntax:   syntax,
		Parts:    parts,
		exprType: liftOperationType(StringType, parts...),
	}, diagnostics
}

// bindTemplateJoinExpression binds a template join expression. If any of the parts of the expression are eventual,
// the result is eventual.
func (b *expressionBinder) bindTemplateJoinExpression(
	syntax *hclsyntax.TemplateJoinExpr) (Expression, hcl.Diagnostics) {

	tuple, diagnostics := b.bindExpression(syntax.Tuple)

	return &TemplateJoinExpression{
		Syntax:   syntax,
		Tuple:    tuple,
		exprType: liftOperationType(StringType, tuple),
	}, diagnostics
}

// bindTemplateWrapExpression binds a template wrap expression.
func (b *expressionBinder) bindTemplateWrapExpression(
	syntax *hclsyntax.TemplateWrapExpr) (Expression, hcl.Diagnostics) {

	return b.bindExpression(syntax.Wrapped)
}

// bindTupleConsExpression binds a tuple construction expression. The result is a tuple(T_0, ..., T_N).
func (b *expressionBinder) bindTupleConsExpression(syntax *hclsyntax.TupleConsExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics
	exprs := make([]Expression, len(syntax.Exprs))
	for i, syntax := range syntax.Exprs {
		expr, exprDiags := b.bindExpression(syntax)
		exprs[i], diagnostics = expr, append(diagnostics, exprDiags...)
	}

	elementTypes := make([]Type, len(exprs))
	for i, expr := range exprs {
		elementTypes[i] = expr.Type()
	}

	return &TupleConsExpression{
		Syntax:      syntax,
		Expressions: exprs,
		exprType:    NewTupleType(elementTypes...),
	}, diagnostics
}

// bindUnaryOpExpression binds a unary operator expression. If the operand to the unary operator contains eventuals,
// the result of the unary operator is eventual.
func (b *expressionBinder) bindUnaryOpExpression(syntax *hclsyntax.UnaryOpExpr) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	// Bind the operand.
	operand, operandDiags := b.bindExpression(syntax.Val)
	diagnostics = append(diagnostics, operandDiags...)

	// Compute the signature for the operator and typecheck the arguments.
	signature := getOperationSignature(syntax.Op)
	contract.Assert(len(signature.Parameters) == 1)

	typecheckDiags := typecheckArgs(syntax.Range(), signature, operand)
	diagnostics = append(diagnostics, typecheckDiags...)

	return &UnaryOpExpression{
		Syntax:   syntax,
		Operand:  operand,
		exprType: liftOperationType(signature.ReturnType, operand),
	}, diagnostics
}
