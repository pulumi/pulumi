// Copyright 2016-2020, Pulumi Corporation.
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

// nolint: lll, interfacer
package pulumi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Map map[string]AnyOutput

// Apply is a generic method for applying a computation once an Output[T]'s value is known. Unfortunately, Go
// does not support parametric methods, so this needs to be a global function. For more information, see
// https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#No-parameterized-methods.
func Apply[T, U any](o Output[T], applier func(v T) U) Output[U] {
	return ApplyWithContext[T, U](context.Background(), o, applier)
}

func ApplyWithContext[T, U any](ctx context.Context, o Output[T], applier func(v T) U) Output[U] {
	return ApplyWithContextErr[T, U](ctx, o, func(v T) (U, error) {
		result := applier(v)
		return result, nil
	})
}

func ApplyErr[T, U any](o Output[T], applier func(v T) (U, error)) Output[U] {
	return ApplyWithContextErr[T, U](context.Background(), o, applier)
}

func ApplyWithContextErr[T, U any](ctx context.Context, o Output[T], applier func(v T) (U, error)) Output[U] {
	result := newOutput[U](o.getState().join, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.awaitT(ctx)
		if err != nil || !known {
			result.getUnsafeResolvers().fulfill(nil, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		final, err := applier(v)
		result.getUnsafeResolvers().fulfill(final, true, secret, deps, err)
	}()
	return result
}

func Cast[T any](o AnyOutput) Output[T] {
	return CastWithContext[T](context.Background(), o)
}

func CastWithContext[T any](ctx context.Context, o AnyOutput) Output[T] {
	result := newOutput[T](o.snap().join, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.await(ctx)
		if err != nil || !known {
			result.getUnsafeResolvers().fulfill(nil, known, secret, deps, err)
			return
		}

		// Attempt to convert the value to the desired type, and propagate an
		// error if it fails.
		if t, ok := v.(T); ok {
			result.getUnsafeResolvers().fulfill(t, true, secret, deps, nil)
		} else {
			result.getUnsafeResolvers().fulfill(nil, false, secret, deps,
				fmt.Errorf("failed to convert value of type '%s' in Cast[%s] operation",
					reflect.TypeOf(v), reflect.TypeOf(t)))
		}
	}()
	return result
}

func Weak(o AnyOutput) Output[interface{}] {
	return Cast[interface{}](o)
}

// TODO: any combinators.

type workGroups []*workGroup

func (wgs workGroups) add() {
	for _, g := range wgs {
		g.Add(1)
	}
}

func (wgs workGroups) done() {
	for _, g := range wgs {
		g.Done()
	}
}

const (
	outputPending = iota
	outputResolved
	outputRejected
)

type AnyOutput interface {
	ElementType() reflect.Type

	ApplyT(applier func(any) any) AnyOutput
	ApplyTErr(applier func(any) (any, error)) AnyOutput
	ApplyTWithContext(ctx context.Context, applier func(any) any) AnyOutput
	ApplyTWithContextErr(ctx context.Context, applier func(any) (any, error)) AnyOutput

	await(ctx context.Context) (any, bool, bool, []Resource, error)
	dependencies() []Resource
	snap() outputSnapshot

	// getUnsafeResolvers returns a handle to reject and resolve functionality that
	// can be used internal to the runtime to perform unsafe operations.
	getUnsafeResolvers() outputStateResolvers
	unsafeSetSecret(secret bool)
}

type outputSnapshot struct {
	join   *workGroup
	state  uint32
	value  interface{}
	err    error
	known  bool
	secret bool
}

type outputStateResolvers interface {
	fulfill(value interface{}, known, secret bool, deps []Resource, err error)
	fulfillValue(value reflect.Value, known, secret bool, deps []Resource, err error)
	reject(err error)
	resolve(value interface{}, known, secret bool, deps []Resource)
	resolveValue(value reflect.Value, known, secret bool, deps []Resource)
}

type emptyOutput interface {
	dynamicInit(join *workGroup, elemType reflect.Type, deps ...Resource)
}

// Output helps encode the relationship between resources in a Pulumi application. Specifically an output property
// holds onto a value and the resource it came from. An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output[T any] struct {
	state *OutputState[T]
}

var _ AnyOutput = Output[interface{}]{}

func (o Output[T]) Nil() bool { return o.state == nil }

// Awkward initialization required due to lack of generic reflection APIs.
func (o *Output[T]) dynamicInit(join *workGroup, elemType reflect.Type, deps ...Resource) {
	o.state = newOutput[T](join, deps...).state
}

func (o Output[T]) awaitT(ctx context.Context) (T, bool, bool, []Resource, error) {
	return o.state.awaitT(ctx)
}

// Make Output[T] work like an AnyOutput by delegating to the underlying *OutputState[T].
func (o Output[T]) ElementType() reflect.Type { return o.state.elementType() }
func (o Output[T]) ApplyT(applier func(any) any) AnyOutput {
	return o.state.applyT(applier)
}
func (o Output[T]) ApplyTErr(applier func(any) (any, error)) AnyOutput {
	return o.state.applyTErr(applier)
}
func (o Output[T]) ApplyTWithContext(ctx context.Context, applier func(any) any) AnyOutput {
	return o.state.applyTWithContext(ctx, applier)
}
func (o Output[T]) ApplyTWithContextErr(ctx context.Context, applier func(any) (any, error)) AnyOutput {
	return o.state.applyTWithContextErr(ctx, applier)
}
func (o Output[T]) await(ctx context.Context) (any, bool, bool, []Resource, error) {
	return o.state.await(ctx)
}
func (o Output[T]) dependencies() []Resource                 { return o.state.dependencies() }
func (o Output[T]) snap() outputSnapshot                     { return o.state.snap() }
func (o Output[T]) getUnsafeResolvers() outputStateResolvers { return o.state.getUnsafeResolvers() }
func (o Output[T]) unsafeSetSecret(secret bool)              { o.state.unsafeSetSecret(secret) }
func (o Output[T]) getState() *OutputState[T]                { return o.state }

type OutputState[T any] struct {
	mutex sync.Mutex
	cond  *sync.Cond

	join *workGroup // the wait group associated with this output, if any.

	state uint32 // one of output{Pending,Resolved,Rejected}

	value  T     // the value of this output if it is resolved.
	err    error // the error associated with this output if it is rejected.
	known  bool  // true if this output's value is known.
	secret bool  // true if this output's value is secret

	element reflect.Type // the element type of this output.
	deps    []Resource   // the dependencies associated with this output property.
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called. This acts like a promise.
//
// Deprecated: use Context.NewOutput instead.
func NewOutput[T any](wg *workGroup) (Output[T], func(T), func(error)) {
	out := newOutput[T](wg)

	resolve := func(v T) {
		out.getUnsafeResolvers().resolve(v, true, false, nil)
	}
	reject := func(err error) {
		out.getUnsafeResolvers().reject(err)
	}

	return out, resolve, reject
}

func newOutput[T any](join *workGroup, deps ...Resource) Output[T] {
	if join != nil {
		join.Add(1)
	}

	out := &OutputState[T]{
		join:    join,
		element: reflect.TypeOf((*T)(nil)).Elem(),
		deps:    deps,
	}
	out.cond = sync.NewCond(&out.mutex)
	return Output[T]{state: out}
}

func newDynamicOutput(join *workGroup, elemType reflect.Type, deps ...Resource) Output[interface{}] {
	if join != nil {
		join.Add(1)
	}

	out := &OutputState[interface{}]{
		join:    join,
		element: elemType,
		deps:    deps,
	}
	out.cond = sync.NewCond(&out.mutex)
	return Output[interface{}]{state: out}
}

func newAnyOutput(wg *workGroup) (AnyOutput, func(interface{}), func(error)) {
	return NewOutput[interface{}](wg)
}

func (o *OutputState[T]) elementType() reflect.Type {
	if o == nil {
		return anyType
	}
	return o.element
}

func (o *OutputState[T]) getState() *OutputState[T]                { return o }
func (o *OutputState[T]) getUnsafeResolvers() outputStateResolvers { return o }

func (o *OutputState[T]) dependencies() []Resource {
	if o == nil {
		return nil
	}
	return o.deps
}

func (o *OutputState[T]) snap() outputSnapshot {
	return outputSnapshot{
		join:   o.join,
		state:  o.state,
		value:  o.value,
		err:    o.err,
		known:  o.known,
		secret: o.secret,
	}
}

func (o *OutputState[T]) fulfill(value interface{}, known, secret bool, deps []Resource, err error) {
	o.fulfillValue(reflect.ValueOf(value), known, secret, deps, err)
}

func (o *OutputState[T]) fulfillValue(value reflect.Value, known, secret bool, deps []Resource, err error) {
	if o == nil {
		return
	}

	o.mutex.Lock()
	defer func() {
		o.mutex.Unlock()
		o.cond.Broadcast()
	}()

	if o.state != outputPending {
		return
	}

	// If there is a wait group associated with this output--which should be the case in all outputs created
	// by a Context or a combinator that was passed any non-prompt value--ensure that we decrement its count
	// before this function returns. This allows Contexts to remain alive until all outstanding asynchronous
	// work that may reference that context has completed.
	//
	// Code that creates an output must take care to bump the count for any relevant waitgroups prior to
	// creating asynchronous work associated with that output. For combinators, this means digging through
	// inputs, collecting all wait groups, and calling Add (see toOutputTWithContext for an example). For
	// code that creates outputs directly, this is as simple as passing the wait group for the associated
	// context to newOutput.
	//
	// User code should use combinators or Context.NewOutput to ensure that all asynchronous work is
	// associated with a Context.
	if o.join != nil {
		// If this output is being resolved to another output O' with a different wait group, ensure that we
		// don't decrement the current output's wait group until O' completes.
		if other, ok := value.Interface().(AnyOutput); ok && other.snap().join != o.join {
			go func() {
				//nolint:errcheck
				other.await(context.Background())
				o.join.Done()
			}()
		} else {
			defer o.join.Done()
		}
	}

	if err != nil {
		o.state, o.err, o.known, o.secret = outputRejected, err, true, secret
	} else {
		if value.IsValid() {
			reflect.ValueOf(&o.value).Elem().Set(value)
		}
		o.state, o.known, o.secret = outputResolved, known, secret

		// If needed, merge the up-front provided dependencies with fulfilled dependencies, pruning duplicates.
		if len(deps) == 0 {
			// We didn't get any new dependencies, so no need to merge.
			return
		}
		depSet := make(map[Resource]struct{})
		for _, d := range o.deps {
			depSet[d] = struct{}{}
		}
		for _, d := range deps {
			depSet[d] = struct{}{}
		}
		mergedDeps := make([]Resource, 0, len(depSet))
		for d := range depSet {
			mergedDeps = append(mergedDeps, d)
		}
		o.deps = mergedDeps
	}
}

func (o *OutputState[T]) resolve(value interface{}, known, secret bool, deps []Resource) {
	o.fulfill(value, known, secret, deps, nil)
}

func (o *OutputState[T]) resolveValue(value reflect.Value, known, secret bool, deps []Resource) {
	o.fulfillValue(value, known, secret, deps, nil)
}

func (o *OutputState[T]) reject(err error) {
	o.fulfill(nil, true, false, nil, err)
}

func (o *OutputState[T]) unsafeSetSecret(secret bool) {
	o.secret = secret
}

func (o *OutputState[T]) await(ctx context.Context) (interface{}, bool, bool, []Resource, error) {
	v, known, secret, deps, err := o.awaitT(ctx)
	return v, known, secret, deps, err
}

func (o *OutputState[T]) awaitT(ctx context.Context) (T, bool, bool, []Resource, error) {
	var zero T
	if o == nil {
		// If the state is nil, treat its value as resolved and unknown.
		return zero, false, false, nil, nil
	}

	o.mutex.Lock()
	for o.state == outputPending {
		if ctx.Err() != nil {
			return zero, true, false, nil, ctx.Err()
		}
		o.cond.Wait()
	}
	o.mutex.Unlock()

	if !o.known || o.err != nil {
		return zero, o.known, o.secret, o.deps, o.err
	}

	return o.value, true, o.secret, o.deps, nil
}

var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()

func makeContextful(fn interface{}, elementType reflect.Type) interface{} {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		panic(errors.New("applier must be a function"))
	}

	ft := fv.Type()
	if ft.NumIn() != 1 || !elementType.AssignableTo(ft.In(0)) {
		panic(fmt.Errorf("applier must have 1 input parameter assignable from %v", elementType))
	}

	var outs []reflect.Type
	switch ft.NumOut() {
	case 1:
		// Okay
		outs = []reflect.Type{ft.Out(0)}
	case 2:
		// Second out parameter must be of type error
		if !ft.Out(1).AssignableTo(errorType) {
			panic(errors.New("applier's second return type must be assignable to error"))
		}
		outs = []reflect.Type{ft.Out(0), ft.Out(1)}
	default:
		panic(errors.New("applier must return exactly one or two values"))
	}

	ins := []reflect.Type{contextType, ft.In(0)}
	contextfulType := reflect.FuncOf(ins, outs, ft.IsVariadic())
	contextfulFunc := reflect.MakeFunc(contextfulType, func(args []reflect.Value) []reflect.Value {
		// Slice off the context argument and call the applier.
		return fv.Call(args[1:])
	})
	return contextfulFunc.Interface()
}

func checkApplier(fn interface{}, elementType reflect.Type) reflect.Value {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		panic(errors.New("applier must be a function"))
	}

	ft := fv.Type()
	if ft.NumIn() != 2 || !contextType.AssignableTo(ft.In(0)) || !elementType.AssignableTo(ft.In(1)) {
		panic(fmt.Errorf("applier's input parameters must be assignable from %v and %v", contextType, elementType))
	}

	switch ft.NumOut() {
	case 1:
		// Okay
	case 2:
		// Second out parameter must be of type error
		if !ft.Out(1).AssignableTo(errorType) {
			panic(errors.New("applier's second return type must be assignable to error"))
		}
	default:
		panic(errors.New("applier must return exactly one or two values"))
	}

	// Okay
	return fv
}

