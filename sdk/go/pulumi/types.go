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

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// Output helps encode the relationship between resources in a Pulumi application. Specifically an output property
// holds onto a value and the resource it came from. An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output interface {
	ElementType() reflect.Type

	Apply(applier func(interface{}) (interface{}, error)) AnyOutput
	ApplyWithContext(ctx context.Context, applier func(context.Context, interface{}) (interface{}, error)) AnyOutput
	ApplyT(applier interface{}) Output
	ApplyTWithContext(ctx context.Context, applier interface{}) Output

	getState() *OutputState
	dependencies() []Resource
	fulfillValue(value reflect.Value, known, secret bool, deps []Resource, err error)
	resolveValue(value reflect.Value, known, secret bool, deps []Resource)
	fulfill(value interface{}, known, secret bool, deps []Resource, err error)
	resolve(value interface{}, known, secret bool, deps []Resource)
	reject(err error)
	await(ctx context.Context) (interface{}, bool, bool, []Resource, error)
	isSecret() bool
}

var outputType = reflect.TypeOf((*Output)(nil)).Elem()
var inputType = reflect.TypeOf((*Input)(nil)).Elem()

var concreteTypeToOutputType sync.Map // map[reflect.Type]reflect.Type

// RegisterOutputType registers an Output type with the Pulumi runtime. If a value of this type's concrete type is
// returned by an Apply, the Apply will return the specific Output type.
func RegisterOutputType(output Output) {
	elementType := output.ElementType()
	existing, hasExisting := concreteTypeToOutputType.LoadOrStore(elementType, reflect.TypeOf(output))
	if hasExisting {
		panic(fmt.Errorf("an output type for %v is already registered: %v", elementType, existing))
	}
}

const (
	outputPending = iota
	outputResolved
	outputRejected
)

// OutputState holds the internal details of an Output and implements the Apply and ApplyWithContext methods.
type OutputState struct {
	mutex sync.Mutex
	cond  *sync.Cond

	state uint32 // one of output{Pending,Resolved,Rejected}

	value  interface{} // the value of this output if it is resolved.
	err    error       // the error associated with this output if it is rejected.
	known  bool        // true if this output's value is known.
	secret bool        // true if this output's value is secret

	element reflect.Type // the element type of this output.
	deps    []Resource   // the dependencies associated with this output property.
}

func (o *OutputState) elementType() reflect.Type {
	if o == nil {
		return anyType
	}
	return o.element
}

func (o *OutputState) dependencies() []Resource {
	if o == nil {
		return nil
	}
	return o.deps
}

func (o *OutputState) fulfill(value interface{}, known, secret bool, deps []Resource, err error) {
	o.fulfillValue(reflect.ValueOf(value), known, secret, deps, err)
}

