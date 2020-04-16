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

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type NameInfo interface {
	IsReservedWord(name string) bool
}

// The applyRewriter is responsible for transforming expressions involving Pulumi output properties into a call to the
// __apply intrinsic and replacing the output properties with appropriate calls to the __applyArg intrinsic.
type applyRewriter struct {
	nameInfo      NameInfo
	applyPromises bool

	root            model.Expression
	applyArgs       []*model.ScopeTraversalExpression
	callbackParams  []*model.Variable
	paramReferences []*model.ScopeTraversalExpression

	assignedNames codegen.StringSet
	nameCounts    map[string]int
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

func (r *applyRewriter) isPromptArg(paramType, argType model.Type) bool {
	if isEventualArg := model.ResolveOutputs(argType) != argType; !isEventualArg {
		return true
	}

	if union, ok := paramType.(*model.UnionType); ok {
		for _, t := range union.ElementTypes {
			if t != model.DynamicType && t.ConversionFrom(argType) != model.NoConversion {
				return true
			}
		}
		return false
	}
	return paramType != model.DynamicType && paramType.ConversionFrom(argType) != model.NoConversion
}

func (r *applyRewriter) isEventualExpr(x model.Expression) bool {
	if _, isEventual := r.isEventualType(x.Type()); !isEventual {
		return false
	}

	call, isCall := x.(*model.FunctionCallExpression)
	if !isCall {
		return true
	}

	for i, arg := range call.Args {
		if !r.isPromptArg(call.Signature.Parameters[i].Type, arg.Type()) {
			return true
		}
	}
	return false
}

// disambiguateName ensures that the given name is unambiguous by appending an integer starting with 1 if necessary.
func (r *applyRewriter) disambiguateName(name string) string {
	if name == "" {
		name = "arg"
	} else if r.nameInfo.IsReservedWord(name) {
		name = "_" + name
	}

	if !r.assignedNames.Has(name) {
		return name
	}

	root := name
	for i := 1; r.nameCounts[name] != 0; i++ {
		name = fmt.Sprintf("%s%d", root, i)
	}
	return name
}

// bestArgName computes the "best" name for a given apply argument. If this name is unambiguous after all best names
// have been calculated, it will be assigned to the argument. Otherwise, it will go through the disambiguation process
// in disambiguateArgName.
func (r *applyRewriter) bestArgName(x *model.ScopeTraversalExpression) string {
	switch n := x.Parts[0].(type) {
	case *ConfigVariable, *LocalVariable, *OutputVariable:
		return n.(Node).Name()
	case *Resource:
		// If dealing with a broken access, use the resource's variable name. Otherwise, use the name
		// of the traversal's first field.
		if len(x.Traversal) == 1 {
			return n.Name()
		}
		switch t := x.Traversal[1].(type) {
		case hcl.TraverseAttr:
			return t.Name
		case hcl.TraverseIndex:
			return t.Key.AsString()
		default:
			return n.Name()
		}
	case *model.SplatVariable:
		return n.Name
	case *model.Variable:
		return n.Name
	default:
		panic(fmt.Errorf("unexpected definition in assignApplyArgName: %T", n))
	}
}

// disambiguateArgName applies type-specific disambiguation to an argument name.
func (r *applyRewriter) disambiguateArgName(x *model.ScopeTraversalExpression, bestName string) string {
	if n, ok := x.Parts[0].(*Resource); ok {
		// If dealing with a broken access, defer to the generic disambiguator. Otherwise, attempt to disambiguate
		// by prepending the resource's variable name.
		if len(x.Traversal) > 1 {
			return r.disambiguateName(n.Name() + titleCase(bestName))
		}
	}
	// Hand off to the generic disambiguator.
	return r.disambiguateName(bestName)
}

// rewriteScopeTraversalExpression replaces a single access to an ouptut-typed ScopeTraversalExpression with a call to
// the __applyArg intrinsic.
func (r *applyRewriter) rewriteScopeTraversalExpression(expr *model.ScopeTraversalExpression,
	isRoot bool) model.Expression {

	// If the access is not an output() or a promise(), return the node as-is.
	_, isEventual := r.isEventualType(expr.Type())
	if !isEventual {
		// If this is a reference to a named variable, put the name in scope.
		if definition, ok := expr.Traversal[0].(Node); ok {
			r.assignedNames.Add(definition.Name())
			r.nameCounts[definition.Name()] = 1
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

	splitTraversal := expr.Syntax.Traversal.SimpleSplit()
	if rootResolvedType, rootIsEventual := r.isEventualType(model.GetTraversableType(expr.Parts[0])); rootIsEventual {
		applyArg = &model.ScopeTraversalExpression{
			Parts:     expr.Parts[:1],
			RootName:  splitTraversal.Abs.RootName(),
			Traversal: splitTraversal.Abs,
		}
		paramType, traversal, parts = rootResolvedType, expr.Syntax.Traversal.SimpleSplit().Rel, expr.Parts[1:]
	} else {
		for i := range splitTraversal.Rel {
			if resolvedType, isEventual := r.isEventualType(model.GetTraversableType(expr.Parts[i+1])); isEventual {
				absTraversal, relTraversal := expr.Syntax.Traversal[:i+2], expr.Syntax.Traversal[i+2:]

				applyArg = &model.ScopeTraversalExpression{
					Parts:     expr.Parts[:i+2],
					RootName:  absTraversal.RootName(),
					Traversal: absTraversal,
				}
				paramType, traversal, parts = resolvedType, relTraversal, expr.Parts[i+2:]
				break
			}
		}
	}

	if len(traversal) == 0 && isRoot {
		return expr
	}

	callbackParam := &model.Variable{
		Name:         fmt.Sprintf("<arg%d>", len(r.callbackParams)),
		VariableType: paramType,
	}

	r.applyArgs, r.callbackParams = append(r.applyArgs, applyArg), append(r.callbackParams, callbackParam)

	// TODO(pdg): this risks information loss for nested output-typed properties... The `Types` array on traversals
	// ought to store the original types.
	resolvedParts := make([]model.Traversable, len(parts)+1)
	resolvedParts[0] = callbackParam
	for i, p := range parts {
		resolved, isEventual := r.isEventualType(model.GetTraversableType(p))
		contract.Assert(isEventual)
		resolvedParts[i+1] = resolved
	}

	result := &model.ScopeTraversalExpression{
		Parts:     resolvedParts,
		RootName:  callbackParam.Name,
		Traversal: hcl.TraversalJoin(hcl.Traversal{hcl.TraverseRoot{Name: callbackParam.Name}}, traversal),
	}
	r.paramReferences = append(r.paramReferences, result)
	return result
}

// rewriteRoot replaces the root node in a bound expression with a call to the __apply intrinsic if necessary.
func (r *applyRewriter) rewriteRoot(expr model.Expression) model.Expression {
	contract.Require(expr == r.root, "expr")

	// Clear the root context so that future calls to enterNode recognize new expression roots.
	r.root = nil
	if len(r.applyArgs) == 0 {
		return expr
	}

	// Assign argument names.
	for i, arg := range r.applyArgs {
		bestName := r.bestArgName(arg)
		r.callbackParams[i].Name, r.nameCounts[bestName] = bestName, r.nameCounts[bestName]+1
	}
	for i, param := range r.callbackParams {
		if r.nameCounts[param.Name] > 1 {
			param.Name = r.disambiguateArgName(r.applyArgs[i], param.Name)
			if r.nameCounts[param.Name] == 0 {
				r.nameCounts[param.Name] = 1
			}
			r.assignedNames.Add(param.Name)
		}
	}

	// Update parameter references with the assigned names.
	for _, x := range r.paramReferences {
		v := x.Parts[0].(*model.Variable)
		rootTraversal := x.Traversal[0].(hcl.TraverseRoot)
		x.RootName, rootTraversal.Name = v.Name, v.Name
		x.Traversal[0] = rootTraversal
	}

	// Create a new anonymous function definition.
	callback := &model.AnonymousFunctionExpression{
		Signature: model.StaticFunctionSignature{
			Parameters: make([]model.Parameter, len(r.callbackParams)),
			ReturnType: expr.Type(),
		},
		Parameters: r.callbackParams,
		Body:       expr,
	}
	for i, p := range r.callbackParams {
		callback.Signature.Parameters[i] = model.Parameter{Name: p.Name, Type: p.VariableType}
	}

	return NewApplyCall(r.applyArgs, callback)
}

// rewriteExpression performs the apply rewrite on a single expression, delegating to type-specific functions as
// necessary.
func (r *applyRewriter) rewriteExpression(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	isRoot := expr == r.root

	// TODO(pdg): arrays of outputs, for expressions, etc.

	if traversal, isScopeTraversal := expr.(*model.ScopeTraversalExpression); isScopeTraversal {
		expr = r.rewriteScopeTraversalExpression(traversal, isRoot)
	}
	if isRoot {
		r.root = expr
		expr = r.rewriteRoot(expr)
	}
	return expr, nil
}

// enterExpression is a pre-order visitor that is used to find roots for bound expression trees. This approach is
// intended to allow consumers of the apply rewrite to call RewriteApplies on a list or map property that may contain
// multiple independent bound expressions rather than requiring that they find and rewrite these expressions
// individually.
func (r *applyRewriter) enterExpression(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	if r.root == nil {
		if r.isEventualExpr(expr) {
			r.root, r.applyArgs, r.callbackParams, r.paramReferences = expr, nil, nil, nil
			r.assignedNames, r.nameCounts = codegen.StringSet{}, map[string]int{}
		}
	}
	return expr, nil
}

// RewriteApplies transforms all bound expression trees in the given Expression that reference output-typed properties
// into appropriate calls to the __apply and __applyArg intrinsic. Given an expression tree, the rewrite proceeds as
// follows:
// - let the list of outputs be an empty list
// - for each node in post-order:
//     - if the node is the root of the expression tree:
//         - if the node is a variable access:
//             - if the access has an output-typed element on its path, replace the variable access with a call to the
//               __applyArg intrinsic and append the access to the list of outputs.
//             - otherwise, the access does not need to be transformed; return it as-is.
//         - if the list of outputs is empty, the root does not need to be transformed; return it as-is.
//         - otherwise, replace the root with a call to the __apply intrinstic. The first n arguments to this call are
//           the elementss of the list of outputs. The final argument is the original root node.
//     - otherwise, if the root is an output-typed variable access, replace the variable access with a call to the
//       __applyArg instrinsic and append the access to the list of outputs.
//
// As an example, this transforms the following expression:
//     (output string
//         "#!/bin/bash -xe\n\nCA_CERTIFICATE_DIRECTORY=/etc/kubernetes/pki\necho \""
//         (aws_eks_cluster.demo.certificate_authority.0.data output<unknown> *config.ResourceVariable)
//         "\" | base64 -d >  $CA_CERTIFICATE_FILE_PATH\nsed -i s,MASTER_ENDPOINT,"
//         (aws_eks_cluster.demo.endpoint output<string> *config.ResourceVariable)
//         ",g /var/lib/kubelet/kubeconfig\nsed -i s,CLUSTER_NAME,"
//         (var.cluster-name string *config.UserVariable)
//         ",g /var/lib/kubelet/kubeconfig\nsed -i s,REGION,"
//         (data.aws_region.current.name output<string> *config.ResourceVariable)
//         ",g /etc/systemd/system/kubelet.servicesed -i s,MASTER_ENDPOINT,"
//         (aws_eks_cluster.demo.endpoint output<string> *config.ResourceVariable)
//         ",g /etc/systemd/system/kubelet.service"
//     )
//
// into this expression:
//     (call output<unknown> __apply
//         (aws_eks_cluster.demo.certificate_authority.0.data output<unknown> *config.ResourceVariable)
//         (aws_eks_cluster.demo.endpoint output<string> *config.ResourceVariable)
//         (data.aws_region.current.name output<string> *config.ResourceVariable)
//         (aws_eks_cluster.demo.endpoint output<string> *config.ResourceVariable)
//         (output string
//             "#!/bin/bash -xe\n\nCA_CERTIFICATE_DIRECTORY=/etc/kubernetes/pki\necho \""
//             (call unknown __applyArg
//                 0
//             )
//             "\" | base64 -d >  $CA_CERTIFICATE_FILE_PATH\nsed -i s,MASTER_ENDPOINT,"
//             (call string __applyArg
//                 1
//             )
//             ",g /var/lib/kubelet/kubeconfig\nsed -i s,CLUSTER_NAME,"
//             (var.cluster-name string *config.UserVariable)
//             ",g /var/lib/kubelet/kubeconfig\nsed -i s,REGION,"
//             (call string __applyArg
//                 2
//             )
//             ",g /etc/systemd/system/kubelet.servicesed -i s,MASTER_ENDPOINT,"
//             (call string __applyArg
//                 3
//             )
//             ",g /etc/systemd/system/kubelet.service"
//         )
//     )
//
// This form is amenable to code generation for targets that require that outputs are resolved before their values are
// accessible (e.g. Pulumi's JS/TS libraries).
func RewriteApplies(expr model.Expression, nameInfo NameInfo, applyPromises bool) (model.Expression, hcl.Diagnostics) {
	rewriter := &applyRewriter{nameInfo: nameInfo, applyPromises: applyPromises}
	return model.VisitExpression(expr, rewriter.enterExpression, rewriter.rewriteExpression)
}