// ApplyT transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
//
// The applier function must have one of the following signatures:
//
//    func (v U) T
//    func (v U) (T, error)
//
// U must be assignable from the ElementType of the Output. If T is a type that has a registered Output type, the
// result of ApplyT will be of the registered Output type, and can be used in an appropriate type assertion:
//
//    stringOutput := pulumi.String("hello").ToStringOutput()
//    intOutput := stringOutput.ApplyT(func(v string) int {
//        return len(v)
//    }).(pulumi.IntOutput)
//
// Otherwise, the result will be of type AnyOutput:
//
//    stringOutput := pulumi.String("hello").ToStringOutput()
//    intOutput := stringOutput.ApplyT(func(v string) []rune {
//        return []rune(v)
//    }).(pulumi.AnyOutput)
//
func (o *OutputState[T]) applyT(applier func(any) any) AnyOutput {
	return o.applyTWithContext(context.Background(), makeContextful(applier, o.elementType()).(func(any) any))
}

func (o *OutputState[T]) applyTErr(applier func(any) (any, error)) AnyOutput {
	return o.applyTWithContextErr(context.Background(), makeContextful(applier, o.elementType()).(func(any) (any, error)))
}

// ApplyTWithContext transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
// The provided context can be used to reject the output as canceled.
//
// The applier function must have one of the following signatures:
//
//    func (ctx context.Context, v U) T
//    func (ctx context.Context, v U) (T, error)
//
// U must be assignable from the ElementType of the Output. If T is a type that has a registered Output type, the
// result of ApplyT will be of the registered Output type, and can be used in an appropriate type assertion:
//
//    stringOutput := pulumi.String("hello").ToStringOutput()
//    intOutput := stringOutput.ApplyTWithContext(func(_ context.Context, v string) int {
//        return len(v)
//    }).(pulumi.IntOutput)
//
// Otherwise, the result will be of type AnyOutput:
//
//    stringOutput := pulumi.String("hello").ToStringOutput()
//    intOutput := stringOutput.ApplyT(func(_ context.Context, v string) []rune {
//        return []rune(v)
//    }).(pulumi.AnyOutput)
//

