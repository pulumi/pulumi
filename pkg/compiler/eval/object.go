// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Object is a value allocated and stored on the heap.  In MuIL's interpreter, all values are heap allocated, since we
// are less concerned about performance of the evaluation (compared to the cost of provisioning cloud resources).
type Object struct {
	Type       symbols.Type // the runtime type of the object.
	Value      Value        // any constant data associated with this object.
	Properties Properties   // the full set of known properties and their values.
}

var _ fmt.Stringer = (*Object)(nil)

type Value interface{}                   // a literal object value.
type Properties map[tokens.Name]*Pointer // an object's properties.

// NewObject creates a new empty object of the given type.
func NewObject(t symbols.Type) *Object {
	return &Object{
		Type:       t,
		Properties: make(Properties),
	}
}

// NewPrimitiveObject creates a new primitive object with the given primitive type.
func NewPrimitiveObject(t symbols.Type, v interface{}) *Object {
	if glog.V(9) {
		glog.V(9).Infof("New primitive object: t=%v; v=%v", t, v)
	}
	return &Object{
		Type:  t,
		Value: v,
	}
}

// NewBoolObject creates a new primitive number object.
func NewBoolObject(v bool) *Object {
	return NewPrimitiveObject(types.Bool, v)
}

// NewNumberObject creates a new primitive number object.
func NewNumberObject(v float64) *Object {
	return NewPrimitiveObject(types.Number, v)
}

// NewStringObject creates a new primitive number object.
func NewStringObject(v string) *Object {
	return NewPrimitiveObject(types.String, v)
}

// NewFunctionObject creates a new function object that can be invoked, with the given symbol.
func NewFunctionObject(fnc symbols.Function, this *Object) *Object {
	return &Object{
		Type:  fnc.FuncType(),
		Value: funcStub{Func: fnc, This: this},
	}
}

// funcStub is a stub that captures a symbol plus an optional instance 'this' object.
type funcStub struct {
	Func symbols.Function
	This *Object
}

// NewPointerObject allocates a new pointer-like object that wraps the given reference.
func NewPointerObject(t symbols.Type, ptr *Pointer) *Object {
	contract.Require(ptr != nil, "ptr")
	ptrt := symbols.NewPointerType(t)
	return NewPrimitiveObject(ptrt, ptr)
}

// NewErrorObject creates a new exception with the given message.
func NewErrorObject(message string, args ...interface{}) *Object {
	// TODO: capture a stack trace.
	return NewPrimitiveObject(types.Error, fmt.Sprintf(message, args...))
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

// BoolValue asserts that the target is a boolean literal and returns its value.
func (o *Object) BoolValue() bool {
	contract.Assertf(o.Type == types.Bool, "Expected object type to be Bool; got %v", o.Type)
	contract.Assertf(o.Value != nil, "Expected Bool object to carry a Value; got nil")
	b, ok := o.Value.(bool)
	contract.Assertf(ok, "Expected Bool object's Value to be boolean literal")
	return b
}

// NumberValue asserts that the target is a numeric literal and returns its value.
func (o *Object) NumberValue() float64 {
	contract.Assertf(o.Type == types.Number, "Expected object type to be Number; got %v", o.Type)
	contract.Assertf(o.Value != nil, "Expected Number object to carry a Value; got nil")
	n, ok := o.Value.(float64)
	contract.Assertf(ok, "Expected Number object's Value to be numeric literal")
	return n
}

// StringValue asserts that the target is a string and returns its value.
func (o *Object) StringValue() string {
	contract.Assertf(o.Type == types.String, "Expected object type to be String; got %v", o.Type)
	contract.Assertf(o.Value != nil, "Expected String object to carry a Value; got nil")
	s, ok := o.Value.(string)
	contract.Assertf(ok, "Expected String object's Value to be string")
	return s
}

// FunctionValue asserts that the target is a reference and returns its value.
func (o *Object) FunctionValue() funcStub {
	contract.Assertf(o.Value != nil, "Expected Function object to carry a Value; got nil")
	r, ok := o.Value.(funcStub)
	contract.Assertf(ok, "Expected Function object's Value to be a Function")
	return r
}

// PointerValue asserts that the target is a reference and returns its value.
func (o *Object) PointerValue() *Pointer {
	contract.Assertf(o.Value != nil, "Expected Pointer object to carry a Value; got nil")
	r, ok := o.Value.(*Pointer)
	contract.Assertf(ok, "Expected Pointer object's Value to be a Pointer")
	return r
}

// GetPropertyPointer returns the reference to an object's property, lazily initializing if 'init' is true, or
// returning nil otherwise.
func (o *Object) GetPropertyPointer(nm tokens.Name, init bool) *Pointer {
	ref, has := o.Properties[nm]
	if !has {
		ref = &Pointer{}
		o.Properties[nm] = ref
	}
	return ref
}

// String can be used to print the contents of an object; it tries to be smart about the display.
func (o *Object) String() string {
	switch o.Type {
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
		if _, isfnc := o.Type.(*symbols.FunctionType); isfnc {
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
		if _, isptr := o.Type.(*symbols.PointerType); isptr {
			return o.PointerValue().String()
		}

		// Otherwise it's an arbitrary object; just dump out the type and properties.
		var p string
		for prop, ptr := range o.Properties {
			if p != "" {
				p += ","
			}
			p += prop.String() + "=" + ptr.String()
		}
		return "obj{type=" + o.Type.Token().String() + ",props={" + p + "}}"
	}
}

// Pointer is a slot that can be used for indirection purposes (since Go maps are not stable).
type Pointer struct {
	obj      *Object // the object to which the value refers.
	readonly bool    // true prevents writes to this slot (by abandoning).
}

var _ fmt.Stringer = (*Pointer)(nil)

func (ptr *Pointer) Readonly() bool { return ptr.readonly }
func (ptr *Pointer) Obj() *Object   { return ptr.obj }

func (ptr *Pointer) Set(obj *Object) {
	contract.Assertf(!ptr.readonly, "Unexpected write to readonly reference")
	ptr.obj = obj
}

func (ptr *Pointer) String() string {
	var prefix string
	if ptr.readonly {
		prefix = "&"
	} else {
		prefix = "*"
	}
	if ptr.obj == nil {
		return prefix + "{<nil>}"
	}
	return prefix + "{" + ptr.obj.String() + "}"
}
