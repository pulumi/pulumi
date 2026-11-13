// Copyright 2026, Pulumi Corporation.
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

package policyx

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// UnknownValueError indicates that a property value read by a policy is not known
// during preview, because it depends on a computation whose result is not yet
// available (for example, an IP address a cloud provider has not allocated yet).
//
// A policy can return an UnknownValueError from its validation function. The server
// converts it into an advisory diagnostic that tells the user the policy could not
// run during preview, instead of failing the whole Analyze call. This mirrors the
// behavior of the Node.js and Python policy SDKs, where reading an unknown value
// throws UnknownValueError.
type UnknownValueError struct {
	// Path is the path to the unknown property, for example "name" or
	// "spec.tags". It is empty when the path is not known, for example when the
	// error is synthesized by the server from a panicking property access.
	Path string
	// Type is the expected type of the property, for example "string" or
	// "float64". It is empty when the expected type is not known.
	Type string
	// Value is the computed value that could not be read.
	Value property.Value
}

// Error implements the error interface.
func (e *UnknownValueError) Error() string {
	typeName := e.Type
	if typeName == "" {
		typeName = "property"
	}
	if e.Path == "" {
		return fmt.Sprintf("%s value can't be known during preview", typeName)
	}
	return fmt.Sprintf("%s value at .%s can't be known during preview", typeName, e.Path)
}
