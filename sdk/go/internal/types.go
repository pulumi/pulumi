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

//nolint:lll, interfacer
package internal

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// AnyOutputType is the reflected type of pulumi.AnyOutput.
//
// This type is set by the pulumi package at init().
var AnyOutputType reflect.Type

// FullyResolvedTypes is a collection of Input types
// that are known to be fully resolved and do not need to be awaited.
//
// This map is filled by the pulumi package at init().
var FullyResolvedTypes = make(map[reflect.Type]struct{})

// Output encodes the relationship between resources in a Pulumi
// application. See pulumi.Output for more details.
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

type workGroups []*WorkGroup

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

// OutputStatus is an enum defining
// the possible states of an Output.
type OutputStatus uint32

// States that an Output can be in.
const (
	OutputPending OutputStatus = iota
	OutputResolved
	OutputRejected
)

// OutputState holds the internal details of an Output.
type OutputState struct {
	cond *sync.Cond

	join *WorkGroup // the wait group associated with this output, if any.

	state OutputStatus // one of Output{Pending,Resolved,Rejected}

	value  interface{} // the value of this output if it is resolved.
	err    error       // the error associated with this output if it is rejected.
	known  bool        // true if this output's value is known.
	secret bool        // true if this output's value is secret

	element reflect.Type // the element type of this output.

	// The dependencies associated with this output property.
	// This is a []pulumi.Resource, but we can't use that type here because
	// it would create a circular dependency.
	deps []Resource
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

	if o.state != OutputPending {
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
		if other, ok := getOutputState(value); ok && other != nil && other.join != o.join {
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
		o.state, o.err, o.known, o.secret = OutputRejected, err, true, secret
	} else {
		if value.IsValid() {
			reflect.ValueOf(&o.value).Elem().Set(value)
		}
		o.state, o.known, o.secret = OutputResolved, known, secret

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
	for o.state == OutputPending {
		if ctx.Err() != nil {
			return nil, true, false, nil, ctx.Err()
		}
		o.cond.Wait()
	}

	return o.value, o.known, o.secret, o.deps, o.err
}

func (o *OutputState) await(ctx context.Context) (interface{}, bool, bool, []Resource, error) {
	// For type-unsafe await, we'll unwrap nested outputs.
	return o.awaitWithOptions(ctx, true /* unwrapNested */)
}

func (o *OutputState) awaitWithOptions(ctx context.Context, unwrapNested bool) (interface{}, bool, bool, []Resource, error) {
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

		if unwrapNested {
			// If the result is an Output, await it in turn.
			//
			// NOTE: this isn't exactly type safe! The element type of the inner output really needs to be assignable to
			// the element type of the outer output. We should reconsider this.
			if ov, ok := value.(Output); ok {
				o = ov.getState()
				continue
			}
		}

		return value, known, secret, deps, nil
	}
}

func (o *OutputState) getState() *OutputState {
	return o
}

// NewOutputState creates a new OutputState that will hold a value of the given type.
func NewOutputState(join *WorkGroup, elementType reflect.Type, deps ...Resource) *OutputState {
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
	outputStateType = reflect.TypeOf((*OutputState)(nil))

	// outputTypeToOutputState is a map from a type
	// to the index of the field that embeds *OutputState.
	outputTypeToOutputState sync.Map // map[reflect.Type]int
)

// SetOutputState sets the OutputState field of the given output to the given state.
// The output must be a pointer to a struct that embeds a field of type `*OutputState`.
func SetOutputState(output reflect.Value, state *OutputState) {
	typ := output.Type()

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

	output.Field(outputFieldV.(int)).Set(reflect.ValueOf(state))
}

// NewOutput builds a new unresolved output with the given output type.
// The given type MUST embed a field of type `*OutputState` in order to be valid.
func NewOutput(wg *WorkGroup, typ reflect.Type, deps ...Resource) Output {
	contract.Requiref(typ.Implements(outputType), "type", "type %v does not implement Output", typ)
	// Create the new output.
	output := reflect.New(typ).Elem()
	state := NewOutputState(wg, output.Interface().(Output).ElementType(), deps...)
	SetOutputState(output, state)
	return output.Interface().(Output)
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
		return nil, fmt.Errorf("applier must be a function, got %T", fn)
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
	contract.Assertf(AnyOutputType != nil, "AnyOutputType must be initialized")

	resultType := AnyOutputType
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

	result := NewOutput(o.join, resultType, o.dependencies()...)
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
	o := toOutputWithContext(ctx, input.getState().join, input, &secret, nil /* output Type */)
	return o
}

// ToSecret wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecret(input interface{}) Output {
	return ToSecretWithContext(context.Background(), input)
}

