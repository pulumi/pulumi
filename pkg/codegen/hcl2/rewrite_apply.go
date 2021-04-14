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

	"github.com/gedex/inflector"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type NameInfo interface {
	Format(name string) string
}

// The applyRewriter is responsible for driving the apply rewrite process. The rewriter uses a stack of contexts to
// deal with the possibility of expressions that observe outputs nested inside expressions that do not.
type applyRewriter struct {
	nameInfo      NameInfo
	applyPromises bool

	activeContext applyRewriteContext
	exprStack     []model.Expression
}

type applyRewriteContext interface {
	PreVisit(x model.Expression) (model.Expression, hcl.Diagnostics)
	PostVisit(x model.Expression) (model.Expression, hcl.Diagnostics)
}

// An inspectContext is used when we are inside an expression that does not observe eventual values. When it
// encounters an expression that observes eventual values, it pushes a new observeContext onto the stack.
type inspectContext struct {
	*applyRewriter

	parent *observeContext

	root model.Expression
}

// An observeContext is used when we are inside an expression that does observe eventual values. It is responsible for
// finding the values that are observed, replacing them with references to apply parameters, and replacing the root
// expression with a call to the __apply intrinsic.
type observeContext struct {
	*applyRewriter

	parent applyRewriteContext

	root            model.Expression
	applyArgs       []model.Expression
	callbackParams  []*model.Variable
	paramReferences []*model.ScopeTraversalExpression

	assignedNames codegen.StringSet
	nameCounts    map[string]int
}

func (r *applyRewriter) hasEventualTypes(t model.Type) bool {
	resolved := model.ResolveOutputs(t)
	return resolved != t
}

func (r *applyRewriter) hasEventualValues(x model.Expression) bool {
	return r.hasEventualTypes(x.Type())
}

func (r *applyRewriter) isEventualType(t model.Type) (model.Type, bool) {
	switch t := t.(type) {
	case *model.OutputType:
		return t.ElementType, true
	case *model.PromiseType:
		if r.applyPromises {
			return t.ElementType, true
		}
	case *model.UnionType:
		types, isEventual := make([]model.Type, len(t.ElementTypes)), false
		for i, t := range t.ElementTypes {
			if element, elementIsEventual := r.isEventualType(t); elementIsEventual {
				t, isEventual = element, true
			}
			types[i] = t
		}
		if isEventual {
			return model.NewUnionType(types...), true
		}
	}
	return nil, false
}

func (r *applyRewriter) hasEventualElements(x model.Expression) bool {
	t := x.Type()
	if resolved, ok := r.isEventualType(t); ok {
		t = resolved
	}
	return r.hasEventualTypes(t)
}

func (r *applyRewriter) isPromptArg(paramType model.Type, arg model.Expression) bool {
	if !r.hasEventualValues(arg) {
		return true
	}

	if union, ok := paramType.(*model.UnionType); ok {
		for _, t := range union.ElementTypes {
			if t != model.DynamicType && t.ConversionFrom(arg.Type()) != model.NoConversion {
				return true
			}
		}
		return false
	}
	return paramType != model.DynamicType && paramType.ConversionFrom(arg.Type()) != model.NoConversion
}

func (r *applyRewriter) isIteratorExpr(x model.Expression) (bool, model.Type) {
	if len(r.exprStack) < 2 {
		return false, nil
	}

	parent := r.exprStack[len(r.exprStack)-2]
	switch parent := parent.(type) {
	case *model.ForExpression:
		return x != parent.Collection, parent.ValueVariable.Type()
	case *model.SplatExpression:
		return x != parent.Source, parent.Item.Type()
	default:
		return false, nil
	}
}

func (r *applyRewriter) inspectsEventualValues(x model.Expression) bool {
	switch x := x.(type) {
	case *model.ConditionalExpression:
		return r.hasEventualValues(x.TrueResult) || r.hasEventualValues(x.FalseResult)
	case *model.ForExpression:
		return r.hasEventualElements(x.Collection)
	case *model.FunctionCallExpression:
		_, isEventual := r.isEventualType(x.Signature.ReturnType)
		if isEventual {
			return true
		}
		for i, arg := range x.Args {
			if r.hasEventualValues(arg) && r.isPromptArg(x.Signature.Parameters[i].Type, arg) {
				return true
			}
		}
		return false
	case *model.IndexExpression:
		_, isCollectionEventual := r.isEventualType(x.Collection.Type())
		return !isCollectionEventual && r.hasEventualValues(x.Collection)
	case *model.SplatExpression:
		return r.hasEventualElements(x.Source)
	default:
		if isIteratorExpr, elementType := r.isIteratorExpr(x); isIteratorExpr {
			_, isElementEventual := r.isEventualType(elementType)
			return !isElementEventual && r.hasEventualTypes(elementType)
		}
		return false
	}
}