func (o *OutputState) fulfillValue(value reflect.Value, known, secret bool, deps []Resource, err error) {
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

func (o *OutputState) resolve(value interface{}, known, secret bool, deps []Resource) {
	o.fulfill(value, known, secret, deps, nil)
}

func (o *OutputState) resolveValue(value reflect.Value, known, secret bool, deps []Resource) {
	o.fulfillValue(value, known, secret, deps, nil)
}

func (o *OutputState) reject(err error) {
	o.fulfill(nil, true, false, nil, err)
}

func (o *OutputState) await(ctx context.Context) (interface{}, bool, bool, []Resource, error) {
	for {
		if o == nil {
			// If the state is nil, treat its value as resolved and unknown.
			return nil, false, false, nil, nil
		}

		o.mutex.Lock()
		for o.state == outputPending {
			if ctx.Err() != nil {
				return nil, true, false, nil, ctx.Err()
			}
			o.cond.Wait()
		}
		o.mutex.Unlock()

		if !o.known || o.err != nil {
			return nil, o.known, o.secret, o.deps, o.err
		}

		// If the result is an Output, await it in turn.
		//
		// NOTE: this isn't exactly type safe! The element type of the inner output really needs to be assignable to
		// the element type of the outer output. We should reconsider this.
		ov, ok := o.value.(Output)
		if !ok {
			return o.value, true, o.secret, o.deps, nil
		}
		o = ov.getState()
	}
}

func (o *OutputState) getState() *OutputState {
	return o
}

func newOutputState(elementType reflect.Type, deps ...Resource) *OutputState {
	out := &OutputState{
		element: elementType,
		deps:    deps,
	}
	out.cond = sync.NewCond(&out.mutex)
	return out
}

var outputStateType = reflect.TypeOf((*OutputState)(nil))
var outputTypeToOutputState sync.Map // map[reflect.Type]int

func newOutput(typ reflect.Type, deps ...Resource) Output {
	contract.Assert(typ.Implements(outputType))

	// All values that implement Output must embed a field of type `*OutputState` by virtue of the unexported
	// `isOutput` method. If we yet haven't recorded the index of this field for the ouptut type `typ`, find and
	// record it.
	outputFieldV, ok := outputTypeToOutputState.Load(typ)
	if !ok {
		outputField := -1
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Anonymous && f.Type == outputStateType {
				outputField = i
				break
			}
		}
		contract.Assert(outputField != -1)
		outputTypeToOutputState.Store(typ, outputField)
		outputFieldV = outputField
	}

	// Create the new output.
	output := reflect.New(typ).Elem()
	state := newOutputState(output.Interface().(Output).ElementType(), deps...)
	output.Field(outputFieldV.(int)).Set(reflect.ValueOf(state))
	return output.Interface().(Output)
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called. This acts like a promise.
func NewOutput() (Output, func(interface{}), func(error)) {
	out := newOutputState(anyType)

	resolve := func(v interface{}) {
		out.resolve(v, true, false, nil)
	}
	reject := func(err error) {
		out.reject(err)
	}

	return AnyOutput{out}, resolve, reject
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
		panic(errors.New("appplier must return exactly one or two values"))
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
		panic(errors.New("appplier must return exactly one or two values"))
	}

	// Okay
	return fv
}

// Apply transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
func (o *OutputState) Apply(applier func(interface{}) (interface{}, error)) AnyOutput {
	return o.ApplyWithContext(context.Background(), func(_ context.Context, v interface{}) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
func (o *OutputState) ApplyWithContext(ctx context.Context, applier func(context.Context, interface{}) (interface{}, error)) AnyOutput {
	return o.ApplyTWithContext(ctx, applier).(AnyOutput)
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
func (o *OutputState) ApplyT(applier interface{}) Output {
	return o.ApplyTWithContext(context.Background(), makeContextful(applier, o.elementType()))
}

var anyOutputType = reflect.TypeOf((*AnyOutput)(nil)).Elem()

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
func (o *OutputState) ApplyTWithContext(ctx context.Context, applier interface{}) Output {
	fn := checkApplier(applier, o.elementType())

	resultType := anyOutputType
	if ot, ok := concreteTypeToOutputType.Load(fn.Type().Out(0)); ok {
		resultType = ot.(reflect.Type)
	}

	result := newOutput(resultType, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.await(ctx)
		if err != nil || !known {
			result.fulfill(nil, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		val := reflect.ValueOf(v)
		if !val.IsValid() {
			val = reflect.Zero(o.elementType())
		}
		results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), val})
		if len(results) == 2 && !results[1].IsNil() {
			result.reject(results[1].Interface().(error))
			return
		}

		// Fulfill the result.
		result.fulfillValue(results[0], true, secret, deps, nil)
	}()
	return result
}

// isSecret returns a bool representing the secretness of the Output
func (o *OutputState) isSecret() bool {
	return o.getState().secret
}

// ToSecret wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecret(input interface{}) Output {
	return ToSecretWithContext(context.Background(), input)
}

// ToSecretWithContext wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecretWithContext(ctx context.Context, input interface{}) Output {
	o := toOutputWithContext(ctx, input, true)
	// set immediate secretness ahead of resolution/fufillment
	o.getState().secret = true
	return o
}

// All returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each element of the
// array will contain the resolved value of the corresponding output. The output will be rejected if any of the inputs
// is rejected.
func All(inputs ...interface{}) ArrayOutput {
	return AllWithContext(context.Background(), inputs...)
}

