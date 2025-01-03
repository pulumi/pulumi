// Copyright 2016-2022, Pulumi Corporation.
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

//nolint:lll, interfacer
package pulumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
)

// Output helps encode the relationship between resources in a Pulumi application. Specifically an output property
// holds onto a value and the resource it came from. An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output = internal.Output

var (
	outputType = reflect.TypeOf((*Output)(nil)).Elem()
	inputType  = reflect.TypeOf((*Input)(nil)).Elem()
)

// RegisterOutputType registers an Output type with the Pulumi runtime. If a value of this type's concrete type is
// returned by an Apply, the Apply will return the specific Output type.
func RegisterOutputType(output Output) {
	internal.RegisterOutputType(output)
}

// RegisterInputType registers an Input type with the Pulumi runtime. This allows the input type to be instantiated
// for a given input interface.
func RegisterInputType(interfaceType reflect.Type, input Input) {
	internal.RegisterInputType(interfaceType, input)
}

// OutputState holds the internal details of an Output and implements the Apply and ApplyWithContext methods.
type OutputState = internal.OutputState

func newAnyOutput(wg *workGroup) (Output, func(interface{}), func(error)) {
	out := internal.NewOutputState(wg, anyType)

	resolve := func(v interface{}) {
		internal.ResolveOutput(out, v, true, false, nil)
	}
	reject := func(err error) {
		internal.RejectOutput(out, err)
	}

	return AnyOutput{out}, resolve, reject
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called. This acts like a promise.
//
// Deprecated: use Context.NewOutput instead.
func NewOutput() (Output, func(interface{}), func(error)) {
	return newAnyOutput(nil)
}

var anyOutputType = reflect.TypeOf((*AnyOutput)(nil)).Elem()

// IsSecret returns a bool representing the secretness of the Output
//
// IsSecret may return an inaccurate results if the Output is unknowable (during a
// preview) or contains an error.
func IsSecret(o Output) bool {
	return internal.IsSecret(o)
}

// Unsecret will unwrap a secret output as a new output with a resolved value and no secretness
func Unsecret(input Output) Output {
	return internal.Unsecret(input)
}

// UnsecretWithContext will unwrap a secret output as a new output with a resolved value and no secretness
func UnsecretWithContext(ctx context.Context, input Output) Output {
	return internal.UnsecretWithContext(ctx, input)
}

// ToSecret wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecret(input interface{}) Output {
	return internal.ToSecret(input)
}

// UnsafeUnknownOutput Creates an unknown output. This is a low level API and should not be used in programs as this
// will cause "pulumi up" to fail if called and used during a non-dryrun deployment.
func UnsafeUnknownOutput(deps []Resource) Output {
	output, _, _ := newAnyOutput(nil)
	internal.ResolveOutput(output, nil, false, false, resourcesToInternal(deps))
	return output
}

// ToSecretWithContext wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecretWithContext(ctx context.Context, input interface{}) Output {
	return internal.ToSecretWithContext(ctx, input)
}

// All returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each element of the
// array will contain the resolved value of the corresponding output. The output will be rejected if any of the inputs
// is rejected.
//
// For example:
//
//	connectionString := pulumi.All(sqlServer.Name, database.Name).ApplyT(
//		func (args []interface{}) pulumi.Output {
//			return Connection{
//				Server: args[0].(string),
//				Database: args[1].(string),
//			}
//		}
//	)
func All(inputs ...interface{}) ArrayOutput {
	return AllWithContext(context.Background(), inputs...)
}

// AllWithContext returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each
// element of the array will contain the resolved value of the corresponding output. The output will be rejected if any
// of the inputs is rejected.
//
// For example:
//
//	connectionString := pulumi.AllWithContext(ctx.Context(), sqlServer.Name, database.Name).ApplyT(
//		func (args []interface{}) pulumi.Output {
//			return Connection{
//				Server: args[0].(string),
//				Database: args[1].(string),
//			}
//		}
//	)
func AllWithContext(ctx context.Context, inputs ...interface{}) ArrayOutput {
	return ToOutputWithContext(ctx, inputs).(ArrayOutput)
}

