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

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// conversionFrom is the conversion relation `conv(dst, src)` of README §4. The rules apply in order; the first
// rule whose shapes match decides. A result of NoConversion carries the diagnostics that explain it.
func conversionFrom(dst, src Type, seen *cycleSet) (ConversionKind, lazyDiagnostics) {
	if dst.Equals(src) || dst == DynamicType { // C-Eq, C-Dyn
		return SafeConversion, nil
	}
	if src, ok := src.(*UnionType); ok { // C-USrc
		return conversionFromUnion(dst, src, seen)
	}
	switch dst := dst.(type) {
	case *UnionType: // C-UDst
		return conversionToUnion(dst, src, seen)
	case *OutputType: // C-Out
		return conversionFrom(dst.ElementType, ResolveOutputs(src), seen)
	case *PromiseType: // C-Prom
		if _, ok := src.(*OutputType); ok {
			return NoConversion, notConvertible(dst, src)
		}
		return conversionFrom(dst.ElementType, ResolvePromises(src), seen)
	}
	if src == DynamicType { // C-DynSrc
		return UnsafeConversion, nil
	}
	switch dst := dst.(type) {
	case *ConstType:
		if src, ok := src.(*ConstType); ok { // C-Const
			if dst.Value.RawEquals(src.Value) {
				return SafeConversion, nil
			}
			return NoConversion, notConvertible(dst, src)
		}
		return unsafeIfExists(dst, src, dst.Type, seen) // C-ConstSrc
	case *EnumType:
		if src, ok := src.(*ConstType); ok { // C-EnumConst
			if slices.ContainsFunc(dst.Elements, func(e cty.Value) bool {
				return e.Type().Equals(src.Value.Type()) && e.Equals(src.Value).True()
			}) {
				return SafeConversion, nil
			}
			return NoConversion, notConvertible(dst, src)
		}
		return unsafeIfExists(dst, src, dst.Type, seen) // C-EnumSrc
	}
	switch src := src.(type) { // C-Widen
	case *ConstType:
		return conversionFrom(dst, src.Type, seen)
	case *EnumType:
		return conversionFrom(dst, src.Type, seen)
	}
	switch dst := dst.(type) {
	case *OpaqueType: // C-Scalar
		if kind := scalarConversion(dst, src); kind != NoConversion {
			return kind, nil
		}
	case *ListType: // C-List
		switch src := src.(type) {
		case *ListType:
			return conversionFrom(dst.ElementType, src.ElementType, seen)
		case *SetType:
			return conversionFrom(dst.ElementType, src.ElementType, seen)
		case *TupleType:
			return minConversion(len(src.ElementTypes), func(i int) (ConversionKind, lazyDiagnostics) {
				return conversionFrom(dst.ElementType, src.ElementTypes[i], seen)
			})
		}
	case *SetType: // C-Set
		switch src := src.(type) {
		case *SetType:
			return conversionFrom(dst.ElementType, src.ElementType, seen)
		case *ListType:
			return capConversion(conversionFrom(dst.ElementType, src.ElementType, seen))
		case *TupleType:
			return capConversion(minConversion(len(src.ElementTypes), func(i int) (ConversionKind, lazyDiagnostics) {
				return conversionFrom(dst.ElementType, src.ElementTypes[i], seen)
			}))
		}
	case *MapType: // C-Map
		switch src := src.(type) {
		case *MapType:
			return conversionFrom(dst.ElementType, src.ElementType, seen)
		case *ObjectType:
			return conversionThroughObject(dst, src, seen, func() (ConversionKind, lazyDiagnostics) {
				keys := slices.Sorted(maps.Keys(src.Properties))
				return minConversion(len(keys), func(i int) (ConversionKind, lazyDiagnostics) {
					return conversionFrom(dst.ElementType, src.Properties[keys[i]], seen)
				})
			})
		}
	case *TupleType: // C-Tuple
		switch src := src.(type) {
		case *TupleType:
			if len(dst.ElementTypes) != len(src.ElementTypes) {
				return NoConversion, func() hcl.Diagnostics { return hcl.Diagnostics{tuplesHaveDifferentLengths(dst, src)} }
			}
			return minConversion(len(dst.ElementTypes), func(i int) (ConversionKind, lazyDiagnostics) {
				return conversionFrom(dst.ElementTypes[i], src.ElementTypes[i], seen)
			})
		case *ListType:
			return capConversion(minConversion(len(dst.ElementTypes), func(i int) (ConversionKind, lazyDiagnostics) {
				return conversionFrom(dst.ElementTypes[i], src.ElementType, seen)
			}))
		case *SetType:
			return capConversion(minConversion(len(dst.ElementTypes), func(i int) (ConversionKind, lazyDiagnostics) {
				return conversionFrom(dst.ElementTypes[i], src.ElementType, seen)
			}))
		}
	case *ObjectType: // C-Object
		switch src := src.(type) {
		case *ObjectType:
			return conversionThroughObject(dst, src, seen, func() (ConversionKind, lazyDiagnostics) {
				keys := slices.Sorted(maps.Keys(dst.Properties))
				return minConversion(len(keys), func(i int) (ConversionKind, lazyDiagnostics) {
					property, ok := src.Properties[keys[i]]
					if !ok {
						property = NoneType
					}
					return conversionFrom(dst.Properties[keys[i]], property, seen)
				})
			})
		case *MapType:
			return conversionThroughObject(dst, src, seen, func() (ConversionKind, lazyDiagnostics) {
				keys := slices.Sorted(maps.Keys(dst.Properties))
				return capConversion(minConversion(len(keys), func(i int) (ConversionKind, lazyDiagnostics) {
					return conversionFrom(dst.Properties[keys[i]], src.ElementType, seen)
				}))
			})
		}
	}
	return NoConversion, notConvertible(dst, src) // C-No
}