// ToSecretWithContext wraps the input in an Output marked as secret
// that will resolve when all Inputs contained in the given value have resolved.
func ToSecretWithContext(ctx context.Context, input interface{}) Output {
	x := true
	o := toOutputWithContext(ctx, nil, input, &x, nil /* output Type */)
	return o
}

func gatherJoins(v interface{}) workGroups {
	if v == nil {
		return nil
	}

	joinSet := make(map[*WorkGroup]struct{})
	gatherJoinSet(reflect.ValueOf(v), joinSet)

	var joins workGroups
	if len(joinSet) > 0 {
		joins = slice.Prealloc[*WorkGroup](len(joinSet))
		for j := range joinSet {
			joins = append(joins, j)
		}
	}

	return joins
}

var resourceType = reflect.TypeOf((*Resource)(nil)).Elem()

func gatherJoinSet(v reflect.Value, joins map[*WorkGroup]struct{}) {
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

		//nolint:exhaustive // We only need to further process a few kinds of values.
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

func CallToOutputMethod(ctx context.Context, input reflect.Value, resolvedType reflect.Type) (Output, bool) {
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
			if newOutput, ok := CallToOutputMethod(ctx, reflect.ValueOf(input), resolved.Type()); ok {
				// We were able to convert the input. Use the result as the new input value.
				input = newOutput
			} else if !valueType.AssignableTo(resolved.Type()) {
				// If the value type is not assignable to the destination, see if we can assign the input value itself
				// to the destination.
				if !v.Type().AssignableTo(resolved.Type()) {
					panic(fmt.Errorf("cannot convert an input of type %T to a value of type %v",
						input, resolved.Type()))
				}
				assignInput = true
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
		if _, ok := FullyResolvedTypes[reflect.TypeOf(input)]; ok {
			resolved.Set(reflect.ValueOf(input))
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
	//nolint:exhaustive // The default case is equipped to handle the rest of the types.
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
		getMappedField := MapStructTypes(typ, resolved.Type())
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

func toOutputTWithContext(ctx context.Context, join *WorkGroup, outputType reflect.Type, v interface{}, result reflect.Value, forceSecretVal *bool) Output {
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
			join = &WorkGroup{}
			done = func() {
				join.Wait()
				joins.done()
			}
		}
	}
	joins.add()

	output := NewOutput(join, outputType)
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
	return toOutputWithContext(ctx, nil, v, nil, nil /* outputType */)
}

func ToOutputWithOutputType(
	ctx context.Context,
	outType reflect.Type,
	v interface{},
) Output {
	contract.Requiref(outType.Implements(outputType), "outType", "type %v does not implement Output", outType)
	return toOutputWithContext(ctx, nil, v, nil, outType)
}

func toOutputWithContext(
	ctx context.Context,
	join *WorkGroup,
	v interface{},
	forceSecretVal *bool,
	// Optional explicit output type.
	// If not provided, the output type will be inferred from the type of the value.
	outType reflect.Type,
) Output {
	contract.Assertf(AnyOutputType != nil, "AnyOutputType must be initialized")

	resultType := reflect.TypeOf(v)
	if input, ok := v.(Input); ok {
		resultType = input.ElementType()
	}
	var result reflect.Value
	if v != nil {
		result = reflect.New(resultType).Elem()
	}

	if outType == nil {
		outType = AnyOutputType
		if ot, ok := concreteTypeToOutputType.Load(resultType); ok {
			outType = ot.(reflect.Type)
		}
	}

	return toOutputTWithContext(ctx, join, outType, v, result, forceSecretVal)
}

func OutputWithDependencies(ctx context.Context, o Output, deps ...Resource) Output {
	state := o.getState()
	mergedDeps := mergeDependencies(state.deps, deps)
	resultType := reflect.TypeOf(o)
	result := NewOutput(state.join, resultType, mergedDeps...)
	go func() {
		v, known, secret, deps, err := o.getState().await(ctx)
		if err != nil || !known {
			result.getState().fulfill(nil, known, secret, deps, err)
			return
		}

		val := reflect.ValueOf(v)
		if !val.IsValid() {
			val = reflect.Zero(state.elementType())
		}

		var fulfilledDeps []Resource
		fulfilledDeps = append(fulfilledDeps, deps...)
		if resultOutput, ok := val.Interface().(Output); ok {
			fulfilledDeps = append(fulfilledDeps, resultOutput.getState().dependencies()...)
		}
		// Fulfill the result.
		result.getState().fulfillValue(val, true, secret, fulfilledDeps, nil)
	}()
	return result
}

// Input is an input value for a Pulumi resource.
//
// This is an untyped version of the Input type
// that relies on runtime reflection to determine the type.
type Input interface {
	ElementType() reflect.Type
}

var anyType = reflect.TypeOf((*interface{})(nil)).Elem()
