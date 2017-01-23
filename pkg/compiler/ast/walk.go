// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/util/contract"
)

// Visitor is a pluggable interface invoked during walks of an AST.
type Visitor interface {
	// Visit visits the given AST node.  If it returns nil, the calling code will stop visiting immediately after the
	// call to Visit returns.  If it returns a non-nil Visitor, the calling code will continue visiting.  The order in
	// which nodes are visitied is specific to the specific visitation API being used.
	Visit(node Node) Visitor

	// After is invoked after visitation of a given node.
	After(node Node)
}

// Walk visits an AST node and all of its children.  It walks the AST in depth-first order.  A pre- and/or
// post-visitation Visitor object may be supplied in order to hook into this walk at the right moments.
func Walk(v Visitor, node Node) {
	contract.Requiref(node != nil, "node", "!= nil")

	// First visit the node; only proceed if the visitor says to do so (and use its returned visitor below).
	if v = v.Visit(node); v == nil {
		return
	}

	// Switch on the node type and walk any children.  Note that we only switch on concrete AST node types and not
	// abstractions.  Also note that the order in which we walk these nodes corresponds to the expected "evaluation"
	// order of them during runtime/interpretation.
	switch n := node.(type) {
	// Nodes
	case *Identifier, *Token, *ModuleToken, *TypeToken:
		// No children, nothing to do.

	// Definitions
	case *Module:
		if n.Members != nil {
			for _, name := range StableModuleMembers(*n.Members) {
				Walk(v, (*n.Members)[name])
			}
		}
	case *Class:
		if n.Members != nil {
			for _, name := range StableClassMembers(*n.Members) {
				Walk(v, (*n.Members)[name])
			}
		}
	case *ModuleMethod:
		if n.Parameters != nil {
			for _, param := range *n.Parameters {
				Walk(v, param)
			}
		}
		Walk(v, n.Body)
	case *ClassMethod:
		if n.Parameters != nil {
			for _, param := range *n.Parameters {
				Walk(v, param)
			}
		}
		if n.Body != nil {
			Walk(v, n.Body)
		}
	case *Export, *LocalVariable, *ModuleProperty, *ClassProperty:
		// No children, nothing to do.

	// Statements
	case *Block:
		for _, stmt := range n.Statements {
			Walk(v, stmt)
		}
	case *LocalVariableDeclaration:
		Walk(v, n.Local)
	case *TryCatchFinally:
		Walk(v, n.TryBlock)
		if n.CatchBlocks != nil {
			for _, catch := range *n.CatchBlocks {
				Walk(v, catch)
			}
		}
		if n.FinallyBlock != nil {
			Walk(v, n.FinallyBlock)
		}
	case *IfStatement:
		Walk(v, n.Condition)
		Walk(v, n.Consequent)
		if n.Alternate != nil {
			Walk(v, *n.Alternate)
		}
	case *LabeledStatement:
		Walk(v, n.Statement)
	case *ReturnStatement:
		if n.Expression != nil {
			Walk(v, *n.Expression)
		}
	case *ThrowStatement:
		if n.Expression != nil {
			Walk(v, *n.Expression)
		}
	case *WhileStatement:
		Walk(v, n.Test)
		Walk(v, n.Body)
	case *MultiStatement:
		for _, stmt := range n.Statements {
			Walk(v, stmt)
		}
	case *ExpressionStatement:
		Walk(v, n.Expression)
	case *BreakStatement, *ContinueStatement, *EmptyStatement:
		// No children, nothing to do.

	// Expressions
	case *ArrayLiteral:
		if n.Size != nil {
			Walk(v, *n.Size)
		}
		if n.Elements != nil {
			for _, expr := range *n.Elements {
				Walk(v, expr)
			}
		}
	case *ObjectLiteral:
		if n.Properties != nil {
			for _, prop := range *n.Properties {
				Walk(v, prop)
			}
		}
	case *ObjectLiteralProperty:
		Walk(v, n.Property)
		Walk(v, n.Value)
	case *LoadLocationExpression:
		if n.Object != nil {
			Walk(v, *n.Object)
		}
		Walk(v, n.Name)
	case *LoadDynamicExpression:
		if n.Object != nil {
			Walk(v, n.Object)
		}
		Walk(v, n.Name)
	case *NewExpression:
		Walk(v, n.Type)
	case *InvokeFunctionExpression:
		Walk(v, n.Function)
	case *LambdaExpression:
		Walk(v, n.Body)
	case *UnaryOperatorExpression:
		Walk(v, n.Operand)
	case *BinaryOperatorExpression:
		Walk(v, n.Left)
		Walk(v, n.Right)
	case *CastExpression:
		Walk(v, n.Expression)
	case *IsInstExpression:
		Walk(v, n.Expression)
	case *TypeOfExpression:
		Walk(v, n.Expression)
	case *ConditionalExpression:
		Walk(v, n.Condition)
		Walk(v, n.Consequent)
		Walk(v, n.Alternate)
	case *SequenceExpression:
		for _, expr := range n.Expressions {
			Walk(v, expr)
		}
	case *NullLiteral, *BoolLiteral, *NumberLiteral, *StringLiteral:
		// No children, nothing to do.

	default:
		contract.Failf("Unrecognized AST node during walk: %v", n.GetKind())
	}

	// Finally let the visitor know that we are done processing this node.
	v.After(node)
}

// Inspector is a very simple Visitor implementation; it simply returns true to continue visitation, or false to stop.
type Inspector func(Node) bool

func (insp Inspector) Visit(node Node) Visitor {
	if insp(node) {
		return insp
	}
	return nil
}

func (insp Inspector) After(node Node) {
	// nothing to do.
}

// AfterInspector is a very simple Visitor implementation; it simply runs after visitation has occurred on nodes.
type AfterInspector func(Node)

func (insp AfterInspector) Visit(node Node) Visitor {
	// nothing to do.
	return insp
}

func (insp AfterInspector) After(node Node) {
	insp(node)
}
