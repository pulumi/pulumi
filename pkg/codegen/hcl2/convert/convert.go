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

// Package convert decides whether a value of one PCL type converts to another, following §4 and §12.1 of the
// PCL type specification in pkg/codegen/hcl2/model/README.md.
package convert

import (
	"iter"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
)

// Kind is the result of a conversion check. The kinds are ordered No < Unsafe < Safe.
type Kind uint8

const (
	// No means no conversion exists.
	No Kind = iota
	// Unsafe means a conversion exists, but it can fail for some values or lose information.
	Unsafe
	// Safe means every value of the source type is a value of the destination type after a total conversion.
	Safe
)

func (k Kind) String() string {
	switch k {
	case No:
		return "No"
	case Unsafe:
		return "Unsafe"
	case Safe:
		return "Safe"
	}
	panic("unreachable")
}

// To reports whether a value of type src converts to type dst.
func To(dst, src types.Type) Kind {
	c := checker{inFlight: map[[2]types.Type]struct{}{}}
	return c.conv(dst, src)
}

// checker carries the (dst, src) pairs in flight. A pair that is entered again lies on a cycle of a recursive type
// and converts under the coinductive assumption.
type checker struct {
	inFlight map[[2]types.Type]struct{}
}

func (c *checker) conv(dst, src types.Type) Kind {
	if dst == src {
		return Safe // C-Eq
	}
	pair := [2]types.Type{dst, src}
	if _, ok := c.inFlight[pair]; ok {
		return Safe
	}
	c.inFlight[pair] = struct{}{}
	defer delete(c.inFlight, pair)
	return c.rules(dst, src)
}

// rules applies the first rule of §4.2 whose shapes match.
func (c *checker) rules(dst, src types.Type) Kind {
	dk, sk := dst.Kind(), src.Kind()
	switch {
	case dk == types.KindDynamic: // C-Dyn
		return Safe
	case sk == types.KindUnion: // C-USrc
		return members(c.each(src.UnionValues(), func(s types.Type) Kind { return c.conv(dst, s) }))
	case dk == types.KindUnion: // C-UDst
		return maxOf(c.each(dst.UnionValues(), func(d types.Type) Kind { return c.conv(d, src) }))
	case dk == types.KindOutput: // C-Out
		return c.conv(dst.Element(), types.ResolveOutputs(src))
	case dk == types.KindPromise: // C-Prom
		if sk == types.KindOutput {
			return No
		}
		return c.conv(dst.Element(), types.ResolvePromises(src))
	case sk == types.KindDynamic: // C-DynSrc
		return Unsafe
	case dk == types.KindConst && sk == types.KindConst: // C-Const, with equal constants decided by C-Eq
		return No
	case dk == types.KindConst: // C-ConstSrc
		return unsafeIf(c.conv(dst.ConstBase(), src) != No)
	case dk == types.KindEnum && sk == types.KindConst: // C-EnumConst
		return safeIf(slices.Contains(dst.EnumValues(), src))
	case dk == types.KindEnum: // C-EnumSrc
		return unsafeIf(c.conv(dst.EnumBase(), src) != No)
	case sk == types.KindConst: // C-Widen
		return c.conv(dst, src.ConstBase())
	case sk == types.KindEnum: // C-Widen
		return c.conv(dst, src.EnumBase())
	case scalar(dk) && scalar(sk): // C-Scalar
		return scalarTable[dk][sk]
	case dk == types.KindList && (sk == types.KindList || sk == types.KindSet): // C-List
		return c.conv(dst.Element(), src.Element())
	case dk == types.KindList && sk == types.KindTuple:
		return c.eachTo(dst.Element(), src.TupleValues())
	case dk == types.KindSet && sk == types.KindSet: // C-Set
		return c.conv(dst.Element(), src.Element())
	case dk == types.KindSet && sk == types.KindList:
		return capUnsafe(c.conv(dst.Element(), src.Element()))
	case dk == types.KindSet && sk == types.KindTuple:
		return capUnsafe(c.eachTo(dst.Element(), src.TupleValues()))
	case dk == types.KindMap && sk == types.KindMap: // C-Map
		return c.conv(dst.Element(), src.Element())
	case dk == types.KindMap && sk == types.KindObject:
		return c.eachTo(dst.Element(), slices.Collect(values(src)))
	case dk == types.KindTuple && sk == types.KindTuple: // C-Tuple
		ds, ss := dst.TupleValues(), src.TupleValues()
		if len(ds) != len(ss) {
			return No
		}
		kinds := make([]Kind, len(ds))
		for i := range ds {
			kinds[i] = c.conv(ds[i], ss[i])
		}
		return minOf(kinds)
	case dk == types.KindTuple && (sk == types.KindList || sk == types.KindSet):
		return capUnsafe(c.eachFrom(dst.TupleValues(), src.Element()))
	case dk == types.KindObject && sk == types.KindObject: // C-Object
		return c.object(dst, src)
	case dk == types.KindObject && sk == types.KindMap:
		return capUnsafe(c.eachFrom(slices.Collect(values(dst)), src.Element()))
	}
	return No // C-No
}

