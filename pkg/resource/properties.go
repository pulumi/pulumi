// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"fmt"
	"reflect"
	"sort"

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
	return NewPropertyMapRepl(s, nil, nil)
}

// NewPropertyMapRepl turns a struct into a property map, using any JSON tags inside to determine naming.  If non-nil
// replk or replv function(s) are provided, key and/or value transformations are performed during the mapping.
func NewPropertyMapRepl(s interface{},
	replk func(string) (PropertyKey, bool), replv func(interface{}) (PropertyValue, bool)) PropertyMap {
	m, err := mapper.Unmap(s)
	contract.Assertf(err == nil, "Struct of properties failed to map correctly: %v", err)
	return NewPropertyMapFromMapRepl(m, replk, replv)
}

// NewPropertyMapFromMap creates a resource map from a regular weakly typed JSON-like map.
func NewPropertyMapFromMap(m map[string]interface{}) PropertyMap {
	return NewPropertyMapFromMapRepl(m, nil, nil)
}

// NewPropertyMapFromMapRepl optionally replaces keys/values in an existing map while creating a new resource map.
func NewPropertyMapFromMapRepl(m map[string]interface{},
	replk func(string) (PropertyKey, bool), replv func(interface{}) (PropertyValue, bool)) PropertyMap {
	result := make(PropertyMap)
	for k, v := range m {
		key := PropertyKey(k)
		if replk != nil {
			if rk, repl := replk(k); repl {
				key = rk
			}
		}
		result[key] = NewPropertyValueRepl(v, replk, replv)
	}
	return result
}

// PropertyValue is the value of a property, limited to a select few types (see below).
type PropertyValue struct {
	V interface{}
}

// Computed represents the absence of a property value, because it will be computed at some point in the future.  It
// contains a property value which represents the underlying expected type of the eventual property value.
type Computed struct {
	Element PropertyValue // the eventual value (type) of the computed property.
}

// Output is a property value that will eventually be computed by the resource provider.  If an output property is
// encountered, it means the resource has not yet been created, and so the output value is unavailable.  Note that an
// output property is a special case of computed, but carries additional semantic meaning.
type Output struct {
	Element PropertyValue // the eventual value (type) of the output property.
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
	return m.MapRepl(nil, nil)
}

// MapRepl returns a mapper-compatible object map, suitable for deserialization into structures.  A key and/or value
// replace function, replk/replv, may be passed that will replace elements using custom logic if appropriate.
func (m PropertyMap) MapRepl(replk func(string) (string, bool),
	replv func(PropertyValue) (interface{}, bool)) map[string]interface{} {
	obj := make(map[string]interface{})
	for _, k := range m.StableKeys() {
		key := string(k)
		if replk != nil {
			if rk, repk := replk(key); repk {
				key = rk
			}
		}
		obj[key] = m[k].MapRepl(replk, replv)
	}
	return obj
}

// StableKeys returns all of the map's keys in a stable order.
func (m PropertyMap) StableKeys() []PropertyKey {
	sorted := make([]PropertyKey, 0, len(m))
	for k := range m {
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted
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
func NewComputedProperty(v Computed) PropertyValue     { return PropertyValue{v} }
func NewOutputProperty(v Output) PropertyValue         { return PropertyValue{v} }

func MakeComputed(v PropertyValue) PropertyValue {
	return NewComputedProperty(Computed{Element: v})
}

func MakeOutput(v PropertyValue) PropertyValue {
	return NewOutputProperty(Output{Element: v})
}

// NewPropertyValue turns a value into a property value, provided it is of a legal "JSON-like" kind.
func NewPropertyValue(v interface{}) PropertyValue {
	return NewPropertyValueRepl(v, nil, nil)
}

// NewPropertyValueRepl turns a value into a property value, provided it is of a legal "JSON-like" kind.  The
// replacement functions, replk and replv, may be supplied to transform keys and/or values as the mapping takes place.
func NewPropertyValueRepl(v interface{},
	replk func(string) (PropertyKey, bool), replv func(interface{}) (PropertyValue, bool)) PropertyValue {
	// If a replacement routine is supplied, use that.
	if replv != nil {
		if rv, repl := replv(v); repl {
			return rv
		}
	}

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
			arr = append(arr, NewPropertyValueRepl(elem.Interface(), replk, replv))
		}
		return NewArrayProperty(arr)
	case reflect.Ptr:
		// If a pointer, recurse and return the underlying value.
		if rv.IsNil() {
			return NewNullProperty()
		}
		return NewPropertyValueRepl(rv.Elem().Interface(), replk, replv)
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
			if replk != nil {
				if rk, repl := replk(string(pk)); repl {
					pk = rk
				}
			}
			val := rv.MapIndex(key)
			pv := NewPropertyValueRepl(val.Interface(), replk, replv)
			obj[pk] = pv
		}
		return NewObjectProperty(obj)
	case reflect.Struct:
		obj := NewPropertyMapRepl(v, replk, replv)
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
		return v.ComputedValue().Element.CanBool()
	}
	if v.IsOutput() {
		return v.OutputValue().Element.CanBool()
	}
	return false
}

// CanNumber returns true if the target property is capable of holding a number value.
func (v PropertyValue) CanNumber() bool {
	if v.IsNull() || v.IsNumber() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Element.CanNumber()
	}
	if v.IsOutput() {
		return v.OutputValue().Element.CanNumber()
	}
	return false
}

// CanString returns true if the target property is capable of holding a string value.
func (v PropertyValue) CanString() bool {
	if v.IsNull() || v.IsString() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Element.CanString()
	}
	if v.IsOutput() {
		return v.OutputValue().Element.CanString()
	}
	return false
}

// CanArray returns true if the target property is capable of holding an array value.
func (v PropertyValue) CanArray() bool {
	if v.IsNull() || v.IsArray() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Element.CanArray()
	}
	if v.IsOutput() {
		return v.OutputValue().Element.CanArray()
	}
	return false
}

// CanObject returns true if the target property is capable of holding an object value.
func (v PropertyValue) CanObject() bool {
	if v.IsNull() || v.IsObject() {
		return true
	}
	if v.IsComputed() {
		return v.ComputedValue().Element.CanObject()
	}
	if v.IsOutput() {
		return v.OutputValue().Element.CanObject()
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
	} else if v.IsComputed() {
		return "computed<" + v.ComputedValue().Element.TypeString() + ">"
	} else if v.IsOutput() {
		return "output<" + v.OutputValue().Element.TypeString() + ">"
	}
	contract.Failf("Unrecognized PropertyValue type")
	return ""
}

// Mappable returns a mapper-compatible value, suitable for deserialization into structures.
func (v PropertyValue) Mappable() interface{} {
	return v.MapRepl(nil, nil)
}

// MapRepl returns a mapper-compatible object map, suitable for deserialization into structures.  A key and/or value
// replace function, replk/replv, may be passed that will replace elements using custom logic if appropriate.
func (v PropertyValue) MapRepl(replk func(string) (string, bool),
	replv func(PropertyValue) (interface{}, bool)) interface{} {
	if replv != nil {
		if rv, repv := replv(v); repv {
			return rv
		}
	}
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
			arr = append(arr, e.MapRepl(replk, replv))
		}
		return arr
	}
	contract.Assert(v.IsObject())
	return v.ObjectValue().MapRepl(replk, replv)
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

// Property is a pair of key and value.
type Property struct {
	Key   PropertyKey
	Value PropertyValue
}
