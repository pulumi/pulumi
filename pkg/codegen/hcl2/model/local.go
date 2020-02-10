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
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// LocalVariable represents a program- or component-scoped local variable.
type LocalVariable struct {
	// The syntax node associated with the local variable.
	Syntax *hclsyntax.Attribute

	// The name of the local variable.
	Name string
	// The type of the local variable.
	VariableType Type
	// The value of the local variable.
	Value Expression

	state bindState
	deps  []Node
}

// SyntaxNode returns the syntax node associated with the local variable.
func (lv *LocalVariable) SyntaxNode() hclsyntax.Node {
	return lv.Syntax
}

// Type returns the type of the local variable.
func (lv *LocalVariable) Type() Type {
	return lv.VariableType
}

func (lv *LocalVariable) getState() bindState {
	return lv.state
}

func (lv *LocalVariable) setState(s bindState) {
	lv.state = s
}

func (lv *LocalVariable) getDependencies() []Node {
	return lv.deps
}

func (lv *LocalVariable) setDependencies(nodes []Node) {
	lv.deps = nodes
}

func (*LocalVariable) isNode() {}