func (c *checker) each(ts []types.Type, f func(types.Type) Kind) []Kind {
	kinds := make([]Kind, len(ts))
	for i, t := range ts {
		kinds[i] = f(t)
	}
	return kinds
}

// eachTo is the least kind of converting each source to one destination.
func (c *checker) eachTo(dst types.Type, srcs []types.Type) Kind {
	return minOf(c.each(srcs, func(s types.Type) Kind { return c.conv(dst, s) }))
}

// eachFrom is the least kind of converting one source to each destination.
func (c *checker) eachFrom(dsts []types.Type, src types.Type) Kind {
	return minOf(c.each(dsts, func(d types.Type) Kind { return c.conv(d, src) }))
}

// object is C-Object: the least kind of converting each property of src to the property of dst with the same name,
// where a property that src lacks converts from None. Both objects list their properties in name order, so one walk
// over the two lists pairs them.
func (c *checker) object(dst, src types.Type) Kind {
	next, stop := iter.Pull2(src.ObjectValues())
	defer stop()
	sn, sv, ok := next()
	kinds := make([]Kind, 0, dst.Len())
	for name, d := range dst.ObjectValues() {
		for ok && sn < name {
			sn, sv, ok = next()
		}
		s := types.None
		if ok && sn == name {
			s = sv
			sn, sv, ok = next()
		}
		kinds = append(kinds, c.conv(d, s))
	}
	return minOf(kinds)
}

func values(object types.Type) iter.Seq[types.Type] {
	return func(yield func(types.Type) bool) {
		for _, t := range object.ObjectValues() {
			if !yield(t) {
				return
			}
		}
	}
}

func scalar(k types.Kind) bool {
	switch k {
	case types.KindBool, types.KindInt, types.KindNumber, types.KindString, types.KindID:
		return true
	case types.KindNone, types.KindDynamic, types.KindConst, types.KindEnum, types.KindList, types.KindSet,
		types.KindMap, types.KindTuple, types.KindObject, types.KindUnion, types.KindOutput, types.KindPromise:
	}
	return false
}

// scalarTable is the table of §4.4, indexed by destination then source. The diagonal is decided by C-Eq.
var scalarTable = [...]map[types.Kind]Kind{
	types.KindBool: {
		types.KindInt: Unsafe, types.KindNumber: Unsafe, types.KindString: Unsafe, types.KindID: Unsafe,
	},
	types.KindInt: {
		types.KindBool: Unsafe, types.KindNumber: Unsafe, types.KindString: Unsafe, types.KindID: Unsafe,
	},
	types.KindNumber: {
		types.KindBool: Unsafe, types.KindInt: Safe, types.KindString: Unsafe, types.KindID: Unsafe,
	},
	types.KindString: {
		types.KindBool: Safe, types.KindInt: Safe, types.KindNumber: Safe, types.KindID: Safe,
	},
	types.KindID: {
		types.KindBool: Safe, types.KindInt: Safe, types.KindNumber: Safe, types.KindString: Safe,
	},
}

// minOf is the least kind; minOf() is Safe.
func minOf(kinds []Kind) Kind {
	k := Safe
	for _, kind := range kinds {
		k = min(k, kind)
	}
	return k
}

// maxOf is the greatest kind; maxOf() is No.
func maxOf(kinds []Kind) Kind {
	k := No
	for _, kind := range kinds {
		k = max(k, kind)
	}
	return k
}

// members is Safe when every kind is Safe, No when every kind is No, and Unsafe otherwise.
func members(kinds []Kind) Kind {
	switch {
	case minOf(kinds) == Safe:
		return Safe
	case maxOf(kinds) == No:
		return No
	}
	return Unsafe
}

// capUnsafe bounds a kind by Unsafe.
func capUnsafe(k Kind) Kind { return min(Unsafe, k) }

func unsafeIf(b bool) Kind {
	if b {
		return Unsafe
	}
	return No
}

func safeIf(b bool) Kind {
	if b {
		return Safe
	}
	return No
}
