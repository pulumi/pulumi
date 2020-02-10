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

// bindTraversalTypes computes the type for each element of the given traversal.
func (b *binder) bindTraversalTypes(receiver Type, traversal hcl.Traversal) ([]Type, hcl.Diagnostics) {
	types := make([]Type, len(traversal)+1)
	types[0] = receiver

	var diagnostics hcl.Diagnostics
	for i, part := range traversal {
		var index cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			index = cty.StringVal(part.Name)
		case hcl.TraverseIndex:
			index = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
		}

		nextReceiver, indexDiags := b.bindIndexType(receiver, ctyTypeToType(index.Type(), false), index, part.SourceRange())
		types[i+1], receiver, diagnostics = nextReceiver, nextReceiver, append(diagnostics, indexDiags...)
	}

	return types, diagnostics
}

// bindIndexType computes the type of the result of applying the given index to the given receiver type.
// - If the receiver is an optional(T), the result will be optional(bindIndexType(T, ...))
// - If the receiver is an output(T), the result will be output(bindIndexType(T, ...))
// - If the receiver is a promise(T), the result will be promise(bindIndexType(T, ...))
// - If the receiver is a map(T), the index must be assignable to string and the result will be T
// - If the receiver is an array(T), the index must be assignable to number and the result will be T
// - If the receiver is an object({K_0 = T_0, ..., K_N = T_N}), the index must be assignable to string and the result
//   will be object[K].
func (b *binder) bindIndexType(receiver Type, indexType Type, indexVal cty.Value,
	indexRange hcl.Range) (Type, hcl.Diagnostics) {

	switch receiver := receiver.(type) {
	case *OptionalType:
		elementType, diagnostics := b.bindIndexType(receiver.ElementType, indexType, indexVal, indexRange)
		return NewOptionalType(elementType), diagnostics
	case *OutputType:
		elementType, diagnostics := b.bindIndexType(receiver.ElementType, indexType, indexVal, indexRange)
		return NewOutputType(elementType), diagnostics
	case *PromiseType:
		elementType, diagnostics := b.bindIndexType(receiver.ElementType, indexType, indexVal, indexRange)
		return NewPromiseType(elementType), diagnostics
	case *MapType:
		var diagnostics hcl.Diagnostics
		if !inputType(StringType).AssignableFrom(indexType) {
			diagnostics = hcl.Diagnostics{unsupportedMapKey(indexRange)}
		}
		return receiver.ElementType, diagnostics
	case *ArrayType:
		var diagnostics hcl.Diagnostics
		if !inputType(NumberType).AssignableFrom(indexType) {
			diagnostics = hcl.Diagnostics{unsupportedArrayIndex(indexRange)}
		}
		return receiver.ElementType, diagnostics
	case *ObjectType:
		if !inputType(StringType).AssignableFrom(indexType) {
			return AnyType, hcl.Diagnostics{unsupportedObjectProperty(indexRange)}
		}

		if indexVal == cty.DynamicVal {
			return AnyType, nil
		}

		propertyName := indexVal.AsString()
		propertyType, hasProperty := receiver.Properties[propertyName]
		if !hasProperty {
			return AnyType, hcl.Diagnostics{unknownObjectProperty(propertyName, indexRange)}
		}
		return propertyType, nil
	default:
		if receiver == AnyType {
			return AnyType, nil
		}
		return AnyType, hcl.Diagnostics{unsupportedReceiverType(receiver, indexRange)}
	}
}
