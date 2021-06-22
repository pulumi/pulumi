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
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A BodyItemVisitor is a function that visits and optionally replaces the contents of a body item.
type BodyItemVisitor func(n BodyItem) (BodyItem, hcl.Diagnostics)

func BodyItemIdentityVisitor(n BodyItem) (BodyItem, hcl.Diagnostics) {
	return n, nil
}

func visitBlock(n *Block, pre, post BodyItemVisitor) (BodyItem, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	var items []BodyItem
	for _, item := range n.Body.Items {
		newItem, diags := VisitBodyItem(item, pre, post)
		diagnostics = append(diagnostics, diags...)

		if newItem != nil {
			items = append(items, newItem)
		}
	}
	n.Body.Items = items

	block, diags := post(n)
	return block, append(diagnostics, diags...)
}

func VisitBodyItem(n BodyItem, pre, post BodyItemVisitor) (BodyItem, hcl.Diagnostics) {
	if n == nil {
		return nil, nil
	}

	if pre == nil {
		pre = BodyItemIdentityVisitor
	}

	nn, preDiags := pre(n)

	var postDiags hcl.Diagnostics
	if post != nil {
		switch n := nn.(type) {
		case *Attribute:
			nn, postDiags = post(n)
		case *Block:
			nn, postDiags = visitBlock(n, pre, post)
		default:
			contract.Failf("unexpected node type in visitExpression: %T", n)
			return nil, nil
		}
	}

	return nn, append(preDiags, postDiags...)
}

// An ExpressionVisitor is a function that visits and optionally replaces a node in an expression tree.
type ExpressionVisitor func(n Expression) (Expression, hcl.Diagnostics)

// IdentityVisitor is a ExpressionVisitor that returns the input node unchanged.
func IdentityVisitor(n Expression) (Expression, hcl.Diagnostics) {
	return n, nil
}