func (o *OutputState[T]) applyTWithContext(ctx context.Context, applier func(any) any) AnyOutput {
	fn := checkApplier(applier, o.elementType())
	result := newDynamicOutput(o.join, anyType, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.getState().await(ctx)
		if err != nil || !known {
			result.getUnsafeResolvers().fulfill(nil, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		val := reflect.ValueOf(v)
		if !val.IsValid() {
			val = reflect.Zero(o.elementType())
		}
		results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), val})
		if len(results) == 2 && !results[1].IsNil() {
			result.getUnsafeResolvers().reject(results[1].Interface().(error))
			return
		}

		// Fulfill the result.
		result.getUnsafeResolvers().fulfillValue(results[0], true, secret, deps, nil)
	}()
	return result
}

func (o *OutputState[T]) applyTWithContextErr(ctx context.Context, applier func(any) (any, error)) AnyOutput {
	fn := checkApplier(applier, o.elementType())
	result := newDynamicOutput(o.join, anyType, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.getState().await(ctx)
		if err != nil || !known {
			result.getUnsafeResolvers().fulfill(nil, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		val := reflect.ValueOf(v)
		if !val.IsValid() {
			val = reflect.Zero(o.elementType())
		}
		results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), val})
		if len(results) == 2 && !results[1].IsNil() {
			result.getUnsafeResolvers().reject(results[1].Interface().(error))
			return
		}

		// Fulfill the result.
		result.getUnsafeResolvers().fulfillValue(results[0], true, secret, deps, nil)
	}()
	return result
}

