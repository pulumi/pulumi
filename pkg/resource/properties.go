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

package resource

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
)

// PropertyKey is the name of a property.
type PropertyKey tokens.Name

// PropertySet is a simple set keyed by property name.
type PropertySet map[PropertyKey]bool

// PropertyMap is a simple map keyed by property name with "JSON-like" values.
type PropertyMap map[PropertyKey]PropertyValue

// NewPropertyMap turns a struct into a property map, using any JSON tags inside to determine naming.
func NewPropertyMap(s interface{}) PropertyMap {
	m, err := mapper.Unmap(s)
	contract.Assertf(err == nil, "Struct of properties failed to map correctly: %v", err)
	return NewPropertyMapFromMap(m)
}

func NewPropertyMapFromMap(m map[string]interface{}) PropertyMap {
	result := make(PropertyMap)
	for k, v := range m {
		result[PropertyKey(k)] = NewPropertyValue(v)
	}
	return result
}

// PropertyValue is the value of a property, limited to a select few types (see below).
type PropertyValue struct {
	V interface{}
}

// Computed represents the absence of a property value, because it will be computed at some point in the future.  It
// contains a property value which represents the underlying expected type of the eventual property value.
type Computed PropertyValue

// Eventual reflects the eventual type of property value that a computed property will contain.
func (v Computed) Eventual() PropertyValue {
	return PropertyValue(v)
}

// Output is a property value that will eventually be computed by the resource provider.  If an output property is
// encountered, it means the resource has not yet been created, and so the output value is unavailable.  Note that an
// output property is a special case of computed, but carries additional semantic meaning.
type Output PropertyValue

// Eventual reflects the eventual type of property value that an output property will contain.
func (v Output) Eventual() PropertyValue {
	return PropertyValue(v)
}

type ReqError struct {
	K PropertyKey
}

func IsReqError(err error) bool {
	_, isreq := err.(*ReqError)
	return isreq
}

func (err *ReqError) Error() string {
	return fmt.Sprintf("required property '%v' is missing", err.K)
}

