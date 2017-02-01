// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"fmt"
	"strconv"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/tokens"
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

type Value interface{}                   // a literal object value.
type Properties map[tokens.Name]*Pointer // an object's properties.

// NewObject allocates a new object with the given type, primitive value, and properties.
func NewObject(t symbols.Type, value Value, properties Properties) *Object {
	return &Object{t: t, value: value, properties: properties}
}

func (o *Object) Type() symbols.Type     { return o.t }
func (o *Object) Value() Value           { return o.value }
func (o *Object) Properties() Properties { return o.properties }

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

// FunctionValue asserts that the target is a reference and returns its value.
func (o *Object) FunctionValue() FuncStub {
	contract.Assertf(o.value != nil, "Expected Function object to carry a Value; got nil")
	r, ok := o.value.(FuncStub)
	contract.Assertf(ok, "Expected Function object's Value to be a Function")
	return r
}

// PointerValue asserts that the target is a reference and returns its value.
func (o *Object) PointerValue() *Pointer {
	contract.Assertf(o.value != nil, "Expected Pointer object to carry a Value; got nil")
	r, ok := o.value.(*Pointer)
	contract.Assertf(ok, "Expected Pointer object's Value to be a Pointer")
	return r
}

// GetPropertyAddr returns the reference to an object's property, lazily initializing if 'init' is true, or
// returning nil otherwise.
func (o *Object) GetPropertyAddr(nm tokens.Name, init bool) *Pointer {
	ptr, hasprop := o.properties[nm]
	if !hasprop {
		// Look up the property definition (if any) in the members list, to seed a default value.
		var obj *Object
		var readonly bool
		if class, isclass := o.t.(*symbols.Class); isclass {
			if member, hasmember := class.Members[tokens.ClassMemberName(nm)]; hasmember {
				switch m := member.(type) {
				case *symbols.ClassProperty:
					if m.Default() != nil {
						obj = NewConstantObject(*m.Default())
					}
					readonly = m.Readonly()
				case *symbols.ClassMethod:
					if m.Static() {
						obj = NewFunctionObject(m, nil)
					} else {
						obj = NewFunctionObject(m, o)
					}
					readonly = true // TODO[marapongo/mu#56]: consider permitting JS-style overwriting of methods.
				default:
					contract.Failf("Unexpected member type: %v", member)
				}
			}
		}
		ptr = NewPointer(obj, readonly)
		o.properties[nm] = ptr
	}
	return ptr
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
			p += prop.String() + "=" + ptr.String()
		}
		return "obj{type=" + o.t.Token().String() + ",props={" + p + "}}"
	}
}

// NewPrimitiveObject creates a new primitive object with the given primitive type.
func NewPrimitiveObject(t symbols.Type, v interface{}) *Object {
	return NewObject(t, v, nil)
}

// NewBoolObject creates a new primitive number object.
func NewBoolObject(v bool) *Object {
	return NewPrimitiveObject(types.Bool, v)
}

// NewNumber creates a new primitive number object.
func NewNumber(v float64) *Object {
	return NewPrimitiveObject(types.Number, v)
}

// NewString creates a new primitive number object.
func NewString(v string) *Object {
	return NewPrimitiveObject(types.String, v)
}

// NewFunctionObject creates a new function object that can be invoked, with the given symbol.
func NewFunctionObject(fnc symbols.Function, this *Object) *Object {
	stub := FuncStub{Func: fnc, This: this}
	return NewObject(fnc.FuncType(), stub, nil)
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

// FuncStub is a stub that captures a symbol plus an optional instance 'this' object.
type FuncStub struct {
	Func symbols.Function
	This *Object
}
