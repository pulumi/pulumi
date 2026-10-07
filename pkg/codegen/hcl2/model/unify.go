// Copyright 2026, Pulumi Corporation.
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
	"maps"
	"slices"
)

// UnifyTypes returns the type that converts safely from every input type (README §5). The result is a function
// of the set of inputs: it does not depend on their order, and a union appears in it only between members that
// no structural rule merges.
func UnifyTypes(types ...Type) Type {
	// unify builds eventual wrappers around unresolved elements, because an element may hold an object that is
	// still under construction (README §12). The result is complete here, so one walk resolves every element.
	normalized, _ := resolveEventuals(unify(types, &mappingMerges{}), makeIdentity)
	return normalized
}

// mappingMerges holds the member lists of the mapping class under unification, innermost last (README §12). A list
// of objects maps to the object that its unification builds; a list with a map maps to nil.
type mappingMerges struct {
	inputs  [][]Type
	results []*ObjectType
}

// reentry resolves a member list that is already under unification. A list of objects unifies to the object in
// progress. A list with a map is recursive: when an object in progress lies between its two entries, the second
// entry reaches that object and ends, so the list unifies again; otherwise the recursion would not end, and the
// list unifies to its union, because a cycle through a map is not a valid type.
func (m *mappingMerges) reentry(members []Type) (Type, bool) {
	for i := len(m.inputs) - 1; i >= 0; i-- {
		if !slices.EqualFunc(m.inputs[i], members, Type.Equals) {
			continue
		}
		switch {
		case m.results[i] != nil:
			return m.results[i], true
		case slices.ContainsFunc(m.results[i+1:], func(o *ObjectType) bool { return o != nil }):
			return nil, false
		}
		return NewUnionType(members...), true
	}
	return nil, false
}

func (m *mappingMerges) push(members []Type, result *ObjectType) {
	m.inputs, m.results = append(m.inputs, members), append(m.results, result)
}

func (m *mappingMerges) pop() {
	m.inputs, m.results = m.inputs[:len(m.inputs)-1], m.results[:len(m.results)-1]
}

func unify(types []Type, seen *mappingMerges) Type {
	if len(types) == 0 { // U-Empty
		return NoneType
	}
	if !slices.ContainsFunc(types[1:], func(t Type) bool { return !t.Equals(types[0]) }) { // U-Eq
		return types[0]
	}
	members := canonicalMembers(types) // U-Flatten

	var result []Type
	if slices.Contains(members, NoneType) { // U-None
		result = append(result, NoneType)
		members = slices.DeleteFunc(members, func(t Type) bool { return t == NoneType })
	}
	if slices.Contains(members, Type(DynamicType)) { // U-Dynamic
		return NewUnionType(append(result, DynamicType)...)
	}

	var sequences, mappings, scalars []Type
	anyOutput, anyPromise, anyContainsOutputs := false, false, false
	for _, member := range members {
		switch member.(type) {
		case *OutputType:
			anyOutput = true
		case *PromiseType:
			anyPromise = true
		case *ListType, *SetType, *TupleType:
			sequences = append(sequences, member)
		case *MapType, *ObjectType:
			mappings = append(mappings, member)
		default:
			scalars = append(scalars, member)
		}
		anyContainsOutputs = anyContainsOutputs || ContainsOutputs(member)
	}

	switch {
	case anyOutput || (anyPromise && anyContainsOutputs): // U-Output
		result = append(result, newOutputType(unify(stripEventuals(members), seen)))
	case anyPromise: // U-Promise
		result = append(result, newPromiseType(unify(stripEventuals(members), seen)))
	default: // U-Union
		if sequence := unifySequences(sequences, seen); sequence != nil {
			result = append(result, sequence)
		}
		if mapping := unifyMappings(mappings, seen); mapping != nil {
			result = append(result, mapping)
		}
		result = append(result, maximalScalars(scalars)...)
	}
	return NewUnionType(result...)
}