func (r *applyRewriter) observesEventualValues(x model.Expression) bool {
	_, isEventual := r.isEventualType(x.Type())
	if !isEventual {
		return false
	}

	switch x := x.(type) {
	case *model.AnonymousFunctionExpression:
		return false
	case *model.ConditionalExpression:
		return r.hasEventualValues(x.Condition)
	case *model.ForExpression:
		_, collectionIsEventual := r.isEventualType(x.Collection.Type())
		return collectionIsEventual
	case *model.FunctionCallExpression:
		for i, arg := range x.Args {
			if !r.isPromptArg(x.Signature.Parameters[i].Type, arg) {
				return true
			}
		}
		return false
	case *model.IndexExpression:
		if _, collectionIsEventual := r.isEventualType(x.Collection.Type()); collectionIsEventual {
			return true
		}
		return r.hasEventualValues(x.Key)
	case *model.RelativeTraversalExpression:
		// A traversal is eventual if at least one of its nonterminals is eventual.
		for _, p := range x.Parts[:len(x.Parts)-1] {
			if _, isEventual := r.isEventualType(model.GetTraversableType(p)); isEventual {
				return true
			}
		}
		return false
	case *model.ScopeTraversalExpression:
		// A traversal is eventual if at least one of its nonterminals is eventual.
		for _, p := range x.Parts[:len(x.Parts)-1] {
			if _, isEventual := r.isEventualType(model.GetTraversableType(p)); isEventual {
				return true
			}
		}
		return false
	case *model.SplatExpression:
		_, sourceIsEventual := r.isEventualType(x.Source.Type())
		return sourceIsEventual
	default:
		return true
	}
}

func (r *applyRewriter) preVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	r.exprStack = append(r.exprStack, expr)
	return r.activeContext.PreVisit(expr)
}

func (r *applyRewriter) postVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	x, diags := r.activeContext.PostVisit(expr)
	r.exprStack = r.exprStack[:len(r.exprStack)-1]
	return x, diags
}

// disambiguateName ensures that the given name is unambiguous by appending an integer starting with 1 if necessary.
func (ctx *observeContext) disambiguateName(name string) string {
	if name == "" {
		name = "arg"
	}

	if !ctx.assignedNames.Has(name) {
		return name
	}

	root := name
	for i := 1; ctx.nameCounts[name] != 0; i++ {
		name = fmt.Sprintf("%s%d", root, i)
	}
	return name
}

func (ctx *observeContext) bestTraversalName(rootName string, traversal hcl.Traversal) string {
	for i := len(traversal) - 1; i >= 0; i-- {
		switch t := traversal[i].(type) {
		case hcl.TraverseAttr:
			return t.Name
		case hcl.TraverseIndex:
			if t.Key.Type().Equals(cty.String) {
				return t.Key.AsString()
			}
			return inflector.Singularize(ctx.bestTraversalName(rootName, traversal[:i]))
		}
	}
	return rootName
}

