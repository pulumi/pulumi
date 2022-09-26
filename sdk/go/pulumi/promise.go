//go:build !nongeneric

package pulumi

import (
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"golang.org/x/net/context"
)

type Promise[T any] interface {
	// ToOutput returns an AnyOutput. This value is to be used with non-generic Apply methods and
	// Provider SDKs.
	ToOutput() AnyOutput

	// IsSecret returns true if the value of this promise is secret.
	IsSecret() bool

	// Marks this promise as a secret. If this promise is already marked as a secret. If this value
	// has already been used, an error will be logged to indicate that the value may have been leaked.
	Secret() Promise[T]

	getPromiseState() *promiseState[T]
}

type AnyPromise interface {
	asAny() Promise[any]
}

func (p *promiseState[T]) getPromiseState() *promiseState[T] {
	return p
}

func (p *promiseState[T]) ToOutput() AnyOutput {
	return AnyOutput{p.getState()}
}

// IsSecret returns true if the value of this promise is secret.
func (p *promiseState[T]) IsSecret() bool {
	return p.secret
}

func (p *promiseState[T]) Secret() Promise[T] {
	p.cond.L.Lock()
	defer func() {
		p.cond.L.Unlock()
	}()

	if p.awaited {
		logging.Errorf("Secret() called on a promise that has already been awaited")
	}

	p.secret = true

	return p
}

func (p *promiseState[T]) asAny() Promise[any] {
	return Apply[T](p, func(v T) any { return v })
}

func (p *promiseState[T]) elementType() reflect.Type {
	var empty T
	return reflect.TypeOf(empty)
}

func (p *promiseState[T]) getState() *OutputState {
	output := OutputState{p.asAny().getPromiseState(), p.elementType()}
	return &output
}

func newPromise[T any](deps ...Resource) (Promise[T], func(T), func(error)) {
	out := newPromiseState[T](nil, deps...)

	resolve := func(v T) {
		out.fulfillValue(v, true, false, nil, nil)
	}
	reject := func(err error) {
		var empty T
		out.fulfillValue(empty, true, false, nil, err)
	}

	return out, resolve, reject
}

func newPromiseState[T any](join *workGroup, deps ...Resource) *promiseState[T] {
	if join != nil {
		join.Add(1)
	}

	var m sync.Mutex
	return &promiseState[T]{
		join: join,
		deps: deps,
		// Note: Calling registerResource or readResource with the same resource state can report a
		// spurious data race here. See note in https://github.com/pulumi/pulumi/pull/10081.
		//
		// To reproduce, revert changes in PR to file pkg/engine/lifecycletest/golang_sdk_test.go.
		cond: sync.NewCond(&m),
	}
}

type promiseState[T any] struct {
	cond *sync.Cond

	join *workGroup // the wait group associated with this output, if any.

	state uint32 // one of output{Pending,Resolved,Rejected}

	value  T     // the value of this output if it is resolved.
	err    error // the error associated with this output if it is rejected.
	known  bool  // true if this output's value is known.
	secret bool  // true if this output's value is secret

	awaited bool // true if this value has been used

	deps []Resource // the dependencies associated with this output property.
}

func (p *promiseState[T]) dependencies() []Resource {
	return p.deps
}

func (p *promiseState[T]) joinWg() *workGroup {
	panic("joinWg called on Promise")
}

func (o *promiseState[T]) fulfillValue(value T, known, secret bool, deps []Resource, err error) {
	o.cond.L.Lock()
	defer func() {
		o.cond.L.Unlock()
		o.cond.Broadcast()
	}()

	if o.state != outputPending {
		return
	}

	if err != nil {
		o.state, o.err, o.known, o.secret, o.err = outputRejected, err, true, secret, err
	} else {
		o.value, o.state, o.known, o.secret, o.err = value, outputResolved, known, secret, nil

		// If needed, merge the up-front provided dependencies with fulfilled dependencies, pruning duplicates.
		if len(deps) == 0 {
			// We didn't get any new dependencies, so no need to merge.
			return
		}
		o.deps = mergeDependencies(o.deps, deps)
	}
}

func (o *promiseState[T]) await(ctx context.Context) (T, bool, bool, []Resource, error) {
	var empty T

	if o == nil {
		// If the state is nil, treat its value as resolved and unknown.
		return empty, false, false, nil, nil
	}

	o.cond.L.Lock()
	for o.state == outputPending {
		if ctx.Err() != nil {
			return empty, true, false, nil, ctx.Err()
		}
		o.cond.Wait()
	}
	o.cond.L.Unlock()

	return o.value, o.known, o.secret, o.deps, o.err
}

