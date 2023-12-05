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

// GPtrOutput is an Output value holding a pointer of type T.
// It may be used as an Input[*T].
//
// O is the kind of Output value that this GPtrOutput unwraps to.
type GPtrOutput[T any, O OutputOf[T]] struct{ *internal.OutputState }

var _ Input[*int] = GPtrOutput[int, Output[int]]{}

// ElementType returns the reflected type of *T.
func (o GPtrOutput[T, O]) ElementType() reflect.Type {
	return typeOf[*T]()
}

// Untyped converts a GPtrOutput[T] to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o GPtrOutput[T, O]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// ToOutput converts the GPtrOutput to an Output.
func (o GPtrOutput[T, O]) ToOutput(ctx context.Context) Output[*T] {
	return Output[*T](o)
}

// Elem dereferences the pointer, returning the underlying value in an Output.
// If the pointer is nil, the returned Output holds the zero value of T.
func (o GPtrOutput[T, O]) Elem() O {
	result := PtrOutput[T](o).Elem()
	return Cast[O, T](result)
}

// PtrOutput is an Output value holding a pointer of type T.
// It may be used as an Input[*T].
//
// PtrOutput is a simpler variant of GPtrOutput
// that is not parameterized on the kind of Output value it unwraps to.
type PtrOutput[T any] struct{ *internal.OutputState }

// Ptr builds an Output holding a pointer to the given value.
func Ptr[T any](v T) PtrOutput[T] {
	return PtrOutput[T](Val(&v))
}

// PtrOf returns an output holding a pointer to the value
// inside the given input.
//
// For example,
//
//	var s pulumi.StringOutput = ... // implements Input[string]
//	ps := PtrOf(s)                  // implements Input[*string]
func PtrOf[T any, I Input[T]](i I) PtrOutput[T] {
	po := Apply[T](i, func(v T) *T { return &v })
	return Cast[PtrOutput[T], *T](po)
}

// ElementType returns the reflected type of *T.
func (o PtrOutput[T]) ElementType() reflect.Type {
	return typeOf[*T]()
}

// Untyped converts a PtrOutput[T] to a pulumi.Output.
//
// See [Output.Untyped] for more details.
func (o PtrOutput[T]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// ToOutput converts the PtrOutput to an Output.
func (o PtrOutput[T]) ToOutput(ctx context.Context) Output[*T] {
	return Output[*T](o)
}

// Elem dereferences the pointer, returning the underlying value in an Output.
// If the pointer is nil, the returned Output holds the zero value of T.
func (o PtrOutput[T]) Elem() Output[T] {
	return Apply[*T](o, func(v *T) T {
		if v == nil {
			var zero T
			return zero
		}
		return *v
	})
}