// bestArgName computes the "best" name for a given apply argument. If this name is unambiguous after all best names
// have been calculated, it will be assigned to the argument. Otherwise, it will go through the disambiguation process
// in disambiguateArgName.
func (ctx *observeContext) bestArgName(x model.Expression) string {
	switch x := x.(type) {
	case *model.ForExpression:
		if x.Key == nil {
			return inflector.Pluralize(ctx.bestArgName(x.Value))
		}
	case *model.FunctionCallExpression:
		switch x.Name {
		case IntrinsicApply:
			_, then := ParseApplyCall(x)
			return ctx.bestArgName(then.Body)
		case "element":
			return ctx.bestArgName(x.Args[0])
		case "fileArchive", "fileAsset", "readDir", "readFile":
			return ctx.bestArgName(x.Args[0])
		case "lookup":
			return ctx.bestArgName(x.Args[1])
		}
		return x.Name
	case *model.IndexExpression:
		switch model.ResolveOutputs(x.Collection.Type()).(type) {
		case *model.ListType, *model.SetType, *model.TupleType:
			return inflector.Singularize(ctx.bestArgName(x.Collection))
		case *model.MapType, *model.ObjectType:
			return ctx.bestArgName(x.Key)
		}
	case *model.LiteralValueExpression:
		if x.Value.Type().Equals(cty.String) {
			return x.Value.AsString()
		}
	case *model.RelativeTraversalExpression:
		if n := ctx.bestTraversalName(ctx.bestArgName(x.Source), x.Traversal); n != "" {
			return n
		}
	case *model.ScopeTraversalExpression:
		if n := ctx.bestTraversalName(x.RootName, x.Traversal[1:]); n != "" {
			return n
		}
	case *model.SplatExpression:
		return inflector.Pluralize(ctx.bestArgName(x.Each))
	}

	switch t := model.ResolveOutputs(x.Type()).(type) {
	case *model.ListType, *model.SetType, *model.TupleType:
		return "values"
	case *model.MapType, *model.ObjectType:
		return "obj"
	case *model.UnionType:
		return "value"
	default:
		switch t {
		case model.BoolType:
			return "b"
		case model.IntType:
			return "i"
		case model.NumberType:
			return "n"
		case model.StringType:
			return "s"
		case model.DynamicType:
			return "obj"
		default:
			return "v"
		}
	}
}

// disambiguateArgName applies type-specific disambiguation to an argument name.
func (ctx *observeContext) disambiguateArgName(x model.Expression, bestName string) string {
	if x, ok := x.(*model.ScopeTraversalExpression); ok {
		if n, ok := x.Parts[0].(*Resource); ok {
			// If dealing with a broken access, defer to the generic disambiguator. Otherwise, attempt to disambiguate
			// by prepending the resource's variable name.
			if len(x.Traversal) > 1 {
				return ctx.disambiguateName(n.Name() + titleCase(bestName))
			}
		}
	}
	// Hand off to the generic disambiguator.
	return ctx.disambiguateName(bestName)
}

// rewriteApplyArg replaces a single expression with an apply parameter.
func (ctx *observeContext) rewriteApplyArg(applyArg model.Expression, paramType model.Type, traversal hcl.Traversal,
	parts []model.Traversable, isRoot bool) model.Expression {

	if len(traversal) == 0 && isRoot {
		return applyArg
	}

	callbackParam := &model.Variable{
		Name:         fmt.Sprintf("<arg%d>", len(ctx.callbackParams)),
		VariableType: paramType,
	}

	ctx.applyArgs, ctx.callbackParams = append(ctx.applyArgs, applyArg), append(ctx.callbackParams, callbackParam)

	// TODO(pdg): this risks information loss for nested output-typed properties... The `Types` array on traversals
	// ought to store the original types.
	resolvedParts := make([]model.Traversable, len(parts)+1)
	resolvedParts[0] = callbackParam
	for i, p := range parts {
		resolvedParts[i+1] = model.ResolveOutputs(model.GetTraversableType(p))
	}

	result := &model.ScopeTraversalExpression{
		Parts:     resolvedParts,
		RootName:  callbackParam.Name,
		Traversal: hcl.TraversalJoin(hcl.Traversal{hcl.TraverseRoot{Name: callbackParam.Name}}, traversal),
	}
	ctx.paramReferences = append(ctx.paramReferences, result)
	return result
}

// rewriteRelativeTraversalExpression replaces a single access to an ouptut-typed RelativeTraversalExpression with an
// apply parameter.
func (ctx *observeContext) rewriteRelativeTraversalExpression(expr *model.RelativeTraversalExpression,
	isRoot bool) model.Expression {

	// If the access is not an output() or a promise(), return the node as-is.
	paramType, isEventual := ctx.isEventualType(expr.Type())
	if !isEventual {
		return expr
	}

	// If the receiver is an eventual type, we're done.
	if receiverResolvedType, isEventual := ctx.isEventualType(model.GetTraversableType(expr.Parts[0])); isEventual {
		return ctx.rewriteApplyArg(expr.Source, receiverResolvedType, expr.Traversal, expr.Parts[1:], isRoot)
	}

	// Compute the type of the apply and callback arguments.
	parts, traversal := expr.Parts, expr.Traversal
	for i := range expr.Traversal {
		partResolvedType, isEventual := paramType, true
		if i < len(expr.Traversal)-1 {
			partResolvedType, isEventual = ctx.isEventualType(model.GetTraversableType(expr.Parts[i+1]))
		}
		if isEventual {
			expr.Traversal, expr.Parts = expr.Traversal[:i+1], expr.Parts[:i+1]
			paramType, traversal, parts = partResolvedType, expr.Traversal[i+1:], expr.Parts[i+1:]
			break
		}
	}

	return ctx.rewriteApplyArg(expr, paramType, traversal, parts, isRoot)
}

