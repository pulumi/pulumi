// Copyright 2016, Pulumi Corporation.
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

// resolveEventuals rebuilds a type without the eventual wrappers that strip names: every output and promise for
// makeOutput, every promise for makePromise, and none for makeIdentity. The wrappers it keeps hold resolved
// elements (README §2.3). The transform it returns is the outermost kind of eventual that the type held.
func resolveEventuals(t Type, strip typeTransform) (Type, typeTransform) {
	return resolveEventualsImpl(t, strip, map[resolveKey]Type{})
}

// resolveKey identifies an object under resolution. One object resolves to one copy for each context in which the
// walk reaches it, because a copy inside an output holds no eventuals and a copy outside may.
type resolveKey struct {
	t     Type
	strip typeTransform
}

func resolveEventualsImpl(t Type, strip typeTransform, seen map[resolveKey]Type) (Type, typeTransform) {
	switch t := t.(type) {
	case *OutputType:
		element, _ := resolveEventualsImpl(t.ElementType, makeOutput, seen)
		if strip == makeOutput {
			return element, makeOutput
		}
		return newOutputType(element), makeOutput
	case *PromiseType:
		element, transform := resolveEventualsImpl(t.ElementType, max(strip, makePromise), seen)
		if strip != makeIdentity {
			return element, max(transform, makePromise)
		}
		return newPromiseType(element), makePromise
	case *MapType:
		resolved, transform := resolveEventualsImpl(t.ElementType, strip, seen)
		return NewMapType(resolved), transform
	case *ListType:
		resolved, transform := resolveEventualsImpl(t.ElementType, strip, seen)
		return NewListType(resolved), transform
	case *SetType:
		resolved, transform := resolveEventualsImpl(t.ElementType, strip, seen)
		return NewSetType(resolved), transform
	case *UnionType:
		transform := makeIdentity
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			element, elementTransform := resolveEventualsImpl(t, strip, seen)
			transform = max(transform, elementTransform)
			elementTypes[i] = element
		}
		return NewUnionTypeAnnotated(elementTypes, t.Annotations...), transform
	case *ObjectType:
		transform := makeIdentity
		if already, ok := seen[resolveKey{t, strip}]; ok {
			return already, transform
		}
		properties := map[string]Type{}
		objType := NewObjectType(properties, t.Annotations...)
		seen[resolveKey{t, strip}] = objType
		for k, t := range t.Properties {
			property, propertyTransform := resolveEventualsImpl(t, strip, seen)
			transform = max(transform, propertyTransform)
			properties[k] = property
		}
		return objType, transform
	case *TupleType:
		transform := makeIdentity
		elements := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			element, elementTransform := resolveEventualsImpl(t, strip, seen)
			transform = max(transform, elementTransform)
			elements[i] = element
		}
		return NewTupleType(elements...), transform
	default:
		return t, makeIdentity
	}
}

// ResolveOutputs recursively replaces all output(T) and promise(T) types in the input type with their element type.
func ResolveOutputs(t Type) Type {
	containsOutputs, containsPromises := ContainsEventuals(t)
	if !containsOutputs && !containsPromises {
		return t
	}

	resolved, _ := resolveEventuals(t, makeOutput)
	return resolved
}

// ResolvePromises recursively replaces all promise(T) types in the input type with their element type.
func ResolvePromises(t Type) Type {
	if !ContainsPromises(t) {
		return t
	}

	resolved, _ := resolveEventuals(t, makePromise)
	return resolved
}

// ContainsEventuals returns true if the input type contains output or promise types.
func ContainsEventuals(t Type) (containsOutputs, containsPromises bool) {
	containsOutputs = ContainsOutputs(t)
	containsPromises = ContainsPromises(t)
	return containsOutputs, containsPromises
}

// ContainsOutputs returns true if the input type contains output types.
func ContainsOutputs(t Type) bool {
	seenTypes := map[Type]struct{}{}
	return containsOutputsImpl(t, seenTypes)
}

func containsOutputsImpl(t Type, seen map[Type]struct{}) bool {
	if _, ok := seen[t]; ok {
		return false
	}
	seen[t] = struct{}{}

	switch t := t.(type) {
	case *OutputType:
		return true
	case *PromiseType:
		return containsOutputsImpl(t.ElementType, seen)
	case *MapType:
		return containsOutputsImpl(t.ElementType, seen)
	case *ListType:
		return containsOutputsImpl(t.ElementType, seen)
	case *SetType:
		return containsOutputsImpl(t.ElementType, seen)
	case *UnionType:
		for _, t := range t.ElementTypes {
			if containsOutputsImpl(t, seen) {
				return true
			}
		}
		return false
	case *ObjectType:
		for _, t := range t.Properties {
			if containsOutputsImpl(t, seen) {
				return true
			}
		}
		return false
	case *TupleType:
		for _, t := range t.ElementTypes {
			if containsOutputsImpl(t, seen) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// ContainsPromises returns true if the input type contains promise types.
func ContainsPromises(t Type) bool {
	seenTypes := map[Type]struct{}{}
	return containsPromisesImpl(t, seenTypes)
}

func containsPromisesImpl(t Type, seen map[Type]struct{}) bool {
	if _, ok := seen[t]; ok {
		return false
	}
	seen[t] = struct{}{}

	switch t := t.(type) {
	case *PromiseType:
		return true
	case *OutputType:
		return containsPromisesImpl(t.ElementType, seen)
	case *MapType:
		return containsPromisesImpl(t.ElementType, seen)
	case *ListType:
		return containsPromisesImpl(t.ElementType, seen)
	case *SetType:
		return containsPromisesImpl(t.ElementType, seen)
	case *UnionType:
		for _, t := range t.ElementTypes {
			if containsPromisesImpl(t, seen) {
				return true
			}
		}
		return false
	case *ObjectType:
		for _, t := range t.Properties {
			if containsPromisesImpl(t, seen) {
				return true
			}
		}
		return false
	case *TupleType:
		for _, t := range t.ElementTypes {
			if containsPromisesImpl(t, seen) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func liftOperationType(resultType Type, arguments ...Expression) Type {
	var transform typeTransform
	for _, arg := range arguments {
		_, t := resolveEventuals(arg.Type(), makeOutput)
		if t > transform {
			transform = t
		}
	}
	return transform.do(resultType)
}

// InputType returns the result of replacing each type in T with union(T, output(T)).
func InputType(t Type) Type {
	return inputTypeImpl(t, map[Type]Type{})
}

func inputTypeImpl(t Type, seen map[Type]Type) Type {
	if t == DynamicType || t == NoneType {
		return t
	}

	var src Type
	switch t := t.(type) {
	case *OutputType:
		return t
	case *PromiseType:
		src = NewPromiseType(inputTypeImpl(t.ElementType, seen))
	case *MapType:
		src = NewMapType(inputTypeImpl(t.ElementType, seen))
	case *ListType:
		src = NewListType(inputTypeImpl(t.ElementType, seen))
	case *TupleType:
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			elementTypes[i] = inputTypeImpl(t, seen)
		}
		src = NewTupleType(elementTypes...)
	case *UnionType:
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			elementTypes[i] = inputTypeImpl(t, seen)
		}
		src = NewUnionTypeAnnotated(elementTypes, t.Annotations...)
	case *ObjectType:
		if already, ok := seen[t]; ok {
			return already
		}

		properties := map[string]Type{}
		src = NewObjectType(properties, t.Annotations...)
		seen[t] = src
		for k, t := range t.Properties {
			properties[k] = inputTypeImpl(t, seen)
		}
	default:
		src = t
	}

	return NewUnionType(src, NewOutputType(src))
}
