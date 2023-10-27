// Copyright 2023, Pulumi Corporation.
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

package esc

import (
	"encoding/json"

	"github.com/pulumi/esc/schema"
)

// An Environment contains the result of evaluating an environment definition.
type Environment struct {
	// Exprs contains the AST for each expression in the environment definition.
	Exprs map[string]Expr `json:"exprs,omitempty"`

	// Properties contains the detailed values produced by the environment.
	Properties map[string]Value `json:"properties,omitempty"`

	// Schema contains the schema for Properties.
	Schema *schema.Schema `json:"schema,omitempty"`
}

// GetEnvironmentVariables returns any environment variables defined by the environment.
//
// Environment variables are any scalar values in the top-level `environmentVariables` property. Boolean and
// number values are converted to their string representation. The results remain Values in order to retain
// secret- and unknown-ness.
func (e *Environment) GetEnvironmentVariables() map[string]Value {
	obj, ok := e.Properties["environmentVariables"].Value.(map[string]Value)
	if !ok {
		return nil
	}

	var vars map[string]Value
	for k, v := range obj {
		switch v.Value.(type) {
		case nil, bool, json.Number, string:
			if vars == nil {
				vars = make(map[string]Value)
			}
			str := v.ToString(false)
			if v.Secret {
				vars[k] = NewSecret(str)
			} else {
				vars[k] = NewValue(str)
			}
		}
	}
	return vars
}

// GetTemporaryFiles returns any temporary files defined by the environment.
//
// Temporary files are any string values in the top-level `files` property. The key for each file is the name
// of the environment variable that should hold the actual path to the temporary file once it is written.
func (e *Environment) GetTemporaryFiles() map[string]Value {
	obj, ok := e.Properties["files"].Value.(map[string]Value)
	if !ok {
		return nil
	}

	var files map[string]Value
	for k, v := range obj {
		switch v.Value.(type) {
		case nil, bool, json.Number, string:
			if files == nil {
				files = make(map[string]Value)
			}
			str := v.ToString(false)
			if v.Secret {
				files[k] = NewSecret(str)
			} else {
				files[k] = NewValue(str)
			}
		}
	}
	return files
}
