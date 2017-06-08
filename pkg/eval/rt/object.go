// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rt

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

const nilString = "<nil>"

// Object is a value allocated and stored on the heap.  In LumiIL's interpreter, all values are heap allocated, since we
// are less concerned about performance of the evaluation (compared to the cost of provisioning cloud resources).
type Object struct {
	t          symbols.Type // the runtime type of the object.
	value      Value        // any constant data associated with this object.
	properties *PropertyMap // the full set of known properties and their values.
	proto      *Object      // the super (prototype) object if the object has a base class.
}

var _ fmt.Stringer = (*Object)(nil)

type Value interface{} // a literal object value.

// NewObject allocates a new object with the given type, primitive value, properties, and prototype object.
func NewObject(t symbols.Type, value Value, properties *PropertyMap, proto *Object) *Object {
	if properties == nil {
		properties = NewPropertyMap()
	}
	return &Object{t: t, value: value, properties: properties, proto: proto}
}

func (o *Object) Type() symbols.Type       { return o.t }
func (o *Object) Value() Value             { return o.value }
func (o *Object) Properties() *PropertyMap { return o.properties }
func (o *Object) Proto() *Object           { return o.proto }

// IsArray returns true if the target object is a runtime array.
func (o *Object) IsArray() bool {
	_, is := o.t.(*symbols.ArrayType)
	return is
}

// ArrayValue asserts that the target is an array literal and returns its value.
func (o *Object) ArrayValue() *[]*Pointer {
	_, isarr := o.t.(*symbols.ArrayType)
	contract.Assertf(isarr, "Expected object type to be Array; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Array object to carry a Value; got nil")
	arr, ok := o.value.(*[]*Pointer)
	contract.Assertf(ok, "Expected Array object's Value to be a *[]interface{}")
	return arr
}

// TryArrayValue tests the target for an array literal value and, if it is one, returns its value.
func (o *Object) TryArrayValue() (*[]*Pointer, bool) {
	if o.IsArray() {
		return o.ArrayValue(), true
	}
	return nil, false
}

// IsBool returns true if the target object is a runtime boolean.
func (o *Object) IsBool() bool {
	return o.t == types.Bool
}

// BoolValue asserts that the target is a boolean literal and returns its value.
func (o *Object) BoolValue() bool {
	contract.Assertf(o.t == types.Bool, "Expected object type to be Bool; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Bool object to carry a Value; got nil")
	b, ok := o.value.(bool)
	contract.Assertf(ok, "Expected Bool object's Value to be boolean literal")
	return b
}

// TryBoolValue tests the target for an boolean literal value and, if it is one, returns its value.
func (o *Object) TryBoolValue() (bool, bool) {
	if o.IsBool() {
		return o.BoolValue(), true
	}
	return false, false
}

// IsNumber returns true if the target object is a runtime number.
func (o *Object) IsNumber() bool {
	return o.t == types.Number
}

// NumberValue asserts that the target is a numeric literal and returns its value.
func (o *Object) NumberValue() float64 {
	contract.Assertf(o.t == types.Number, "Expected object type to be Number; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected Number object to carry a Value; got nil")
	n, ok := o.value.(float64)
	contract.Assertf(ok, "Expected Number object's Value to be numeric literal")
	return n
}

// TryNumberValue tests the target for a number literal value and, if it is one, returns its value.
func (o *Object) TryNumberValue() (float64, bool) {
	if o.IsNumber() {
		return o.NumberValue(), true
	}
	return 0.0, false
}

// IsString returns true if the target object is a runtime string.
func (o *Object) IsString() bool {
	return o.t == types.String
}

// StringValue asserts that the target is a string and returns its value.
func (o *Object) StringValue() string {
	contract.Assertf(o.t == types.String, "Expected object type to be String; got %v", o.t)
	contract.Assertf(o.value != nil, "Expected String object to carry a Value; got nil")
	s, ok := o.value.(string)
	contract.Assertf(ok, "Expected String object's Value to be string")
	return s
}

