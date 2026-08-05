// Copyright 2020, Pulumi Corporation.
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

package python

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/zclconf/go-cty/cty"
)

// Keep this set in sync with the non-private members of Output in sdk/python/lib/pulumi/output.py.
// Output.__getattr__ cannot project properties whose names are already defined by Output.
var outputMemberNames = codegen.NewStringSet(
	"all",
	"apply",
	"concat",
	"format",
	"from_input",
	"future",
	"get",
	"is_known",
	"is_secret",
	"json_dumps",
	"json_loads",
	"recover",
	"resources",
	"secret",
	"unsecret",
)

func canLiftOutputTraversal(traversal hcl.Traversal) bool {
	for _, traverser := range traversal {
		var name string
		switch traverser := traverser.(type) {
		case hcl.TraverseAttr:
			name = PyName(traverser.Name)
		case hcl.TraverseIndex:
			if traverser.Key.Type() != cty.String {
				continue
			}
			name = PyName(traverser.Key.AsString())
		default:
			continue
		}

		// Attribute lifting uses Output.__getattr__, so it cannot project names already defined by Output.
		if strings.HasPrefix(name, "_") || outputMemberNames.Has(name) {
			return false
		}
	}
	return true
}

func isDirectParameterReference(parameters codegen.Set, x model.Expression) bool {
	scopeTraversal, ok := x.(*model.ScopeTraversalExpression)
	if !ok || len(scopeTraversal.Parts) != 1 {
		return false
	}

	return parameters.Has(scopeTraversal.Parts[0])
}

func parameterRelativeTraversal(parameters codegen.Set, x model.Expression) (hcl.Traversal, bool) {
	switch x := x.(type) {
	case *model.ScopeTraversalExpression:
		if !parameters.Has(x.Parts[0]) {
			return nil, false
		}
		return x.Traversal.SimpleSplit().Rel, true
	case *model.RelativeTraversalExpression:
		if !isDirectParameterReference(parameters, x.Source) {
			return nil, false
		}
		return x.Traversal, true
	default:
		return nil, false
	}
}

// parseProxyApply attempts to match and rewrite the given parsed apply using the following patterns:
//
// - __apply(<expr>, eval(x, x[index])) -> <expr>[index]
// - __apply(<expr>, eval(x, x.attr))) -> <expr>.attr
// - __apply(traversal, eval(x, x.attr)) -> traversal.attr
//
// Each of these patterns matches an apply that can be handled by `pulumi.Output`'s `__getitem__` or `__getattr__`
// method. The rewritten expressions will use those methods rather than calling `apply`.
func (g *generator) parseProxyApply(parameters codegen.Set, args []model.Expression,
	then model.Expression,
) (model.Expression, bool) {
	if len(args) != 1 {
		return nil, false
	}

	arg := args[0]
	switch then := then.(type) {
	case *model.IndexExpression:
		// Rewrite `__apply(<expr>, eval(x, x[index]))` to `<expr>[index]`.
		if !isDirectParameterReference(parameters, then.Collection) {
			return nil, false
		}
		// Create a new IndexExpression instead of mutating the original
		newIndex := &model.IndexExpression{
			Collection: arg,
			Key:        then.Key,
		}
		// Typecheck to set the type
		_ = newIndex.Typecheck(false)
		return newIndex, true
	case *model.ScopeTraversalExpression, *model.RelativeTraversalExpression:
		traversal, ok := parameterRelativeTraversal(parameters, then)
		if !ok || !canLiftOutputTraversal(traversal) {
			return nil, false
		}

		newTraversal := &model.RelativeTraversalExpression{
			Source:    arg,
			Traversal: traversal,
		}
		if diags := newTraversal.Typecheck(false); diags.HasErrors() {
			return nil, false
		}
		return newTraversal, true
	default:
		return nil, false
	}
}

// lowerProxyApplies lowers certain calls to the apply intrinsic into proxied property accesses. Concretely, this
// boils down to rewriting the following shapes
//
// - __apply(<expr>, eval(x, x[index]))
// - __apply(<expr>, eval(x, x.attr)))
// - __apply(scope.traversal, eval(x, x.attr))
//
// into (respectively)
//
// - <expr>[index]
// - <expr>.attr
// - scope.traversal.attr
//
// These forms will use `pulumi.Output`'s `__getitem__` and `__getattr__` instead of calling `apply`.
func (g *generator) lowerProxyApplies(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	rewriter := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to the apply intrinsic.
		apply, ok := expr.(*model.FunctionCallExpression)
		if !ok || apply.Name != pcl.IntrinsicApply {
			return expr, nil
		}

		// Parse the apply call.
		args, then := pcl.ParseApplyCall(apply)

		parameters := codegen.Set{}
		for _, p := range then.Parameters {
			parameters.Add(p)
		}

		// Attempt to match (call __apply (rvar) (call __applyArg 0))
		if v, ok := g.parseProxyApply(parameters, args, then.Body); ok {
			return v, nil
		}

		return expr, nil
	}
	return model.VisitExpression(expr, model.IdentityVisitor, rewriter)
}
