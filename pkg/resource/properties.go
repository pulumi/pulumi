// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"fmt"
	"reflect"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
)

// PropertyKey is the name of a property.
type PropertyKey tokens.Name

// PropertyMap is a simple map keyed by property name with "JSON-like" values.
type PropertyMap map[PropertyKey]PropertyValue

// PropertyValue is the value of a property, limited to a select few types (see below).
type PropertyValue struct {
	V interface{}
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
			return nil, fmt.Errorf("property '%v' is not a bool (%v)", k, reflect.TypeOf(v.V))
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
			return nil, fmt.Errorf("property '%v' is not a number (%v)", k, reflect.TypeOf(v.V))
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
			return nil, fmt.Errorf("property '%v' is not a string (%v)", k, reflect.TypeOf(v.V))
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
			return nil, fmt.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
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
			return nil, fmt.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
		}
		a := v.ArrayValue()
		var objs []PropertyMap
		for i, e := range a {
			if e.IsObject() {
				objs = append(objs, e.ObjectValue())
			} else {
				return nil, fmt.Errorf(
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
			return nil, fmt.Errorf("property '%v' is not an array (%v)", k, reflect.TypeOf(v.V))
		}
		a := v.ArrayValue()
		var strs []string
		for i, e := range a {
			if e.IsString() {
				strs = append(strs, e.StringValue())
			} else {
				return nil, fmt.Errorf(
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
			return nil, fmt.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
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
			return nil, fmt.Errorf("property '%v' is not an object (%v)", k, reflect.TypeOf(v.V))
		}
		m := v.ResourceValue()
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

// Mappable returns a mapper-compatible object map, suitable for deserialization into structures.
func (props PropertyMap) Mappable() mapper.Object {
	obj := make(mapper.Object)
	for _, k := range StablePropertyKeys(props) {
		obj[string(k)] = props[k].Mappable()
	}
	return obj
}

// AllResources finds all resource URNs, transitively throughout the property map, and returns them.
func (props PropertyMap) AllResources() map[URN]bool {
	URNs := make(map[URN]bool)
	for _, k := range StablePropertyKeys(props) {
		for m, v := range props[k].AllResources() {
			URNs[m] = v
		}
	}
	return URNs
}

// ReplaceResources finds all resources and lets an updater function update them if necessary.  This is often used
// during a "replacement"-style updated, to replace all URNs of a certain value with another.
func (props PropertyMap) ReplaceResources(updater func(URN) URN) PropertyMap {
	result := make(PropertyMap)
	for _, k := range StablePropertyKeys(props) {
		result[k] = props[k].ReplaceResources(updater)
	}
	return result
}

func NewPropertyNull() PropertyValue                   { return PropertyValue{nil} }
func NewPropertyBool(v bool) PropertyValue             { return PropertyValue{v} }
func NewPropertyNumber(v float64) PropertyValue        { return PropertyValue{v} }
func NewPropertyString(v string) PropertyValue         { return PropertyValue{v} }
func NewPropertyArray(v []PropertyValue) PropertyValue { return PropertyValue{v} }
func NewPropertyObject(v PropertyMap) PropertyValue    { return PropertyValue{v} }
func NewPropertyResource(v URN) PropertyValue          { return PropertyValue{v} }

func (v PropertyValue) BoolValue() bool             { return v.V.(bool) }
func (v PropertyValue) NumberValue() float64        { return v.V.(float64) }
func (v PropertyValue) StringValue() string         { return v.V.(string) }
func (v PropertyValue) ArrayValue() []PropertyValue { return v.V.([]PropertyValue) }
func (v PropertyValue) ObjectValue() PropertyMap    { return v.V.(PropertyMap) }
func (v PropertyValue) ResourceValue() URN          { return v.V.(URN) }

func (b PropertyValue) IsNull() bool {
	return b.V == nil
}
func (b PropertyValue) IsBool() bool {
	_, is := b.V.(bool)
	return is
}
func (b PropertyValue) IsNumber() bool {
	_, is := b.V.(float64)
	return is
}
func (b PropertyValue) IsString() bool {
	_, is := b.V.(string)
	return is
}
func (b PropertyValue) IsArray() bool {
	_, is := b.V.([]PropertyValue)
	return is
}
func (b PropertyValue) IsObject() bool {
	_, is := b.V.(PropertyMap)
	return is
}
func (b PropertyValue) IsResource() bool {
	_, is := b.V.(URN)
	return is
}

// Mappable returns a mapper-compatible value, suitable for deserialization into structures.
func (v PropertyValue) Mappable() mapper.Value {
	if v.IsNull() {
		return nil
	} else if v.IsBool() {
		return v.BoolValue()
	} else if v.IsNumber() {
		return v.NumberValue()
	} else if v.IsString() {
		return v.StringValue()
	} else if v.IsArray() {
		var arr []mapper.Value
		for _, e := range v.ArrayValue() {
			arr = append(arr, e.Mappable())
		}
		return arr
	} else {
		contract.Assert(v.IsObject())
		return v.ObjectValue().Mappable()
	}
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
		return NewPropertyResource(updater(m))
	} else if v.IsArray() {
		arr := v.ArrayValue()
		elems := make([]PropertyValue, len(arr))
		for i, elem := range arr {
			elems[i] = elem.ReplaceResources(updater)
		}
		return NewPropertyArray(elems)
	} else if v.IsObject() {
		rep := v.ObjectValue().ReplaceResources(updater)
		return NewPropertyObject(rep)
	}
	return v
}
