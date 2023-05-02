package pulumi

import (
	"fmt"
	"reflect"
)

/*
type TypeInfo[T any] struct {
	typ reflect.Type
}

func NewTypeInfo[T any]() TypeInfo[T] {
	var t T
	return TypeInfo[T]{typ: reflect.TypeOf(&t).Elem()}
}
*/

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

type InputT[T any] interface {
	Input

	// TODO: Probably rename to something else.
	// Maybe make it accept the value instead of returning it.
	Sample() *T
	// MUST be assignable to Input.ElementType.
}

type produces[T any] struct{}

func (produces[T]) Sample() *T {
	var t T
	return &t
}

func zeroPtr[T any]() *T {
	var t T
	return &t
}

type OutputT[T any] struct {
	*OutputState
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
	_ Output      = (*OutputT[any])(nil)
	_ Input       = (*OutputT[any])(nil)
	_ InputT[int] = (*OutputT[int])(nil)
)

func (OutputT[T]) ElementType() reflect.Type {
	return typeOf[T]()
}

func (OutputT[T]) Sample() *T {
	var t T
	return &t
}

type ArrayOutputT[T any] struct{ *OutputState }

var (
	_ Output        = (*ArrayOutputT[Output])(nil)
	_ Input         = (*ArrayOutputT[Output])(nil)
	_ InputT[[]int] = (*ArrayOutputT[int])(nil)
)

func (ArrayOutputT[T]) ElementType() reflect.Type {
	var t T
	return reflect.SliceOf(reflect.TypeOf(&t).Elem())
}

func (ArrayOutputT[T]) Sample() *[]T {
	var t []T
	return &t
}

func (o ArrayOutputT[T]) Index(i InputT[int]) OutputT[T] {
	out := All(o, i).ApplyT(func(args []interface{}) T {
		items := args[0].([]T)
		idx := args[1].(int)
		if idx < 0 || idx >= len(items) {
			var zero T
			return zero
		}
		return items[idx]
	})
	return Cast[T](out)
}

type PtrOutputT[T any] struct{ *OutputState }

func ApplyT[O InputT[T], T, U any](o O, f func(T) U) OutputT[U] {
	panic("TODO")
}

/*
Apply2(a, b, f) -> Output[U]
Apply3(a, b, c, f) -> Output[U]
// ...
Apply8(a, b, c, d, e, f, g, h, f) -> Output[U]
*/

// Make a typed outputState[T].
// Make the embeds above private.