// BoolOrErr checks that the given property has the type bool, issuing an error if not; req indicates if required.
func (m PropertyMap) BoolOrErr(k PropertyKey, req bool) (*bool, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsBool() {
			return nil, errors.Errorf("property '%v' is not a bool (%v)", k, reflect.TypeOf(v.V))
		}
		b := v.BoolValue()
		return &b, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// NumberOrErr checks that the given property has the type float64, issuing an error if not; req indicates if required.
func (m PropertyMap) NumberOrErr(k PropertyKey, req bool) (*float64, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsNumber() {
			return nil, errors.Errorf("property '%v' is not a number (%v)", k, reflect.TypeOf(v.V))
		}
		n := v.NumberValue()
		return &n, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// StringOrErr checks that the given property has the type string, issuing an error if not; req indicates if required.
func (m PropertyMap) StringOrErr(k PropertyKey, req bool) (*string, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsString() {
			return nil, errors.Errorf("property '%v' is not a string (%v)", k, reflect.TypeOf(v.V))
		}
		s := v.StringValue()
		return &s, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ArrayOrErr checks that the given property has the type array, issuing an error if not; req indicates if required.
func (m PropertyMap) ArrayOrErr(k PropertyKey, req bool) (*[]PropertyValue, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsArray() {
			return nil, errors.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
		}
		a := v.ArrayValue()
		return &a, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ObjectArrayOrErr ensures a property is an array of objects, issuing an error if not; req indicates if required.
func (m PropertyMap) ObjectArrayOrErr(k PropertyKey, req bool) (*[]PropertyMap, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsArray() {
			return nil, errors.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
		}
		a := v.ArrayValue()
		var objs []PropertyMap
		for i, e := range a {
			if e.IsObject() {
				objs = append(objs, e.ObjectValue())
			} else {
				return nil, errors.Errorf(
					"property '%v' array element %v is not an object (%v)", k, i, reflect.TypeOf(e))
			}
		}
		return &objs, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// StringArrayOrErr ensures a property is an array of strings, issuing an error if not; req indicates if required.
func (m PropertyMap) StringArrayOrErr(k PropertyKey, req bool) (*[]string, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsArray() {
			return nil, errors.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
		}
		a := v.ArrayValue()
		var strs []string
		for i, e := range a {
			if e.IsString() {
				strs = append(strs, e.StringValue())
			} else {
				return nil, errors.Errorf(
					"property '%v' array element %v is not a string (%v)", k, i, reflect.TypeOf(e))
			}
		}
		return &strs, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ObjectOrErr checks that the given property is an object, issuing an error if not; req indicates if required.
func (m PropertyMap) ObjectOrErr(k PropertyKey, req bool) (*PropertyMap, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsObject() {
			return nil, errors.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
		}
		o := v.ObjectValue()
		return &o, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ResourceOrErr checks that the given property is a resource, issuing an error if not; req indicates if required.
func (m PropertyMap) ResourceOrErr(k PropertyKey, req bool) (*URN, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsResource() {
			return nil, errors.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
		}
		m := v.ResourceValue()
		return &m, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ComputedOrErr checks that the given property is computed, issuing an error if not; req indicates if required.
func (m PropertyMap) ComputedOrErr(k PropertyKey, req bool) (*Computed, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsComputed() {
			return nil, errors.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
		}
		m := v.ComputedValue()
		return &m, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// OutputOrErr checks that the given property is an output, issuing an error if not; req indicates if required.
func (m PropertyMap) OutputOrErr(k PropertyKey, req bool) (*Output, error) {
	if v, has := m[k]; has && !v.IsNull() {
		if !v.IsOutput() {
			return nil, errors.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
		}
		m := v.OutputValue()
		return &m, nil
	} else if req {
		return nil, &ReqError{k}
	}
	return nil, nil
}

// ReqBoolOrErr checks that the given property exists and has the type bool.
func (m PropertyMap) ReqBoolOrErr(k PropertyKey) (bool, error) {
	b, err := m.BoolOrErr(k, true)
	if err != nil {
		return false, err
	}
	return *b, nil
}

// ReqNumberOrErr checks that the given property exists and has the type float64.
func (m PropertyMap) ReqNumberOrErr(k PropertyKey) (float64, error) {
	n, err := m.NumberOrErr(k, true)
	if err != nil {
		return 0, err
	}
	return *n, nil
}

// ReqStringOrErr checks that the given property exists and has the type string.
func (m PropertyMap) ReqStringOrErr(k PropertyKey) (string, error) {
	s, err := m.StringOrErr(k, true)
	if err != nil {
		return "", err
	}
	return *s, nil
}

// ReqArrayOrErr checks that the given property exists and has the type array.
func (m PropertyMap) ReqArrayOrErr(k PropertyKey) ([]PropertyValue, error) {
	a, err := m.ArrayOrErr(k, true)
	if err != nil {
		return nil, err
	}
	return *a, nil
}

// ReqObjectArrayOrErr checks that the given property exists and has the type array of objects.
func (m PropertyMap) ReqObjectArrayOrErr(k PropertyKey) ([]PropertyMap, error) {
	a, err := m.ObjectArrayOrErr(k, true)
	if err != nil {
		return nil, err
	}
	return *a, nil
}

// ReqStringArrayOrErr checks that the given property exists and has the type array of objects.
func (m PropertyMap) ReqStringArrayOrErr(k PropertyKey) ([]string, error) {
	a, err := m.StringArrayOrErr(k, true)
	if err != nil {
		return nil, err
	}
	return *a, nil
}

// ReqObjectOrErr checks that the given property exists and has the type object.
func (m PropertyMap) ReqObjectOrErr(k PropertyKey) (PropertyMap, error) {
	o, err := m.ObjectOrErr(k, true)
	if err != nil {
		return nil, err
	}
	return *o, nil
}

// ReqResourceOrErr checks that the given property exists and has the type URN.
func (m PropertyMap) ReqResourceOrErr(k PropertyKey) (URN, error) {
	r, err := m.ResourceOrErr(k, true)
	if err != nil {
		return URN(""), err
	}
	return *r, nil
}

// ReqComputedOrErr checks that the given property exists and is computed.
func (m PropertyMap) ReqComputedOrErr(k PropertyKey) (Computed, error) {
	v, err := m.ComputedOrErr(k, true)
	if err != nil {
		return Computed{}, err
	}
	return *v, nil
}

// ReqOutputOrErr checks that the given property exists and is an output property.
func (m PropertyMap) ReqOutputOrErr(k PropertyKey) (Output, error) {
	v, err := m.OutputOrErr(k, true)
	if err != nil {
		return Output{}, err
	}
	return *v, nil
}

// OptBoolOrErr checks that the given property has the type bool, if it exists.
func (m PropertyMap) OptBoolOrErr(k PropertyKey) (*bool, error) {
	return m.BoolOrErr(k, false)
}

// OptNumberOrErr checks that the given property has the type float64, if it exists.
func (m PropertyMap) OptNumberOrErr(k PropertyKey) (*float64, error) {
	return m.NumberOrErr(k, false)
}

// OptStringOrErr checks that the given property has the type string, if it exists.
func (m PropertyMap) OptStringOrErr(k PropertyKey) (*string, error) {
	return m.StringOrErr(k, false)
}

// OptArrayOrErr checks that the given property has the type array, if it exists.
func (m PropertyMap) OptArrayOrErr(k PropertyKey) (*[]PropertyValue, error) {
	return m.ArrayOrErr(k, false)
}

// OptObjectArrayOrErr checks that the given property has the type array of objects, if it exists.
func (m PropertyMap) OptObjectArrayOrErr(k PropertyKey) (*[]PropertyMap, error) {
	return m.ObjectArrayOrErr(k, false)
}

// OptStringArrayOrErr checks that the given property has the type array of objects, if it exists.
func (m PropertyMap) OptStringArrayOrErr(k PropertyKey) (*[]string, error) {
	return m.StringArrayOrErr(k, false)
}

// OptObjectOrErr checks that the given property has the type object, if it exists.
func (m PropertyMap) OptObjectOrErr(k PropertyKey) (*PropertyMap, error) {
	return m.ObjectOrErr(k, false)
}

// OptResourceOrErr checks that the given property has the type URN, if it exists.
func (m PropertyMap) OptResourceOrErr(k PropertyKey) (*URN, error) {
	return m.ResourceOrErr(k, false)
}

// OptComputedOrErr checks that the given property is computed, if it exists.
func (m PropertyMap) OptComputedOrErr(k PropertyKey) (*Computed, error) {
	return m.ComputedOrErr(k, false)
}

// OptOutputOrErr checks that the given property is an output property, if it exists.
func (m PropertyMap) OptOutputOrErr(k PropertyKey) (*Output, error) {
	return m.OutputOrErr(k, false)
}

// HasValue returns true if the slot associated with the given property key contains a real value.  It returns false
// if a value is null or an output property that is awaiting a value to be assigned.  That is to say, HasValue indicates
// a semantically meaningful value is present (even if it's a computed one whose concrete value isn't yet evaluated).
func (m PropertyMap) HasValue(k PropertyKey) bool {
	v, has := m[k]
	return has && v.HasValue()
}

// Mappable returns a mapper-compatible object map, suitable for deserialization into structures.
func (m PropertyMap) Mappable() map[string]interface{} {
	obj := make(map[string]interface{})
	for _, k := range StablePropertyKeys(m) {
		obj[string(k)] = m[k].Mappable()
	}
	return obj
}

// AllResources finds all resource URNs, transitively throughout the property map, and returns them.
func (m PropertyMap) AllResources() map[URN]bool {
	URNs := make(map[URN]bool)
	for _, k := range StablePropertyKeys(m) {
		for m, v := range m[k].AllResources() {
			URNs[m] = v
		}
	}
	return URNs
}

// ReplaceResources finds all resources and lets an updater function update them if necessary.  This is often used
// during a "replacement"-style updated, to replace all URNs of a certain value with another.
func (m PropertyMap) ReplaceResources(updater func(URN) URN) PropertyMap {
	result := make(PropertyMap)
	for _, k := range StablePropertyKeys(m) {
		result[k] = m[k].ReplaceResources(updater)
	}
	return result
}

func (m PropertyMap) ShallowClone() PropertyMap {
	copy := make(PropertyMap)
	m.ShallowCloneInto(copy)
	return copy
}

func (m PropertyMap) ShallowCloneInto(other PropertyMap) {
	for k, v := range m {
		other[k] = v
	}
}

func (s PropertySet) ShallowClone() PropertySet {
	copy := make(PropertySet)
	for k, v := range s {
		copy[k] = v
	}
	return copy
}

func NewNullProperty() PropertyValue                   { return PropertyValue{nil} }
func NewBoolProperty(v bool) PropertyValue             { return PropertyValue{v} }
func NewNumberProperty(v float64) PropertyValue        { return PropertyValue{v} }
func NewStringProperty(v string) PropertyValue         { return PropertyValue{v} }
func NewArrayProperty(v []PropertyValue) PropertyValue { return PropertyValue{v} }
func NewObjectProperty(v PropertyMap) PropertyValue    { return PropertyValue{v} }
func NewResourceProperty(v URN) PropertyValue          { return PropertyValue{v} }
func NewComputedProperty(v Computed) PropertyValue     { return PropertyValue{v} }
func NewOutputProperty(v Output) PropertyValue         { return PropertyValue{v} }

func MakeComputed(v PropertyValue) PropertyValue { return NewComputedProperty(Computed(v)) }
func MakeOutput(v PropertyValue) PropertyValue   { return NewOutputProperty(Output(v)) }

// NewPropertyValue turns a value into a property value, provided it is of a legal "JSON-like" kind.
func NewPropertyValue(v interface{}) PropertyValue {
	// If nil, easy peasy, just return a null.
	if v == nil {
		return NewNullProperty()
	}

	// Else, check for some known primitive types.
	switch t := v.(type) {
	case bool:
		return NewBoolProperty(t)
	case float64:
		return NewNumberProperty(t)
	case string:
		return NewStringProperty(t)
	case URN:
		return NewResourceProperty(t)
	case Computed:
		return NewComputedProperty(t)
	}

	// Next, see if it's an array, slice, pointer or struct, and handle each accordingly.
	rv := reflect.ValueOf(v)
	switch rk := rv.Type().Kind(); rk {
	case reflect.Array, reflect.Slice:
		// If an array or slice, just create an array out of it.
		var arr []PropertyValue
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i)
			arr = append(arr, NewPropertyValue(elem.Interface()))
		}
		return NewArrayProperty(arr)
	case reflect.Ptr:
		// If a pointer, recurse and return the underlying value.
		if rv.IsNil() {
			return NewNullProperty()
		}
		return NewPropertyValue(rv.Elem().Interface())
	case reflect.Map:
		// If a map, create a new property map, provided the keys and values are okay.
		obj := PropertyMap{}
		for _, key := range rv.MapKeys() {
			var pk PropertyKey
			switch k := key.Interface().(type) {
			case string:
				pk = PropertyKey(k)
			case PropertyKey:
				pk = k
			default:
				contract.Failf("Unrecognized PropertyMap key type: %v", reflect.TypeOf(key))
			}
			val := rv.MapIndex(key)
			pv := NewPropertyValue(val.Interface())
			obj[pk] = pv
		}
		return NewObjectProperty(obj)
	case reflect.Struct:
		obj := NewPropertyMap(v)
		return NewObjectProperty(obj)
	default:
		contract.Failf("Unrecognized value type: type=%v kind=%v", rv.Type(), rk)
	}

	return NewNullProperty()
}

// BoolValue fetches the underlying bool value (panicking if it isn't a bool).
func (v PropertyValue) BoolValue() bool { return v.V.(bool) }

// NumberValue fetches the underlying number value (panicking if it isn't a number).
func (v PropertyValue) NumberValue() float64 { return v.V.(float64) }

// StringValue fetches the underlying string value (panicking if it isn't a string).
func (v PropertyValue) StringValue() string { return v.V.(string) }

// ArrayValue fetches the underlying array value (panicking if it isn't a array).
func (v PropertyValue) ArrayValue() []PropertyValue { return v.V.([]PropertyValue) }

// ObjectValue fetches the underlying object value (panicking if it isn't a object).
func (v PropertyValue) ObjectValue() PropertyMap { return v.V.(PropertyMap) }

// ResourceValue fetches the underlying resource value (panicking if it isn't a resource).
func (v PropertyValue) ResourceValue() URN { return v.V.(URN) }

// ComputedValue fetches the underlying computed value (panicking if it isn't a computed).
func (v PropertyValue) ComputedValue() Computed { return v.V.(Computed) }

// OutputValue fetches the underlying output value (panicking if it isn't a output).
func (v PropertyValue) OutputValue() Output { return v.V.(Output) }

// IsNull returns true if the underlying value is a null.
func (v PropertyValue) IsNull() bool {
	return v.V == nil
}

// IsBool returns true if the underlying value is a bool.
func (v PropertyValue) IsBool() bool {
	_, is := v.V.(bool)
	return is
}

// IsNumber returns true if the underlying value is a number.
func (v PropertyValue) IsNumber() bool {
	_, is := v.V.(float64)
	return is
}

// IsString returns true if the underlying value is a string.
func (v PropertyValue) IsString() bool {
	_, is := v.V.(string)
	return is
}

// IsArray returns true if the underlying value is an array.
func (v PropertyValue) IsArray() bool {
	_, is := v.V.([]PropertyValue)
	return is
}

// IsObject returns true if the underlying value is an object.
func (v PropertyValue) IsObject() bool {
	_, is := v.V.(PropertyMap)
	return is
}

// IsResource returns true if the underlying value is a resource.
func (v PropertyValue) IsResource() bool {
	_, is := v.V.(URN)
	return is
}

// IsComputed returns true if the underlying value is a computed value.
func (v PropertyValue) IsComputed() bool {
	_, is := v.V.(Computed)
	return is
}

// IsOutput returns true if the underlying value is an output value.
func (v PropertyValue) IsOutput() bool {
	_, is := v.V.(Output)
	return is
}

// CanNull returns true if the target property is capable of holding a null value.
func (v PropertyValue) CanNull() bool {
	return true // all properties can be null
}

// CanBool returns true if the target property is capable of holding a bool value.
func (v PropertyValue) CanBool() bool {
	if v.IsNull() || v.IsBool() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanBool()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanBool()
	}
	return false
}

// CanNumber returns true if the target property is capable of holding a number value.
func (v PropertyValue) CanNumber() bool {
	if v.IsNull() || v.IsNumber() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanNumber()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanNumber()
	}
	return false
}

// CanString returns true if the target property is capable of holding a string value.
func (v PropertyValue) CanString() bool {
	if v.IsNull() || v.IsString() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanString()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanString()
	}
	return false
}

// CanArray returns true if the target property is capable of holding an array value.
func (v PropertyValue) CanArray() bool {
	if v.IsNull() || v.IsArray() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanArray()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanArray()
	}
	return false
}

// CanObject returns true if the target property is capable of holding an object value.
func (v PropertyValue) CanObject() bool {
	if v.IsNull() || v.IsObject() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanObject()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanObject()
	}
	return false
}

// CanResource returns true if the target property is capable of holding a resource value.
func (v PropertyValue) CanResource() bool {
	if v.IsNull() || v.IsResource() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Eventual().CanResource()
	}
	if v.IsOutput() {
		return v.OutputValue().Eventual().CanResource()
	}
	return false
}

// HasValue returns true if a value is semantically meaningful.
func (v PropertyValue) HasValue() bool {
	return !v.IsNull() && !v.IsOutput()
}

// TypeString returns a type representation of the property value's holder type.
func (v PropertyValue) TypeString() string {
	if v.IsNull() {
		return "null"
	} else if v.IsBool() {
		return "bool"
	} else if v.IsNumber() {
		return "number"
	} else if v.IsString() {
		return "string"
	} else if v.IsArray() {
		return "[]"
	} else if v.IsObject() {
		return "object"
	} else if v.IsResource() {
		return "resource"
	} else if v.IsComputed() {
		return "computed<" + v.ComputedValue().Eventual().TypeString() + ">"
	} else if v.IsOutput() {
		return "output<" + v.OutputValue().Eventual().TypeString() + ">"
	}
	contract.Failf("Unrecognized PropertyValue type")
	return ""
}

// Mappable returns a mapper-compatible value, suitable for deserialization into structures.
func (v PropertyValue) Mappable() interface{} {
	if v.IsNull() {
		return nil
	} else if v.IsBool() {
		return v.BoolValue()
	} else if v.IsNumber() {
		return v.NumberValue()
	} else if v.IsString() {
		return v.StringValue()
	} else if v.IsArray() {
		var arr []interface{}
		for _, e := range v.ArrayValue() {
			arr = append(arr, e.Mappable())
		}
		return arr
	}
	contract.Assert(v.IsObject())
	return v.ObjectValue().Mappable()
}

// String implements the fmt.Stringer interface to add slightly more information to the output.
func (v PropertyValue) String() string {
	if v.IsComputed() || v.IsOutput() {
		// For computed and output properties, show their type followed by an empty object string.
		return fmt.Sprintf("%v{}", v.TypeString())
	}
	// For all others, just display the underlying property value.
	return fmt.Sprintf("{%v}", v.V)
}

// AllResources finds all resource URNs, transitively throughout the property value, and returns them.
func (v PropertyValue) AllResources() map[URN]bool {
	URNs := make(map[URN]bool)
	if v.IsResource() {
		URNs[v.ResourceValue()] = true
	} else if v.IsArray() {
		for _, elem := range v.ArrayValue() {
			for m, v := range elem.AllResources() {
				URNs[m] = v
			}
		}
	} else if v.IsObject() {
		for m, v := range v.ObjectValue().AllResources() {
			URNs[m] = v
		}
	}
	return URNs
}

// ReplaceResources finds all resources and lets an updater function update them if necessary.  This is often used
// during a "replacement"-style updated, to replace all URNs of a certain value with another.
func (v PropertyValue) ReplaceResources(updater func(URN) URN) PropertyValue {
	if v.IsResource() {
		m := v.ResourceValue()
		return NewResourceProperty(updater(m))
	} else if v.IsArray() {
		arr := v.ArrayValue()
		elems := make([]PropertyValue, len(arr))
		for i, elem := range arr {
			elems[i] = elem.ReplaceResources(updater)
		}
		return NewArrayProperty(elems)
	} else if v.IsObject() {
		rep := v.ObjectValue().ReplaceResources(updater)
		return NewObjectProperty(rep)
	}
	return v
}
