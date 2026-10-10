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
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/rapidmodel"
)

// Tests that UnifyTypes is a function of its inputs: repeated calls on the same pair produce the same type.
func TestUnifyTypesIsDeterministic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		a, b := rapidmodel.Type().Draw(t, "a"), rapidmodel.Type().Draw(t, "b")
		unified := model.UnifyTypes(a, b)
		for range 20 {
			again := model.UnifyTypes(a, b)
			require.True(t, unified.Equals(again),
				"UnifyTypes(%v, %v) gave %v and then %v", a, b, unified, again)
		}
	})
}

// Tests S1 of README §6: the result of UnifyTypes converts safely from every input.
func TestUnifyTypesConvertsSafelyFromInputs(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		a, b := rapidmodel.Type().Draw(t, "a"), rapidmodel.Type().Draw(t, "b")
		unified := model.UnifyTypes(a, b)
		require.Equal(t, model.SafeConversion, unified.ConversionFrom(a), "UnifyTypes(%v, %v) = %v", a, b, unified)
		require.Equal(t, model.SafeConversion, unified.ConversionFrom(b), "UnifyTypes(%v, %v) = %v", a, b, unified)
	})
}

// Tests S4 of README §6: every permutation of the inputs gives one result.
func TestUnifyTypesIsSymmetric(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		a, b, c := rapidmodel.Type().Draw(t, "a"), rapidmodel.Type().Draw(t, "b"), rapidmodel.Type().Draw(t, "c")
		expected := model.UnifyTypes(a, b, c)
		for _, order := range [][]model.Type{{a, c, b}, {b, a, c}, {b, c, a}, {c, a, b}, {c, b, a}} {
			actual := model.UnifyTypes(order...)
			require.True(t, expected.Equals(actual),
				"UnifyTypes(%v, %v, %v) = %v but UnifyTypes(%v) = %v", a, b, c, expected, order, actual)
		}
	})
}

// Tests S5 of README §6: a type unifies with itself to itself.
func TestUnifyTypesIsIdempotent(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		a := rapidmodel.Type().Draw(t, "a")
		unified := model.UnifyTypes(a, a)
		require.True(t, a.Equals(unified), "UnifyTypes(%v, %v) = %v", a, a, unified)
	})
}

// Tests S8 of README §6: unification distributes over the constructors that take one element type. The output
// constructor resolves the eventuals of its element, so its law is checked on inputs without eventuals.
func TestUnifyTypesDistributesOverConstructors(t *testing.T) {
	t.Parallel()

	constructors := map[string]func(model.Type) model.Type{
		"list":   func(t model.Type) model.Type { return model.NewListType(t) },
		"set":    func(t model.Type) model.Type { return model.NewSetType(t) },
		"map":    func(t model.Type) model.Type { return model.NewMapType(t) },
		"output": func(t model.Type) model.Type { return model.NewOutputType(t) },
		"tuple":  func(t model.Type) model.Type { return model.NewTupleType(t) },
		"object": func(t model.Type) model.Type { return model.NewObjectType(map[string]model.Type{"k": t}) },
	}
	for name, construct := range constructors {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				a, b := rapidmodel.Type().Draw(t, "a"), rapidmodel.Type().Draw(t, "b")
				if name == "output" {
					a, b = model.ResolveOutputs(a), model.ResolveOutputs(b)
				}
				expected := construct(model.UnifyTypes(a, b))
				actual := model.UnifyTypes(construct(a), construct(b))
				require.True(t, expected.Equals(actual), "expected %v, got %v", expected, actual)
			})
		})
	}
}