// TryStringValue tests the target for a string literal value and, if it is one, returns its value.
func (o *Object) TryStringValue() (string, bool) {
	if o.IsString() {
		return o.StringValue(), true
	}
	return "", false
}

// IsFunction returns true if the target object is a runtime function.
func (o *Object) IsFunction() bool {
	_, is := o.t.(*symbols.FunctionType)
	return is
}

// FunctionValue asserts that the target is a function and returns its value.
func (o *Object) FunctionValue() FuncStub {
	contract.Assertf(o.value != nil, "Expected Function object to carry a Value; got nil")
	r, ok := o.value.(FuncStub)
	contract.Assertf(ok, "Expected Function object's Value to be a Function")
	return r
}

// TryFunctionValue tests the target for a function literal value and, if it is one, returns its value.
func (o *Object) TryFunctionValue() (FuncStub, bool) {
	if o.IsFunction() {
		return o.FunctionValue(), true
	}
	return FuncStub{}, false
}

// IsPointer returns true if the target object is a runtime pointer.
func (o *Object) IsPointer() bool {
	_, is := o.t.(*symbols.PointerType)
	return is
}

// PointerValue asserts that the target is a pointer and returns its value.
func (o *Object) PointerValue() *Pointer {
	contract.Assertf(o.value != nil, "Expected Pointer object to carry a Value; got nil")
	r, ok := o.value.(*Pointer)
	contract.Assertf(ok, "Expected Pointer object's Value to be a Pointer")
	return r
}

// TryPointerValue tests the target for a pointer literal value and, if it is one, returns its value.
func (o *Object) TryPointerValue() (*Pointer, bool) {
	if o.IsPointer() {
		return o.PointerValue(), true
	}
	return nil, false
}

// IsNull returns true if the target object is a runtime null value.
func (o *Object) IsNull() bool {
	return o.t == types.Null
}

// IsComputed returns true if the target object is a computed value.
func (o *Object) IsComputed() bool {
	_, is := o.t.(*symbols.ComputedType)
	return is
}

// ComputedValue asserts that the target is a computed and returns its underlying source, if it is available.
func (o *Object) ComputedValue() ComputedStub {
	contract.Assertf(o.value != nil, "Expected Computed object to carry a Value; got nil")
	r, ok := o.value.(ComputedStub)
	contract.Assertf(ok, "Expected Computed object's Value to be a ComputedStub")
	contract.Assert(o.IsComputed())
	return r
}

// TryComputedValue tests the target for a computed value and returns its underlying source, if it is available.
func (o *Object) TryComputedValue() (ComputedStub, bool) {
	if o.IsComputed() {
		return o.ComputedValue(), true
	}
	return ComputedStub{}, false
}

// Details prints the contents of an object deeply, detecting cycles as it goes.
func (o *Object) Details(funcs bool, indent string) string {
	visited := make(map[*Object]bool)
	return indent + o.details(funcs, visited, indent)
}

func (o *Object) details(funcs bool, visited map[*Object]bool, indent string) string {
	if visited[o] {
		return "<cycle>"
	}
	visited[o] = true

	switch o.t {
	case types.Bool:
		if o.BoolValue() {
			return "true"
		}
		return "false"
	case types.String:
		return "\"" + o.StringValue() + "\""
	case types.Number:
		return strconv.FormatFloat(o.NumberValue(), 'f', -1, 64)
	case types.Null:
		return nilString
	default:
		// See if it's a func; if yes, do function formatting.
		if _, isfnc := o.t.(*symbols.FunctionType); isfnc {
			stub := o.FunctionValue()
			var this string
			if stub.This == nil {
				this = nilString
			} else {
				this = stub.This.String()
			}
			s := fmt.Sprintf("func{this=%v,sig=%v", this, stub.Sig)
			if stub.Sym != nil {
				s = fmt.Sprintf("%v,target=%v", s, stub.Sym.Token().String())
			}
			return s + "}"
		}

		// See if it's an array; if yes, do array formatting.
		if _, isarr := o.t.(*symbols.ArrayType); isarr {
			arr := o.ArrayValue()
			s := "[\n"
			if arr != nil {
				elemindent := indent + "    "
				for i := 0; i < len(*arr); i++ {
					elem := (*arr)[i]
					s += elemindent + elem.Obj().details(funcs, visited, elemindent) + "\n"
				}
			}
			return s + indent + "]"
		}

		// See if it's a pointer; if yes, format the reference.
		if _, isptr := o.t.(*symbols.PointerType); isptr {
			return o.PointerValue().String()
		}

		// Otherwise it's an arbitrary object; just print the type (we can't recurse, due to possible cycles).
		s := fmt.Sprintf("@%v{\n", o.t.Token())
		propindent := indent + "    "
		props := o.PropertyValues()
		for _, k := range props.Stable() {
			v := props.Get(k)
			if !funcs {
				// If skipping funcs, check the type and, well, skip them.
				if _, isfnc := v.t.(*symbols.FunctionType); isfnc {
					continue
				}
			}
			s += propindent + string(k) + ": " + v.details(funcs, visited, propindent) + "\n"
		}
		return s + indent + "}"
	}
}

