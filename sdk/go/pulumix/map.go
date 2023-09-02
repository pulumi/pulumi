// Copyright 2016-2023, Pulumi Corporation.
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

package pulumix

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

// Map is a map from strings to input values of the same type.
// It may be used as an Input[map[string]T].
type Map[T any] map[string]Input[T]

var _ Input[map[string]int] = Map[int]{}

// ElementType returns the reflected type of map[string]T.
func (Map[T]) ElementType() reflect.Type {
	return typeOf[map[string]T]()
}

// ToOutput builds an Output from the values in the map.
func (m Map[T]) ToOutput(ctx context.Context) Output[map[string]T] {
	return Output[map[string]T]{
		OutputState: internal.GetOutputState(internal.ToOutputWithContext(ctx, m)),
	}
}

// AsAny turns the map into an Output[any].
func (m Map[T]) AsAny() Output[any] {
	return m.ToOutput(context.Background()).AsAny()
}

// GMapOutput is an Output value holding a map[string]T.
// It may be used as an Input[map[string]T].
//
// O is the kind of Output value that this GMapOutput unwraps to.
type GMapOutput[T any, O OutputOf[T]] struct{ *internal.OutputState }

var _ Input[map[string]int] = GMapOutput[int, Output[int]]{}

// ElementType returns the reflected type of map[string]T.
func (o GMapOutput[T, O]) ElementType() reflect.Type {
	return typeOf[map[string]T]()
}

// Untyped converts a GMapOutput to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o GMapOutput[T, O]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// ToOutput converts the GMapOutput to an Output.
func (o GMapOutput[T, O]) ToOutput(ctx context.Context) Output[map[string]T] {
	return Output[map[string]T](o)
}

// MapIndex returns an Output holding the value inside the map
// with the given key.
//
// If there is no value with the given key,
// the returned Output holds the zero value of T.
func (o GMapOutput[T, O]) MapIndex(key Input[string]) O {
	result := MapOutput[T](o).MapIndex(key)
	return Cast[O, T](result)
}

// MapOutput is an Output value holding a map[string]T.
// It may be used as an Input[map[string]T].
type MapOutput[T any] struct{ *internal.OutputState }

var _ Input[map[string]int] = MapOutput[int]{}

// ElementType returns the reflected type of map[string]T.
func (o MapOutput[T]) ElementType() reflect.Type {
	return typeOf[map[string]T]()
}

// Untyped converts a MapOutput to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o MapOutput[T]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// ToOutput converts the MapOutput to an Output.
func (o MapOutput[T]) ToOutput(ctx context.Context) Output[map[string]T] {
	return Output[map[string]T](o)
}

// MapIndex returns an Output holding the value inside the map
// with the given key.
//
// If there is no value with the given key,
// the returned Output holds the zero value of T.
func (o MapOutput[T]) MapIndex(key Input[string]) Output[T] {
	// Note: These type parameters are not necessary in Go 1.21+.
	return Apply2[map[string]T, string](o, key, func(vs map[string]T, key string) T {
		if v, ok := vs[key]; ok {
			return v
		}
		var zero T
		return zero
	})
}
