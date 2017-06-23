// Copyright 2016-2017, Pulumi Corporation
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

package ast

// Statement is an element inside of an executable function body.
type Statement interface {
	Node
	statement()
}

type StatementNode struct {
	NodeValue
}

func (node *StatementNode) statement() {}

/* Imports */

// Import is mostly used to trigger the side-effect of importing another module.  It is also used to make bound modules
// or module member names available in the context of the importing module or function.
type Import struct {
	StatementNode
	Referent *Token      `json:"referent"`       // the module or member token to import.
	Name     *Identifier `json:"name,omitempty"` // the name that token is bound to (if any).
}

var _ Node = (*Import)(nil)
var _ Statement = (*Import)(nil)

const ImportKind NodeKind = "Import"

/* Blocks */

// Block is a grouping of statements that enjoy their own lexical scope.
type Block struct {
	StatementNode
	Statements []Statement `json:"statements"`
}

var _ Node = (*Block)(nil)
var _ Statement = (*Block)(nil)

const BlockKind NodeKind = "Block"

/* Local Variables */

type LocalVariableDeclaration struct {
	StatementNode
	Local *LocalVariable `json:"local"`
}

var _ Node = (*LocalVariableDeclaration)(nil)
var _ Statement = (*LocalVariableDeclaration)(nil)

const LocalVariableDeclarationKind NodeKind = "LocalVariableDeclaration"

/* Try/Catch/Finally */

type TryCatchFinally struct {
	StatementNode
	TryClause     Statement          `json:"tryClause"`
	CatchClauses  *[]*TryCatchClause `json:"catchClauses,omitempty"`
	FinallyClause Statement          `json:"finallyClause"`
}

var _ Node = (*TryCatchFinally)(nil)
var _ Statement = (*TryCatchFinally)(nil)

const TryCatchFinallyKind NodeKind = "TryCatchFinally"

type TryCatchClause struct {
	NodeValue
	Exception *LocalVariable `json:"exception,omitempty"`
	Body      Statement      `json:"body"`
}

var _ Node = (*TryCatchClause)(nil)

const TryCatchClauseKind NodeKind = "TryCatchClause"

/* Branches */

// BreakStatement is the usual C-style `break` (only valid within loops).
type BreakStatement struct {
	StatementNode
	Label *Identifier `json:"identifier,omitempty"`
}

var _ Node = (*BreakStatement)(nil)
var _ Statement = (*BreakStatement)(nil)

const BreakStatementKind NodeKind = "BreakStatement"

// ContinueStatement is the usual C-style `continue` (only valid within loops).
type ContinueStatement struct {
	StatementNode
	Label *Identifier `json:"identifier,omitempty"`
}

var _ Node = (*ContinueStatement)(nil)
var _ Statement = (*ContinueStatement)(nil)

const ContinueStatementKind NodeKind = "ContinueStatement"

// IfStatement is the usual C-style `if`.
type IfStatement struct {
	StatementNode
	Condition  Expression `json:"condition"`           // a `bool` conditional expression.
	Consequent Statement  `json:"consequent"`          // the statement to execute if `true`.
	Alternate  *Statement `json:"alternate,omitempty"` // the optional statement to execute if `false`.
}

var _ Node = (*IfStatement)(nil)
var _ Statement = (*IfStatement)(nil)

const IfStatementKind NodeKind = "IfStatement"

// SwitchStatement is like a typical C-style `switch`.
type SwitchStatement struct {
	StatementNode
	Expression Expression    `json:"expression"` // the value being switched upon.
	Cases      []*SwitchCase `json:"cases"`      // the list of switch cases to be matched, in order.
}

var _ Node = (*SwitchStatement)(nil)
var _ Statement = (*SwitchStatement)(nil)

const SwitchStatementKind NodeKind = "SwitchStatement"

// SwitchCase is a single case of a switch to be matched.
type SwitchCase struct {
	NodeValue
	Clause     *Expression `json:"clause,omitempty"` // the optional switch clause; if nil, default.
	Consequent Statement   `json:"consequent"`       // the statement to execute if there is a match.
}

var _ Node = (*SwitchCase)(nil)

const SwitchCaseKind NodeKind = "SwitchCase"

// LabeledStatement associates an identifier with a statement for purposes of labeled jumps.
type LabeledStatement struct {
	StatementNode
	Label     *Identifier `json:"label"`
	Statement Statement   `json:"statement"`
}

var _ Node = (*LabeledStatement)(nil)
var _ Statement = (*LabeledStatement)(nil)

const LabeledStatementKind NodeKind = "LabeledStatement"

// ReturnStatement is the usual C-style `return`, to exit a function.
type ReturnStatement struct {
	StatementNode
	Expression *Expression `json:"expression,omitempty"`
}

var _ Node = (*ReturnStatement)(nil)
var _ Statement = (*ReturnStatement)(nil)

const ReturnStatementKind NodeKind = "ReturnStatement"

// ThrowStatement maps to raising an exception, usually `throw`, in the source language.
type ThrowStatement struct {
	StatementNode
	Expression Expression `json:"expression"`
}

var _ Node = (*ThrowStatement)(nil)
var _ Statement = (*ThrowStatement)(nil)

const ThrowStatementKind NodeKind = "ThrowStatement"

// WhileStatement is the usual C-style `while`.
type WhileStatement struct {
	StatementNode
	Condition *Expression `json:"condition,omitempty"` // a `bool` statement indicating whether to cotinue.
	Body      Statement   `json:"body"`                // the body to execute provided the condition remains `true`.
}

var _ Node = (*WhileStatement)(nil)
var _ Statement = (*WhileStatement)(nil)

const WhileStatementKind NodeKind = "WhileStatement"

// ForStatement is the usual C-style `while`.
type ForStatement struct {
	StatementNode
	Init      *Statement  `json:"init,omitempty"`      // an initialization statement.
	Condition *Expression `json:"condition,omitempty"` // a `bool` statement indicating whether to continue.
	Post      *Statement  `json:"post,omitempty"`      // a statement to run after the body, before the next iteration.
	Body      Statement   `json:"body"`                // the body to execute provided the condition remains `true`.
}

var _ Node = (*ForStatement)(nil)
var _ Statement = (*ForStatement)(nil)

const ForStatementKind NodeKind = "ForStatement"

/* Miscellaneous */

// EmptyStatement is a statement with no effect.
type EmptyStatement struct {
	StatementNode
}

var _ Node = (*EmptyStatement)(nil)
var _ Statement = (*EmptyStatement)(nil)

const EmptyStatementKind NodeKind = "EmptyStatement"

// MultiStatement groups multiple statements into one; unlike a block, it doesn't introduce a new lexical scope.
type MultiStatement struct {
	StatementNode
	Statements []Statement `json:"statements"`
}

var _ Node = (*MultiStatement)(nil)
var _ Statement = (*MultiStatement)(nil)

const MultiStatementKind NodeKind = "MultiStatement"

// ExpressionStatement performs an expression, in a statement position, and ignores its result.
type ExpressionStatement struct {
	StatementNode
	Expression Expression `json:"expression"`
}

var _ Node = (*ExpressionStatement)(nil)
var _ Statement = (*ExpressionStatement)(nil)

const ExpressionStatementKind NodeKind = "ExpressionStatement"