// IsSecret returns a bool representing the secretness of the Output
func IsSecret(o AnyOutput) bool {
	return o.snap().secret
}

// Unsecret will unwrap a secret output as a new output with a resolved value and no secretness
func Unsecret[T any](input Output[T]) Output[T] {
	return UnsecretWithContext(context.Background(), input)
}

// UnsecretWithContext will unwrap a secret output as a new output with a resolved value and no secretness
func UnsecretWithContext[T any](ctx context.Context, input Output[T]) Output[T] {
	other := ApplyWithContext(ctx, input, func(v T) T { return v })
	// set immediate secretness ahead of resolution/fufillment
	other.unsafeSetSecret(false)
	return other
}

// ToSecret wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecret[T any](input T) Output[T] {
	return ToSecretWithContext(context.Background(), input)
}

// ToSecretWithContext wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecretWithContext[T any](ctx context.Context, input T) Output[T] {
	other := ToOutputWithContext(ctx, input)
	// set immediate secretness ahead of resolution/fufillment
	other.unsafeSetSecret(true)
	return other
}

// All returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each element of the
// array will contain the resolved value of the corresponding output. The output will be rejected if any of the inputs
// is rejected.
func All(inputs ...AnyOutput) Output[[]interface{}] {
	return AllWithContext(context.Background(), inputs...)
}

