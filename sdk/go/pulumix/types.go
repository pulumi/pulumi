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
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

// Input is a generic input for a Pulumi resource or function.
//
// All Input values are convertible to Output values,
// allowing them to be composed together with other inputs
// and resource outputs.
type Input[T any] interface {
	internal.Input

	// ToOutput converts this input to an Output[T].
	ToOutput(context.Context) Output[T]
}

// OutputOf[T] is a constraint satisfies by any output type
// that produces a value of type T.
// All such types MUST embed *pulumi.OutputState.
//
// For example, OutputOf[int] is satisfied by pulumix.Output[int]
// as well as pulumi.IntOutput.
type OutputOf[T any] interface {
	Input[T]
	internal.Output
}

// Output is a promise of a value of type T.
// It encodes the relationship between resources in a Pulumi application.
// The value may be unknown, a secret, and may have dependencies.
//
// Output[T] implements pulumix.Input[T], as well as pulumi.Input.
type Output[T any] struct{ *internal.OutputState }

var (
	_ internal.Output = Output[any]{}
	_ internal.Input  = Output[any]{}
	_ OutputOf[any]   = Output[any]{}
	_ Input[int]      = Output[int]{}
)

// isOutput is a special method implemented only by Output.
// It's used to identify Output[T] types dynamically
// since we can't match uninstantiated generic types directly.
//
// See InputElementType for more details.
func (o Output[T]) isOutput() {}

var (
	// isOutputType is a reflected interfaced type
	// that will match the isOutput method defined above.
	isOutputType = typeOf[interface{ isOutput() }]()

	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

// InputElementType returns the element type of an Input[T]
// or false if the type is not a Input[T].
func InputElementType(t reflect.Type) (e reflect.Type, ok bool) {
	// This is slightly complicated because we effectively have to match
	// the Input[T] constraint dynamically and then extract the T.
	//
	// The requirements for the Input constraint are:
	//
	//  1. the type must implement Input
	//  2. it must have a ToOutput method
	//  3. the ToOutput method must return an Output[T]
	//
	// Since Output is a generic type, we can't match the type directly.
	// However, we can match a special Output[T]-only method.
	if t == nil {
		return nil, false
	}

	input, ok := reflect.Zero(t).Interface().(internal.Input)
	if !ok {
		// Doesn't implement Input interface.
		return nil, false
	}

	m, ok := t.MethodByName("ToOutput")
	if !ok {
		return nil, false
	}

	mt := m.Type
	ok = mt.NumIn() == 2 && // receiver + context
		mt.In(1) == contextType &&
		mt.NumOut() == 1 && // Output[T]
		mt.Out(0).Implements(isOutputType)
	if ok {
		return input.ElementType(), true
	}
	return nil, false
}

// Untyped converts an Output[T] to a pulumi.Output.
//
// The concrete type of the returned value will be the most specific known
// implementation of pulumi.Output that matches the element type of the input,
// assuming it was registered with pulumi.RegisterOutputType.
//
// Use this to call legacy APIs that expect a concrete pulumi.Output.
// For example,
//
//	var o pulumix.Output[string] = // ...
//	legacyAPI(o.Untyped().(pulumi.StringOutput))
func (o Output[T]) Untyped() internal.Output {
	return internal.ToOutput(o)
}

// AsAny casts this Output[T] to an Output[any].
func (o Output[T]) AsAny() Output[any] {
	return Apply[T](o, func(v T) any { return v })
}

// ElementType reports the kind of value produced by this output.
//
// This is the same as the type argument T.
func (o Output[T]) ElementType() reflect.Type {
	return typeOf[T]()
}

// ToOutput returns this value back.
//
// This is necessary to implement Input[T].
func (o Output[T]) ToOutput(ctx context.Context) Output[T] {
	return o
}

// Val builds an Output holding the given value.
func Val[T any](v T) Output[T] {
	state := internal.NewOutputState(nil /* joinGroup */, typeOf[T]())
	internal.ResolveOutput(state, v, true, false, nil /* deps */)
	return Output[T]{OutputState: state}
}

// ConvertTyped builds an [Output] with the given untyped pulumi.Output,
// which must produce a value assignable to type T.
//
// Returns an error if o does not produce a compatible value.
func ConvertTyped[T any](o internal.Output) (Output[T], error) {
	typ := typeOf[T]()
	if elt := o.ElementType(); !elt.AssignableTo(typ) {
		return Output[T]{}, fmt.Errorf("cannot convert %v to %v", elt, typ)
	}

	return Output[T]{
		OutputState: internal.GetOutputState(o),
	}, nil
}

// MustConvertTyped is a variant of [ConvertTyped] that panics if the type of value
// returned by o is not assignable to T.
func MustConvertTyped[T any](o internal.Output) Output[T] {
	v, err := ConvertTyped[T](o)
	if err != nil {
		panic(err)
	}
	return v
}

// Cast turns any Input[T] into a concrete Output type O.
//
// O must meet the following requirements:
//
//   - implement ElementType() returning a type compatible with T
//   - embed *OutputState as a field
//
// The above is true for all output types generated by the Pulumi SDK.
//
// As an example, you can use Cast to convert interchangeably between
// Output[[]string], ArrayOutput[string], and StringArrayOutput.
//
//	var o pulumix.Output[[]string] = // ...
//	ao := pulumix.Cast[pulumix.ArrayOutput[string]](o)
//	sao := pulumix.Cast[pulumi.StringArrayOutput](o)
func Cast[O OutputOf[T], T any](i Input[T]) O {
	state := internal.GetOutputState(i.ToOutput(context.Background()))
	output := reflect.New(typeOf[O]()).Elem()
	internal.SetOutputState(output, state)
	return output.Interface().(O)
}

// typeOf reports the reflect.Type of T.
//
// This may be deleted if https://github.com/golang/go/issues/60088 lands.
func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