// JSONMarshal uses "encoding/json".Marshal to serialize the given Output value into a JSON string.
//
// JSONMarshal *does not* support marshaling values that contain nested unknowns. You will need to manually create
// a top level unknown with [pulumi.Input.ApplyT] or [All]. This does not work:
//
//	pulumi.JSONMarshal(map[string]any{"key": myResource.Name})
//
// You need to move the output myResource.Name to a top level output:
//
//	pulumi.JSONMarshal(myResource.Name.Apply(func(name string) map[string]any{
//		return map[string]any{"key": name}
//	}))
//
// Supporting nested unknowns is tracked in https://github.com/pulumi/pulumi/issues/12460
func JSONMarshal(v interface{}) StringOutput {
	return JSONMarshalWithContext(context.Background(), v)
}

// JSONMarshalWithContext uses "encoding/json".Marshal to serialize the given Output value into a JSON string.
//
// JSONMarshalWithContext *does not* support marshaling values that contain nested unknowns. You will need to
// manually create a top level unknown with [pulumi.Input.ApplyT] or [All]. This does not work:
//
//	pulumi.JSONMarshalWithContext(ctx.Context(), map[string]any{"key": myResource.Name})
//
// You need to move the output myResource.Name to a top level output:
//
//	pulumi.JSONMarshalWithContext(ctx.Context(), myResource.Name.Apply(func(name string) map[string]any{
//		return map[string]any{"key": name}
//	}))
//
// Supporting nested unknowns is tracked in https://github.com/pulumi/pulumi/issues/12460
func JSONMarshalWithContext(ctx context.Context, v interface{}) StringOutput {
	o := ToOutputWithContext(ctx, v)
	return o.ApplyTWithContext(ctx, func(_ context.Context, v interface{}) (string, error) {
		json, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(json), nil
	}).(StringOutput)
}

// JSONUnmarshal uses "encoding/json".Unmarshal to deserialize the given Input JSON string into a value.
func JSONUnmarshal(data StringInput) AnyOutput {
	return JSONUnmarshalWithContext(context.Background(), data)
}

// JSONUnmarshalWithContext uses "encoding/json".Unmarshal to deserialize the given Input JSON string into a value.
func JSONUnmarshalWithContext(ctx context.Context, data StringInput) AnyOutput {
	o := ToOutputWithContext(ctx, data)
	return o.ApplyTWithContext(ctx, func(_ context.Context, data string) (interface{}, error) {
		var v interface{}
		err := json.Unmarshal([]byte(data), &v)
		if err != nil {
			return nil, err
		}
		return v, nil
	}).(AnyOutput)
}

// ToOutput returns an Output that will resolve when all Inputs contained in the given value have resolved.
func ToOutput(v interface{}) Output {
	return internal.ToOutput(v)
}

// ToOutputWithContext returns an Output that will resolve when all Outputs contained in the given value have
// resolved.
func ToOutputWithContext(ctx context.Context, v interface{}) Output {
	return internal.ToOutputWithContext(ctx, v)
}

func OutputWithDependencies(ctx context.Context, o Output, deps ...Resource) Output {
	r := make([]internal.Resource, len(deps))
	for i, d := range deps {
		r[i] = d.(internal.Resource)
	}
	return internal.OutputWithDependencies(ctx, o, r...)
}

func init() {
	internal.AnyOutputType = anyOutputType
}

