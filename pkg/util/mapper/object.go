// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapper

import (
	"reflect"
)

// AsObject attempts to coerce an existing value to an object map, returning a non-nil error if it cannot be done.
func AsObject(v interface{}, ty reflect.Type, key string) (map[string]interface{}, FieldError) {
	if vmap, ok := v.(map[string]interface{}); ok {
		return vmap, nil
	}
	return nil, NewWrongTypeError(
		ty, key, reflect.TypeOf(make(map[string]interface{})), reflect.TypeOf(v))
}

// AsString attempts to coerce an existing value to a string, returning a non-nil error if it cannot be done.
func AsString(v interface{}, ty reflect.Type, key string) (*string, FieldError) {
	if s, ok := v.(string); ok {
		return &s, nil
	}
	return nil, NewWrongTypeError(ty, key, reflect.TypeOf(""), reflect.TypeOf(v))
}

// FieldObject looks up a field by name within an object map, coerces it to an object itself, and returns it.  If the
// field exists but is not an object map, or it is missing and optional is false, a non-nil error is returned.
func FieldObject(obj map[string]interface{}, ty reflect.Type,
	key string, optional bool) (map[string]interface{}, FieldError) {
	if o, has := obj[key]; has {
		return AsObject(o, ty, key)
	} else if !optional {
		// The field doesn't exist and yet it is required; issue an error.
		return nil, NewMissingError(ty, key)
	}
	return nil, nil
}

// FieldString looks up a field by name within an object map, coerces it to a string, and returns it.  If the
// field exists but is not a string, or it is missing and optional is false, a non-nil error is returned.
func FieldString(obj map[string]interface{}, ty reflect.Type, key string, optional bool) (*string, FieldError) {
	if s, has := obj[key]; has {
		return AsString(s, ty, key)
	} else if !optional {
		// The field doesn't exist and yet it is required; issue an error.
		return nil, NewMissingError(ty, key)
	}
	return nil, nil
}
