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

type statement struct {
	node
}

func (node *statement) statement() {}

/* Blocks */

type Block struct {
	statement
	Statements []Statement `json:"statements"`
}

/* Local Variables */

type LocalVariableDeclaration struct {
	statement
	Local *LocalVariable `json:"local"`
}

/* Try/Catch/Finally */

type TryCatchFinally struct {
	statement
	TryBlock     *Block            `json:"tryBlock"`
	CatchBlocks  *[]*TryCatchBlock `json:"catchBlocks,omitempty"`
	FinallyBlock *Block            `json:"finallyBlock"`
}

type TryCatchBlock struct {
	node
	Block     *Block             `json:"block"`
	Exception *symbols.TypeToken `json:"exception,omitempty"`
}

/* Branches */

// BreakStatement is the usual C-style `break` (only valid within loops).
type BreakStatement struct {
	statement
	Label *Identifier `json:"identifier,omitempty"`
}

// ContinueStatement is the usual C-style `continue` (only valid within loops).
type ContinueStatement struct {
	statement
	Label *Identifier `json:"identifier,omitempty"`
}

// IfStatement is the usual C-style `if`.  To simplify the MuIL AST, this is the only conditional statement available.
// All higher-level conditional constructs such as `switch`, if`/`else if`/..., etc., must be desugared into it.
type IfStatement struct {
	statement
	Condition  Expression `json:"expression"`            // a `bool` conditional expression.
	Consequent Statement  `json:"consequent"`            // the statement to execute if `true`.
	Alternate  *Statement `json:"alternative,omitempty"` // the optional statement to execute if `false`.
}

// LabeledStatement associates an identifier with a statement for purposes of labeled jumps.
type LabeledStatement struct {
	statement
	Label     *Identifier `json:"label"`
	Statement Statement   `json:"statement"`
}

// ReturnStatement is the usual C-style `return`, to exit a function.
type ReturnStatement struct {
	statement
	Expression *Expression `json:"expression,omitempty"`
}

// ThrowStatement maps to raising an exception, usually `throw`, in the source language.
type ThrowStatement struct {
	statement
	Expression *Expression `json:"expression,omitempty"`
}

// WhileStatement is the usual C-style `while`.  To simplify the MuIL AST, this is the only looping statement available.
// All higher-level looping constructs such as `for`, `foreach`, `do`/`while`, etc. must be desugared into it.
type WhileStatement struct {
	statement
	Test Expression `json:"test"`  // a `bool` statement indicating whether to condition.
	Body *Block     `json:"block"` // the body to execute provided the test remains `true`.
}

/* Miscellaneous */

// EmptyStatement is a statement with no effect.
type EmptyStatement struct {
	statement
}

// MultiStatement groups multiple statements into one; unlike a block, it doesn't introduce a new lexical scope.
type MultiStatement struct {
	statement
	Statements []Statement `json:"statements"`
}

// ExpressionStatement performs an expression, in a statement position, and ignores its result.
type ExpressionStatement struct {
	statement
	Expression Expression `json:"expression"`
}