// String can be used to print the contents of an object using a condensed string.  It omits many details that the
// Details function would contain, so that single-line displays are not egregiously lengthy.
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
		return strconv.FormatFloat(o.NumberValue(), 'f', -1, 64)
	case types.Null:
		return nilString
	default:
		// See if it's a func; if yes, do function formatting.
		if _, isfnc := o.t.(*symbols.FunctionType); isfnc {
			stub := o.FunctionValue()
			var this string
			if stub.This == nil {
				this = nilString
			} else {
				this = stub.This.String()
			}
			s := fmt.Sprintf("func{this=%v,sig=%v", this, stub.Sig)
			if stub.Sym != nil {
				s = fmt.Sprintf("%v,target=%v", s, stub.Sym.Token().String())
			}
			return s + "}"
		}

		// See if it's an array; if yes, do array formatting.
		if _, isarr := o.t.(*symbols.ArrayType); isarr {
			arr := o.ArrayValue()
			s := "["
			if arr != nil {
				for i := 0; i < len(*arr); i++ {
					if i > 0 {
						s += ","
					}
					s += (*arr)[i].String()
				}
			}
			return s + "]"
		}

		// See if it's a pointer; if yes, format the reference.
		if _, isptr := o.t.(*symbols.PointerType); isptr {
			return o.PointerValue().String()
		}

		// Otherwise it's an arbitrary object; just print the type (we can't recurse, due to possible cycles).
		return fmt.Sprintf("object{type=%v,props={...}}", o.t.Token())
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

	// Add a `length` property to the object
	arrayProps := NewPropertyMap()
	lengthGetter := NewBuiltinIntrinsic(
		tokens.Token("lumi:builtin/array:getLength"),
		symbols.NewFunctionType([]symbols.Type{}, types.Number),
	)
	lengthSetter := NewBuiltinIntrinsic(
		tokens.Token("lumi:builtin/array:setLength"),
		symbols.NewFunctionType([]symbols.Type{types.Number}, nil),
	)
	arrayProps.InitAddr(PropertyKey("length"), nil, false, lengthGetter, lengthSetter)

	return NewObject(arrt, arr, arrayProps, nil)
}

var trueObj = NewPrimitiveObject(types.Bool, true)
var falseObj = NewPrimitiveObject(types.Bool, false)

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

var nullObj = NewPrimitiveObject(types.Null, nil)

// NewNullObject creates a new null object; null objects are not expected to have distinct identity.
func NewNullObject() *Object {
	return nullObj
}

// NewStringObject creates a new primitive number object.
func NewStringObject(v string) *Object {

	// Add a `length` property to the object
	arrayProps := NewPropertyMap()
	lengthGetter := NewBuiltinIntrinsic(
		tokens.Token("lumi:builtin/string:getLength"),
		symbols.NewFunctionType([]symbols.Type{}, types.Number),
	)
	arrayProps.InitAddr(PropertyKey("length"), nil, true, lengthGetter, nil)

	stringProto := StringPrototypeObject()

	return NewObject(types.String, v, arrayProps, stringProto)
}

