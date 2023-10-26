// Copyright 2023, Pulumi Corporation.
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

package analysis

import "github.com/pulumi/esc"

// A traverser represents part of a path through a document.
type traverser struct {
	receiver esc.Expr
	index    int
	key      string
}

func newRootTraverser(key string) traverser {
	return traverser{key: key}
}

func newArrayTraverser(receiver esc.Expr, index int) traverser {
	return traverser{receiver: receiver, index: index}
}

func newObjectTraverser(receiver esc.Expr, key string) traverser {
	return traverser{receiver: receiver, key: key}
}

// ExpressionAtPos returns the expression that contains pos. If no such expression exists, expressionAtPos returns
// (nil, false). The returned expression is the "smallest" expression that contains pos.
func (a *Analysis) ExpressionAtPos(pos esc.Pos) (*esc.Expr, bool) {
	x, _, ok := a.expressionAtPos(pos)
	return x, ok
}

// expressionAtPos returns the expression that contains pos. If no such expression exists, expressionAtPos returns
// (nil, nil, false). The returned expression is the "smallest" expression that contains pos. The path to the given
// expression is also returned.
func (a *Analysis) expressionAtPos(pos esc.Pos) (*esc.Expr, []traverser, bool) {
	for key, x := range a.env.Exprs {
		if x, where, ok := expressionAtPos(x, []traverser{newRootTraverser(key)}, pos); ok {
			return x, where, true
		}
	}
	return nil, nil, false
}

func expressionAtPos(root esc.Expr, where []traverser, pos esc.Pos) (*esc.Expr, []traverser, bool) {
	if !root.Range.Contains(pos) {
		return nil, nil, false
	}

	switch {
	case len(root.List) != 0:
		for i, element := range root.List {
			here := append(where, newArrayTraverser(root, i))
			if x, where, ok := expressionAtPos(element, here, pos); ok {
				return x, where, true
			}
		}
	case len(root.Object) != 0:
		for key, property := range root.Object {
			here := append(where, newObjectTraverser(root, key))
			if x, where, ok := expressionAtPos(property, here, pos); ok {
				return x, where, true
			}
		}
	case root.Builtin != nil:
		here := append(where, newObjectTraverser(root, root.Builtin.Name))
		if x, where, ok := expressionAtPos(root.Builtin.Arg, here, pos); ok {
			return x, where, true
		}
	}

	return &root, where, true
}
