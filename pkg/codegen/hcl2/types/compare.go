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

package types

import (
	"cmp"
	"math"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Compare returns 0 when a == b, and otherwise a negative or positive value that orders them. The order is total,
// structural, and arbitrary. A recursive type sorts after every type that is not one.
func Compare(a, b Type) int {
	if a == b {
		return 0
	}
	na, nb := a.raw(), b.raw()
	if c := cmp.Compare(na.kind, nb.kind); c != 0 {
		return c
	}
	switch na.kind {
	case kindVar:
		contract.Assertf(na.v == nil && nb.v == nil,
			"a placeholder has no order outside the Recursive call that created it")
		return cmp.Compare(na.num, nb.num)
	case kindRec:
		if na.b == nb.b {
			return cmp.Compare(na.num, nb.num)
		}
		return cmp.Or(cmp.Compare(height(na.b), height(nb.b)), compareCells(na.b, nb.b), cmp.Compare(na.num, nb.num))
	case KindConst:
		return cmp.Or(Compare(na.a, nb.a), compareConstValues(na, nb))
	case KindEnum:
		return cmp.Or(strings.Compare(na.str, nb.str), Compare(na.a, nb.a), compareCells(na.b, nb.b))
	case KindList, KindSet, KindMap, KindOutput, KindPromise:
		return Compare(na.a, nb.a)
	case KindTuple, KindUnion, KindObject:
		return compareCells(na.b, nb.b)
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic:
		return 0
	}
	panic("unreachable")
}

// compareCells orders two lists by length, then by each cell in order: by name, then by value. Two objects with
// the same number of properties and different names are ordered by the first name that differs, so an object that
// has a name sorts before one that lacks it.
func compareCells(a, b list) int {
	if c := cmp.Compare(a.len(), b.len()); c != 0 {
		return c
	}
	for a != (list{}) {
		ca, cb := a.h.Value(), b.h.Value()
		if c := cmp.Or(strings.Compare(ca.name, cb.name), Compare(ca.value, cb.value)); c != 0 {
			return c
		}
		a, b = ca.next, cb.next
	}
	return 0
}

// compareConstValues orders two constants with the same base. Numbers are ordered by value and then by bit
// pattern, so that 0 and -0 are ordered and distinct.
func compareConstValues(a, b node) int {
	switch a.a {
	case Bool:
		return cmp.Compare(a.num, b.num)
	case Int:
		return cmp.Compare(int64(a.num), int64(b.num))
	case Number:
		return cmp.Or(
			cmp.Compare(math.Float64frombits(a.num), math.Float64frombits(b.num)),
			cmp.Compare(a.num, b.num),
		)
	}
	return strings.Compare(a.str, b.str)
}
