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
	"fmt"
	"reflect"
)

// MappingError represents a collection of decoding errors, defined below.
type MappingError interface {
	error
	Failures() []error    // the full set of errors (each of one of the below types).
	AddFailure(err error) // registers a new failure.
}

// mappingError is a concrete implementation of MappingError; it is private, and we prefer to use the above interface
// type, to avoid tricky non-nil nils in common usage patterns (see https://golang.org/doc/faq#nil_error).
type mappingError struct {
	failures []error
}

var _ error = (*mappingError)(nil) // ensure this implements the error interface.

func NewMappingError(errs []error) MappingError {
	return &mappingError{failures: errs}
}

func (e *mappingError) Failures() []error { return e.failures }

func (e *mappingError) AddFailure(err error) {
	e.failures = append(e.failures, err)
}

func (e *mappingError) Error() string {
	str := fmt.Sprintf("%d failures decoding:", len(e.failures))
	for _, failure := range e.failures {
		switch f := failure.(type) {
		case FieldError:
			str += fmt.Sprintf("\n\t%v: %v", f.Field(), f.Reason())
		default:
			str += fmt.Sprintf("\n\t%v", f)
		}
	}
	return str
}

// FieldError represents a failure during decoding of a specific field.
type FieldError interface {
	error
	Field() string  // returns the name of the field with a problem.
	Reason() string // returns a full diagnostic string about the error.
}

// fieldError is used when a general purpose error occurs decoding a field.
type fieldError struct {
	Type    string
	Fld     string
	Message string
}

var _ error = (*fieldError)(nil)      // ensure this implements the error interface.
var _ FieldError = (*fieldError)(nil) // ensure this implements the fieldError interface.

func NewFieldError(ty string, fld string, err error) FieldError {
	return &fieldError{
		Type:    ty,
		Fld:     fld,
		Message: fmt.Sprintf("An error occurred decoding '%v.%v': %v", ty, fld, err),
	}
}

func NewTypeFieldError(ty reflect.Type, fld string, err error) FieldError {
	return NewFieldError(ty.Name(), fld, err)
}

func (e *fieldError) Error() string  { return e.Message }
func (e *fieldError) Field() string  { return e.Fld }
func (e *fieldError) Reason() string { return e.Message }

// MissingError is used when a required field is missing on an object of a given type.
type MissingError struct {
	Type    reflect.Type
	Fld     string
	Message string
}

var _ error = (*MissingError)(nil)      // ensure this implements the error interface.
var _ FieldError = (*MissingError)(nil) // ensure this implements the FieldError interface.

func NewMissingError(ty reflect.Type, fld string) *MissingError {
	return &MissingError{
		Type:    ty,
		Fld:     fld,
		Message: fmt.Sprintf("Missing required field '%v' on '%v'", fld, ty),
	}
}

func (e *MissingError) Error() string  { return e.Message }
func (e *MissingError) Field() string  { return e.Fld }
func (e *MissingError) Reason() string { return e.Message }

// UnrecognizedError is used when a field is unrecognized on the given type.
type UnrecognizedError struct {
	Type    reflect.Type
	Fld     string
	Message string
}

var _ error = (*UnrecognizedError)(nil)      // ensure this implements the error interface.
var _ FieldError = (*UnrecognizedError)(nil) // ensure this implements the FieldError interface.

func NewUnrecognizedError(ty reflect.Type, fld string) *UnrecognizedError {
	return &UnrecognizedError{
		Type:    ty,
		Fld:     fld,
		Message: fmt.Sprintf("Unrecognized field '%v' on '%v'", fld, ty),
	}
}

func (e *UnrecognizedError) Error() string  { return e.Message }
func (e *UnrecognizedError) Field() string  { return e.Fld }
func (e *UnrecognizedError) Reason() string { return e.Message }

type WrongTypeError struct {
	Type    reflect.Type
	Fld     string
	Expect  reflect.Type
	Actual  reflect.Type
	Message string
}

var _ error = (*WrongTypeError)(nil)      // ensure this implements the error interface.
var _ FieldError = (*WrongTypeError)(nil) // ensure this implements the FieldError interface.

func NewWrongTypeError(ty reflect.Type, fld string, expect reflect.Type, actual reflect.Type) *WrongTypeError {
	return &WrongTypeError{
		Type:   ty,
		Fld:    fld,
		Expect: expect,
		Actual: actual,
		Message: fmt.Sprintf(
			"Field '%v' on '%v' must be a '%v'; got '%v' instead", fld, ty, expect, actual),
	}
}

func (e *WrongTypeError) Error() string  { return e.Message }
func (e *WrongTypeError) Field() string  { return e.Fld }
func (e *WrongTypeError) Reason() string { return e.Message }
