package pulumi

import (
	"context"
	"fmt"
	"reflect"
)

type TypeInfo[T any] struct {
	// Zero-width phantom field to prevent:
	//
	// - casting from TypeInfo[T] to TypeInfo[U]
	// - comparison with == and !=
	//
	// Having a field of a type that incoprorates T prevents the first,
	// and making it have a function type prevents the second.
	// Without this, the following is allowed.
	//
	//	var t1 TypeInfo[int]
	//	var t2 TypeInfo[string]
	//	t1 == TypeInfo[int](t2) // true
	_ [0]func() T
}

func NewTypeInfo[T any]() TypeInfo[T] {
	return TypeInfo[T]{}
}

func (t *TypeInfo[T]) Type() reflect.Type {
	return typeOf[T]()
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

type InputT[T any] interface {
	Input

	TypeInfo() TypeInfo[T]
	// TODO: Can we enforce that T is assignable to ElementType?
}

type OutputT[T any] struct {
	*OutputState // TODO: hide me
}

func Cast[T any](o Output) OutputT[T] {
	typ := typeOf[T]()
	if o.ElementType().AssignableTo(typ) {
		return OutputT[T]{o.getState()}
	}

	// TODO: should this return an error instead?
	// With a MustCast[T] function that panics?
	state := newOutputState(nil /* joinGroup */, typ)
	state.reject(fmt.Errorf("cannot cast %v to %v", o.ElementType(), typ))
	return OutputT[T]{state}
}

var (
	_ Output      = OutputT[any]{}
	_ Input       = OutputT[any]{}
	_ InputT[int] = OutputT[int]{}
)

func (o OutputT[T]) ElementType() reflect.Type {
	return typeOf[T]()
}

func (OutputT[T]) TypeInfo() TypeInfo[T] {
	return NewTypeInfo[T]()
}

func (o OutputT[T]) ToAnyOutput() AnyOutput {
	return AnyOutput(o)
}

// awaitT is a type-safe variant of OutputState.await.
func awaitT[I InputT[T], T any](ctx context.Context, o InputT[T]) (v T, known, secret bool, deps []Resource, err error) {
	var value any
	// TODO: make OutputState type-safe internally.
	value, known, secret, deps, err = ToOutput(o).getState().await(ctx)
	if err == nil {
		v = value.(T)
		// TODO: should this turn into an error?
	}
	return v, known, secret, deps, err
}

type ArrayOutputT[T any] struct{ *OutputState }

var (
	_ Output        = ArrayOutputT[any]{}
	_ Input         = ArrayOutputT[any]{}
	_ InputT[[]int] = ArrayOutputT[int]{}
)

func (ArrayOutputT[T]) ElementType() reflect.Type {
	return reflect.SliceOf(typeOf[T]())
}

func (ArrayOutputT[T]) TypeInfo() TypeInfo[[]T] {
	return NewTypeInfo[[]T]()
}

func (o ArrayOutputT[T]) Index(i InputT[int]) OutputT[T] {
	return ApplyT2(o, i, func(items []T, idx int) T {
		if idx < 0 || idx >= len(items) {
			var zero T
			return zero
		}
		return items[idx]
	})
}

type PtrOutputT[T any] struct{ *OutputState }

var (
	_ Output       = PtrOutputT[any]{}
	_ Input        = PtrOutputT[any]{}
	_ InputT[*int] = PtrOutputT[int]{}
)

func PtrOf[T any](o OutputT[T]) PtrOutputT[T] {
	// As of Go 1.20, Output[T] cannot refer to Output[*T] directly.
	// This refers to the following limitation at
	// https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#generic-types:
	//
	//  A generic type can refer to itself in cases
	//  where a type can ordinarily refer to itself,
	//  but when it does so the type arguments must be the type parameters,
	//  listed in the same order.
	//  This restriction prevents infinite recursion of type instantiation.
	//
	// In other words, Output[T]'s methods can only refer to Output[T],
	// or concrete instances of Output[T] (e.g. Output[int]).
	// They cannot refer to variants of T (e.g. Output[*T] or Output[[]T]).
	// Doing so will result in a compile error.
	//
	//   func (o Output[T]) Ptr() Output[*T] {
	//     var _ = OutputT[*T]{}
	//     // error: instantiation cycle
	//
	// This restriction applies for both, direct and indirect references
	// so a method on Output[T] also cannot call a function
	// that instantiates Output[*T].
	//
	//   func (o Output[T]) Ptr() Output[*T] {
	//     return PtrOf(o)
	//     // error: instantiation cycle
	//
	// This restriction may be lifted in the future,
	// but meanwhile it means that PtrOf must be a top-level function.
	p := ApplyT(o, func(v T) *T { return &v })
	return PtrFrom(p)
}

func PtrFrom[T any](o OutputT[*T]) PtrOutputT[T] {
	// No need to check if o.ElementType() is assignable.
	// It's already a pointer type.
	return PtrOutputT[T]{o.getState()}
}

func (PtrOutputT[T]) ElementType() reflect.Type {
	return reflect.PtrTo(typeOf[T]())
}

func (PtrOutputT[T]) TypeInfo() TypeInfo[*T] {
	return NewTypeInfo[*T]()
}

func (o PtrOutputT[T]) Elem() OutputT[T] {
	return ApplyT(o, func(v *T) T {
		if v == nil {
			var zero T
			return zero
		}
		return *v
	})
}

type MapOutputT[K comparable, V any] struct{ *OutputState }

var (
	_ Output                 = MapOutputT[string, any]{}
	_ Input                  = MapOutputT[string, any]{}
	_ InputT[map[string]int] = MapOutputT[string, int]{}
)

func (MapOutputT[K, V]) ElementType() reflect.Type {
	return reflect.MapOf(typeOf[K](), typeOf[V]())
}

func (MapOutputT[K, V]) TypeInfo() TypeInfo[map[K]V] {
	return NewTypeInfo[map[K]V]()
}

func (o MapOutputT[K, V]) MapIndex(i InputT[K]) OutputT[V] {
	return ApplyT2(o, i, func(items map[K]V, idx K) V {
		return items[idx]
	})
}

// TODO: Should we parameterize the applier so context, etc. can be optionally
// passed in?
func ApplyT[O any, I InputT[T], T any](o I, f func(T) O) OutputT[O] {
	// TODO: make this type safe
	return OutputT[O]{
		ToOutput(o).getState().ApplyT(f).getState(),
	}
}

func ApplyT2[O any, I1 InputT[T1], I2 InputT[T2], T1, T2 any](o1 I1, o2 I2, f func(T1, T2) O) OutputT[O] {
	// TODO: context
	state := newOutputState(nil, typeOf[O]())
	go func() {
		v1, known, secret, deps, err := awaitT[I1, T1](context.Background(), o1)
		if err != nil || !known {
			var zero O
			state.fulfill(zero, known, secret, deps, err)
			return
		}

		v2, known2, secret2, deps2, err := awaitT[I2, T2](context.Background(), o2)
		known = known && known2
		secret = secret || secret2
		deps = mergeDependencies(deps, deps2)
		if err != nil || !known {
			var zero O
			state.fulfill(zero, known, secret, deps, err)
			return
		}

		state.fulfill(f(v1, v2), known, secret, deps, nil)
	}()
	return OutputT[O]{state}
}

/*
Apply2(a, b, f) -> Output[U]
Apply3(a, b, c, f) -> Output[U]
// ...
Apply8(a, b, c, d, e, f, g, h, f) -> Output[U]
*/

// TODO Make a typed outputState[T].
// TODO Make the embeds above private.
