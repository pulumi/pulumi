// Copyright 2017 Pulumi, Inc. All rights reserved.

package eval

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
)

// Allocator is a factory for creating objects.
type Allocator struct {
	hooks InterpreterHooks // an optional set of allocation lifetime callback hooks.
}

// NewAllocator allocates a fresh allocator instance.
func NewAllocator(hooks InterpreterHooks) *Allocator {
	return &Allocator{hooks: hooks}
}

// onNewObject is invoked for each allocation and emits an appropriate event.
func (a *Allocator) onNewObject(tree diag.Diagable, o *rt.Object) {
	if a.hooks != nil {
		a.hooks.OnNewObject(tree, o)
	}
}

// New creates a new empty object of the given type.
func (a *Allocator) New(tree diag.Diagable, t symbols.Type, properties *rt.PropertyMap, super *rt.Object) *rt.Object {
	obj := rt.NewObject(t, nil, properties, super)
	a.onNewObject(tree, obj)
	return obj
}

// NewArray creates a new array object of the given element type.
func (a *Allocator) NewArray(tree diag.Diagable, elem symbols.Type, arr *[]*rt.Pointer) *rt.Object {
	obj := rt.NewArrayObject(elem, arr)
	a.onNewObject(tree, obj)
	return obj
}

// NewDynamic creates a new dynamic object, optionally using a set of existing properties.
func (a *Allocator) NewDynamic(tree diag.Diagable, properties *rt.PropertyMap) *rt.Object {
	obj := rt.NewObject(types.Dynamic, nil, properties, nil)
	a.onNewObject(tree, obj)
	return obj
}

// NewPrimitive creates a new primitive object with the given primitive type.
func (a *Allocator) NewPrimitive(tree diag.Diagable, t symbols.Type, v interface{}) *rt.Object {
	obj := rt.NewPrimitiveObject(t, v)
	a.onNewObject(tree, obj)
	return obj
}

// NewBool creates a new primitive number object.
func (a *Allocator) NewBool(tree diag.Diagable, v bool) *rt.Object {
	obj := rt.NewBoolObject(v)
	a.onNewObject(tree, obj)
	return obj
}

// NewNumber creates a new primitive number object.
func (a *Allocator) NewNumber(tree diag.Diagable, v float64) *rt.Object {
	obj := rt.NewNumberObject(v)
	a.onNewObject(tree, obj)
	return obj
}

// NewNull creates a new null object.
func (a *Allocator) NewNull(tree diag.Diagable) *rt.Object {
	obj := rt.NewNullObject()
	a.onNewObject(tree, obj)
	return obj
}

// NewString creates a new primitive number object.
func (a *Allocator) NewString(tree diag.Diagable, v string) *rt.Object {
	obj := rt.NewStringObject(v)
	a.onNewObject(tree, obj)
	return obj
}

// NewFunction creates a new function object that can be invoked, with the given symbol.
func (a *Allocator) NewFunction(tree diag.Diagable, fnc symbols.Function, this *rt.Object) *rt.Object {
	obj := rt.NewFunctionObjectFromSymbol(fnc, this)
	a.onNewObject(tree, obj)
	return obj
}

// NewPointer allocates a new pointer-like object that wraps the given reference.
func (a *Allocator) NewPointer(tree diag.Diagable, t symbols.Type, ptr *rt.Pointer) *rt.Object {
	obj := rt.NewPointerObject(t, ptr)
	a.onNewObject(tree, obj)
	return obj
}

// NewConstant returns a new object with the right type and value, based on some constant data.
func (a *Allocator) NewConstant(tree diag.Diagable, v interface{}) *rt.Object {
	obj := rt.NewConstantObject(v)
	a.onNewObject(tree, obj)
	return obj
}
