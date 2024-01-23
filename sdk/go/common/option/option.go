// Copyright 2016-2024, Pulumi Corporation.
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

package option

import (
	"encoding/json"
	"errors"
	"reflect"
	_ "unsafe" // unsafe is needed to use go:linkname
)

type option[T any] struct {
	value T
	set   bool
}

// Option is a type that safely records that it's either the zero value or has been set to a non-zero value.
type Option[T any] *option[T]

// Some creates a new Option[T] with the given value.
func Some[T any](value T) Option[T] {
	// We don't allow nil values, the whole point of Option[T] is to be able to distinguish between nil and a
	// value safely.
	v := reflect.ValueOf(value)
	kind := v.Kind()
	// If the value is a nil interface then ValueOf returns the zero Value and the Kind is Invalid.
	if kind == reflect.Invalid {
		panic(errors.New("Option must not be nil"))
	}
	// Must be one of these types to be nillable
	if (kind == reflect.Ptr ||
		kind == reflect.UnsafePointer ||
		kind == reflect.Interface ||
		kind == reflect.Slice ||
		kind == reflect.Map ||
		kind == reflect.Chan ||
		kind == reflect.Func) &&
		v.IsNil() {
		panic(errors.New("Option must not be nil"))
	}

	return &option[T]{value: value, set: true}
}

// None creates a new Option[T] with the zero value. This is equivalent to nil.
func None[T any]() Option[T] {
	return nil
}

// Value returns the value of the Option[T] and if it's been set. If this returns true then the value is
// guaranteed to not be nil.
func Value[T any](o Option[T]) (T, bool) {
	if o == nil {
		var t T
		return t, false
	}
	if !o.set {
		panic(errors.New("Option must be initialized"))
	}
	return o.value, true
}

// MarshalJSON marshals the Option[T] to JSON.
func (o *option[T]) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	if !o.set {
		panic(errors.New("Option must be initialized"))
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON unmarshals the Option[T] from JSON.
func (o *option[T]) UnmarshalJSON(data []byte) error {
	var value *T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil {
		var zero T
		o.value = zero
		o.set = false
	} else {
		o.value = *value
		o.set = true
	}
	return nil
}
