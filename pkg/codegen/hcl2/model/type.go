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

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

type lazyDiagnostics func() hcl.Diagnostics

type ConversionKind int

const (
	NoConversion     ConversionKind = 0
	UnsafeConversion ConversionKind = 1
	SafeConversion   ConversionKind = 2
)

func (k ConversionKind) GoString() string {
	switch k {
	case NoConversion:
		return "model.NoConversion"
	case UnsafeConversion:
		return "model.UnsafeConversion"
	case SafeConversion:
		return "model.SafeConversion"
	default:
		return fmt.Sprintf("model.ConversionKind(%d)", int(k))
	}
}

func (k ConversionKind) Exists() bool {
	switch k {
	case UnsafeConversion, SafeConversion:
		return true
	case NoConversion:
		return false
	default:
		panic("invalid conversion kind " + k.GoString())
	}
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

	equals(other Type, seen equalPairs) bool
	string(seen map[Type]struct{}) string
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
	// IDType represents resource IDs. It is distinct from string in the type
	// system but safely convertible to and from string.
	IDType = NewOpaqueType("id")
	// DynamicType represents the set of all values.
	DynamicType = NewOpaqueType("dynamic")
)

func assignableFrom(dest, src Type, assignableFromImpl func() bool) bool {
	if dest.Equals(src) || dest == DynamicType {
		return true
	}

	switch src := src.(type) {
	case *ConstType:
		return assignableFrom(dest, src.Type, assignableFromImpl)
	case *UnionType:
		// A union U(U_0, U_1, ...) is assignable to a type T when each of its members is assignable to T.
		for _, element := range src.ElementTypes {
			if !dest.AssignableFrom(element) {
				return false
			}
		}
		return true
	default:
		return assignableFromImpl()
	}
}

// equalPairs holds the pairs of object types whose comparison is in flight. A pair that is entered again belongs
// to a recursive type and compares as equal (README §12).
type equalPairs map[[2]Type]struct{}

// typeCache holds the kind of conversion to a type from each source type that a public ConversionFrom call has
// computed. README §10: only the outermost call stores a result, because the results of the calls that run while a
// pair of recursive object types is in flight hold under the coinductive assumption only.
type typeCache = gsync.Map[Type, ConversionKind]

// cycleSet holds the `(destination, source)` pairs that a conversion check has in flight, where either side is an
// object type (README §12). A pair that is entered again belongs to a recursive type and converts safely.
type cycleSet struct {
	pairs map[[2]Type]struct{}
}

func (c *cycleSet) has(dst, src Type) bool {
	_, ok := c.pairs[[2]Type{dst, src}]
	return ok
}

func (c *cycleSet) push(dst, src Type) {
	if c.pairs == nil {
		c.pairs = map[[2]Type]struct{}{}
	}
	c.pairs[[2]Type{dst, src}] = struct{}{}
}

func (c *cycleSet) pop(dst, src Type) {
	delete(c.pairs, [2]Type{dst, src})
}

// cachedConversionFrom is the public entry of the conversion relation: it serves a repeated question from the
// destination's cache and stores the result of a new one.
func cachedConversionFrom(dst, src Type, cache *typeCache) ConversionKind {
	if cache != nil {
		if kind, ok := cache.Load(src); ok {
			return kind
		}
	}
	kind, _ := conversionFrom(dst, src, &cycleSet{})
	if cache != nil {
		cache.Store(src, kind)
	}
	return kind
}