// rewriteScopeTraversalExpression replaces a single access to an ouptut-typed ScopeTraversalExpression with an apply
// parameter.
func (ctx *observeContext) rewriteScopeTraversalExpression(expr *model.ScopeTraversalExpression,
	isRoot bool) model.Expression {

	// If the access is not an output() or a promise(), return the node as-is.
	resolvedType, isEventual := ctx.isEventualType(expr.Type())
	if !isEventual {
		// If this is a reference to a named variable, put the name in scope.
		if definition, ok := expr.Traversal[0].(Node); ok {
			ctx.assignedNames.Add(definition.Name())
			ctx.nameCounts[definition.Name()] = 1
		}
		return expr
	}

	// Otherwise, append the access to the list of apply arguments and return an appropriate call to __applyArg.
	//
	// TODO: deduplicate multiple accesses to the same variable and field.

	// Compute the type of the apply and callback arguments.
	var applyArg *model.ScopeTraversalExpression
	var paramType model.Type
	var parts []model.Traversable
	var traversal hcl.Traversal

	splitTraversal := expr.Traversal.SimpleSplit()
	rootResolvedType, rootIsEventual := resolvedType, true
	if len(splitTraversal.Rel) > 0 {
		rootResolvedType, rootIsEventual = ctx.isEventualType(model.GetTraversableType(expr.Parts[0]))
	}
	if rootIsEventual {
		applyArg = &model.ScopeTraversalExpression{
			Parts:     expr.Parts[:1],
			RootName:  splitTraversal.Abs.RootName(),
			Traversal: splitTraversal.Abs,
		}
		paramType, traversal, parts = rootResolvedType, expr.Traversal.SimpleSplit().Rel, expr.Parts[1:]
	} else {
		for i := range splitTraversal.Rel {
			partResolvedType, isEventual := resolvedType, true
			if i < len(splitTraversal.Rel)-1 {
				partResolvedType, isEventual = ctx.isEventualType(model.GetTraversableType(expr.Parts[i+1]))
			}
			if isEventual {
				absTraversal, relTraversal := expr.Traversal[:i+2], expr.Traversal[i+2:]

				applyArg = &model.ScopeTraversalExpression{
					Parts:     expr.Parts[:i+2],
					RootName:  absTraversal.RootName(),
					Traversal: absTraversal,
				}
				paramType, traversal, parts = partResolvedType, relTraversal, expr.Parts[i+2:]
				break
			}
		}
	}

	return ctx.rewriteApplyArg(applyArg, paramType, traversal, parts, isRoot)
}

// rewriteRoot replaces the root node in a bound expression with a call to the __apply intrinsic if necessary.
func (ctx *observeContext) rewriteRoot(expr model.Expression) model.Expression {
	contract.Require(expr == ctx.root, "expr")

	if len(ctx.applyArgs) == 0 {
		return expr
	}

	// Assign argument names.
	for i, arg := range ctx.applyArgs {
		bestName := ctx.nameInfo.Format(ctx.bestArgName(arg))
		ctx.callbackParams[i].Name, ctx.nameCounts[bestName] = bestName, ctx.nameCounts[bestName]+1
	}
	for i, param := range ctx.callbackParams {
		if ctx.nameCounts[param.Name] > 1 {
			param.Name = ctx.disambiguateArgName(ctx.applyArgs[i], param.Name)
			if ctx.nameCounts[param.Name] == 0 {
				ctx.nameCounts[param.Name] = 1
			}
			ctx.assignedNames.Add(param.Name)
		}
	}

	// Update parameter references with the assigned names.
	for _, x := range ctx.paramReferences {
		v := x.Parts[0].(*model.Variable)
		rootTraversal := x.Traversal[0].(hcl.TraverseRoot)
		x.RootName, rootTraversal.Name = v.Name, v.Name
		x.Traversal[0] = rootTraversal
	}

	// Create a new anonymous function definition.
	callback := &model.AnonymousFunctionExpression{
		Signature: model.StaticFunctionSignature{
			Parameters: make([]model.Parameter, len(ctx.callbackParams)),
			ReturnType: expr.Type(),
		},
		Parameters: ctx.callbackParams,
		Body:       expr,
	}
	for i, p := range ctx.callbackParams {
		callback.Signature.Parameters[i] = model.Parameter{Name: p.Name, Type: p.VariableType}
	}

	return NewApplyCall(ctx.applyArgs, callback)
}

