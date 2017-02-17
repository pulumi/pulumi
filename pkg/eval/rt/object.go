// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Object is a value allocated and stored on the heap.  In MuIL's interpreter, all values are heap allocated, since we
// are less concerned about performance of the evaluation (compared to the cost of provisioning cloud resources).
type Object struct {
	t          symbols.Type // the runtime type of the object.
	value      Value        // any constant data associated with this object.
	properties PropertyMap  // the full set of known properties and their values.
	proto      *Object      // the super (prototype) object if the object has a base class.
}

var _ fmt.Stringer = (*Object)(nil)

type Value interface{} // a literal object value.

// NewObject allocates a new object with the given type, primitive value, properties, and prototype object.
func NewObject(t symbols.Type, value Value, properties PropertyMap, proto *Object) *Object {
	if properties == nil {
		properties = make(PropertyMap)
	}
	return &Object{t: t, value: value, properties: properties, proto: proto}
}

func (o *Object) Type() symbols.Type      { return o.t }
func (o *Object) Value() Value            { return o.value }
func (o *Object) Properties() PropertyMap { return o.properties }
func (o *Object) Proto() *Object          { return o.proto }

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
				",sig=" + stub.Func.Signature().String() +
				",targ=" + stub.Func.Token().String() + "}"
		}

		// See if it's a pointer; if yes, format the reference.
		if _, isptr := o.t.(*symbols.PointerType); isptr {
			return o.PointerValue().String()
		}

		// Otherwise it's an arbitrary object; just print the type (we can't recurse, due to possible cycles).
		return "obj{type=" + o.t.Token().String() + ",props={...}}"
	}
}

// NewPrimitiveObject creates a new primitive object with the given primitive type.
func NewPrimitiveObject(t symbols.Type, v interface{}) *Object {
	return NewObject(t, v, nil, nil)
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
	return NewObject(fnc.Signature(), stub, nil, nil)
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

// FreezeReadonlyProperties freezes the readonly fields of an object, possibly copying properties down from the
// prototype chain as necessary to accomplish the freezing (since modifying the prototype chain would clearly be wrong).
// TODO[marapongo/mu#70]: this could cause subtle compatibility issues; e.g., it's possible to write to the prototype
//     for an inherited property postconstruction; in vanilla ECMAscript, that write would be seen; in MuJS it won't.
func (obj *Object) FreezeReadonlyProperties() {
	current := obj.Type()
	for current != nil {
		members := current.TypeMembers()
		for _, member := range symbols.StableClassMemberMap(members) {
			if m := members[member]; !m.Static() {
				if prop, isprop := m.(*symbols.ClassProperty); isprop && prop.Readonly() {
					ptr := obj.GetPropertyAddr(PropertyKey(member), true, true)
					if !ptr.Readonly() {
						ptr.Freeze() // ensure we cannot write to this any longer.
					}
				}
			}
		}

		// Keep going up the type hierarchy.
		contract.Assert(current != current.Base())
		current = current.Base()
	}
}

// GetPropertyAddr locates a property with the given key in an object's property map and/or prototype chain.
// If that property is not found, and init is true, then it will be added to the object's property map.  If direct is
// true, then this function ensures that the property is in the object's map, versus being in the prototype chain.
func (obj *Object) GetPropertyAddr(key PropertyKey, init bool, direct bool) *Pointer {
	return obj.GetPropertyAddrForThis(obj, key, init, direct)
}

// GetPropertyAddr locates a property with the given key, similar to GetPropertyAddr, except that it ensures the
// resulting pointer references the target `this` object (e.g., when loading a prototype function member).
func (obj *Object) GetPropertyAddrForThis(this *Object, key PropertyKey, init bool, direct bool) *Pointer {
	var ptr *Pointer  // the resulting pointer to return
	var where *Object // the object the pointer was found on

	// If it's in the object's property map already, just return it.
	if ptr = obj.Properties().GetAddr(key, false); ptr != nil {
		where = obj
	} else {
		// Otherwise, consult the prototype chain.
		proto := obj.Proto()
		for proto != nil {
			if ptr = proto.Properties().GetAddr(key, false); ptr != nil {
				where = proto
				break
			}
			proto = proto.Proto()
		}
	}

	if ptr == nil {
		// If we didn't find anything, and were asked to initialize, do so now.
		if init {
			ptr = this.Properties().InitAddr(key, nil, false)
			if glog.V(9) {
				glog.V(9).Infof("Object(%v).GetPropertyAddr(%v, %v, %v) not found; initialized: %v",
					this.Type(), key, init, direct, ptr)
			}
		} else if glog.V(9) {
			glog.V(9).Infof("Object(%v).GetPropertyAddr(%v, %v, %v) not found", this.Type(), key, init, direct)
		}
	} else if where == this {
		// If found in the this object, great, nothing else needs to be done other than returning.
		if glog.V(9) {
			glog.V(9).Infof("Object(%v).GetPropertyAddr(%v, %v, %v) found in object map",
				this.Type(), key, init, direct)
		}
	} else {
		// Function objects will have `this` bound to the prototype; we need to rebind it to the correct object.
		ptr = adjustPointerForThis(where, this, ptr)

		// If we were asked to make this property directly on the object, copy it down; else return as-is.
		if direct {
			ptr = this.Properties().InitAddr(key, ptr.Obj(), ptr.Readonly())
			if glog.V(9) {
				glog.V(9).Infof(
					"Object(%v).GetPropertyAddr(%v, %v, %v) found in prototype %v; copied to object map: %v",
					this.Type(), key, init, direct, where.Type(), ptr)
			}
		} else if glog.V(9) {
			glog.V(9).Infof("Object(%v).GetPropertyAddr(%v, %v, %v) found in prototype %v: %v",
				this.Type(), key, init, direct, where.Type(), ptr)
		}
	}

	return ptr
}

// PropertyValues returns a snapshot of the object's properties, by walking its prototype chain.  Note that mutations in
// the map returned will not be reflected in the object state; this is a *snapshot*.
func (o *Object) PropertyValues() PropertyMap {
	properties := make(PropertyMap)

	// First add our own object properties.
	for k, v := range o.properties {
		properties[k] = v
	}

	// Now walk the prototype hierarchy and, for anything that doesn't already exist, adjust and add it.
	proto := o.proto
	for proto != nil {
		for k, v := range proto.properties {
			if _, has := properties[k]; !has {
				properties[k] = adjustPointerForThis(proto, o, v)
			}
		}

		proto = proto.proto
	}

	return properties
}

// adjustPointerForThis conditionally adjusts a property pointer for a given `this` object.
func adjustPointerForThis(parent *Object, this *Object, prop *Pointer) *Pointer {
	if parent != this {
		if value := prop.Obj(); value != nil {
			if _, isfunc := value.Type().(*symbols.FunctionType); isfunc {
				contract.Assert(prop.Readonly()) // otherwise, writes to the resulting pointer could go missing.
				stub := value.FunctionValue()
				contract.Assert(stub.This == parent)
				value = NewFunctionObject(stub.Func, this)
				prop = NewPointer(value, true)
			}
		}
	}
	return prop
}
