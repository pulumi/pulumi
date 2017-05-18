// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package symbols

import (
	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Variable is an interface common to all variables.
type Variable interface {
	Symbol
	Type() Type
	Default() *interface{}
	Readonly() bool
	VarNode() ast.Variable
}

var _ Symbol = (Variable)(nil)

// LocalVariable is a fully bound local variable symbol.
type LocalVariable struct {
	Node *ast.LocalVariable
	Nm   tokens.Name
	Ty   Type
	Spec bool // true if this local variable is "special" (usually this/super).
}

var _ Symbol = (*LocalVariable)(nil)
var _ Variable = (*LocalVariable)(nil)

func (node *LocalVariable) Name() tokens.Name   { return node.Nm }
func (node *LocalVariable) Token() tokens.Token { return tokens.Token(node.Name()) }
func (node *LocalVariable) Special() bool       { return node.Spec }
func (node *LocalVariable) Tree() diag.Diagable { return node.Node }
func (node *LocalVariable) Readonly() bool {
	return node.Node != nil && node.Node.Readonly != nil && *node.Node.Readonly
}
func (node *LocalVariable) Type() Type            { return node.Ty }
func (node *LocalVariable) Default() *interface{} { return nil }
func (node *LocalVariable) VarNode() ast.Variable { return node.Node }
func (node *LocalVariable) String() string        { return string(node.Token()) }

// NewLocalVariableSym returns a new LocalVariable symbol associated with the given AST node.
func NewLocalVariableSym(node *ast.LocalVariable, ty Type) *LocalVariable {
	return &LocalVariable{Node: node, Nm: node.Name.Ident, Ty: ty}
}

// NewSpecialVariableSym returns a "special" LocalVariable symbol that has no corresponding AST node and has a name.
func NewSpecialVariableSym(nm tokens.Name, ty Type) *LocalVariable {
	return &LocalVariable{Node: nil, Nm: nm, Ty: ty, Spec: true}
}
