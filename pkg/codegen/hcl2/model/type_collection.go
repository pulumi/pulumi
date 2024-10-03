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
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// unwrapIterableSourceType removes any eventual types that wrap a type intended for iteration.
func unwrapIterableSourceType(t Type) Type {
	// TODO(pdg): unions
	for {
		switch tt := t.(type) {
		case *OutputType:
			t = tt.ElementType
		case *PromiseType:
			t = tt.ElementType
		case *UnionType:
			// option(T) is implemented as union(T, None)
			// so we unwrap the optional type here
			if len(tt.ElementTypes) == 2 && tt.ElementTypes[0] == NoneType {
				t = tt.ElementTypes[1]
			} else if len(tt.ElementTypes) == 2 && tt.ElementTypes[1] == NoneType {
				t = tt.ElementTypes[0]
			} else {
				return t
			}
		default:
			return t
		}
	}
}

// wrapIterableResultType adds optional or eventual types to a type intended for iteration per the structure of the
// source type.
func wrapIterableResultType(sourceType, iterableType Type) Type {
	// TODO(pdg): unions
	for {
		switch t := sourceType.(type) {
		case *OutputType:
			sourceType, iterableType = t.ElementType, NewOutputType(iterableType)
		case *PromiseType:
			sourceType, iterableType = t.ElementType, NewPromiseType(iterableType)
		default:
			return iterableType
		}
	}
}

// GetCollectionTypes returns the key and value types of the given type if it is a collection.
func GetCollectionTypes(collectionType Type, rng hcl.Range, strict bool) (Type, Type, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics
	var keyType, valueType Type
	// Poke through any eventual and optional types that may wrap the collection type.
	unwrappedCollectionType := unwrapIterableSourceType(collectionType)
	switch collectionType := unwrappedCollectionType.(type) {
	case *ListType:
		keyType, valueType = NumberType, collectionType.ElementType
	case *MapType:
		keyType, valueType = StringType, collectionType.ElementType
	case *TupleType:
		keyType = NumberType
		valueType, _ = UnifyTypes(collectionType.ElementTypes...)
	case *ObjectType:
		keyType = StringType

		types := slice.Prealloc[Type](len(collectionType.Properties))
		for _, t := range collectionType.Properties {
			types = append(types, t)
		}
		valueType, _ = UnifyTypes(types...)
	default:
		// If the collection is a dynamic type, treat it as an iterable(dynamic, dynamic).
		// Otherwise, if we are in strict-mode, issue an error.
		if collectionType != DynamicType {
			unsupportedError := unsupportedCollectionType(collectionType, rng)
			if !strict {
				unsupportedError.Severity = hcl.DiagWarning
			}
			diagnostics = append(diagnostics, unsupportedError)
		}
		keyType, valueType = DynamicType, DynamicType
	}
	return keyType, valueType, diagnostics
}