// stripEventuals removes one top-level output or promise wrapper from each member.
func stripEventuals(members []Type) []Type {
	stripped := make([]Type, len(members))
	for i, member := range members {
		switch member := member.(type) {
		case *OutputType:
			stripped[i] = member.ElementType
		case *PromiseType:
			stripped[i] = member.ElementType
		default:
			stripped[i] = member
		}
	}
	return stripped
}

// unifySequences is rule U-Seq: the lists, sets, and tuples of a member list merge to one sequence, or to nil
// when there are none.
func unifySequences(members []Type, seen *mappingMerges) Type {
	if len(members) == 0 {
		return nil
	}
	if len(members) == 1 { // U-Eq: the member keeps its identity and its annotations
		return members[0]
	}
	if length, ok := commonTupleLength(members); ok { // U-Tuple
		elements := make([]Type, length)
		for i := range elements {
			column := make([]Type, len(members))
			for j, member := range members {
				column[j] = member.(*TupleType).ElementTypes[i]
			}
			elements[i] = unify(column, seen)
		}
		return NewTupleType(elements...)
	}
	var elements []Type
	allSets := true
	for _, member := range members {
		switch member := member.(type) {
		case *ListType:
			elements, allSets = append(elements, member.ElementType), false
		case *SetType:
			elements = append(elements, member.ElementType)
		case *TupleType:
			elements, allSets = append(elements, member.ElementTypes...), false
		}
	}
	if allSets { // U-Set
		return NewSetType(unify(elements, seen))
	}
	return NewListType(unify(elements, seen)) // U-List
}

// commonTupleLength returns the length of the members when every member is a tuple of that length.
func commonTupleLength(members []Type) (int, bool) {
	first, ok := members[0].(*TupleType)
	if !ok {
		return 0, false
	}
	for _, member := range members[1:] {
		if tuple, ok := member.(*TupleType); !ok || len(tuple.ElementTypes) != len(first.ElementTypes) {
			return 0, false
		}
	}
	return len(first.ElementTypes), true
}

// unifyMappings is rule U-Map: the maps and objects of a member list merge to one mapping, or to nil when there
// are none. A member list that is already under unification is recursive (README §12).
func unifyMappings(members []Type, seen *mappingMerges) Type {
	if len(members) == 0 {
		return nil
	}
	if len(members) == 1 { // U-Eq: the member keeps its identity and its annotations
		return members[0]
	}
	if result, ok := seen.reentry(members); ok {
		return result
	}
	objects := make([]*ObjectType, 0, len(members))
	var values []Type
	for _, member := range members {
		switch member := member.(type) {
		case *ObjectType:
			objects = append(objects, member)
			values = slices.AppendSeq(values, maps.Values(member.Properties))
		case *MapType:
			values = append(values, member.ElementType)
		}
	}
	if len(objects) != len(members) { // U-MapOf
		seen.push(members, nil)
		defer seen.pop()
		return NewMapType(unify(values, seen))
	}

	// U-Object
	result := NewObjectType(map[string]Type{})
	seen.push(members, result)
	defer seen.pop()
	for _, object := range objects {
		for key := range object.Properties {
			if _, done := result.Properties[key]; done {
				continue
			}
			properties := make([]Type, len(objects))
			for i, object := range objects {
				if property, ok := object.Properties[key]; ok {
					properties[i] = property
				} else {
					properties[i] = NoneType
				}
			}
			result.Properties[key] = unify(properties, seen)
		}
	}
	return result
}

// maximalScalars is rule U-Scalar: the members of the scalar class that no other member is above.
func maximalScalars(members []Type) []Type {
	var maximal []Type
	for _, member := range members {
		if !slices.ContainsFunc(members, func(other Type) bool { return below(member, other) }) {
			maximal = append(maximal, member)
		}
	}
	return maximal
}

// below is the scalar order of README §5.3: `above` converts safely from `s`, and either the converse does not
// hold or `above` precedes `s` in the canonical order. The tie-break decides between string and id, the one pair
// that converts safely in both directions: string wins.
func below(s, above Type) bool {
	if s.Equals(above) || above.ConversionFrom(s) != SafeConversion {
		return false
	}
	return s.ConversionFrom(above) != SafeConversion || above == StringType
}
