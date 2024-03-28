// Copyright 2016-2024, Pulumi Corporation.
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

package test

import (
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func Value(maxDepth int) *rapid.Generator[property.Value] {
	if maxDepth <= 1 {
		return Primitive()
	}
	return rapid.OneOf(
		Primitive(),
		Array(maxDepth),
		Map(maxDepth),
		Secret(maxDepth),
		Dependencies(maxDepth),
	)
}

func Primitive() *rapid.Generator[property.Value] {
	return rapid.OneOf(
		String(),
		Bool(),
		Number(),
		Null(),
		Computed(),
	)
}

func String() *rapid.Generator[property.Value] {
	return rapid.Map(rapid.String(), property.New[string])
}

func Bool() *rapid.Generator[property.Value] { return rapid.Map(rapid.Bool(), property.New[bool]) }

func Number() *rapid.Generator[property.Value] {
	return rapid.Map(rapid.Float64(), property.New[float64])
}

func Null() *rapid.Generator[property.Value] { return rapid.Just(property.Value{}) }

func Computed() *rapid.Generator[property.Value] { return rapid.Just(property.New(property.Computed)) }

func Array(maxDepth int) *rapid.Generator[property.Value] { return ArrayOf(Value(maxDepth - 1)) }

func Map(maxDepth int) *rapid.Generator[property.Value] { return MapOf(Value(maxDepth - 1)) }

func Secret(maxDepth int) *rapid.Generator[property.Value] { return SecretOf(Value(maxDepth - 1)) }

func Dependencies(maxDepth int) *rapid.Generator[property.Value] {
	return DependenciesOf(Value(maxDepth - 1))
}

func ArrayOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return property.New(rapid.SliceOf(value).Draw(t, "V"))
	})
}

func MapOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return property.New(rapid.MapOf(
			rapid.String(),
			value,
		).Draw(t, "V"))
	})
}

func SecretOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return value.Draw(t, "V").WithSecret(true)
	})
}

func DependenciesOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return value.Draw(t, "V").WithDependencies(
			rapid.SliceOfN(URN(), 1, 10).Draw(t, "urns"),
		)
	})
}

// A rapid generator for resource.URN.
//
// Because the github.com/pulumi/pulumi/sdk/v3/go/property does not enforce URN validity,
// we don't enforce it here.
func URN() *rapid.Generator[urn.URN] {
	return rapid.Custom(func(t *rapid.T) urn.URN {
		return urn.URN(rapid.String().Draw(t, "urn-body"))
	})
}
