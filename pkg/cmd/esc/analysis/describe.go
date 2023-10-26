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

import (
	"strings"

	"github.com/pulumi/esc"
)

// Describe returns a Markdown-formatted description of the entity at the indicated position.
func (a *Analysis) Describe(pos esc.Pos) (string, bool) {
	// Fetch the expression (if any) at pos.
	x, where, ok := a.expressionAtPos(pos)
	if !ok {
		return "", false
	}

	// The description we return depends on the type of the expression and where the position lies inside that
	// expression.
	//
	// - If the position is within a builtin (but not within its argument), we return a description of the builtin
	// - If the position is within a symbol, we return the type of the symbol
	switch {
	case x.Builtin != nil:
		return a.describeBuiltin(x.Builtin)
	case len(x.Symbol) != 0:
		return a.describeSymbol(x)
	}

	// Find the nearest builtin in the traversal. If a builtin is found, return the description of the argument on
	// the path from the builtin.
	for i := len(where) - 1; i >= 0; i-- {
		r := where[i].receiver
		if r.Builtin != nil {
			return a.describeArgument(r.Builtin, where[i+1:], x, pos)
		}
	}

	return "", false
}

func (a *Analysis) describeBuiltin(builtin *esc.BuiltinExpr) (string, bool) {
	switch builtin.Name {
	case "fn::fromJSON":
		return "Decodes a value from its JSON representation.", true
	case "fn::fromBase64":
		return "Decodes a string from its Base64 representation.", true
	case "fn::join":
		return "Concatenates the elements of its second argument to create a single string. The first argument is " +
			"placed between each element in the result.", true
	case "fn::open":
		return "Fetches values from an external source when the environment is opened.", true
	case "fn::secret":
		return "Marks a value as secret.", true
	case "fn::toBase64":
		return "Encodes a string into its Base64 representation.", true
	case "fn::toJSON":
		return "Encodes a value into its JSON representation.", true
	case "fn::toString":
		return "Encodes a value into its string representation.", true
	default:
		if strings.HasPrefix(builtin.Name, "fn::open::") {
			return "Fetches values from an external source when the environment is opened.", true
		}
		return "", false
	}
}

func (a *Analysis) describeSymbol(x *esc.Expr) (string, bool) {
	if x.Schema.Always {
		return "any", true
	}
	return x.Schema.Type, x.Schema.Type != ""
}

func (a *Analysis) describeArgument(builtin *esc.BuiltinExpr, path []traverser, x *esc.Expr, pos esc.Pos) (string, bool) {
	// If the last element is an object expression, extend the path with the key associated with pos.
	if len(x.Object) != 0 {
		for k, rng := range x.KeyRanges {
			if rng.Contains(pos) {
				path = append(path, newObjectTraverser(*x, k))
				break
			}
		}
	}

	s := builtin.ArgSchema
	for _, t := range path {
		if t.key != "" {
			s = s.Property(t.key)
		} else {
			s = s.Item(t.index)
		}
	}
	return s.Description, s.Description != ""
}
