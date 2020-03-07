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
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type Traversable interface {
	Traverse(t hcl.Traverser) (Traversable, hcl.Diagnostics)
}

type TypedTraversable interface {
	Type() Type
}

func GetTraversableType(t Traversable) Type {
	switch t := t.(type) {
	case TypedTraversable:
		return t.Type()
	case Type:
		return t
	default:
		return AnyType
	}
}

func GetTraverserKey(t hcl.Traverser) (cty.Value, Type) {
	switch t := t.(type) {
	case hcl.TraverseAttr:
		return cty.StringVal(t.Name), StringType
	case hcl.TraverseIndex:
		if t.Key.Type().Equals(typeCapsule) {
			return cty.DynamicVal, *(t.Key.EncapsulatedValue().(*Type))
		}
		return t.Key, ctyTypeToType(t.Key.Type(), false)
	default:
		contract.Failf("unexpected traverser of type %T (%v)", t, t.SourceRange())
		return cty.DynamicVal, AnyType
	}
}

// bindTraversalParts computes the type for each element of the given traversal.
func (b *expressionBinder) bindTraversalParts(receiver Traversable,
	traversal hcl.Traversal) ([]Traversable, hcl.Diagnostics) {

	parts := make([]Traversable, len(traversal)+1)
	parts[0] = receiver

	var diagnostics hcl.Diagnostics
	for i, part := range traversal {
		nextReceiver, partDiags := parts[i].Traverse(part)
		parts[i+1], diagnostics = nextReceiver, append(diagnostics, partDiags...)
	}

	// TODO: expand this to all untyped traversables
	if _, isScope := parts[len(parts)-1].(*Scope); isScope {
		diagnostics = append(diagnostics, undefinedVariable(traversal.SourceRange()))
	}

	return parts, diagnostics
}
