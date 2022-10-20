package pulumi

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"golang.org/x/net/context"
)

// Promise implements Output and Input. The type parameter T is a phantom type.
type Promise[T any] struct {
	*OutputState
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func (p Promise[T]) ElementType() reflect.Type {
	return typeOf[T]()
}

func (Promise[T]) isPromise() bool {
	return true
}

func (o *OutputState) isPromise() bool {
	return false
}

type _promiseLike interface {
	promiseElement() reflect.Type
}

func (Promise[T]) promiseElement() reflect.Type {
	return typeOf[T]()
}

var promiseType = typeOf[_promiseLike]()

func Cast[T any](o Output) Promise[T] {
	if o.ElementType().AssignableTo(typeOf[T]()) {
		// fmt.Printf("🔵🔵🔵 o is: %#v, elementType is: %v\n", o, o.ElementType().Name())
		return Promise[T]{o.getState()}
	}

	dest := newPromiseState[T](nil)
	dest.reject(fmt.Errorf("cannot cast %v to %v", o.ElementType(), typeOf[T]()))
	return dest
}

func ToPromise[T any](value T) Promise[T] {
	state := newPromiseState[T](nil)
	state.resolve(value, true, false, nil)
	return state
}

func T[T any](value T) Promise[T] {
	return ToPromise(value)
}

// IsSecret returns true if the value of this promise is secret.
func (p Promise[T]) IsSecret() bool {
	return p.secret
}

func (p Promise[T]) Secret() Promise[T] {
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

func newPromise[T any](deps ...Resource) (Promise[T], func(T), func(error)) {
	out := newPromiseState[T](nil, deps...)

	resolve := func(v T) {
		out.fulfillValue(reflect.ValueOf(v), true, false, nil, nil)
	}
	reject := func(err error) {
		var empty T
		out.fulfillValue(reflect.ValueOf(empty), true, false, nil, err)
	}

	return out, resolve, reject
}

func (p Promise[T]) fulfill(value T, known, secret bool, deps []Resource, err error) {
	p.OutputState.fulfillValue(reflect.ValueOf(value), known, secret, deps, err)
}

func newPromiseState[T any](join *workGroup, deps ...Resource) Promise[T] {
	state := newOutputState(join, typeOf[T](), deps...)
	return Promise[T]{state}
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
	state := o.getState().ApplyT(func(v T) (U, error) {
		return applier(v)
	}).getState()
	return Promise[U]{state}
}

func FlatMap[T, U any](o Promise[T], applier func(v T) Promise[U]) Promise[U] {
	return FlatMapWithContextError(context.Background(), o, func(v T) (Promise[U], error) { return applier(v), nil })
}

func FlatMapWithContext[T, U any](ctx context.Context, o Promise[T], applier func(v T) Promise[U]) Promise[U] {
	return FlatMapWithContextError(ctx, o, func(v T) (Promise[U], error) { return applier(v), nil })
}

func FlatMapErr[T, U any](o Promise[T], applier func(v T) (Promise[U], error)) Promise[U] {
	return FlatMapWithContextError(context.Background(), o, applier)
}

// Monadic bind operation for Promises.
func FlatMapWithContextError[T, U any](ctx context.Context, o Promise[T], applier func(v T) (Promise[U], error)) Promise[U] {
	oState := o.getState()
	result := newPromiseState[U](oState.join, oState.deps...)

	go func() {
		v, known, secret, deps, err := oState.await(ctx)
		if err != nil || !known {
			var empty U
			result.fulfill(empty, known, secret, deps, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		final, err := applier(v.(T))
		if err != nil {
			var empty U
			result.fulfill(empty, known, secret, deps, err)
			return
		}

		v2, fKnown, fSecret, fDeps, err := final.await(ctx)
		known = known && fKnown
		secret = secret || fSecret
		deps = mergeDependencies(deps, fDeps)
		if err != nil || !known {
			var empty U
			result.fulfill(empty, known, secret, deps, err)
			return
		}

		result.fulfill(v2.(U), known, secret, deps, nil)
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
	return FlatMapWithContextError(context.Background(), o, pureResult[Promise[T]])
}

func joinWithContext[T any](ctx context.Context, o Promise[Promise[T]]) Promise[T] {
	return FlatMapWithContextError(ctx, o, pureResult[Promise[T]])
}

func PromiseAll[T any](o Promise[T], os ...Promise[T]) Promise[[]T] {
	return PromiseAllWithContext(context.Background(), o, os...)
}

func PromiseAllWithContext[T any](ctx context.Context, p Promise[T], ps ...Promise[T]) Promise[[]T] {
	result := newPromiseState[[]T](nil)

	go func() {
		var values []T
		v, known, secret, deps, err := p.await(ctx)
		if err != nil || !known {
			var empty []T
			result.fulfillValue(reflect.ValueOf(empty), known, secret, deps, err)
			return
		}
		values = append(values, v.(T))

		for _, nextPromise := range ps {
			v, nextKnown, nextSecret, nextDeps, err := nextPromise.await(ctx)
			known = known && nextKnown
			secret = secret || nextSecret
			deps = mergeDependencies(deps, nextDeps)
			if err != nil || !known {
				var empty []T
				result.fulfillValue(reflect.ValueOf(empty), known, secret, deps, err)
				return
			}
			values = append(values, v.(T))
		}

		result.resolve(values, known, secret, deps)
	}()

	return result
}