// Apply is a generic method for applying a computation once an Output[T]'s value is known. Unfortunately, Go
// does not support parametric methods, so this needs to be a global function. For more information, see
// https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#No-parameterized-methods.
func Apply[T, U any](o Promise[T], applier func(v T) U) Promise[U] {
	return ApplyWithContext(context.Background(), o, applier)
}

func ApplyWithContext[T, U any](ctx context.Context, o Promise[T], applier func(v T) U) Promise[U] {
	return ApplyWithContextErr(ctx, o, func(v T) (U, error) {
		result := applier(v)
		return result, nil
	})
}

func ApplyErr[T, U any](o Promise[T], applier func(v T) (U, error)) Promise[U] {
	return ApplyWithContextErr(context.Background(), o, applier)
}

func ApplyWithContextErr[T, U any](ctx context.Context, o Promise[T], applier func(v T) (U, error)) Promise[U] {
	oState := o.getPromiseState()
	result := newPromiseState[U](oState.join, oState.deps...)
	go func() {
		v, known, secret, deps, err := oState.await(ctx)
		if err != nil || !known {
			var empty U
			result.fulfillValue(empty, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		final, err := applier(v)
		result.fulfillValue(final, true, secret, deps, err)
	}()
	return result
}

// Monadic bind operation for Promises.
func bindWithContextErr[T, U any](ctx context.Context, o Promise[T], applier func(v T) (Promise[U], error)) Promise[U] {
	oState := o.getPromiseState()
	result := newPromiseState[U](oState.join, oState.deps...)

	go func() {
		v, known, secret, deps, err := oState.await(ctx)
		if err != nil || !known {
			var empty U
			result.fulfillValue(empty, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		final, err := applier(v)
		if err != nil {
			var empty U
			result.fulfillValue(empty, known, secret, deps, err)
			return
		}

		v2, fKnown, fSecret, fDeps, err := final.getPromiseState().await(ctx)
		known = known && fKnown
		secret = secret || fSecret
		deps = mergeDependencies(deps, fDeps)
		if err != nil || !known {
			var empty U
			result.fulfillValue(empty, known, secret, deps, err)
			return
		}

		result.fulfillValue(v2, known, secret, deps, nil)
	}()
	return result
}

// If (t, error) is an applicative functor, then this is the pure operation lifting a value into it.
func pureResult[T any](v T) (T, error) {
	return v, nil
}

// Flattens a promise by using the identity that Join(p) = bind(x, id), though our "values" are of
// type (T, error).
func Join[T any](o Promise[Promise[T]]) Promise[T] {
	return bindWithContextErr(context.Background(), o, pureResult[Promise[T]])
}

func joinWithContext[T any](ctx context.Context, o Promise[Promise[T]]) Promise[T] {
	return bindWithContextErr(ctx, o, pureResult[Promise[T]])
}

func PromiseAll[T any](o Promise[T], os ...Promise[T]) Promise[[]T] {
	return PromiseAllWithContext(context.Background(), o, os...)
}

func PromiseAllWithContext[T any](ctx context.Context, p Promise[T], ps ...Promise[T]) Promise[[]T] {
	result := newPromiseState[[]T](nil)

	go func() {
		var values []T
		v, known, secret, deps, err := p.getPromiseState().await(ctx)
		if err != nil || !known {
			var empty []T
			result.fulfillValue(empty, known, secret, deps, err)
			return
		}
		values = append(values, v)

		for _, nextPromise := range ps {
			v, nextKnown, nextSecret, nextDeps, err := nextPromise.getPromiseState().await(ctx)
			known = known && nextKnown
			secret = secret || nextSecret
			deps = mergeDependencies(deps, nextDeps)
			if err != nil || !known {
				var empty []T
				result.fulfillValue(empty, known, secret, deps, err)
				return
			}
			values = append(values, v)
		}

		result.fulfillValue(values, known, secret, deps, nil)
	}()

	return result
}

func PromiseAllAny(p AnyPromise, ps ...AnyPromise) Promise[[]any] {
	return PromiseAllAnyWithContext(context.Background(), p, ps...)
}

func PromiseAllAnyWithContext(ctx context.Context, p AnyPromise, ps ...AnyPromise) Promise[[]any] {
	promises := []Promise[any]{p.asAny()}

	for _, pNext := range ps {
		promises = append(promises, pNext.asAny())
	}

	return PromiseAllWithContext(context.Background(), promises[0], promises[1:]...)
}
