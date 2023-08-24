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
// See the License for the specific mlanguage governing permissions and
// limitations under the License.

package pulumix

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

// Array is a list of input values of the same type.
// It may be used as an Input[[]T].
type Array[T any] []Input[T]

var _ Input[[]int] = Array[int]{}

// ElementType returns the reflected type of []T.
func (Array[T]) ElementType() reflect.Type {
	return typeOf[[]T]()
}

// ToOutput builds an Output from the values in the array.
func (a Array[T]) ToOutput(ctx context.Context) Output[[]T] {
	return Output[[]T]{
		OutputState: internal.GetOutputState(internal.ToOutputWithContext(ctx, a)),
	}
}

// AsAny turns the array into an Output[any].
func (a Array[T]) AsAny() Output[any] {
	return a.ToOutput(context.Background()).AsAny()
}

// GArrayOutput is an Output value holding a slice of type T.
// It may be used as an Input[[]T].
//
// O is the kind of Output value that this GArrayOutput unwraps to.
type GArrayOutput[T any, O OutputOf[T]] struct{ *internal.OutputState }

var _ Input[[]int] = GArrayOutput[int, Output[int]]{}

// ElementType returns the reflected type of []T.
func (o GArrayOutput[T, O]) ElementType() reflect.Type {
	return typeOf[[]T]()
}

// ToOutput converts the GArrayOutput to an Output.
func (o GArrayOutput[T, O]) ToOutput(ctx context.Context) Output[[]T] {
	return Output[[]T](o)
}

// Untyped converts a GArrayOutput to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o GArrayOutput[T, O]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// Index returns an Output holding the value inside the slice
// at the given index.
//
// If the index is out of bounds,
// the returned Output holds the zero value of T.
func (o GArrayOutput[T, O]) Index(idx Input[int]) O {
	result := ArrayOutput[T](o).Index(idx)
	return Cast[O, T](result)
}

// ArrayOutput is an Output value holding a slice of type T.
// It may be used as an Input[[]T].
type ArrayOutput[T any] struct{ *internal.OutputState }

var _ Input[[]int] = ArrayOutput[int]{}

// ElementType returns the reflected type of []T.
func (o ArrayOutput[T]) ElementType() reflect.Type {
	return typeOf[[]T]()
}

// ToOutput converts the ArrayOutput to an Output.
func (o ArrayOutput[T]) ToOutput(ctx context.Context) Output[[]T] {
	return Output[[]T](o)
}

// Untyped converts a ArrayOutput to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o ArrayOutput[T]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// Index returns an Output holding the value inside the slice
// at the given index.
//
// If the index is out of bounds,
// the returned Output holds the zero value of T.
func (o ArrayOutput[T]) Index(idx Input[int]) Output[T] {
	// Note: These type parameters are not necessary in Go 1.21+.
	return Apply2[[]T, int](o, idx, func(vs []T, idx int) T {
		if idx < 0 || idx >= len(vs) {
			var zero T
			return zero
		}
		return vs[idx]
	})
}
