// Copyright 2016 Pulumi, Inc. All rights reserved.

package mapper

import (
	"reflect"
)

// Object is a "JSON-like" object map.
type Object map[string]interface{}

// Array is a "JSON-like" array of values.
type Array []interface{}

// AsObject attempts to coerce an existing value to an object map, returning a non-nil error if it cannot be done.
func AsObject(v interface{}, ty reflect.Type, key string) (*Object, error) {
	if vmap, ok := v.(map[string]interface{}); ok {
		vobj := Object(vmap)
		return &vobj, nil
	}
	return nil, ErrWrongType(
		ty, key, reflect.TypeOf(make(map[string]interface{})), reflect.TypeOf(v))
}

// AsString attempts to coerce an existing value to a string, returning a non-nil error if it cannot be done.
func AsString(v interface{}, ty reflect.Type, key string) (*string, error) {
	if s, ok := v.(string); ok {
		return &s, nil
	}
	return nil, ErrWrongType(ty, key, reflect.TypeOf(""), reflect.TypeOf(v))
}

// FieldObject looks up a field by name within an object map, coerces it to an object itself, and returns it.  If the
// field exists but is not an object map, or it is missing and optional is false, a non-nil error is returned.
func FieldObject(tree Object, ty reflect.Type, key string, optional bool) (*Object, error) {
	if o, has := tree[key]; has {
		return AsObject(o, ty, key)
	} else if !optional {
		// The field doesn't exist and yet it is required; issue an error.
		return nil, ErrMissing(ty, key)
	}
	return nil, nil
}

// FieldString looks up a field by name within an object map, coerces it to a string, and returns it.  If the
// field exists but is not a string, or it is missing and optional is false, a non-nil error is returned.
func FieldString(tree Object, ty reflect.Type, key string, optional bool) (*string, error) {
	if s, has := tree[key]; has {
		return AsString(s, ty, key)
	} else if !optional {
		// The field doesn't exist and yet it is required; issue an error.
		return nil, ErrMissing(ty, key)
	}
	return nil, nil
}
