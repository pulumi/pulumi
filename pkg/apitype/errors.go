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

package apitype

import "fmt"

// ErrorType is an enum for various types of common errors that occur.
type ErrorType string

const (
	// NotFoundErrorType is used when a resource or resource field was not found.
	// For example, updating a Stack that doesn't exist.
	NotFoundErrorType ErrorType = "not_found"
	// RequiredErrorType is used when a resource or resource field is missing and is required.
	// For example, creating a Stack without a project name.
	RequiredErrorType ErrorType = "required"
	// InvalidErrorType is used when the a resource or field was passed with an invalid value or state.
	// For example, creating a Stack with an invalid name.
	InvalidErrorType ErrorType = "invalid"
	// AlreadyExistsErrorType is used if the resource or field already exists (and must be unique).
	// For example, creating two Stacks with the same name.
	AlreadyExistsErrorType ErrorType = "already_exists"

	// CustomErrorType is used to describe any ErrorType not found in this file, and must be paired with
	// a custom error message.
	CustomErrorType ErrorType = "custom"
)

// RequestError describes a request error in more detail, such the specific validation
// error(s) that caused the request to fail and links to the relevant documentation.
type RequestError struct {
	// Resource is the user-friendly description of the resource, e.g. "stack" or "tag".
	Resource string `json:"resource"`

	// Attribute describes the property of the resource (if applicable) that is problematic, e.g.
	// "name" or "length".
	Attribute *string `json:"attribute,omitempty"`

	// ErrorType is the type of error the attribute's value caused.
	ErrorType ErrorType `json:"errorType"`
	// CustomMessage is the error message to display with CustomErrorType.
	CustomMessage *string `json:"customMessage,omitempty"`
}

// ErrorResponse is returned from the API when an actual response body is not appropriate. i.e.
// in all error situations.
type ErrorResponse struct {
	// Code is the HTTP status code for the error response.
	Code int `json:"code"`
	// Message is the user-facing message describing the error.
	Message string `json:"message"`

	// DocumentationURL is an optional URL the user can to go learn more about the error.
	DocumentationURL *string `json:"documentationUrl,omitempty"`
	// Errors optionally include more specific data about why the request failed.
	Errors []RequestError `json:"errors,omitempty"`
}

// Error implements the Error interface.
func (err ErrorResponse) Error() string {
	return fmt.Sprintf("[%d] %s", err.Code, err.Message)
}