// Input is the type of a generic input value for a Pulumi resource. This type is used in conjunction with Output
// to provide polymorphism over strongly-typed input values.
//
// The intended pattern for nested Pulumi value types is to define an input interface and a plain, input, and output
// variant of the value type that implement the input interface.
//
// For example, given a nested Pulumi value type with the following shape:
//
//	type Nested struct {
//	    Foo int
//	    Bar string
//	}
//
// We would define the following:
//
//	var nestedType = reflect.TypeOf((*Nested)(nil)).Elem()
//
//	type NestedInput interface {
//	    pulumi.Input
//
//	    ToNestedOutput() NestedOutput
//	    ToNestedOutputWithContext(context.Context) NestedOutput
//	}
//
//	type Nested struct {
//	    Foo int `pulumi:"foo"`
//	    Bar string `pulumi:"bar"`
//	}
//
//	type NestedInputValue struct {
//	    Foo pulumi.IntInput `pulumi:"foo"`
//	    Bar pulumi.StringInput `pulumi:"bar"`
//	}
//
//	func (NestedInputValue) ElementType() reflect.Type {
//	    return nestedType
//	}
//
//	func (v NestedInputValue) ToNestedOutput() NestedOutput {
//	    return pulumi.ToOutput(v).(NestedOutput)
//	}
//
//	func (v NestedInputValue) ToNestedOutputWithContext(ctx context.Context) NestedOutput {
//	    return pulumi.ToOutputWithContext(ctx, v).(NestedOutput)
//	}
//
//	type NestedOutput struct { *pulumi.OutputState }
//
//	func (NestedOutput) ElementType() reflect.Type {
//	    return nestedType
//	}
//
//	func (o NestedOutput) ToNestedOutput() NestedOutput {
//	    return o
//	}
//
//	func (o NestedOutput) ToNestedOutputWithContext(ctx context.Context) NestedOutput {
//	    return o
//	}
type Input = internal.Input

var anyType = reflect.TypeOf((*interface{})(nil)).Elem()

func Any(v interface{}) AnyOutput {
	return AnyWithContext(context.Background(), v)
}

func AnyWithContext(ctx context.Context, v interface{}) AnyOutput {
	return internal.ToOutputWithOutputType(ctx, anyOutputType, v).(AnyOutput)
}

// DeferredOutput creates an Output whose value can be later resolved from another Output instance.
func DeferredOutput[T any](ctx context.Context) (pulumix.Output[T], func(Output)) {
	var zero T
	rt := reflect.TypeOf(zero)
	state := internal.NewOutputState(nil, rt)
	out := pulumix.Output[T]{OutputState: state}
	resolve := func(o Output) {
		go func() {
			v, known, secret, deps, err := internal.AwaitOutput(ctx, o)
			if err != nil {
				internal.RejectOutput(state, err)
				return
			}
			internal.ResolveOutput(out, v, known, secret, deps)
		}()
	}
	return out, resolve
}

type AnyOutput struct{ *OutputState }

var _ pulumix.Input[any] = AnyOutput{}

func (AnyOutput) MarshalJSON() ([]byte, error) {
	return nil, errors.New("outputs can not be marshaled to JSON")
}

func (AnyOutput) ElementType() reflect.Type {
	return anyType
}

func (o AnyOutput) ToOutput(context.Context) pulumix.Output[any] {
	return pulumix.Output[any]{
		OutputState: o.OutputState,
	}
}

func (in ID) ToStringPtrOutput() StringPtrOutput {
	return in.ToStringPtrOutputWithContext(context.Background())
}

func (in ID) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return in.ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
}

func (o IDOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ToStringPtrOutputWithContext(context.Background())
}

func (o IDOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return o.ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
}

func (o IDOutput) awaitID(ctx context.Context) (ID, bool, bool, error) {
	id, known, secret, _, err := internal.AwaitOutput(ctx, o)
	if !known || err != nil {
		return "", known, false, err
	}
	return ID(convert(id, stringType).(string)), true, secret, nil
}

func (in URN) ToStringPtrOutput() StringPtrOutput {
	return in.ToStringPtrOutputWithContext(context.Background())
}

func (in URN) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return in.ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
}

func (o URNOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ToStringPtrOutputWithContext(context.Background())
}

func (o URNOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return o.ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
}

func (o URNOutput) awaitURN(ctx context.Context) (URN, bool, bool, error) {
	id, known, secret, _, err := internal.AwaitOutput(ctx, o)
	if !known || err != nil {
		return "", known, secret, err
	}
	return URN(convert(id, stringType).(string)), true, secret, nil
}

