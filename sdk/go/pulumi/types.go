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
	"runtime"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Output helps encode the relationship between resources in a Pulumi application. Specifically an output property
// holds onto a value and the resource it came from. An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output interface {
	ElementType() reflect.Type

	ApplyT(applier interface{}) Output
	ApplyTWithContext(ctx context.Context, applier interface{}) Output

	getState() *OutputState
}

var (
	outputType = reflect.TypeOf((*Output)(nil)).Elem()
	inputType  = reflect.TypeOf((*Input)(nil)).Elem()
)

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

var inputInterfaceTypeToConcreteType sync.Map // map[reflect.Type]reflect.Type

// RegisterInputType registers an Input type with the Pulumi runtime. This allows the input type to be instantiated
// for a given input interface.
func RegisterInputType(interfaceType reflect.Type, input Input) {
	if interfaceType.Kind() != reflect.Interface {
		panic(fmt.Errorf("expected %v to be an interface", interfaceType))
	}
	if !interfaceType.Implements(inputType) {
		panic(fmt.Errorf("expected %v to implement %v", interfaceType, inputType))
	}
	concreteType := reflect.TypeOf(input)
	if !concreteType.Implements(interfaceType) {
		panic(fmt.Errorf("expected %v to implement interface %v", concreteType, interfaceType))
	}
	existing, hasExisting := inputInterfaceTypeToConcreteType.LoadOrStore(interfaceType, concreteType)
	if hasExisting {
		panic(fmt.Errorf("an input type for %v is already registered: %v", interfaceType, existing))
	}
}

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

// OutputState holds the internal details of an Output and implements the Apply and ApplyWithContext methods.
type OutputState struct {
	cond *sync.Cond

	join *workGroup // the wait group associated with this output, if any.

	state uint32 // one of output{Pending,Resolved,Rejected}

	value  interface{} // the value of this output if it is resolved.
	err    error       // the error associated with this output if it is rejected.
	known  bool        // true if this output's value is known.
	secret bool        // true if this output's value is secret

	element reflect.Type // the element type of this output.
	deps    []Resource   // the dependencies associated with this output property.
}

func getOutputState(v reflect.Value) (*OutputState, bool) {
	if !v.IsValid() || !v.CanInterface() {
		return nil, false
	}
	out, ok := v.Interface().(Output)
	if !ok {
		return nil, false
	}
	return out.getState(), true
}

func (o *OutputState) elementType() reflect.Type {
	if o == nil {
		return anyType
	}
	return o.element
}

// Fetch the dependencies of an OutputState. It is not thread-safe to mutate values inside
// returned slice.
func (o *OutputState) dependencies() []Resource {
	if o == nil {
		return nil
	}
	o.cond.L.Lock()
	defer o.cond.L.Unlock()
	return o.deps
}

func (o *OutputState) fulfill(value interface{}, known, secret bool, deps []Resource, err error) {
	o.fulfillValue(reflect.ValueOf(value), known, secret, deps, err)
}