// AllWithContext returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each
// element of the array will contain the resolved value of the corresponding output. The output will be rejected if any
// of the inputs is rejected.
func AllWithContext(ctx context.Context, inputs ...AnyOutput) Output[[]interface{}] {
	joined := ToOutputWithContext(ctx, inputs)
	return Apply(joined, func(outs []AnyOutput) []interface{} {
		// TODO: error flowing, etc.
		var rets []interface{}
		for _, out := range outs {
			rets = append(rets, out.snap().value)
		}
		return rets
	})
}

func gatherDependencies(v interface{}) ([]Resource, workGroups) {
	if v == nil {
		return nil, nil
	}

	depSet := make(map[Resource]struct{})
	joinSet := make(map[*workGroup]struct{})
	gatherDependencySet(reflect.ValueOf(v), depSet, joinSet)

	var joins workGroups
	if len(joinSet) > 0 {
		joins = make([]*workGroup, 0, len(joinSet))
		for j := range joinSet {
			joins = append(joins, j)
		}
	}

	var deps []Resource
	if len(depSet) > 0 {
		deps = make([]Resource, 0, len(depSet))
		for d := range depSet {
			deps = append(deps, d)
		}
	}

	return deps, joins
}

var resourceType = reflect.TypeOf((*Resource)(nil)).Elem()

