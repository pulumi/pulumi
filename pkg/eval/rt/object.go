// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"fmt"
	"strconv"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Object is a value allocated and stored on the heap.  In MuIL's interpreter, all values are heap allocated, since we
// are less concerned about performance of the evaluation (compared to the cost of provisioning cloud resources).
type Object struct {
	t          symbols.Type // the runtime type of the object.
	value      Value        // any constant data associated with this object.
	properties Properties   // the full set of known properties and their values.
}

var _ fmt.Stringer = (*Object)(nil)

type Value interface{} // a literal object value.

// NewObject allocates a new object with the given type, primitive value, and properties.
func NewObject(t symbols.Type, value Value, properties Properties) *Object {
	if properties == nil {
		properties = make(Properties)
	}
	return &Object{t: t, value: value, properties: properties}
}

func (o *Object) Type() symbols.Type     { return o.t }
func (o *Object) Value() Value           { return o.value }
func (o *Object) Properties() Properties { return o.properties }

// ArrayValue asserts that the target is an array literal and returns its value.
func (o *Object) ArrayValue() *[]*Pointer {
	_, isarr := o.t.(*symbols.ArrayType)
	contract.Assertf(isarr, "Expected object type to be Array; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Array object to carry a Value; got nil")
	arr, ok := o.value.(*[]*Pointer)
	contract.Assertf(ok, "Expected Array object's Value to be a *[]interface{}")
	return arr
}

// BoolValue asserts that the target is a boolean literal and returns its value.
func (o *Object) BoolValue() bool {
	contract.Assertf(o.t == types.Bool, "Expected object type to be Bool; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Bool object to carry a Value; got nil")
	b, ok := o.value.(bool)
	contract.Assertf(ok, "Expected Bool object's Value to be boolean literal")
	return b
}

// NumberValue asserts that the target is a numeric literal and returns its value.
func (o *Object) NumberValue() float64 {
	contract.Assertf(o.t == types.Number, "Expected object type to be Number; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Number object to carry a Value; got nil")
	n, ok := o.value.(float64)
	contract.Assertf(ok, "Expected Number object's Value to be numeric literal")
	return n
}

// StringValue asserts that the target is a string and returns its value.
func (o *Object) StringValue() string {
	contract.Assertf(o.t == types.String, "Expected object type to be String; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected String object to carry a Value; got nil")
	s, ok := o.value.(string)
	contract.Assertf(ok, "Expected String object's Value to be string")
	return s
}

// FunctionValue asserts that the target is a function and returns its value.
func (o *Object) FunctionValue() FuncStub {
	contract.Assertf(o.value != nil, "Expected Function object to carry a Value; got nil")
	r, ok := o.value.(FuncStub)
	contract.Assertf(ok, "Expected Function object's Value to be a Function")
	return r
}

// PointerValue asserts that the target is a pointer and returns its value.
func (o *Object) PointerValue() *Pointer {
	contract.Assertf(o.value != nil, "Expected Pointer object to carry a Value; got nil")
	r, ok := o.value.(*Pointer)
	contract.Assertf(ok, "Expected Pointer object's Value to be a Pointer")
	return r
}

// ExceptionValue asserts that the target is an exception and returns its value.
func (o *Object) ExceptionValue() ExceptionInfo {
	contract.Assertf(o.value != nil, "Expected Exception object to carry a Value; got nil")
	r, ok := o.value.(ExceptionInfo)
	contract.Assertf(ok, "Expected Exception object's Value to be an ExceptionInfo")
	return r
}

// GetPropertyAddr returns the reference to an object's property, lazily initializing if 'init' is true, or
// returning nil otherwise.
func (o *Object) GetPropertyAddr(key PropertyKey, init bool, ctx symbols.Function) *Pointer {
	// First, try the fast path, so we can avoid allocating an initer if we don't need one.
	if ptr := o.properties.GetAddr(key); ptr != nil || !init {
		return ptr
	}

	// Otherwise, here's the slow path; figure out the defaults and initialize the slot.
	class, _ := o.t.(*symbols.Class)
	obj, readonly := DefaultClassProperty(key, class, o, ctx)
	return o.properties.InitAddr(key, obj, readonly)
}

