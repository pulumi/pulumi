// Copyright 2024, Pulumi Corporation.
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

package errors

import "fmt"

// InputPropertyErrorDetails contains the error details for an input property error.
type InputPropertyErrorDetails struct {
	PropertyPath string
	Reason       string
}

func (d InputPropertyErrorDetails) String() string {
	return fmt.Sprintf("%s: %s", d.PropertyPath, d.Reason)
}

// InputPropertiesError can be used to indicate that the client has made a request with
// bad input properties.
type InputPropertiesError struct {
	Message string
	Errors  []InputPropertyErrorDetails
}

// Create a new InputPropertiesError with a single property error.
func NewInputPropertyError(propertyPath string, reason string) *InputPropertiesError {
	return NewInputPropertiesError("", InputPropertyErrorDetails{
		PropertyPath: propertyPath,
		Reason:       reason,
	})
}

// Create a new InputPropertiesError with a single property error.
func InputPropertyErrorf(propertyPath string, format string, args ...interface{}) *InputPropertiesError {
	return NewInputPropertiesError("", InputPropertyErrorDetails{
		PropertyPath: propertyPath,
		Reason:       fmt.Sprintf(format, args...),
	})
}

// Create a new InputPropertiesError with a message and a list of property errors.
func NewInputPropertiesError(message string, details ...InputPropertyErrorDetails) *InputPropertiesError {
	return &InputPropertiesError{
		Message: message,
		Errors:  details,
	}
}

// Create a new InputPropertiesError with a message.
func InputPropertiesErrorf(format string, args ...interface{}) *InputPropertiesError {
	return NewInputPropertiesError(fmt.Sprintf(format, args...))
}

func (ipe *InputPropertiesError) Error() string {
	message := ipe.Message
	if message != "" && len(ipe.Errors) > 0 {
		message += ": "
	}
	for i, err := range ipe.Errors {
		if i == 0 {
			message += "\n "
		}
		message += err.String()
	}
	return message
}

// WithDetails adds additional property errors to an existing InputPropertiesError.
func (ipe *InputPropertiesError) WithDetails(details ...InputPropertyErrorDetails) *InputPropertiesError {
	ipe.Errors = append(ipe.Errors, details...)
	return ipe
}