func gatherDependencySet(v reflect.Value, deps map[Resource]struct{}, joins map[*workGroup]struct{}) {
	for {
		// Check for an Output that we can pull dependencies off of.
		if output, ok := v.Interface().(AnyOutput); ok {
			if join := output.snap().join; join != nil {
				joins[join] = struct{}{}
			}
			for _, d := range output.dependencies() {
				deps[d] = struct{}{}
			}
			return
		}
		// Check for an actual Resource.
		if v.Type().Implements(resourceType) {
			if v.CanInterface() {
				resource := v.Convert(resourceType).Interface().(Resource)
				deps[resource] = struct{}{}
			}
			return
		}

		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if v.IsNil() {
				return
			}
			v = v.Elem()
			continue
		case reflect.Struct:
			numFields := v.Type().NumField()
			for i := 0; i < numFields; i++ {
				gatherDependencySet(v.Field(i), deps, joins)
			}
		case reflect.Array, reflect.Slice:
			l := v.Len()
			for i := 0; i < l; i++ {
				gatherDependencySet(v.Index(i), deps, joins)
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				gatherDependencySet(iter.Key(), deps, joins)
				gatherDependencySet(iter.Value(), deps, joins)
			}
		}
		return
	}
}

func awaitInputs(ctx context.Context, v, resolved reflect.Value) (bool, bool, []Resource, error) {
	contract.Assert(v.IsValid())

	if !resolved.CanSet() {
		return true, false, nil, nil
	}

	// If the value is an Output with of a different element type, turn it into the appropriate type and await it.
	valueType, isInput := v.Type(), false
	if output, ok := v.Interface().(AnyOutput); ok {
		assignOutput := false
		valueType = output.ElementType()

		// If the element type of the input is not identical to the type of the destination and the destination is not
		// the any type (i.e. interface{}), attempt to convert the input to the appropriately-typed output.
		if valueType != resolved.Type() &&
			resolved.Type() != anyType && resolved.Type() != outputType &&
			!valueType.AssignableTo(resolved.Type()) {
			// If the value type is not assignable to the destination, see if we can assign the input value itself
			// to the destination.
			if !v.Type().AssignableTo(resolved.Type()) {
				panic(fmt.Errorf("cannot convert an input of type %T to a value of type %v",
					output, resolved.Type()))
			} else {
				assignOutput = true
			}
		}

		// Await its value. The returned value is fully resolved.
		e, known, secret, deps, err := output.await(ctx)
		if err != nil || !known {
			return known, secret, deps, err
		}

		if assignOutput {
			resolved.Set(reflect.ValueOf(e))
		} else {
			val := reflect.ValueOf(e)
			if !val.IsValid() {
				val = reflect.Zero(output.ElementType())
			}
			resolved.Set(val)
		}

		return true, secret, deps, nil
	}

	contract.Assertf(valueType.AssignableTo(resolved.Type()), "%s not assignable to %s", valueType.String(), resolved.Type().String())

	// If the resolved type is an interface, make an appropriate destination from the value's type.
	if resolved.Kind() == reflect.Interface {
		iface := resolved
		defer func() { iface.Set(resolved) }()
		resolved = reflect.New(valueType).Elem()
	}

	known, secret, deps, err := true, false, make([]Resource, 0), error(nil)
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			return awaitInputs(ctx, v.Elem(), resolved)
		}
	case reflect.Ptr:
		if !v.IsNil() {
			resolved.Set(reflect.New(resolved.Type().Elem()))
			return awaitInputs(ctx, v.Elem(), resolved.Elem())
		}
	case reflect.Struct:
		typ := v.Type()
		getMappedField := mapStructTypes(typ, resolved.Type())
		numFields := typ.NumField()
		for i := 0; i < numFields; i++ {
			_, field := getMappedField(resolved, i)
			fknown, fsecret, fdeps, ferr := awaitInputs(ctx, v.Field(i), field)
			known = known && fknown
			secret = secret || fsecret
			deps = append(deps, fdeps...)
			if err == nil {
				err = ferr
			}
		}
	case reflect.Array:
		l := v.Len()
		for i := 0; i < l; i++ {
			eknown, esecret, edeps, eerr := awaitInputs(ctx, v.Index(i), resolved.Index(i))
			known = known && eknown
			secret = secret || esecret
			deps = append(deps, edeps...)
			if err == nil {
				err = eerr
			}
		}
	case reflect.Slice:
		l := v.Len()
		resolved.Set(reflect.MakeSlice(resolved.Type(), l, l))
		for i := 0; i < l; i++ {
			eknown, esecret, edeps, eerr := awaitInputs(ctx, v.Index(i), resolved.Index(i))
			known = known && eknown
			secret = secret || esecret
			deps = append(deps, edeps...)
			if err == nil {
				err = eerr
			}
		}
	case reflect.Map:
		resolved.Set(reflect.MakeMap(resolved.Type()))
		resolvedKeyType, resolvedValueType := resolved.Type().Key(), resolved.Type().Elem()
		iter := v.MapRange()
		for iter.Next() {
			kv := reflect.New(resolvedKeyType).Elem()
			kknown, ksecret, kdeps, kerr := awaitInputs(ctx, iter.Key(), kv)
			if err == nil {
				err = kerr
			}

			vv := reflect.New(resolvedValueType).Elem()
			vknown, vsecret, vdeps, verr := awaitInputs(ctx, iter.Value(), vv)
			if err == nil {
				err = verr
			}

			if kerr == nil && verr == nil && kknown && vknown {
				resolved.SetMapIndex(kv, vv)
			}

			known = known && kknown && vknown
			secret = secret || ksecret || vsecret
			deps = append(append(deps, kdeps...), vdeps...)
		}
	default:
		if isInput {
			v = v.Convert(valueType)
		}
		resolved.Set(v)
	}
	return known, secret, deps, err
}

