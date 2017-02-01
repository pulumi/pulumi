// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Allocator is a factory for creating objects.
type Allocator struct {
	hooks InterpreterHooks // an optional set of allocation lifetime callback hooks.
}

// NewAllocator allocates a fresh allocator instance.
func NewAllocator(hooks InterpreterHooks) *Allocator {
	return &Allocator{hooks: hooks}
}

// newObject is used internally for all allocator-driven object allocation, so that hooks can be run appropriately.
func (a *Allocator) newObject(t symbols.Type, v interface{}, props Properties) *Object {
	o := NewObject(t, nil, props)
	if a.hooks != nil {
		a.hooks.OnNewObject(o)
	}
	return o
}

// New creates a new empty object of the given type.
func (a *Allocator) New(t symbols.Type) *Object {
	return a.newObject(t, nil, nil)
}

// NewPrimitive creates a new primitive object with the given primitive type.
func (a *Allocator) NewPrimitive(t symbols.Type, v interface{}) *Object {
	if glog.V(9) {
		glog.V(9).Infof("New primitive object: t=%v; v=%v", t, v)
	}
	return a.newObject(t, v, nil)
}

// NewBool creates a new primitive number object.
func (a *Allocator) NewBool(v bool) *Object {
	return a.NewPrimitive(types.Bool, v)
}

// NewNumber creates a new primitive number object.
func (a *Allocator) NewNumber(v float64) *Object {
	return a.NewPrimitive(types.Number, v)
}

// NewString creates a new primitive number object.
func (a *Allocator) NewString(v string) *Object {
	return a.NewPrimitive(types.String, v)
}

// NewFunction creates a new function object that can be invoked, with the given symbol.
func (a *Allocator) NewFunction(fnc symbols.Function, this *Object) *Object {
	stub := funcStub{Func: fnc, This: this}
	return a.newObject(fnc.FuncType(), stub, nil)
}

// funcStub is a stub that captures a symbol plus an optional instance 'this' object.
type funcStub struct {
	Func symbols.Function
	This *Object
}

// NewPointerObject allocates a new pointer-like object that wraps the given reference.
func (a *Allocator) NewPointer(t symbols.Type, ptr *Pointer) *Object {
	contract.Require(ptr != nil, "ptr")
	ptrt := symbols.NewPointerType(t)
	return a.NewPrimitive(ptrt, ptr)
}

// NewError creates a new exception with the given message.
func (a *Allocator) NewError(message string, args ...interface{}) *Object {
	// TODO: capture a stack trace.
	return a.NewPrimitive(types.Error, fmt.Sprintf(message, args...))
}

// NewConstant returns a new object with the right type and value, based on some constant data.
func (a *Allocator) NewConstant(v interface{}) *Object {
	if v == nil {
		return a.NewPrimitive(types.Null, nil)
	}
	switch data := v.(type) {
	case bool:
		return a.NewPrimitive(types.Bool, data)
	case string:
		return a.NewPrimitive(types.String, data)
	case float64:
		return a.NewPrimitive(types.Number, data)
	default:
		// TODO: we could support more here (essentially, anything that is JSON serializable).
		contract.Failf("Unrecognized constant data literal: %v", data)
		return nil
	}
}
