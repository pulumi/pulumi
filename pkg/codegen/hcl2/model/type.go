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
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
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
	fmt.Stringer
	Definition

	// Equals returns true if this type is equivalent to the given type.
	Equals(other Type) bool
	// AssignableFrom returns true if a value of the source type is assignable to a variable of
	// this type.
	//
	// For example, if we have a map, the type of the elements of the source map have to match
	// with the destination for it to be assignable.
	AssignableFrom(src Type) bool
	// ConversionFrom returns the kind of conversion from the source type to this type.
	// If no conversion is possible, this returns NoConversion.
	//
	// The ConversionKind indicates whether the conversion is safe (will never fail) or
	// unsafe (may fail at runtime). For example a conversions from a dynamic type to any type
	// is always unsafe. Meanwhile a conversion from `int` to `number` is safe, as ints can
	// always be represented as numbers.
	//
	// Another more complex example is enums. We can convert to an enum from a const type if
	// the const type matches the type of the enums elements, and equals the value of one of
	// the enum's elements.  In that case we have a safe conversion. It's also possible to
	// have an unsafe conversion, in case the types match, but we can't confirm the value is
	// valid.
	ConversionFrom(src Type) ConversionKind
	pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter
	// Pretty returns a pretty-printer for the type.
	Pretty() pretty.Formatter

	equals(other Type, seen map[Type]struct{}) bool
	conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics)
	string(seen map[Type]struct{}) string
	unify(other Type) (Type, ConversionKind)
	isType()
}

var (
	// NoneType represents the undefined/null value.
	NoneType Type = noneType(0)
	// BoolType represents the set of boolean values.
	BoolType = NewOpaqueType("boolean")
	// IntType represents the set of 32-bit integer values.
	IntType = NewOpaqueType("int")
	// NumberType represents the set of arbitrary-precision values.
	NumberType = NewOpaqueType("number")
	// StringType represents the set of UTF-8 string values.
	StringType = NewOpaqueType("string")
	// DynamicType represents the set of all values.
	DynamicType = NewOpaqueType("dynamic")
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

type cacheEntry struct {
	kind  ConversionKind
	diags lazyDiagnostics
}

func conversionFrom(dest, src Type, unifying bool, seen map[Type]struct{},
	cache *gsync.Map[Type, cacheEntry],
	conversionFromImpl func() (ConversionKind, lazyDiagnostics),
) (ConversionKind, lazyDiagnostics) {
	if dest.Equals(src) || dest == DynamicType {
		return SafeConversion, nil
	}

	if c, ok := cache.Load(src); ok {
		return c.kind, c.diags
	}

	switch src := src.(type) {
	case *UnionType:
		kind, diags := src.conversionTo(dest, unifying, seen)
		if cache != nil {
			cache.Store(src, cacheEntry{kind: kind, diags: diags})
		}
		return kind, diags
	case *ConstType:
		// We want `EnumType`s too see const types, since they allow safe
		// conversions.
		if _, ok := dest.(*EnumType); !ok {
			kind, diags := conversionFrom(dest, src.Type, unifying, seen, cache, conversionFromImpl)
			if cache != nil {
				cache.Store(src, cacheEntry{kind, diags})
			}
			return kind, diags
		}
	}
	if src == DynamicType {
		if cache != nil {
			cache.Store(src, cacheEntry{UnsafeConversion, nil})
		}
		return UnsafeConversion, nil
	}
	kind, diags := conversionFromImpl()
	if cache != nil {
		cache.Store(src, cacheEntry{kind, diags})
	}
	return kind, diags
}

func unify(t0, t1 Type, unify func() (Type, ConversionKind)) (Type, ConversionKind) {
	contract.Requiref(t0 != nil, "t0", "must not be nil")

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
		contract.Assertf(conversionKind >= conversionFrom,
			"conversionKind (%v) < conversionFrom (%v)", conversionKind, conversionFrom)
		contract.Assertf(conversionKind >= conversionTo,
			"conversionKind (%v) < conversionTo (%v)", conversionKind, conversionTo)
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
