// Copyright 2016-2020, Pulumi Corporation.
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

package model

import "github.com/hashicorp/hcl/v2"

// Parameter represents a single function parameter.
type Parameter struct {
	Name string // The name of the parameter.
	Type Type   // The type of the parameter.
}

// FunctionSignature represents the parameters and return type of a function.
type FunctionSignature struct {
	Parameters       []Parameter
	VarargsParameter *Parameter
	ReturnType       Type
}

// functionDefinition represents a function definition that can be used to retrieve the signature of a function given a
// list of arguments.
type functionDefinition func(arguments []Expression) (FunctionSignature, hcl.Diagnostics)

// signature gets the signature for this function given a list of arguments.
func (f functionDefinition) signature(arguments []Expression) (FunctionSignature, hcl.Diagnostics) {
	return f(arguments)
}

// getFunctionDefinition fetches the definition for the function of the given name.
func getFunctionDefinition(name string, nameRange hcl.Range) (functionDefinition, hcl.Diagnostics) {
	switch name {
	case "fileAsset":
		return func(arguments []Expression) (FunctionSignature, hcl.Diagnostics) {
			return FunctionSignature{
				Parameters: []Parameter{{
					Name: "path",
					Type: StringType,
				}},
				ReturnType: AssetType,
			}, nil
		}, nil
	case "mimeType":
		return func(arguments []Expression) (FunctionSignature, hcl.Diagnostics) {
			return FunctionSignature{
				Parameters: []Parameter{{
					Name: "path",
					Type: StringType,
				}},
				ReturnType: StringType,
			}, nil
		}, nil
	case "toJSON":
		return func(arguments []Expression) (FunctionSignature, hcl.Diagnostics) {
			return FunctionSignature{
				Parameters: []Parameter{{
					Name: "value",
					Type: AnyType,
				}},
				ReturnType: StringType,
			}, nil
		}, nil
	default:
		return nil, hcl.Diagnostics{unknownFunction(name, nameRange)}
	}
}