// AllWithContext returns an ArrayOutput that will resolve when all of the provided inputs will resolve. Each
// element of the array will contain the resolved value of the corresponding output. The output will be rejected if any
// of the inputs is rejected.
func AllWithContext(ctx context.Context, inputs ...interface{}) ArrayOutput {
	return ToOutputWithContext(ctx, inputs).(ArrayOutput)
}

func gatherDependencies(v interface{}) []Resource {
	if v == nil {
		return nil
	}

	depSet := make(map[Resource]struct{})
	gatherDependencySet(reflect.ValueOf(v), depSet)

	if len(depSet) == 0 {
		return nil
	}

	deps := make([]Resource, 0, len(depSet))
	for d := range depSet {
		deps = append(deps, d)
	}
	return deps
}

var resourceType = reflect.TypeOf((*Resource)(nil)).Elem()

func gatherDependencySet(v reflect.Value, deps map[Resource]struct{}) {
	for {
		// Check for an Output that we can pull dependencies off of.
		if v.Type().Implements(outputType) && v.CanInterface() {
			output := v.Convert(outputType).Interface().(Output)
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
				gatherDependencySet(v.Field(i), deps)
			}
		case reflect.Array, reflect.Slice:
			l := v.Len()
			for i := 0; i < l; i++ {
				gatherDependencySet(v.Index(i), deps)
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				gatherDependencySet(iter.Key(), deps)
				gatherDependencySet(iter.Value(), deps)
			}
		}
		return
	}
}

func checkToOutputMethod(m reflect.Value, outputType reflect.Type) bool {
	if !m.IsValid() {
		return false
	}
	mt := m.Type()
	if mt.NumIn() != 1 || mt.In(0) != contextType {
		return false
	}
	return mt.NumOut() == 1 && mt.Out(0) == outputType
}

func callToOutputMethod(ctx context.Context, input reflect.Value, resolvedType reflect.Type) (Output, bool) {
	ot, ok := concreteTypeToOutputType.Load(resolvedType)
	if !ok {
		return nil, false
	}
	outputType := ot.(reflect.Type)

	toOutputMethodName := "To" + outputType.Name() + "WithContext"
	toOutputMethod := input.MethodByName(toOutputMethodName)
	if !checkToOutputMethod(toOutputMethod, outputType) {
		return nil, false
	}

	return toOutputMethod.Call([]reflect.Value{reflect.ValueOf(ctx)})[0].Interface().(Output), true
}