func visitAnonymousFunction(n *AnonymousFunctionExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	body, diags := VisitExpression(n.Body, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Body = body

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitBinaryOp(n *BinaryOpExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	left, diags := VisitExpression(n.LeftOperand, pre, post)
	diagnostics = append(diagnostics, diags...)

	right, diags := VisitExpression(n.RightOperand, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.LeftOperand, n.RightOperand = left, right

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitConditional(n *ConditionalExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	condition, diags := VisitExpression(n.Condition, pre, post)
	diagnostics = append(diagnostics, diags...)

	trueResult, diags := VisitExpression(n.TrueResult, pre, post)
	diagnostics = append(diagnostics, diags...)

	falseResult, diags := VisitExpression(n.FalseResult, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Condition, n.TrueResult, n.FalseResult = condition, trueResult, falseResult

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitFor(n *ForExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	collection, diags := VisitExpression(n.Collection, pre, post)
	diagnostics = append(diagnostics, diags...)

	key, diags := VisitExpression(n.Key, pre, post)
	diagnostics = append(diagnostics, diags...)

	value, diags := VisitExpression(n.Value, pre, post)
	diagnostics = append(diagnostics, diags...)

	condition, diags := VisitExpression(n.Condition, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Collection, n.Key, n.Value, n.Condition = collection, key, value, condition

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitFunctionCall(n *FunctionCallExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	args, diags := visitExpressions(n.Args, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Args = args

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitIndex(n *IndexExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	collection, diags := VisitExpression(n.Collection, pre, post)
	diagnostics = append(diagnostics, diags...)

	key, diags := VisitExpression(n.Key, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Collection, n.Key = collection, key

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitObjectCons(n *ObjectConsExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	for i, item := range n.Items {
		key, diags := VisitExpression(item.Key, pre, post)
		diagnostics = append(diagnostics, diags...)

		value, diags := VisitExpression(item.Value, pre, post)
		diagnostics = append(diagnostics, diags...)

		n.Items[i] = ObjectConsItem{Key: key, Value: value}
	}

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitRelativeTraversal(n *RelativeTraversalExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	source, diags := VisitExpression(n.Source, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Source = source

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitSplat(n *SplatExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	source, diags := VisitExpression(n.Source, pre, post)
	diagnostics = append(diagnostics, diags...)

	each, diags := VisitExpression(n.Each, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Source, n.Each = source, each

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitTemplate(n *TemplateExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	parts, diags := visitExpressions(n.Parts, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Parts = parts

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitTemplateJoin(n *TemplateJoinExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	tuple, diags := VisitExpression(n.Tuple, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Tuple = tuple

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitTupleCons(n *TupleConsExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	expressions, diags := visitExpressions(n.Expressions, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Expressions = expressions

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitUnaryOp(n *UnaryOpExpression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	operand, diags := VisitExpression(n.Operand, pre, post)
	diagnostics = append(diagnostics, diags...)

	n.Operand = operand

	expr, diags := post(n)
	return expr, append(diagnostics, diags...)
}

func visitExpressions(ns []Expression, pre, post ExpressionVisitor) ([]Expression, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	nils := 0
	for i, e := range ns {
		ee, diags := VisitExpression(e, pre, post)
		diagnostics = append(diagnostics, diags...)
		if ee == nil {
			nils++
		}
		ns[i] = ee
	}
	if nils == 0 {
		return ns, diagnostics
	} else if nils == len(ns) {
		return []Expression{}, diagnostics
	}

	nns := make([]Expression, 0, len(ns)-nils)
	for _, e := range ns {
		if e != nil {
			nns = append(nns, e)
		}
	}
	return nns, diagnostics
}

// VisitExpression visits each node in an expression tree using the given pre- and post-order visitors. If the preorder
// visitor returns a new node, that node's descendents will be visited. VisitExpression returns the result of the
// post-order visitor. All diagnostics are accumulated.
func VisitExpression(n Expression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	if n == nil {
		return nil, nil
	}

	if pre == nil {
		pre = IdentityVisitor
	}

	nn, preDiags := pre(n)

	var postDiags hcl.Diagnostics
	if post != nil {
		switch n := nn.(type) {
		case *AnonymousFunctionExpression:
			nn, postDiags = visitAnonymousFunction(n, pre, post)
		case *BinaryOpExpression:
			nn, postDiags = visitBinaryOp(n, pre, post)
		case *ConditionalExpression:
			nn, postDiags = visitConditional(n, pre, post)
		case *ErrorExpression:
			nn, postDiags = post(n)
		case *ForExpression:
			nn, postDiags = visitFor(n, pre, post)
		case *FunctionCallExpression:
			nn, postDiags = visitFunctionCall(n, pre, post)
		case *IndexExpression:
			nn, postDiags = visitIndex(n, pre, post)
		case *LiteralValueExpression:
			nn, postDiags = post(n)
		case *ObjectConsExpression:
			nn, postDiags = visitObjectCons(n, pre, post)
		case *RelativeTraversalExpression:
			nn, postDiags = visitRelativeTraversal(n, pre, post)
		case *ScopeTraversalExpression:
			nn, postDiags = post(n)
		case *SplatExpression:
			nn, postDiags = visitSplat(n, pre, post)
		case *TemplateExpression:
			nn, postDiags = visitTemplate(n, pre, post)
		case *TemplateJoinExpression:
			nn, postDiags = visitTemplateJoin(n, pre, post)
		case *TupleConsExpression:
			nn, postDiags = visitTupleCons(n, pre, post)
		case *UnaryOpExpression:
			nn, postDiags = visitUnaryOp(n, pre, post)
		default:
			contract.Failf("unexpected node type in visitExpression: %T", n)
			return nil, nil
		}
	}

	return nn, append(preDiags, postDiags...)
}

func visitBlockExpressions(n *Block, pre, post ExpressionVisitor) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	for _, item := range n.Body.Items {
		diags := VisitExpressions(item, pre, post)
		diagnostics = append(diagnostics, diags...)
	}

	return diagnostics
}

// VisitExpressions visits each expression that descends from the given body item.
func VisitExpressions(n BodyItem, pre, post ExpressionVisitor) hcl.Diagnostics {
	if n == nil {
		return nil
	}

	if pre == nil {
		pre = IdentityVisitor
	}

	switch n := n.(type) {
	case *Attribute:
		v, diags := VisitExpression(n.Value, pre, post)
		n.Value = v
		return diags
	case *Block:
		return visitBlockExpressions(n, pre, post)
	default:
		contract.Failf("unexpected node type in visitExpression: %T", n)
		return nil
	}
}
