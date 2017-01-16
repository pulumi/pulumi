// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/pack/symbols"
)

// Statement is an element inside of an executable function body.
type Statement interface {
	Node
	statement()
}

type statementNode struct {
	node
}

func (node *statementNode) statement() {}

/* Blocks */

type Block struct {
	statementNode
	Statements []Statement `json:"statements"`
}

var _ Node = (*Block)(nil)
var _ Statement = (*Block)(nil)

const BlockKind NodeKind = "Block"

/* Local Variables */

type LocalVariableDeclaration struct {
	statementNode
	Local *LocalVariable `json:"local"`
}

var _ Node = (*LocalVariableDeclaration)(nil)
var _ Statement = (*LocalVariableDeclaration)(nil)

const LocalVariableDeclarationKind NodeKind = "LocalVariableDeclaration"

/* Try/Catch/Finally */

type TryCatchFinally struct {
	statementNode
	TryBlock     *Block            `json:"tryBlock"`
	CatchBlocks  *[]*TryCatchBlock `json:"catchBlocks,omitempty"`
	FinallyBlock *Block            `json:"finallyBlock"`
}

var _ Node = (*TryCatchFinally)(nil)
var _ Statement = (*TryCatchFinally)(nil)

const TryCatchFinallyKind NodeKind = "TryCatchFinally"

type TryCatchBlock struct {
	node
	Block     *Block             `json:"block"`
	Exception *symbols.TypeToken `json:"exception,omitempty"`
}

var _ Node = (*TryCatchBlock)(nil)

/* Branches */

// BreakStatement is the usual C-style `break` (only valid within loops).
type BreakStatement struct {
	statementNode
	Label *Identifier `json:"identifier,omitempty"`
}

var _ Node = (*BreakStatement)(nil)
var _ Statement = (*BreakStatement)(nil)

const BreakStatementKind NodeKind = "BreakStatement"

// ContinueStatement is the usual C-style `continue` (only valid within loops).
type ContinueStatement struct {
	statementNode
	Label *Identifier `json:"identifier,omitempty"`
}

var _ Node = (*ContinueStatement)(nil)
var _ Statement = (*ContinueStatement)(nil)

const ContinueStatementKind NodeKind = "ContinueStatement"

// IfStatement is the usual C-style `if`.  To simplify the MuIL AST, this is the only conditional statement available.
// All higher-level conditional constructs such as `switch`, if`/`else if`/..., etc., must be desugared into it.
type IfStatement struct {
	statementNode
	Condition  Expression `json:"expression"`            // a `bool` conditional expression.
	Consequent Statement  `json:"consequent"`            // the statement to execute if `true`.
	Alternate  *Statement `json:"alternative,omitempty"` // the optional statement to execute if `false`.
}

var _ Node = (*IfStatement)(nil)
var _ Statement = (*IfStatement)(nil)

const IfStatementKind NodeKind = "IfStatement"

// LabeledStatement associates an identifier with a statement for purposes of labeled jumps.
type LabeledStatement struct {
	statementNode
	Label     *Identifier `json:"label"`
	Statement Statement   `json:"statement"`
}

var _ Node = (*LabeledStatement)(nil)
var _ Statement = (*LabeledStatement)(nil)

const LabeledStatementKind NodeKind = "LabeledStatement"

// ReturnStatement is the usual C-style `return`, to exit a function.
type ReturnStatement struct {
	statementNode
	Expression *Expression `json:"expression,omitempty"`
}

var _ Node = (*ReturnStatement)(nil)
var _ Statement = (*ReturnStatement)(nil)

const ReturnStatementKind NodeKind = "ReturnStatement"

// ThrowStatement maps to raising an exception, usually `throw`, in the source language.
type ThrowStatement struct {
	statementNode
	Expression *Expression `json:"expression,omitempty"`
}

var _ Node = (*ThrowStatement)(nil)
var _ Statement = (*ThrowStatement)(nil)

const ThrowStatementKind NodeKind = "ThrowStatement"

// WhileStatement is the usual C-style `while`.  To simplify the MuIL AST, this is the only looping statement available.
// All higher-level looping constructs such as `for`, `foreach`, `do`/`while`, etc. must be desugared into it.
type WhileStatement struct {
	statementNode
	Test Expression `json:"test"`  // a `bool` statement indicating whether to condition.
	Body *Block     `json:"block"` // the body to execute provided the test remains `true`.
}

var _ Node = (*WhileStatement)(nil)
var _ Statement = (*WhileStatement)(nil)

const WhileStatementKind NodeKind = "WhileStatement"

/* Miscellaneous */

// EmptyStatement is a statement with no effect.
type EmptyStatement struct {
	statementNode
}

var _ Node = (*EmptyStatement)(nil)
var _ Statement = (*EmptyStatement)(nil)

const EmptyStatementKind NodeKind = "EmptyStatement"

// MultiStatement groups multiple statements into one; unlike a block, it doesn't introduce a new lexical scope.
type MultiStatement struct {
	statementNode
	Statements []Statement `json:"statements"`
}

var _ Node = (*MultiStatement)(nil)
var _ Statement = (*MultiStatement)(nil)

const MultiStatementKind NodeKind = "MultiStatement"

// ExpressionStatement performs an expression, in a statement position, and ignores its result.
type ExpressionStatement struct {
	statementNode
	Expression Expression `json:"expression"`
}

var _ Node = (*ExpressionStatement)(nil)
var _ Statement = (*ExpressionStatement)(nil)

const ExpressionStatementKind NodeKind = "ExpressionStatement"
