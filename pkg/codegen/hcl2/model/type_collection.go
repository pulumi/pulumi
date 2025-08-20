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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
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

func unifyObjects(objects ...*ObjectType) *ObjectType {
	unifiedProperties := make(map[string]Type)
	propertyKeys := codegen.NewStringSet()
	for _, obj := range objects {
		for name := range obj.Properties {
			propertyKeys.Add(name)
		}
	}

	// returns whether a property is partial, meaning that it is not present in all objects.
	// the unified type will be an optional type if any of the objects is missing the property.
	isPartial := func(propertyName string) bool {
		for _, obj := range objects {
			if _, ok := obj.Properties[propertyName]; !ok {
				return true
			}
		}
		return false
	}

	for _, name := range propertyKeys.SortedValues() {
		totalPropertyType := make([]Type, 0, len(objects))
		for _, obj := range objects {
			if propType, ok := obj.Properties[name]; ok {
				totalPropertyType = append(totalPropertyType, propType)
			}
		}
		valueT, _ := UnifyTypes(totalPropertyType...)
		if isPartial(name) {
			valueT = NewOptionalType(valueT)
		}
		unifiedProperties[name] = valueT
	}

	return NewObjectType(unifiedProperties)
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
		valueTypesAreObjects := true
		for _, t := range collectionType.ElementTypes {
			if _, ok := t.(*ObjectType); !ok {
				valueTypesAreObjects = false
				break
			}
		}

		if valueTypesAreObjects {
			objectTypes := make([]*ObjectType, 0, len(collectionType.ElementTypes))
			for _, t := range collectionType.ElementTypes {
				if objType, ok := t.(*ObjectType); ok {
					objectTypes = append(objectTypes, objType)
				}
			}
			valueType = unifyObjects(objectTypes...)
		} else {
			// If the collection is a tuple, we unify the element types.
			valueType, _ = UnifyTypes(collectionType.ElementTypes...)
		}
	case *ObjectType:
		keyType = StringType

		types := slice.Prealloc[Type](len(collectionType.Properties))
		for _, t := range collectionType.Properties {
			types = append(types, t)
		}
		// before unifying, we check if the types are objects themselves, if that is the case
		// we simplify the type into a single object type
		// so that instead of having a union of objects, we have a single object type
		// with the properties of all the objects unified.
		valueTypesAreObjects := true
		for _, t := range types {
			if _, ok := t.(*ObjectType); !ok {
				valueTypesAreObjects = false
				break
			}
		}

		if valueTypesAreObjects {
			objectTypes := make([]*ObjectType, 0, len(types))
			for _, t := range types {
				if objType, ok := t.(*ObjectType); ok {
					objectTypes = append(objectTypes, objType)
				}
			}

			valueType = unifyObjects(objectTypes...)
		} else {
			valueType, _ = UnifyTypes(types...)
		}

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