func (o *OutputState) fulfillValue(value reflect.Value, known, secret bool, deps []Resource, err error) {
	if o == nil {
		return
	}

	o.cond.L.Lock()
	defer func() {
		o.cond.L.Unlock()
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
		if other, ok := getOutputState(value); ok && other.join != o.join {
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
		o.deps = mergeDependencies(o.deps, deps)
	}
}

func mergeDependencies(ours []Resource, theirs []Resource) []Resource {
	if len(ours) == 0 && len(theirs) == 0 {
		return nil
	} else if len(theirs) == 0 {
		return append(slice.Prealloc[Resource](len(ours)), ours...)
	} else if len(ours) == 0 {
		return append(slice.Prealloc[Resource](len(theirs)), theirs...)
	}
	depSet := make(map[Resource]struct{})
	mergedDeps := slice.Prealloc[Resource](len(ours) + len(theirs))
	for _, d := range ours {
		depSet[d] = struct{}{}
	}
	for _, d := range theirs {
		depSet[d] = struct{}{}
	}
	for d := range depSet {
		mergedDeps = append(mergedDeps, d)
	}
	return mergedDeps
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

// awaitOnce is a single iteration of the "await" loop, using the condition variable as a lock to
// guard accessing the fields to avoid tearing reads and writes.
func (o *OutputState) awaitOnce(ctx context.Context) (interface{}, bool, bool, []Resource, error) {
	if o == nil {
		// If the state is nil, treat its value as resolved and unknown.
		return nil, false, false, nil, nil
	}

	o.cond.L.Lock()
	defer o.cond.L.Unlock()
	for o.state == outputPending {
		if ctx.Err() != nil {
			return nil, true, false, nil, ctx.Err()
		}
		o.cond.Wait()
	}

	return o.value, o.known, o.secret, o.deps, o.err
}

func (o *OutputState) await(ctx context.Context) (interface{}, bool, bool, []Resource, error) {
	known := true
	secret := false
	var deps []Resource

	for {
		v, k, s, d, err := o.awaitOnce(ctx)
		value := v
		known = known && k
		secret = secret || s
		deps = mergeDependencies(deps, d)
		if !known || err != nil {
			return nil, known, secret, deps, err
		}

		// If the result is an Output, await it in turn.
		//
		// NOTE: this isn't exactly type safe! The element type of the inner output really needs to be assignable to
		// the element type of the outer output. We should reconsider this.
		if ov, ok := value.(Output); ok {
			o = ov.getState()
		} else {
			return value, known, secret, deps, nil
		}
	}
}

func (o *OutputState) getState() *OutputState {
	return o
}

func newOutputState(join *workGroup, elementType reflect.Type, deps ...Resource) *OutputState {
	if deps == nil && len(deps) != 0 {
		panic(fmt.Sprintf("data race detected - please report to https://github.com/pulumi/pulumi/issues: deps is nil with len %d", len(deps)))
	}

	if join != nil {
		join.Add(1)
	}

	var m sync.Mutex
	out := &OutputState{
		join:    join,
		element: elementType,
		deps:    deps,
		// Note: Calling registerResource or readResource with the same resource state can report a
		// spurious data race here. See note in https://github.com/pulumi/pulumi/pull/10081.
		//
		// To reproduce, revert changes in PR to file pkg/engine/lifecycletest/golang_sdk_test.go.
		cond: sync.NewCond(&m),
	}
	return out
}

var (
	outputStateType         = reflect.TypeOf((*OutputState)(nil))
	outputTypeToOutputState sync.Map // map[reflect.Type]int
)

func newOutput(wg *workGroup, typ reflect.Type, deps ...Resource) Output {
	contract.Requiref(typ.Implements(outputType), "type", "type %v does not implement Output", typ)

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
		contract.Assertf(outputField != -1, "type %v does not embed an OutputState field", typ)
		outputTypeToOutputState.Store(typ, outputField)
		outputFieldV = outputField
	}

	// Create the new output.
	output := reflect.New(typ).Elem()
	state := newOutputState(wg, output.Interface().(Output).ElementType(), deps...)
	output.Field(outputFieldV.(int)).Set(reflect.ValueOf(state))
	return output.Interface().(Output)
}

func newAnyOutput(wg *workGroup) (Output, func(interface{}), func(error)) {
	out := newOutputState(wg, anyType)

	resolve := func(v interface{}) {
		out.resolve(v, true, false, nil)
	}
	reject := func(err error) {
		out.getState().reject(err)
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

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// applier is a normalized version of a function
// passed into either ApplyT or ApplyTWithContext.
//
// Use its Call method instead of calling the fn directly.
type applier struct {
	// Out is the type of output produced by this applier.
	Out reflect.Type

	fn  reflect.Value
	ctx bool // whether fn accepts a context as its first input
	err bool // whether fn return an err as its last result

	// This is non-nil if the input value should be converted
	// with Value.Convert first.
	convertTo reflect.Type
}

func newApplier(fn interface{}, elemType reflect.Type) (_ *applier, err error) {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return nil, errors.New("applier must be a function")
	}

	defer func() {
		// The named return above is necessary
		// to augment the error message in a defer.
		if err == nil {
			return
		}

		f := runtime.FuncForPC(fv.Pointer())
		// Defensively guard against the possibility that
		// fv.Pointer returns an invalid program counter.
		// This will never happen in practice.
		if f == nil {
			return
		}

		file, line := f.FileLine(f.Entry())
		err = fmt.Errorf("%w\napplier defined at %v:%v", err, file, line)
	}()

	ap := applier{fn: fv}
	ft := fv.Type()

	// The function parameters must be in one of the following forms:
	//	(E)
	//	(context.Context, E)
	// Everything else is invalid.
	var elemIdx int
	elemName := "first"
	switch numIn := ft.NumIn(); numIn {
	case 2:
		if t := ft.In(0); !contextType.AssignableTo(t) {
			return nil, fmt.Errorf("applier's first input parameter must be assignable from %v, got %v", contextType, t)
		}
		ap.ctx = true
		elemIdx = 1
		elemName = "second"
		fallthrough // validate element type
	case 1:
		switch t := ft.In(elemIdx); {
		case elemType.AssignableTo(t):
			// Do nothing.
		case elemType.ConvertibleTo(t) && elemType.Kind() == t.Kind():
			// We only support coercion if the types are the same kind.
			//
			// Types with different internal representations
			// do not coerce for "free"
			// (e.g. string([]byte{..}) allocates)
			// and may not match user expectations
			// (e.g. string(42) is "*", not "42"),
			// so we reject those.
			ap.convertTo = t
		default:
			return nil, fmt.Errorf("applier's %s input parameter must be assignable from %v, got %v", elemName, elemType, t)
		}
	default:
		return nil, fmt.Errorf("applier must accept exactly one or two parameters, got %d", numIn)
	}

	// The function results must be in one of the following forms:
	//	(O)
	//	(O, error)
	// Everything else is invalid.
	switch numOut := ft.NumOut(); numOut {
	case 2:
		if t := ft.Out(1); !t.AssignableTo(errorType) {
			return nil, fmt.Errorf("applier's second return type must be assignable to error, got %v", t)
		}
		ap.err = true
		fallthrough // extract output type
	case 1:
		ap.Out = ft.Out(0)
	default:
		return nil, fmt.Errorf("applier must return exactly one or two values, got %d", numOut)
	}

	return &ap, nil
}

// Call executes the applier on the provided value and returns the result.
func (ap *applier) Call(ctx context.Context, in reflect.Value) (reflect.Value, error) {
	args := slice.Prealloc[reflect.Value](2) // ([ctx], in)
	if ap.ctx {
		args = append(args, reflect.ValueOf(ctx))
	}
	if ap.convertTo != nil {
		in = in.Convert(ap.convertTo)
	}
	args = append(args, in)

	var (
		out reflect.Value
		err error
	)
	results := ap.fn.Call(args)
	out = results[0]
	if ap.err {
		// Using the 'x, ok' form for cast here
		// gracefully handles the case when results[1]
		// is nil.
		err, _ = results[1].Interface().(error)
	}

	return out, err
}

// ApplyT transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
//
// The applier function must have one of the following signatures:
//
//	func (v U) T
//	func (v U) (T, error)
//
// U must be assignable from the ElementType of the Output. If T is a type that has a registered Output type, the
// result of ApplyT will be of the registered Output type, and can be used in an appropriate type assertion:
//
//	stringOutput := pulumi.String("hello").ToStringOutput()
//	intOutput := stringOutput.ApplyT(func(v string) int {
//	    return len(v)
//	}).(pulumi.IntOutput)
//
// Otherwise, the result will be of type AnyOutput:
//
//	stringOutput := pulumi.String("hello").ToStringOutput()
//	intOutput := stringOutput.ApplyT(func(v string) []rune {
//	    return []rune(v)
//	}).(pulumi.AnyOutput)
func (o *OutputState) ApplyT(applier interface{}) Output {
	ap, err := newApplier(applier, o.elementType())
	if err != nil {
		panic(err)
	}
	return o.applyTWithApplier(context.Background(), ap)
}

var anyOutputType = reflect.TypeOf((*AnyOutput)(nil)).Elem()

// ApplyTWithContext transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
// The provided context can be used to reject the output as canceled.
//
// The applier function must have one of the following signatures:
//
//	func (ctx context.Context, v U) T
//	func (ctx context.Context, v U) (T, error)
//
// U must be assignable from the ElementType of the Output. If T is a type that has a registered Output type, the
// result of ApplyT will be of the registered Output type, and can be used in an appropriate type assertion:
//
//	stringOutput := pulumi.String("hello").ToStringOutput()
//	intOutput := stringOutput.ApplyTWithContext(func(_ context.Context, v string) int {
//	    return len(v)
//	}).(pulumi.IntOutput)
//
// Otherwise, the result will be of type AnyOutput:
//
//	stringOutput := pulumi.String("hello").ToStringOutput()
//	intOutput := stringOutput.ApplyT(func(_ context.Context, v string) []rune {
//	    return []rune(v)
//	}).(pulumi.AnyOutput)
func (o *OutputState) ApplyTWithContext(ctx context.Context, applier interface{}) Output {
	ap, err := newApplier(applier, o.elementType())
	if err != nil {
		panic(err)
	}
	return o.applyTWithApplier(ctx, ap)
}

func (o *OutputState) applyTWithApplier(ctx context.Context, ap *applier) Output {
	resultType := anyOutputType
	applierReturnType := ap.Out

	if ot, ok := concreteTypeToOutputType.Load(applierReturnType); ok {
		resultType = ot.(reflect.Type)
	} else if applierReturnType.Implements(outputType) {
		resultType = applierReturnType
	} else if applierReturnType.Implements(inputType) {
		if ct, ok := inputInterfaceTypeToConcreteType.Load(applierReturnType); ok {
			applierReturnType = ct.(reflect.Type)
		}

		if applierReturnType.Kind() != reflect.Interface {
			unwrappedType := reflect.New(applierReturnType).Interface().(Input).ElementType()
			if ot, ok := concreteTypeToOutputType.Load(unwrappedType); ok {
				resultType = ot.(reflect.Type)
			}
		}
	}

	result := newOutput(o.join, resultType, o.dependencies()...)
	go func() {
		v, known, secret, deps, err := o.getState().await(ctx)
		if err != nil || !known {
			result.getState().fulfill(nil, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		val := reflect.ValueOf(v)
		if !val.IsValid() {
			val = reflect.Zero(o.elementType())
		}

		out, err := ap.Call(ctx, val)
		if err != nil {
			result.getState().reject(err)
			return
		}
		var fulfilledDeps []Resource
		fulfilledDeps = append(fulfilledDeps, deps...)
		if resultOutput, ok := out.Interface().(Output); ok {
			fulfilledDeps = append(fulfilledDeps, resultOutput.getState().dependencies()...)
		}
		// Fulfill the result.
		result.getState().fulfillValue(out, true, secret, fulfilledDeps, nil)
	}()
	return result
}

// IsSecret returns a bool representing the secretness of the Output
//
// IsSecret may return an inaccurate results if the Output is unknowable (during a
// preview) or contains an error.
func IsSecret(o Output) bool {
	_, _, secret, _, _ := o.getState().await(context.Background())
	// We intentionally ignore both the `known` and `error` values returned by `await`:
	//
	// If a value is not known, it is possible that we will return the wrong result. This
	// is unavoidable. Consider the example:
	//
	// ```go
	// bucket, _ := s3.Bucket("bucket", &s3.BucketArgs{})
	// unknowable := bucket.Bucket.ApplyT(func(b string) OutputString {
	//   if strings.ContainsRune(b, '9') {
	//     return ToSecret(String(b))
	//   else {
	//     return String(b)
	//   }
	// })
	// ```
	//
	// Until we resolve values from the cloud, we can't know the correct value of
	// `IsSecret(unknowable)`. We have the same problem for outputs with non-nil errors.
	//
	// This is tolerable because users will never be able to retrieve values (secret or
	// otherwise) that are unknown or erred.
	return secret
}

// Unsecret will unwrap a secret output as a new output with a resolved value and no secretness
func Unsecret(input Output) Output {
	return UnsecretWithContext(context.Background(), input)
}

// UnsecretWithContext will unwrap a secret output as a new output with a resolved value and no secretness
func UnsecretWithContext(ctx context.Context, input Output) Output {
	secret := false
	o := toOutputWithContext(ctx, input.getState().join, input, &secret)
	return o
}

// ToSecret wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecret(input interface{}) Output {
	return ToSecretWithContext(context.Background(), input)
}

// UnsafeUnknownOutput Creates an unknown output. This is a low level API and should not be used in programs as this
// will cause "pulumi up" to fail if called and used during a non-dryrun deployment.
func UnsafeUnknownOutput(deps []Resource) Output {
	output, _, _ := NewOutput()
	output.getState().resolve(nil, false, false, deps)
	return output
}

// ToSecretWithContext wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecretWithContext(ctx context.Context, input interface{}) Output {
	x := true
	o := toOutputWithContext(ctx, nil, input, &x)
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

// JSONMarshal uses "encoding/json".Marshal to serialize the given Output value into a JSON string.
func JSONMarshal(v interface{}) StringOutput {
	return JSONMarshalWithContext(context.Background(), v)
}

// JSONMarshalWithContext uses "encoding/json".Marshal to serialize the given Output value into a JSON string.
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

func gatherJoins(v interface{}) workGroups {
	if v == nil {
		return nil
	}

	joinSet := make(map[*workGroup]struct{})
	gatherJoinSet(reflect.ValueOf(v), joinSet)

	var joins workGroups
	if len(joinSet) > 0 {
		joins = slice.Prealloc[*workGroup](len(joinSet))
		for j := range joinSet {
			joins = append(joins, j)
		}
	}

	return joins
}

var resourceType = reflect.TypeOf((*Resource)(nil)).Elem()

func gatherJoinSet(v reflect.Value, joins map[*workGroup]struct{}) {
	for {
		// Check for an Output that we can pull dependencies off of.
		if v.Type().Implements(outputType) && v.CanInterface() {
			output := v.Convert(outputType).Interface().(Output)
			if join := output.getState().join; join != nil {
				joins[join] = struct{}{}
			}
			return
		}
		// Check for an actual Resource.
		if v.Type().Implements(resourceType) {
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
				gatherJoinSet(v.Field(i), joins)
			}
		case reflect.Array, reflect.Slice:
			l := v.Len()
			for i := 0; i < l; i++ {
				gatherJoinSet(v.Index(i), joins)
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				gatherJoinSet(iter.Key(), joins)
				gatherJoinSet(iter.Value(), joins)
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

// awaitInputs recursively discovers the Inputs in a value, awaits them, and sets resolved to the result of the await.
// It is essentially an attempt to port the logic in the NodeJS SDK's `pulumi.output` function, which takes a value and
// returns its fully-resolved value. The fully-resolved value `W` of some value `V` has the same shape as `V`, but with
// all outputs recursively replaced with their resolved values. Unforunately, the way Outputs are represented in Go
// combined with Go's strong typing and relatively simplistic type system make this challenging.
//
// The logic to do this is pretty arcane, and very special-casey when it comes to finding Inputs, converting them to
// Outputs, and awaiting their values. Roughly speaking:
//
//  1. If we cannot set resolved--e.g. because it was derived from an unexported field--we do nothing
//  2. If the value is an Input:
//     a. If the value is `nil`, do nothing. The value is already fully-resolved. `resolved` is not set.
//     b. Otherwise, convert the Input to an appropriately-typed Output by calling the corresponding `ToOutput` method.
//     The desired type is determined based on the type of the destination, and the conversion method is determined
//     from the name of the desired type. If no conversion method is available, we will attempt to assign the Input
//     itself, and will panic if that assignment is not well-typed.
//     c. Replace the value to await with the resolved value of the input.
//  3. Depending on the kind of the value:
//     a. If the value is a Resource, stop.
//     b. If the value is a primitive, stop.
//     c. If the value is a slice, array, struct, or map, recur on its contents.
func awaitInputs(ctx context.Context, v, resolved reflect.Value) (bool, bool, []Resource, error) {
	contract.Requiref(v.IsValid(), "v", "must be valid")

	if !resolved.CanSet() {
		return true, false, nil, nil
	}

	// If the value is an Input with of a different element type, turn it into an Output of the appropriate type and
	// await it.
	valueType, isInput := v.Type(), false
	if v.CanInterface() && valueType.Implements(inputType) {
		input, ok := v.Interface().(Input)
		if !ok {
			// A non-input type is already fully-resolved.
			return true, false, nil, nil
		}
		if val := reflect.ValueOf(input); val.Kind() == reflect.Ptr && val.IsNil() {
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
			e, known, secret, deps, err := output.getState().await(ctx)
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
			elemType := v.Interface().(Input).ElementType()
			contract.Assertf(elemType == valueType, "input element type must be %v, got %v", valueType, elemType)
		}

		// If we are assigning the input value itself, update the value type.
		if assignInput {
			valueType = v.Type()
		} else {
			// Handle pointer inputs.
			if v.Kind() == reflect.Ptr && !v.Type().Implements(resourceType) {
				v = v.Elem()
				valueType = valueType.Elem()
				if resolved.Type() != anyType {
					// resolved should be some pointer type U such that value Type is convertable to U.
					resolved.Set(reflect.New(resolved.Type().Elem()))
					resolved = resolved.Elem()
				} else {
					// Allocate storage for a pointer and assign that to resolved, then continue below with resolved set to the inner value of the pointer just allocated
					ptr := reflect.New(valueType)
					resolved.Set(ptr)
					resolved = ptr.Elem()
				}
			}
		}
	}

	contract.Assertf(valueType.AssignableTo(resolved.Type()), "%s not assignable to %s", valueType.String(), resolved.Type().String())

	if v.Type().Implements(resourceType) {
		resolved.Set(v)
		return true, false, nil, nil
	}

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

func toOutputTWithContext(ctx context.Context, join *workGroup, outputType reflect.Type, v interface{}, result reflect.Value, forceSecretVal *bool) Output {
	// forceSecretVal enables ensuring the value is marked secret before the secret field of the
	// output could be observed (read: raced) by any user of the returned Output prior to awaiting.
	joins := gatherJoins(v)

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

	output := newOutput(join, outputType)
	if forceSecretVal != nil {
		output.getState().secret = *forceSecretVal
	}
	go func() {
		defer done()

		if v == nil {
			output.getState().fulfill(nil, true, false, nil, nil)
			return
		}

		known, secret, deps, err := awaitInputs(ctx, reflect.ValueOf(v), result)
		if forceSecretVal != nil {
			secret = *forceSecretVal
		}
		if err != nil || !known {
			output.getState().fulfill(nil, known, secret, deps, err)
			return
		}
		output.getState().resolveValue(result, true, secret, deps)
	}()
	return output
}

// ToOutput returns an Output that will resolve when all Inputs contained in the given value have resolved.
func ToOutput(v interface{}) Output {
	return ToOutputWithContext(context.Background(), v)
}

// ToOutputWithContext returns an Output that will resolve when all Outputs contained in the given value have
// resolved.
func ToOutputWithContext(ctx context.Context, v interface{}) Output {
	return toOutputWithContext(ctx, nil, v, nil)
}

func toOutputWithContext(ctx context.Context, join *workGroup, v interface{}, forceSecretVal *bool) Output {
	resultType := reflect.TypeOf(v)
	if input, ok := v.(Input); ok {
		resultType = input.ElementType()
	}
	var result reflect.Value
	if v != nil {
		result = reflect.New(resultType).Elem()
	}

	outputType := anyOutputType
	if ot, ok := concreteTypeToOutputType.Load(resultType); ok {
		outputType = ot.(reflect.Type)
	}

	return toOutputTWithContext(ctx, join, outputType, v, result, forceSecretVal)
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
type Input interface {
	ElementType() reflect.Type
}

var anyType = reflect.TypeOf((*interface{})(nil)).Elem()

func Any(v interface{}) AnyOutput {
	return AnyWithContext(context.Background(), v)
}

func AnyWithContext(ctx context.Context, v interface{}) AnyOutput {
	return anyWithContext(ctx, nil, v)
}

func anyWithContext(ctx context.Context, join *workGroup, v interface{}) AnyOutput {
	var result interface{}
	return toOutputTWithContext(ctx, join, anyOutputType, v, reflect.ValueOf(&result).Elem(), nil).(AnyOutput)
}

type AnyOutput struct{ *OutputState }

func (AnyOutput) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("Outputs can not be marshaled to JSON")
}

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

// ResourceOutput is an Output that returns Resource values.
// TODO: ResourceOutput and the init() should probably be code generated.
type ResourceOutput struct{ *OutputState }

func (ResourceOutput) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("Outputs can not be marshaled to JSON")
}

// ElementType returns the element type of this Output (Resource).
func (ResourceOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Resource)(nil)).Elem()
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

// ElementType returns the element type of this Input ([]Resource).
func (ResourceArray) ElementType() reflect.Type {
	return resourceArrayType
}

func (in ResourceArray) ToResourceArrayOutput() ResourceArrayOutput {
	return ToOutput(in).(ResourceArrayOutput)
}

func (in ResourceArray) ToResourceArrayOutputWithContext(ctx context.Context) ResourceArrayOutput {
	return ToOutputWithContext(ctx, in).(ResourceArrayOutput)
}

// ResourceArrayOutput is an Output that returns []Resource values.
type ResourceArrayOutput struct{ *OutputState }

func (ResourceArrayOutput) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("Outputs can not be marshaled to JSON")
}

// ElementType returns the element type of this Output ([]Resource).
func (ResourceArrayOutput) ElementType() reflect.Type {
	return resourceArrayType
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