// conversionFromUnion is rule C-USrc: a union source converts safely when every member converts safely, not at all
// when no member converts, and unsafely otherwise.
func conversionFromUnion(dst Type, src *UnionType, seen *cycleSet) (ConversionKind, lazyDiagnostics) {
	allSafe, anyExists := true, false
	for _, member := range src.ElementTypes {
		kind, _ := conversionFrom(dst, member, seen)
		allSafe = allSafe && kind == SafeConversion
		anyExists = anyExists || kind != NoConversion
	}
	switch {
	case allSafe:
		return SafeConversion, nil
	case anyExists:
		return UnsafeConversion, nil
	}
	return NoConversion, notConvertible(dst, src)
}

// conversionToUnion is rule C-UDst: the best conversion to any member of the destination. When no member converts,
// the reason names no single member: ExprNotConvertible then reports the whole union.
func conversionToUnion(dst *UnionType, src Type, seen *cycleSet) (ConversionKind, lazyDiagnostics) {
	kind := NoConversion
	for _, member := range dst.ElementTypes {
		memberKind, _ := conversionFrom(member, src, seen)
		kind = max(kind, memberKind)
	}
	if kind == NoConversion {
		return NoConversion, func() hcl.Diagnostics { return nil }
	}
	return kind, nil
}

// conversionThroughObject runs a rule that expands the properties of an object (C-Object, and C-Map with an object
// source) with the pair in flight. A pair that is entered again is recursive and converts safely (README §12).
func conversionThroughObject(
	dst, src Type, seen *cycleSet, convert func() (ConversionKind, lazyDiagnostics),
) (ConversionKind, lazyDiagnostics) {
	if seen.has(dst, src) {
		return SafeConversion, nil
	}
	seen.push(dst, src)
	defer seen.pop(dst, src)
	return convert()
}

// unsafeIfExists is the result of rules C-ConstSrc and C-EnumSrc: an unsafe conversion when the source converts
// to the base type of the destination, and no conversion otherwise.
func unsafeIfExists(dst, src, base Type, seen *cycleSet) (ConversionKind, lazyDiagnostics) {
	if kind, _ := conversionFrom(base, src, seen); kind != NoConversion {
		return UnsafeConversion, nil
	}
	return NoConversion, notConvertible(dst, src)
}

// scalarConversion is the table of README §4.4 for two distinct scalar types.
func scalarConversion(dst, src Type) ConversionKind {
	if !isScalar(dst) || !isScalar(src) {
		return NoConversion
	}
	switch {
	case dst == StringType, dst == IDType:
		return SafeConversion
	case dst == NumberType && src == IntType:
		return SafeConversion
	default:
		return UnsafeConversion
	}
}

func isScalar(t Type) bool {
	return t == BoolType || t == IntType || t == NumberType || t == StringType || t == IDType
}

// minConversion folds n conversions to their least kind. It stops at the first conversion that does not exist
// and returns its diagnostics. The fold of no conversions is safe.
func minConversion(n int, convert func(i int) (ConversionKind, lazyDiagnostics)) (ConversionKind, lazyDiagnostics) {
	kind, why := SafeConversion, lazyDiagnostics(nil)
	for i := 0; i < n && kind != NoConversion; i++ {
		if k, w := convert(i); k < kind {
			kind, why = k, w
		}
	}
	return kind, why
}

// capConversion lowers a safe conversion to an unsafe one.
func capConversion(kind ConversionKind, why lazyDiagnostics) (ConversionKind, lazyDiagnostics) {
	return min(UnsafeConversion, kind), why
}

func notConvertible(dst, src Type) lazyDiagnostics {
	return func() hcl.Diagnostics { return hcl.Diagnostics{typeNotConvertible(dst, src)} }
}