func convert(v interface{}, to reflect.Type) interface{} {
	rv := reflect.ValueOf(v)
	if !rv.Type().ConvertibleTo(to) {
		panic(fmt.Errorf("cannot convert output value of type %s to %s", rv.Type(), to))
	}
	return rv.Convert(to).Interface()
}

// ResourceOutput is an Output that returns Resource values.
// TODO: ResourceOutput and the init() should probably be code generated.
type ResourceOutput struct{ *OutputState }

var _ pulumix.Input[Resource] = ResourceOutput{}

func (ResourceOutput) MarshalJSON() ([]byte, error) {
	return nil, errors.New("Outputs can not be marshaled to JSON")
}

// ElementType returns the element type of this Output (Resource).
func (ResourceOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Resource)(nil)).Elem()
}

func (o ResourceOutput) ToOutput(context.Context) pulumix.Output[Resource] {
	return pulumix.Output[Resource]{
		OutputState: o.OutputState,
	}
}

func (o ResourceOutput) ToResourceOutput() ResourceOutput {
	return o
}

func (o ResourceOutput) ToResourceOutputWithContext(ctx context.Context) ResourceOutput {
	return o
}

// ResourceInput is an Input type carrying Resource values.
//
// Unfortunately `Resource` values do not implement `ResourceInput` in
// the current version. Use `NewResourceInput` instead.
type ResourceInput interface {
	Input

	ToResourceOutput() ResourceOutput
	ToResourceOutputWithContext(context.Context) ResourceOutput
}

func NewResourceInput(resource Resource) ResourceInput {
	return NewResourceOutput(resource)
}

func NewResourceOutput(resource Resource) ResourceOutput {
	return Int(0).ToIntOutput().ApplyT(func(int) Resource { return resource }).(ResourceOutput)
}

var _ ResourceInput = &ResourceOutput{}

var resourceArrayType = reflect.TypeOf((*[]Resource)(nil)).Elem()

// ResourceArrayInput is an input type that accepts ResourceArray and ResourceArrayOutput values.
type ResourceArrayInput interface {
	Input

	ToResourceArrayOutput() ResourceArrayOutput
	ToResourceArrayOutputWithContext(ctx context.Context) ResourceArrayOutput
}

// ResourceArray is an input type for []ResourceInput values.
type ResourceArray []ResourceInput

var _ pulumix.Input[[]Resource] = ResourceArray{}

// ElementType returns the element type of this Input ([]Resource).
func (ResourceArray) ElementType() reflect.Type {
	return resourceArrayType
}

func (in ResourceArray) ToOutput(ctx context.Context) pulumix.Output[[]Resource] {
	return pulumix.Output[[]Resource]{
		OutputState: internal.GetOutputState(ToOutputWithContext(ctx, in)),
	}
}

func (in ResourceArray) ToResourceArrayOutput() ResourceArrayOutput {
	return ToOutput(in).(ResourceArrayOutput)
}

func (in ResourceArray) ToResourceArrayOutputWithContext(ctx context.Context) ResourceArrayOutput {
	return ToOutputWithContext(ctx, in).(ResourceArrayOutput)
}

// ResourceArrayOutput is an Output that returns []Resource values.
type ResourceArrayOutput struct{ *OutputState }

var _ pulumix.Input[[]Resource] = ResourceArrayOutput{}

func (ResourceArrayOutput) MarshalJSON() ([]byte, error) {
	return nil, errors.New("Outputs can not be marshaled to JSON")
}

// ElementType returns the element type of this Output ([]Resource).
func (ResourceArrayOutput) ElementType() reflect.Type {
	return resourceArrayType
}

func (o ResourceArrayOutput) ToOutput(context.Context) pulumix.Output[[]Resource] {
	return pulumix.Output[[]Resource]{
		OutputState: o.OutputState,
	}
}

func (o ResourceArrayOutput) ToResourceArrayOutput() ResourceArrayOutput {
	return o
}

func (o ResourceArrayOutput) ToResourceArrayOutputWithContext(ctx context.Context) ResourceArrayOutput {
	return o
}