// stringProto is a cached reference to the String prototype object
var stringProto *Object

// StringPrototypeObject returns the String prototype object
func StringPrototypeObject() *Object {
	if stringProto != nil {
		return stringProto
	}

	stringProtoProps := NewPropertyMap()
	stringProto = NewObject(types.String, "", stringProtoProps, nil)
	toLowerCase := NewFunctionObjectFromSymbol(NewBuiltinIntrinsic(
		tokens.Token("lumi:builtin/string:toLowerCase"),
		symbols.NewFunctionType([]symbols.Type{}, types.String),
	), stringProto)
	stringProtoProps.InitAddr(PropertyKey("toLowerCase"), toLowerCase, true, nil, nil)

	return stringProto
}

// NewFunctionObject creates a new function object out of consistuent parts.
func NewFunctionObject(stub FuncStub) *Object {
	contract.Assert(stub.Sig != nil)
	return NewObject(stub.Sig, stub, nil, nil)
}

// NewFunctionObjectFromSymbol creates a new function object for a given function symbol.
func NewFunctionObjectFromSymbol(fnc symbols.Function, this *Object) *Object {
	return NewFunctionObject(FuncStub{
		Func: fnc.Function(),
		Sym:  fnc,
		Sig:  fnc.Signature(),
		This: this,
	})
}

// NewFunctionObjectFromLambda creates a new function object with very specific underlying parts.
func NewFunctionObjectFromLambda(fnc ast.Function, sig *symbols.FunctionType, env Environment) *Object {
	return NewFunctionObject(FuncStub{
		Func: fnc,
		Sig:  sig,
		Env:  env,
	})
}

// FuncStub is a stub that captures a symbol plus an optional instance 'this' object.
type FuncStub struct {
	Func ast.Function          // the function whose body AST to evaluate.
	Sym  symbols.Function      // an optional function symbol that this AST belongs to.
	Sig  *symbols.FunctionType // the function type representing this function's signature.
	This *Object               // an optional "this" pointer to bind when invoking this function.
	Env  Environment           // an optional environment to evaluate this function inside.
}

// NewPointerObject allocates a new pointer-like object that wraps the given reference.
func NewPointerObject(elem symbols.Type, ptr *Pointer) *Object {
	contract.Require(elem != nil, "elem")
	contract.Require(ptr != nil, "ptr")
	ptrt := symbols.NewPointerType(elem)
	return NewPrimitiveObject(ptrt, ptr)
}

// NewComputedObject allocates a new computed object that wraps the given reference, with the given src object being
// responsible for resolving the computation (if any).
func NewComputedObject(elem symbols.Type, expr bool, sources []*Object) *Object {
	contract.Assert(expr || len(sources) == 1)
	stub := ComputedStub{
		Expr:    expr,
		Sources: sources,
	}
	return NewObject(symbols.NewComputedType(elem), stub, nil, nil)
}

// ComputedStub captures information about a computed value.
type ComputedStub struct {
	Expr    bool      // true if this is a sophisticated expression.
	Sources []*Object // the list of sources from which the computed value derives.
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
		// IDEA: we could support more here (essentially, anything that is JSON serializable).
		contract.Failf("Unrecognized constant data literal: %v", data)
		return nil
	}
}

