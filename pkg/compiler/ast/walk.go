// Copyright 2017 Pulumi, Inc. All rights reserved.

package ast

import (
	"github.com/golang/glog"
	"reflect"

	"github.com/pulumi/coconut/pkg/util/contract"
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

	if glog.V(9) {
		glog.V(9).Infof("AST visitor walk: pre-visit %v", reflect.TypeOf(node))
	}

	// First visit the node; only proceed if the visitor says to do so (and use its returned visitor below).
	if v = v.Visit(node); v == nil {
		return
	}

	if glog.V(9) {
		glog.V(9).Infof("AST visitor walk: post-visit, pre-recurse %v", reflect.TypeOf(node))
	}

	// Switch on the node type and walk any children.  Note that we only switch on concrete AST node types and not
	// abstractions.  Also note that the order in which we walk these nodes corresponds to the expected "evaluation"
	// order of them during runtime/interpretation.
	switch n := node.(type) {
	// Nodes
	case *Identifier, *Token, *ClassMemberToken, *ModuleToken, *TypeToken:
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
	case *Import:
		Walk(v, n.Referent)
		if n.Name != nil {
			Walk(v, n.Name)
		}
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
	case *TryCatchBlock:
		Walk(v, n.Exception)
		Walk(v, n.Block)
	case *IfStatement:
		Walk(v, n.Condition)
		Walk(v, n.Consequent)
		if n.Alternate != nil {
			Walk(v, *n.Alternate)
		}
	case *SwitchStatement:
		Walk(v, n.Expression)
		for _, cas := range n.Cases {
			if cas.Clause != nil {
				Walk(v, *cas.Clause)
			}
			Walk(v, cas.Consequent)
		}
	case *LabeledStatement:
		Walk(v, n.Statement)
	case *ReturnStatement:
		if n.Expression != nil {
			Walk(v, *n.Expression)
		}
	case *ThrowStatement:
		Walk(v, n.Expression)
	case *WhileStatement:
		if n.Condition != nil {
			Walk(v, *n.Condition)
		}
		Walk(v, n.Body)
	case *ForStatement:
		if n.Init != nil {
			Walk(v, *n.Init)
		}
		if n.Condition != nil {
			Walk(v, *n.Condition)
		}
		if n.Post != nil {
			Walk(v, *n.Post)
		}
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
			Walk(v, *n.Object)
		}
		Walk(v, n.Name)
	case *NewExpression:
		Walk(v, n.Type)
		if n.Arguments != nil {
			for _, arg := range *n.Arguments {
				Walk(v, arg)
			}
		}
	case *InvokeFunctionExpression:
		Walk(v, n.Function)
		if n.Arguments != nil {
			for _, arg := range *n.Arguments {
				Walk(v, arg)
			}
		}
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

	if glog.V(9) {
		glog.V(9).Infof("AST visitor walk: post-recurse, pre-after %v", reflect.TypeOf(node))
	}

	// Finally let the visitor know that we are done processing this node.
	v.After(node)
	if glog.V(9) {
		glog.V(9).Infof("AST visitor walk: post-after %v", reflect.TypeOf(node))
	}
}

// Inspector is an anonymous visitation struct that implements the Visitor interface.
type Inspector struct {
	V Visitator
	A Afterator
}

func (v Inspector) Visit(node Node) Visitor {
	if v.V != nil {
		if !v.V(node) {
			return nil
		}
	}
	return v
}

func (v Inspector) After(node Node) {
	if v.A != nil {
		v.A(node)
	}
}

// Visitator is a very simple Visitor implementation; it simply returns true to continue visitation, or false to stop.
type Visitator func(Node) bool

func (v Visitator) Visit(node Node) Visitor {
	if v(node) {
		return v
	}
	return nil
}

func (v Visitator) After(node Node) {
	// nothing to do.
}

// Afterator is a very simple Visitor implementation; it simply runs after visitation has occurred on nodes.
type Afterator func(Node)

func (a Afterator) Visit(node Node) Visitor {
	// nothing to do.
	return a
}

func (a Afterator) After(node Node) {
	a(node)
}
