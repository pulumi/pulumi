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

type InputPropertyErrorDetails struct {
	PropertyPath string
	Reason       string
}

type InputPropertiesError struct {
	Message string
	Errors  []InputPropertyErrorDetails
}

func NewInputPropertyError(propertyPath string, reason string) *InputPropertiesError {
	return NewInputPropertiesError("", InputPropertyErrorDetails{
		PropertyPath: propertyPath,
		Reason:       reason,
	})
}

func InputPropertyErrorf(propertyPath string, format string, args ...interface{}) *InputPropertiesError {
	return NewInputPropertiesError("", InputPropertyErrorDetails{
		PropertyPath: propertyPath,
		Reason:       fmt.Sprintf(format, args...),
	})
}

func NewInputPropertiesError(message string, details ...InputPropertyErrorDetails) *InputPropertiesError {
	return &InputPropertiesError{
		Message: message,
		Errors:  details,
	}
}

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
			message += ": "
		}
		message += fmt.Sprintf("%s: %s", err.PropertyPath, err.Reason)
	}
	return message
}

func (ipe *InputPropertiesError) WithDetails(details ...InputPropertyErrorDetails) *InputPropertiesError {
	ipe.Errors = append(ipe.Errors, details...)
	return ipe
}