func toOutputWithContext[T any](ctx context.Context, join *workGroup, v interface{}, forceSecretVal *bool) Output[T] {
	deps, joins := gatherDependencies(v)

	done := joins.done
	if join == nil {
		switch len(joins) {
		case 0:
			// OK
		case 1:
			join, joins, done = joins[0], nil, func() {}
		default:
			join = &workGroup{}
			done = func() {
				join.Wait()
				joins.done()
			}
		}
	}
	joins.add()

	output := newOutput[T](join, deps...)
	go func() {
		defer done()

		if v == nil {
			output.getUnsafeResolvers().fulfill(nil, true, false, nil, nil)
			return
		}

		result := reflect.New(output.ElementType())
		known, secret, deps, err := awaitInputs(ctx, reflect.ValueOf(v), result)
		if forceSecretVal != nil {
			secret = *forceSecretVal
		}
		if err != nil || !known {
			output.getUnsafeResolvers().fulfill(nil, known, secret, deps, err)
			return
		}
		output.getUnsafeResolvers().resolveValue(result, true, secret, deps)
	}()
	return output
}

func In[T any](v T) Output[T] {
	return ToOutputWithContext(context.Background(), v)
}

func Inp[T any](v T) *Output[T] {
	out := ToOutputWithContext(context.Background(), v)
	return &out
}