// String can be used to print the contents of an object; it tries to be smart about the display.
func (o *Object) String() string {
	switch o.t {
	case types.Bool:
		if o.BoolValue() {
			return "true"
		}
		return "false"
	case types.String:
		return "\"" + o.StringValue() + "\""
	case types.Number:
		// TODO: it'd be nice to format as ints if the decimal part is close enough to "nothing".
		return strconv.FormatFloat(o.NumberValue(), 'f', -1, 64)
	case types.Null:
		return "<nil>"
	default:
		// See if it's a func; if yes, do function formatting.
		if _, isfnc := o.t.(*symbols.FunctionType); isfnc {
			stub := o.FunctionValue()
			var this string
			if stub.This == nil {
				this = "<nil>"
			} else {
				this = stub.This.String()
			}
			return "func{this=" + this +
				",type=" + stub.Func.FuncType().String() +
				",targ=" + stub.Func.Token().String() + "}"
		}

		// See if it's a pointer; if yes, format the reference.
		if _, isptr := o.t.(*symbols.PointerType); isptr {
			return o.PointerValue().String()
		}

		// Otherwise it's an arbitrary object; just dump out the type and properties.
		var p string
		for prop, ptr := range o.properties {
			if p != "" {
				p += ","
			}
			p += string(prop) + "=" + ptr.String()
		}
		return "obj{type=" + o.t.Token().String() + ",props={" + p + "}}"
	}
}

// NewPrimitiveObject creates a new primitive object with the given primitive type.
func NewPrimitiveObject(t symbols.Type, v interface{}) *Object {
	return NewObject(t, v, nil)
}

// NewArrayObject allocates a new array object with the given array payload.
func NewArrayObject(elem symbols.Type, arr *[]*Pointer) *Object {
	contract.Require(elem != nil, "elem")
	arrt := symbols.NewArrayType(elem)
	return NewPrimitiveObject(arrt, arr)
}

var trueObj *Object = NewPrimitiveObject(types.Bool, true)
var falseObj *Object = NewPrimitiveObject(types.Bool, false)

// NewBoolObject creates a new primitive number object.
func NewBoolObject(v bool) *Object {
	if v {
		return trueObj
	}
	return falseObj
}

// NewNumberObject creates a new primitive number object.
func NewNumberObject(v float64) *Object {
	return NewPrimitiveObject(types.Number, v)
}

var nullObj *Object = NewPrimitiveObject(types.Null, nil)

// NewNullObject creates a new null object; null objects are not expected to have distinct identity.
func NewNullObject() *Object {
	return nullObj
}

// NewStringObject creates a new primitive number object.
func NewStringObject(v string) *Object {
	return NewPrimitiveObject(types.String, v)
}

// NewFunctionObject creates a new function object that can be invoked, with the given symbol.
func NewFunctionObject(fnc symbols.Function, this *Object) *Object {
	stub := FuncStub{Func: fnc, This: this}
	return NewObject(fnc.FuncType(), stub, nil)
}

// FuncStub is a stub that captures a symbol plus an optional instance 'this' object.
type FuncStub struct {
	Func symbols.Function
	This *Object
}

// NewPointerObject allocates a new pointer-like object that wraps the given reference.
func NewPointerObject(t symbols.Type, ptr *Pointer) *Object {
	contract.Require(ptr != nil, "ptr")
	ptrt := symbols.NewPointerType(t)
	return NewPrimitiveObject(ptrt, ptr)
}

// NewExceptionObject creates a new exception with the given message.
func NewExceptionObject(node diag.Diagable, stack *StackFrame, message string, args ...interface{}) *Object {
	contract.Require(node != nil, "node")
	contract.Require(stack != nil, "stack")
	info := ExceptionInfo{
		Node:    node,
		Stack:   stack,
		Message: fmt.Sprintf(message, args...),
	}
	return NewPrimitiveObject(types.Exception, info)
}

// ExceptionInfo captures information about a thrown exception (source, stack, and message).
type ExceptionInfo struct {
	Node    diag.Diagable // the location that the throw occurred.
	Stack   *StackFrame   // the full linked stack trace.
	Message string        // the optional pre-formatted error message.
}

// NewConstantObject returns a new object with the right type and value, based on some constant data.
func NewConstantObject(v interface{}) *Object {
	if v == nil {
		return NewPrimitiveObject(types.Null, nil)
	}
	switch data := v.(type) {
	case bool:
		return NewPrimitiveObject(types.Bool, data)
	case string:
		return NewPrimitiveObject(types.String, data)
	case float64:
		return NewPrimitiveObject(types.Number, data)
	default:
		// TODO: we could support more here (essentially, anything that is JSON serializable).
		contract.Failf("Unrecognized constant data literal: %v", data)
		return nil
	}
}
