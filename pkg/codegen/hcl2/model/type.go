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
	_ "unsafe" // for linkname

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type lazyDiagnostics func() hcl.Diagnostics

type ConversionKind int

const (
	NoConversion     ConversionKind = 0
	UnsafeConversion ConversionKind = 1
	SafeConversion   ConversionKind = 2
)

func (k ConversionKind) Exists() bool {
	return k > NoConversion && k <= SafeConversion
}

// Type represents a datatype in the Pulumi Schema. Types created by this package are identical if they are
// equal values.
type Type interface {
	Definition

	Equals(other Type) bool
	AssignableFrom(src Type) bool
	ConversionFrom(src Type) ConversionKind
	String() string

	hash(stack objTypeSet) uint32
	equals(other Type, seen map[Type]struct{}) bool
	conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics)
	string(seen map[Type]struct{}) string
	unify(other Type) (Type, ConversionKind)
	isType()
}

type objTypeSet map[*ObjectType]struct{}

func (s *objTypeSet) add(t *ObjectType) bool {
	m := *s
	if m == nil {
		m = map[*ObjectType]struct{}{}
		*s = m
	}

	_, has := m[t]
	m[t] = struct{}{}
	return has
}

func (s objTypeSet) delete(t *ObjectType) {
	delete(s, t)
}

const (
	hashKindNone uint32 = iota + 1
	hashKindConst
	hashKindList
	hashKindMap
	hashKindNamed
	hashKindObject
	hashKindOpaque
	hashKindOutput
	hashKindPromise
	hashKindSet
	hashKindTuple
	hashKindUnion
)

//go:linkname hashString runtime.stringHash
func hashString(s string, seed uintptr) uintptr

type fnvHash uint32

func newFNV() fnvHash {
	return fnvHash(2166136261)
}

func (f *fnvHash) sum() uint32 {
	return uint32(*f)
}

func (f *fnvHash) addString(s string) {
	f.addUint32(uint32(hashString(s, 0)))
}

func (f *fnvHash) addUint32(c uint32) {
	h := uint32(*f)
	h ^= c & 0xff
	h *= 16777619
	h ^= (c >> 8) & 0xff
	h *= 16777619
	h ^= (c >> 16) & 0xff
	h *= 16777619
	h ^= (c >> 24) & 0xff
	h *= 16777619
	*f = fnvHash(h)
}

func hashCombine(code1, code2 uint32) uint32 {
	fnv := newFNV()
	fnv.addUint32(code1)
	fnv.addUint32(code2)
	return fnv.sum()
}

var (
	// NoneType represents the undefined value.
	NoneType Type = noneType(0)
	// BoolType represents the set of boolean values.
	BoolType = MustNewOpaqueType("boolean")
	// IntType represents the set of 32-bit integer values.
	IntType = MustNewOpaqueType("int")
	// NumberType represents the set of arbitrary-precision values.
	NumberType = MustNewOpaqueType("number")
	// StringType represents the set of UTF-8 string values.
	StringType = MustNewOpaqueType("string")
	// DynamicType represents the set of all values.
	DynamicType = MustNewOpaqueType("dynamic")
)

func assignableFrom(dest, src Type, assignableFromImpl func() bool) bool {
	if dest.Equals(src) || dest == DynamicType {
		return true
	}
	if cns, ok := src.(*ConstType); ok {
		return assignableFrom(dest, cns.Type, assignableFromImpl)
	}
	return assignableFromImpl()
}

func conversionFrom(dest, src Type, unifying bool, seen map[Type]struct{},
	conversionFromImpl func() (ConversionKind, lazyDiagnostics)) (ConversionKind, lazyDiagnostics) {

	if dest.Equals(src) || dest == DynamicType {
		return SafeConversion, nil
	}

	switch src := src.(type) {
	case *UnionType:
		return src.conversionTo(dest, unifying, seen)
	case *ConstType:
		return conversionFrom(dest, src.Type, unifying, seen, conversionFromImpl)
	}
	if src == DynamicType {
		return UnsafeConversion, nil
	}
	return conversionFromImpl()
}

func unify(t0, t1 Type, unify func() (Type, ConversionKind)) (Type, ConversionKind) {
	contract.Assert(t0 != nil)

	// Normalize s.t. dynamic is always on the right.
	if t0 == DynamicType {
		t0, t1 = t1, t0
	}

	switch {
	case t0.Equals(t1):
		return t0, SafeConversion
	case t1 == DynamicType:
		// The dynamic type unifies with any other type by selecting that other type.
		return t0, UnsafeConversion
	default:
		conversionFrom, _ := t0.conversionFrom(t1, true, nil)
		conversionTo, _ := t1.conversionFrom(t0, true, nil)
		switch {
		case conversionFrom < conversionTo:
			return t1, conversionTo
		case conversionFrom > conversionTo:
			return t0, conversionFrom
		}
		if conversionFrom == NoConversion {
			return NewUnionType(t0, t1), SafeConversion
		}
		if union, ok := t1.(*UnionType); ok {
			return union.unifyTo(t0)
		}

		unified, conversionKind := unify()
		contract.Assert(conversionKind >= conversionFrom)
		contract.Assert(conversionKind >= conversionTo)
		return unified, conversionKind
	}
}

// UnifyTypes chooses the most general type that is convertible from all of the input types.
func UnifyTypes(types ...Type) (safeType Type, unsafeType Type) {
	for _, t := range types {
		if safeType == nil {
			safeType = t
		} else {
			if safeT, safeConversion := safeType.unify(t); safeConversion >= SafeConversion {
				safeType = safeT
			} else {
				safeType = NewUnionType(safeType, t)
			}
		}

		if unsafeType == nil {
			unsafeType = t
		} else {
			if unsafeT, unsafeConversion := unsafeType.unify(t); unsafeConversion >= UnsafeConversion {
				unsafeType = unsafeT
			} else {
				unsafeType = NewUnionType(unsafeType, t)
			}
		}
	}

	if safeType == nil {
		safeType = NoneType
	}
	if unsafeType == nil {
		unsafeType = NoneType
	}

	contract.Assertf(unsafeType.Equals(safeType) || unsafeType.ConversionFrom(safeType).Exists(),
		"no conversion from %v to %v", safeType, unsafeType)
	return safeType, unsafeType
}