// Index looks up the i'th element of the array if it is in bounds or returns the zero value of the appropriate
// type if the index is out of bounds.
func (o ResourceArrayOutput) Index(i IntInput) ResourceOutput {
	return All(o, i).ApplyT(func(vs []interface{}) Resource {
		arr := vs[0].([]Resource)
		idx := vs[1].(int)
		var ret Resource
		if idx >= 0 && idx < len(arr) {
			ret = arr[idx]
		}
		return ret
	}).(ResourceOutput)
}

func ToResourceArray(in []Resource) ResourceArray {
	return NewResourceArray(in...)
}

func NewResourceArray(in ...Resource) ResourceArray {
	a := make(ResourceArray, len(in))
	for i, v := range in {
		a[i] = NewResourceInput(v)
	}
	return a
}

func ToResourceArrayOutput(in []ResourceOutput) ResourceArrayOutput {
	return NewResourceArrayOutput(in...)
}

func NewResourceArrayOutput(in ...ResourceOutput) ResourceArrayOutput {
	a := make(ResourceArray, len(in))
	for i, v := range in {
		a[i] = v
	}
	return a.ToResourceArrayOutput()
}

func init() {
	RegisterInputType(reflect.TypeOf((*ResourceArrayInput)(nil)).Elem(), ResourceArray{})
	RegisterOutputType(ResourceOutput{})
	RegisterOutputType(ResourceArrayOutput{})
}

// coerceTypeConversion assigns src to dst, performing deep type coercion as necessary.
func coerceTypeConversion(src interface{}, dst reflect.Type) (interface{}, error) {
	makeError := func(src, dst reflect.Value) error {
		return fmt.Errorf("expected value of type %s, not %s", dst.Type(), src.Type())
	}
	var coerce func(reflect.Value, reflect.Value) error
	coerce = func(src, dst reflect.Value) error {
		if src.Type().Kind() == reflect.Interface && !src.IsNil() {
			src = src.Elem()
		}
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
			return nil
		}
		//nolint:exhaustive // We only handle a few types here.
		switch dst.Type().Kind() {
		case reflect.Map:
			if src.Kind() != reflect.Map {
				return makeError(src, dst)
			}

			dst.Set(reflect.MakeMapWithSize(dst.Type(), src.Len()))

			for iter := src.MapRange(); iter.Next(); {
				dstKey := reflect.New(dst.Type().Key()).Elem()
				dstVal := reflect.New(dst.Type().Elem()).Elem()
				if err := coerce(iter.Key(), dstKey); err != nil {
					return fmt.Errorf("invalid key: %w", err)
				}
				if err := coerce(iter.Value(), dstVal); err != nil {
					return fmt.Errorf("[%#v]: %w", dstKey.Interface(), err)
				}
				dst.SetMapIndex(dstKey, dstVal)
			}

			return nil
		case reflect.Slice:
			if src.Kind() != reflect.Slice {
				return makeError(src, dst)
			}
			dst.Set(reflect.MakeSlice(dst.Type(), src.Len(), src.Cap()))
			for i := 0; i < src.Len(); i++ {
				dstVal := reflect.New(dst.Type().Elem()).Elem()
				if err := coerce(src.Index(i), dstVal); err != nil {
					return fmt.Errorf("[%d]: %w", i, err)
				}
				dst.Index(i).Set(dstVal)
			}
			return nil
		default:
			return makeError(src, dst)
		}
	}

	srcV, dstV := reflect.ValueOf(src), reflect.New(dst).Elem()

	if err := coerce(srcV, dstV); err != nil {
		return nil, err
	}
	return dstV.Interface(), nil
}

func getOutputDeps(o Output) []Resource {
	return resourcesFromInternal(internal.OutputDependencies(o))
}

func resourcesToInternal(in []Resource) []internal.Resource {
	if in == nil {
		return nil
	}
	out := make([]internal.Resource, len(in))
	for i, r := range in {
		out[i] = r
	}
	return out
}

func resourcesFromInternal(in []internal.Resource) []Resource {
	if in == nil {
		return nil
	}
	out := make([]Resource, len(in))
	for i, r := range in {
		out[i] = r.(Resource)
	}
	return out
}