func awaitInputs(ctx context.Context, v, resolved reflect.Value) (bool, bool, []Resource, error) {
	contract.Assert(v.IsValid())

	if !resolved.CanSet() {
		return true, false, nil, nil
	}

	// If the value is an Input with of a different element type, turn it into an Output of the appropriate type and
	// await it.
	valueType, isInput := v.Type(), false
	if v.CanInterface() && valueType.Implements(inputType) {
		input, isNonNil := v.Interface().(Input)
		if !isNonNil {
			// A nil input is already fully-resolved.
			return true, false, nil, nil
		}

		valueType = input.ElementType()
		assignInput := false

		// If the element type of the input is not identical to the type of the destination and the destination is not
		// the any type (i.e. interface{}), attempt to convert the input to the appropriately-typed output.
		if valueType != resolved.Type() && resolved.Type() != anyType {
			if newOutput, ok := callToOutputMethod(ctx, reflect.ValueOf(input), resolved.Type()); ok {
				// We were able to convert the input. Use the result as the new input value.
				input = newOutput
			} else if !valueType.AssignableTo(resolved.Type()) {
				// If the value type is not assignable to the destination, see if we can assign the input value itself
				// to the destination.
				if !v.Type().AssignableTo(resolved.Type()) {
					panic(fmt.Errorf("cannot convert an input of type %T to a value of type %v",
						input, resolved.Type()))
				} else {
					assignInput = true
				}
			}
		}

		// If the input is an Output, await its value. The returned value is fully resolved.
		if output, ok := input.(Output); ok {
			e, known, secret, deps, err := output.await(ctx)
			if err != nil || !known {
				return known, secret, deps, err
			}
			if !assignInput {
				val := reflect.ValueOf(e)
				if !val.IsValid() {
					val = reflect.Zero(output.ElementType())
				}
				resolved.Set(val)
			} else {
				resolved.Set(reflect.ValueOf(input))
			}
			return true, secret, deps, nil
		}

		// Check for types that are already fully-resolved.
		if v, ok := getResolvedValue(input); ok {
			resolved.Set(v)
			return true, false, nil, nil
		}

		v, isInput = reflect.ValueOf(input), true

		// We require that the kind of an `Input`'s `ElementType` agrees with the kind of the `Input`'s underlying value.
		// This requirement is trivially (and unintentionally) violated by `*T` if `*T` does not define `ElementType`,
		// but `T` does (https://golang.org/ref/spec#Method_sets).
		// In this case, dereference the pointer to get at its actual value.
		if v.Kind() == reflect.Ptr && valueType.Kind() != reflect.Ptr {
			v = v.Elem()
			contract.Assert(v.Interface().(Input).ElementType() == valueType)
		}

		// If we are assigning the input value itself, update the value type.
		if assignInput {
			valueType = v.Type()
		} else {
			// Handle pointer inputs.
			if v.Kind() == reflect.Ptr {
				v, valueType = v.Elem(), valueType.Elem()

				resolved.Set(reflect.New(resolved.Type().Elem()))
				resolved = resolved.Elem()
			}
		}
	}

	contract.Assert(valueType.AssignableTo(resolved.Type()))

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

// ToOutput returns an Output that will resolve when all Inputs contained in the given value have resolved.
func ToOutput(v interface{}) Output {
	return ToOutputWithContext(context.Background(), v)
}

// ToOutputWithContext returns an Output that will resolve when all Outputs contained in the given value have
// resolved.
func ToOutputWithContext(ctx context.Context, v interface{}) Output {
	return toOutputWithContext(ctx, v, false)
}

func toOutputWithContext(ctx context.Context, v interface{}, forceSecret bool) Output {
	resolvedType := reflect.TypeOf(v)
	if input, ok := v.(Input); ok {
		resolvedType = input.ElementType()
	}

	resultType := anyOutputType
	if ot, ok := concreteTypeToOutputType.Load(resolvedType); ok {
		resultType = ot.(reflect.Type)
	}

	result := newOutput(resultType, gatherDependencies(v)...)
	go func() {
		if v == nil {
			result.fulfill(nil, true, false, nil, nil)
			return
		}

		element := reflect.New(resolvedType).Elem()

		known, secret, deps, err := awaitInputs(ctx, reflect.ValueOf(v), element)
		secret = secret || forceSecret
		if err != nil || !known {
			result.fulfill(nil, known, secret, deps, err)
			return
		}

		result.resolveValue(element, true, secret, deps)
	}()
	return result
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
//     type NestedOutput struct { *pulumi.OutputState }
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
type Input interface {
	ElementType() reflect.Type
}

var anyType = reflect.TypeOf((*interface{})(nil)).Elem()

func Any(v interface{}) AnyOutput {
	return AnyWithContext(context.Background(), v)
}

func AnyWithContext(ctx context.Context, v interface{}) AnyOutput {
	// Return an output that resolves when all nested inputs have resolved.
	out := newOutput(anyOutputType, gatherDependencies(v)...)
	go func() {
		if v == nil {
			out.fulfill(nil, true, false, nil, nil)
			return
		}
		var result interface{}
		known, secret, deps, err := awaitInputs(ctx, reflect.ValueOf(v), reflect.ValueOf(&result).Elem())
		out.fulfill(result, known, secret, deps, err)
	}()
	return out.(AnyOutput)
}

type AnyOutput struct{ *OutputState }

func (AnyOutput) ElementType() reflect.Type {
	return anyType
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
	id, known, secret, _, err := o.await(ctx)
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
	id, known, secret, _, err := o.await(ctx)
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

// TODO: ResourceOutput and the init() should probably be code generated.
// ResourceOutput is an Output that returns Resource values.
type ResourceOutput struct{ *OutputState }

// ElementType returns the element type of this Output (Resource).
func (ResourceOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Resource)(nil)).Elem()
}

func init() {
	RegisterOutputType(ResourceOutput{})
}
