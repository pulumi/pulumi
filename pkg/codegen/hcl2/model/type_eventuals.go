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

type typeTransform int

var (
	makeIdentity = typeTransform(0)
	makePromise  = typeTransform(1)
	makeOutput   = typeTransform(2)
)

func (f typeTransform) do(t Type) Type {
	switch f {
	case makePromise:
		return NewPromiseType(t)
	case makeOutput:
		return NewOutputType(t)
	default:
		return t
	}
}

func resolveEventuals(t Type, resolveOutputs bool) (Type, typeTransform) {
	switch t := t.(type) {
	case *OutputType:
		if resolveOutputs {
			return t.ElementType, makeOutput
		}
		return t, makeIdentity
	case *PromiseType:
		element, transform := resolveEventuals(t.ElementType, resolveOutputs)
		if makePromise > transform {
			transform = makePromise
		}
		return element, transform
	case *MapType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewMapType(resolved), transform
	case *ListType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewListType(resolved), transform
	case *SetType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewSetType(resolved), transform
	case *UnionType:
		transform := makeIdentity
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			element, elementTransform := resolveEventuals(t, resolveOutputs)
			if elementTransform > transform {
				transform = elementTransform
			}
			elementTypes[i] = element
		}
		return NewUnionType(elementTypes...), transform
	case *ObjectType:
		transform := makeIdentity
		properties := map[string]Type{}
		for k, t := range t.Properties {
			property, propertyTransform := resolveEventuals(t, resolveOutputs)
			if propertyTransform > transform {
				transform = propertyTransform
			}
			properties[k] = property
		}
		return NewObjectType(properties), transform
	case *TupleType:
		transform := makeIdentity
		elements := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			element, elementTransform := resolveEventuals(t, resolveOutputs)
			if elementTransform > transform {
				transform = elementTransform
			}
			elements[i] = element
		}
		return NewTupleType(elements...), transform
	default:
		return t, makeIdentity
	}
}

// ResolveOutputs recursively replaces all output(T) and promise(T) types in the input type with their element type.
func ResolveOutputs(t Type) Type {
	resolved, _ := resolveEventuals(t, true)
	return resolved
}

// ResolvePromises recursively replaces all promise(T) types in the input type with their element type.
func ResolvePromises(t Type) Type {
	resolved, _ := resolveEventuals(t, false)
	return resolved
}

func liftOperationType(resultType Type, arguments ...Expression) Type {
	var transform typeTransform
	for _, arg := range arguments {
		_, t := resolveEventuals(arg.Type(), true)
		if t > transform {
			transform = t
		}
	}
	return transform.do(resultType)
}

var inputTypes = map[Type]Type{}

// InputType returns the result of replacing each type in T with union(T, output(T)).
func InputType(t Type) Type {
	if t == DynamicType || t == NoneType {
		return t
	}
	if input, ok := inputTypes[t]; ok {
		return input
	}

	var src Type
	switch t := t.(type) {
	case *OutputType:
		return t
	case *PromiseType:
		src = NewPromiseType(InputType(t.ElementType))
	case *MapType:
		src = NewMapType(InputType(t.ElementType))
	case *ListType:
		src = NewListType(InputType(t.ElementType))
	case *UnionType:
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			elementTypes[i] = InputType(t)
		}
		src = NewUnionType(elementTypes...)
	case *ObjectType:
		properties := map[string]Type{}
		for k, t := range t.Properties {
			properties[k] = InputType(t)
		}
		src = NewObjectType(properties)
	default:
		src = t
	}

	input := NewUnionType(src, NewOutputType(src))
	inputTypes[t] = input
	return input
}