// ToOutput returns an Output that will resolve when all Inputs contained in the given value have resolved.
func ToOutput[T any](v T) Output[T] {
	return ToOutputWithContext(context.Background(), v)
}

// ToOutputWithContext returns an Output that will resolve when all Outputs contained in the given value have
// resolved.
func ToOutputWithContext[T any](ctx context.Context, v T) Output[T] {
	return toOutputWithContext[T](ctx, nil, v, nil)
}

// Input is the type of a generic input value for a Pulumi resource. This type is used in conjunction with Output
// to provide polymorphism over strongly-typed input values.
//
// The intended pattern for nested Pulumi value types is to define an input interface and a plain, input, and output
// variant of the value type that implement the input interface.
//
// For example, given a nested Pulumi value type with the following shape:
//
//     type Nested struct {
//         Foo int
//         Bar string
//     }
//
// We would define the following:
//
//     var nestedType = reflect.TypeOf((*Nested)(nil)).Elem()
//
//     type NestedInput interface {
//         pulumi.Input
//
//         ToNestedOutput() NestedOutput
//         ToNestedOutputWithContext(context.Context) NestedOutput
//     }
//
//     type Nested struct {
//         Foo int `pulumi:"foo"`
//         Bar string `pulumi:"bar"`
//     }
//
//     type NestedInputValue struct {
//         Foo pulumi.IntInput `pulumi:"foo"`
//         Bar pulumi.StringInput `pulumi:"bar"`
//     }
//
//     func (NestedInputValue) ElementType() reflect.Type {
//         return nestedType
//     }
//
//     func (v NestedInputValue) ToNestedOutput() NestedOutput {
//         return pulumi.ToOutput(v).(NestedOutput)
//     }
//
//     func (v NestedInputValue) ToNestedOutputWithContext(ctx context.Context) NestedOutput {
//         return pulumi.ToOutputWithContext(ctx, v).(NestedOutput)
//     }
//
//     type NestedOutput struct { *pulumi.Output }
//
//     func (NestedOutput) ElementType() reflect.Type {
//         return nestedType
//     }
//
//     func (o NestedOutput) ToNestedOutput() NestedOutput {
//         return o
//     }
//
//     func (o NestedOutput) ToNestedOutputWithContext(ctx context.Context) NestedOutput {
//         return o
//     }
//

var (
	anyType     = reflect.TypeOf((*interface{})(nil)).Elem()
	stringType  = reflect.TypeOf((*string)(nil)).Elem()
	idType      = reflect.TypeOf((*ID)(nil)).Elem()
	assetType   = reflect.TypeOf((*Asset)(nil)).Elem()
	archiveType = reflect.TypeOf((*Archive)(nil)).Elem()
	outputType  = reflect.TypeOf((*AnyOutput)(nil)).Elem()
)

func ToString[T any](o Output[T]) Output[string] {
	return Apply(o, func(v T) string {
		return fmt.Sprintf("%v", v)
	})
}

func await[T any](ctx context.Context, o Output[T]) (T, bool, bool, error) {
	v, known, secret, _, err := o.await(ctx)
	if !known || err != nil {
		return *new(T), known, secret, err
	}
	return v.(T), true, secret, nil
}

func convert(v interface{}, to reflect.Type) interface{} {
	rv := reflect.ValueOf(v)
	if !rv.Type().ConvertibleTo(to) {
		panic(fmt.Errorf("cannot convert output value of type %s to %s", rv.Type(), to))
	}
	return rv.Convert(to).Interface()
}
