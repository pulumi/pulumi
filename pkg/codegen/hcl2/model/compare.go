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
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// Compare returns 0 when a and b are equal in the sense of Equals, and otherwise a negative or positive value that
// orders them. The order is total, structural & arbitrary.
func Compare(a, b Type) int {
	return compareTypes(a, b, nil)
}

// comparePairs holds the pairs of object types whose comparison is in flight. A pair that is entered again belongs
// to a recursive type and compares as equal.
type comparePairs map[[2]Type]struct{}

func compareTypes(a, b Type, seen comparePairs) int {
	if a == b {
		return 0
	}
	if c := cmp.Compare(typeRank(a), typeRank(b)); c != 0 {
		return c
	}
	switch a := a.(type) {
	case *OpaqueType:
		return strings.Compare(string(*a), string(*b.(*OpaqueType)))
	case noneType:
		return 0
	case *ConstType:
		b := b.(*ConstType)
		return cmp.Or(compareTypes(a.Type, b.Type, seen), compareValues(a.Value, b.Value))
	case *EnumType:
		return strings.Compare(a.Token, b.(*EnumType).Token)
	case *ListType:
		return compareTypes(a.ElementType, b.(*ListType).ElementType, seen)
	case *SetType:
		return compareTypes(a.ElementType, b.(*SetType).ElementType, seen)
	case *MapType:
		return compareTypes(a.ElementType, b.(*MapType).ElementType, seen)
	case *OutputType:
		return compareTypes(a.ElementType, b.(*OutputType).ElementType, seen)
	case *PromiseType:
		return compareTypes(a.ElementType, b.(*PromiseType).ElementType, seen)
	case *TupleType:
		return compareTypeSlices(a.ElementTypes, b.(*TupleType).ElementTypes, seen)
	case *UnionType:
		return compareTypeSlices(a.sortedElementTypes(seen), b.(*UnionType).sortedElementTypes(seen), seen)
	case *ObjectType:
		return compareObjectTypes(a, b.(*ObjectType), seen)
	default:
		panic(fmt.Sprintf("Unhandled type %T", a))
	}
}

func typeRank(t Type) int {
	switch t.(type) {
	case *OpaqueType:
		return 0
	case noneType:
		return 1
	case *ConstType:
		return 2
	case *EnumType:
		return 3
	case *ListType:
		return 4
	case *SetType:
		return 5
	case *MapType:
		return 6
	case *TupleType:
		return 7
	case *ObjectType:
		return 8
	case *UnionType:
		return 9
	case *OutputType:
		return 10
	case *PromiseType:
		return 11
	default:
		panic(fmt.Sprintf("Unhandled type %T", t))
	}
}

func compareTypeSlices(a, b []Type, seen comparePairs) int {
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}
	for i := range a {
		if c := compareTypes(a[i], b[i], seen); c != 0 {
			return c
		}
	}
	return 0
}

// compareObjectTypes orders objects by their number of properties, then by each key of either object in key order:
// an object that has the key sorts before one that lacks it, and otherwise the property types are compared.
func compareObjectTypes(a, b *ObjectType, seen comparePairs) int {
	if c := cmp.Compare(len(a.Properties), len(b.Properties)); c != 0 {
		return c
	}
	if seen == nil {
		seen = comparePairs{}
	}
	pair := [2]Type{a, b}
	if _, ok := seen[pair]; ok {
		return 0
	}
	seen[pair] = struct{}{}
	defer delete(seen, pair)

	keys := slices.AppendSeq(slices.Collect(maps.Keys(a.Properties)), maps.Keys(b.Properties))
	slices.Sort(keys)
	for _, k := range slices.Compact(keys) {
		pa, oka := a.Properties[k]
		pb, okb := b.Properties[k]
		switch {
		case !oka:
			return 1
		case !okb:
			return -1
		}
		if c := compareTypes(pa, pb, seen); c != 0 {
			return c
		}
	}
	return 0
}

// compareValues orders the values of constants: by kind, then by value. Nulls, unknowns, and other values are
// ordered by their Go representation, which is deterministic and distinguishes the values RawEquals distinguishes.
func compareValues(a, b cty.Value) int {
	if c := cmp.Compare(valueRank(a), valueRank(b)); c != 0 {
		return c
	}
	switch valueRank(a) {
	case 2:
		return cmp.Compare(boolInt(a), boolInt(b))
	case 3:
		return a.AsBigFloat().Cmp(b.AsBigFloat())
	case 4:
		return strings.Compare(a.AsString(), b.AsString())
	}
	return strings.Compare(a.GoString(), b.GoString())
}

func valueRank(v cty.Value) int {
	switch {
	case v.IsNull():
		return 0
	case !v.IsKnown():
		return 1
	case v.Type() == cty.Bool:
		return 2
	case v.Type() == cty.Number:
		return 3
	case v.Type() == cty.String:
		return 4
	}
	return 5
}

func boolInt(v cty.Value) int {
	if v.True() {
		return 1
	}
	return 0
}