func (ctx *observeContext) PreVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	if ctx.inspectsEventualValues(expr) {
		if ctx.observesEventualValues(expr) {
			ctx.activeContext = &observeContext{
				applyRewriter: ctx.applyRewriter,
				parent:        ctx,
				root:          expr,
				assignedNames: codegen.StringSet{},
				nameCounts:    map[string]int{},
			}
		} else {
			ctx.activeContext = &inspectContext{
				applyRewriter: ctx.applyRewriter,
				parent:        ctx,
				root:          expr,
			}
		}
	}
	return expr, nil
}

func (ctx *observeContext) PostVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	isRoot := expr == ctx.root

	// TODO(pdg): arrays of outputs, for expressions, etc.
	diagnostics := expr.Typecheck(false)
	contract.Assert(len(diagnostics) == 0)

	if isIteratorExpr, _ := ctx.isIteratorExpr(expr); isIteratorExpr {
		return expr, nil
	}

	switch x := expr.(type) {
	case *model.RelativeTraversalExpression:
		expr = ctx.rewriteRelativeTraversalExpression(x, isRoot)
	case *model.ScopeTraversalExpression:
		expr = ctx.rewriteScopeTraversalExpression(x, isRoot)
	default:
		_, isEventual := ctx.isEventualType(expr.Type())
		if isEventual && ctx.inspectsEventualValues(x) {
			expr = ctx.rewriteApplyArg(x, model.ResolveOutputs(x.Type()), nil, nil, isRoot)
		}
	}
	if isRoot {
		ctx.root = expr
		expr = ctx.rewriteRoot(expr)

		ctx.activeContext = ctx.parent
		return ctx.activeContext.PostVisit(expr)
	}
	return expr, nil
}

func (ctx *inspectContext) PreVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	if ctx.observesEventualValues(expr) {
		observeCtx := &observeContext{
			applyRewriter: ctx.applyRewriter,
			parent:        ctx,
			root:          expr,
			assignedNames: codegen.StringSet{},
			nameCounts:    map[string]int{},
		}
		ctx.activeContext = observeCtx
	}
	return expr, nil
}

func (ctx *inspectContext) PostVisit(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	if expr == ctx.root {
		ctx.activeContext = ctx.parent
		if ctx.parent != nil {
			return ctx.activeContext.PostVisit(expr)
		}
	}
	return expr, nil
}

// RewriteApplies transforms all expressions that observe the resolved values of outputs and promises into calls to the
// __apply intrinsic. Expressions that generate or inspect outputs or promises are passed as arguments to these calls,
// and are replaced by references to the corresponding parameter.
//
// As an example, assuming that resource.id is an output, this transforms the following expression:
//
//     toJSON({
//         Version = "2012-10-17"
//         Statement = [{
//             Effect = "Allow"
//             Principal = "*"
//             Action = [ "s3:GetObject" ]
//             Resource = [ "arn:aws:s3:::${resource.id}/*" ]
//         }]
//     })
//
// into this expression:
//
//     __apply(resource.id, eval(id, toJSON({
//         Version = "2012-10-17"
//         Statement = [{
//             Effect = "Allow"
//             Principal = "*"
//             Action = [ "s3:GetObject" ]
//             Resource = [ "arn:aws:s3:::${id}/*" ]
//         }]
//     })))
//
// Here is a more advanced example, assuming that resource is an object whose properties are all outputs, this
// expression:
//
//     "v: ${resource[resource.id]}"
//
// is transformed into this expression:
//
//     __apply(__apply(resource.id,eval(id, resource[id])),eval(id, "v: ${id}"))
//
// This form is amenable to code generation for targets that require that outputs are resolved before their values are
// accessible (e.g. Pulumi's JS/TS libraries).
func RewriteApplies(expr model.Expression, nameInfo NameInfo, applyPromises bool) (model.Expression, hcl.Diagnostics) {
	applyRewriter := &applyRewriter{
		nameInfo:      nameInfo,
		applyPromises: applyPromises,
	}
	applyRewriter.activeContext = &inspectContext{
		applyRewriter: applyRewriter,
		root:          expr,
	}
	return model.VisitExpression(expr, applyRewriter.preVisit, applyRewriter.postVisit)
}
