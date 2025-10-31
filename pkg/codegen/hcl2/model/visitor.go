package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// A BodyItemVisitor is a function that visits and optionally replaces the contents of a body item.
type BodyItemVisitor = model.BodyItemVisitor

// An ExpressionVisitor is a function that visits and optionally replaces a node in an expression tree.
type ExpressionVisitor = model.ExpressionVisitor

func BodyItemIdentityVisitor(n BodyItem) (BodyItem, hcl.Diagnostics) {
	return model.BodyItemIdentityVisitor(n)
}

func VisitBodyItem(n BodyItem, pre, post BodyItemVisitor) (BodyItem, hcl.Diagnostics) {
	return model.VisitBodyItem(n, pre, post)
}

// IdentityVisitor is a ExpressionVisitor that returns the input node unchanged.
func IdentityVisitor(n Expression) (Expression, hcl.Diagnostics) {
	return model.IdentityVisitor(n)
}

// VisitExpression visits each node in an expression tree using the given pre- and post-order visitors. If the preorder
// visitor returns a new node, that node's descendents will be visited. VisitExpression returns the result of the
// post-order visitor. All diagnostics are accumulated.
func VisitExpression(n Expression, pre, post ExpressionVisitor) (Expression, hcl.Diagnostics) {
	return model.VisitExpression(n, pre, post)
}

// VisitExpressions visits each expression that descends from the given body item.
func VisitExpressions(n BodyItem, pre, post ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(n, pre, post)
}