// FreezeReadonlyProperties freezes the readonly fields of an object, possibly copying properties down from the
// prototype chain as necessary to accomplish the freezing (since modifying the prototype chain would clearly be wrong).
// TODO[pulumi/lumi#70]: this could cause subtle compatibility issues; e.g., it's possible to write to the prototype
//     for an inherited property postconstruction; in vanilla ECMAscript, that write would be seen; in LumiJS it won't.
func (o *Object) FreezeReadonlyProperties() {
	current := o.Type()
	for current != nil {
		members := current.TypeMembers()
		for _, member := range symbols.StableClassMemberMap(members) {
			if m := members[member]; !m.Static() {
				if prop, isprop := m.(*symbols.ClassProperty); isprop && prop.Readonly() {
					ptr := o.GetPropertyAddr(PropertyKey(member), true, true)
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
func (o *Object) GetPropertyAddr(key PropertyKey, init bool, direct bool) *Pointer {
	return o.GetPropertyAddrForThis(o, key, init, direct)
}

// GetPropertyAddrForThis locates a property with the given key, similar to GetPropertyAddr, except that it ensures the
// resulting pointer references the target `this` object (e.g., when loading a prototype function member).
func (o *Object) GetPropertyAddrForThis(this *Object, key PropertyKey, init bool, direct bool) *Pointer {
	var ptr *Pointer  // the resulting pointer to return
	var where *Object // the object the pointer was found on

	// If it's in the object's property map already, just return it.
	if ptr = o.Properties().GetAddr(key); ptr != nil {
		where = o
	} else {
		// Otherwise, consult the prototype chain.
		proto := o.Proto()
		for proto != nil {
			if ptr = proto.Properties().GetAddr(key); ptr != nil {
				where = proto
				break
			}
			proto = proto.Proto()
		}
	}

	if ptr == nil {
		// If we didn't find anything, and were asked to initialize, do so now.
		if init {
			ptr = this.Properties().InitAddr(key, nil, false, nil, nil)
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
			ptr = this.Properties().InitAddr(key, ptr.Obj(), ptr.Readonly(), ptr.Getter(), ptr.Setter())
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
func (o *Object) PropertyValues() *PropertyMap {
	properties := NewPropertyMap()

	// First add our own object properties.
	for _, k := range o.properties.Stable() {
		properties.SetFrom(k, o.properties.GetAddr(k))
	}

	// Now walk the prototype hierarchy and, for anything that doesn't already exist, adjust and add it.
	proto := o.proto
	for proto != nil {
		for _, k := range proto.properties.Stable() {
			if _, has := properties.TryGetAddr(k); !has {
				v := proto.properties.GetAddr(k)
				v = adjustPointerForThis(proto, o, v)
				properties.SetFrom(k, v)
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
				value = NewFunctionObject(FuncStub{
					Func: stub.Func,
					Sym:  stub.Sym,
					Sig:  stub.Sig,
					This: this,
					Env:  stub.Env,
				})
				prop = NewPointer(value, true, prop.Getter(), prop.Setter())
			}
		}
	}
	return prop
}

// Intrinsic is a special intrinsic function whose behavior is implemented by the runtime.
type Intrinsic struct {
	tok  tokens.Token
	sig  *symbols.FunctionType
	fnc  symbols.Function // the underlying function AST (before mapping to an intrinsic).
	node ast.Function
}

var _ symbols.Function = (*Intrinsic)(nil)

func (intrin *Intrinsic) Name() tokens.Name                { return tokens.Name(intrin.tok.Name()) }
func (intrin *Intrinsic) Token() tokens.Token              { return intrin.tok }
func (intrin *Intrinsic) Special() bool                    { return false }
func (intrin *Intrinsic) SpecialModInit() bool             { return false }
func (intrin *Intrinsic) Tree() diag.Diagable              { return intrin.node }
func (intrin *Intrinsic) Function() ast.Function           { return intrin.node }
func (intrin *Intrinsic) Signature() *symbols.FunctionType { return intrin.sig }
func (intrin *Intrinsic) String() string                   { return string(intrin.Name()) }

func (intrin *Intrinsic) UnderlyingSymbol() symbols.Function { return intrin.fnc }

// NewIntrinsic returns a new intrinsic function symbol with the given information.
func NewIntrinsic(fnc symbols.Function) *Intrinsic {
	return &Intrinsic{
		tok:  fnc.Token(),
		sig:  fnc.Signature(),
		fnc:  fnc,
		node: fnc.Function(),
	}
}

// NewBuiltinIntrinsic returns a new intrinsic function symbol for an intrinsic
// defined within the runtime with no corresponding AST.
func NewBuiltinIntrinsic(token tokens.Token, signature *symbols.FunctionType) *Intrinsic {
	return &Intrinsic{
		tok:  token,
		sig:  signature,
		fnc:  nil,
		node: nil,
	}
}
