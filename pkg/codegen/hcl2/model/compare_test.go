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

package model_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/rapidmodel"
)

// Tests that Compare defines one order for any set of types: sorting a shuffled copy gives the same sequence.
func TestCompareIsCanonical(t *testing.T) {
	t.Parallel()

	typeGen := rapid.SliceOfN(rapidmodel.Type(), 1, 8)
	rapid.Check(t, func(t *rapid.T) {
		types := typeGen.Draw(t, "types")
		sorted := slices.SortedFunc(slices.Values(types), model.Compare)

		shuffled := rapid.Permutation(slices.Clone(types)).Draw(t, "shuffled")
		slices.SortFunc(shuffled, model.Compare)

		for i := range sorted {
			require.True(t, sorted[i].Equals(shuffled[i]), "position %d: %v then %v", i, sorted[i], shuffled[i])
		}
	})
}

// Tests that Compare is a total order that agrees with Equals: it is zero exactly for equal types, antisymmetric,
// and transitive.
func TestCompareIsTotalOrder(t *testing.T) {
	t.Parallel()

	typeGen := rapidmodel.Type()
	rapid.Check(t, func(t *rapid.T) {
		a, b, c := typeGen.Draw(t, "a"), typeGen.Draw(t, "b"), typeGen.Draw(t, "c")
		require.Equal(t, 0, model.Compare(a, a), "%v does not compare equal to itself", a)
		require.Equal(t, a.Equals(b), model.Compare(a, b) == 0, "Compare(%v, %v) disagrees with Equals", a, b)
		require.Equal(t, -model.Compare(b, a), model.Compare(a, b), "Compare(%v, %v) is not antisymmetric", a, b)
		if model.Compare(a, b) <= 0 && model.Compare(b, c) <= 0 {
			require.LessOrEqual(t, model.Compare(a, c), 0, "%v <= %v <= %v but not %v <= %v", a, b, c, a, c)
		}
	})
}

// Tests that comparing recursive object types terminates when the recursion goes through a union, since the union
// sorts its members with the comparison in flight rather than with a fresh one.
func TestCompareRecursiveUnion(t *testing.T) {
	t.Parallel()

	recursive := func(extra model.Type) *model.ObjectType {
		properties := map[string]model.Type{}
		object := model.NewObjectType(properties)
		properties["self"] = model.NewUnionType(model.NoneType, object, extra)
		return object
	}
	a, b := recursive(model.IntType), recursive(model.StringType)
	require.NotEqual(t, 0, model.Compare(a, b))
	require.Equal(t, 0, model.Compare(a, recursive(model.IntType)))
	require.True(t, a.Equals(recursive(model.IntType)))
}
