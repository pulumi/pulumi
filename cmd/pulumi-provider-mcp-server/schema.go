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

package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// ExtractResourceSchema extracts the schema for a specific resource type from a package schema.
func ExtractResourceSchema(pkgSchema *schema.PackageSpec, typeToken string) (map[string]any, error) {
	if pkgSchema.Resources == nil {
		return nil, fmt.Errorf("package has no resources")
	}

	resourceSpec, ok := pkgSchema.Resources[typeToken]
	if !ok {
		return nil, fmt.Errorf("resource type %q not found in package", typeToken)
	}

	// Convert the resource spec to a map for JSON serialization
	result := map[string]any{
		"description": resourceSpec.Description,
	}

	if resourceSpec.InputProperties != nil {
		result["inputProperties"] = resourceSpec.InputProperties
	}

	if len(resourceSpec.RequiredInputs) > 0 {
		result["requiredInputs"] = resourceSpec.RequiredInputs
	}

	if resourceSpec.Properties != nil {
		result["properties"] = resourceSpec.Properties
	}

	if len(resourceSpec.Required) > 0 {
		result["required"] = resourceSpec.Required
	}

	if resourceSpec.DeprecationMessage != "" {
		result["deprecationMessage"] = resourceSpec.DeprecationMessage
	}

	if resourceSpec.IsComponent {
		result["isComponent"] = true
	}

	return result, nil
}

// ExtractFunctionSchema extracts the schema for a specific function/invoke from a package schema.
func ExtractFunctionSchema(pkgSchema *schema.PackageSpec, functionToken string) (map[string]any, error) {
	if pkgSchema.Functions == nil {
		return nil, fmt.Errorf("package has no functions")
	}

	functionSpec, ok := pkgSchema.Functions[functionToken]
	if !ok {
		return nil, fmt.Errorf("function %q not found in package", functionToken)
	}

	// Convert the function spec to a map for JSON serialization
	result := map[string]any{
		"description": functionSpec.Description,
	}

	if functionSpec.Inputs != nil {
		result["inputs"] = map[string]any{
			"properties": functionSpec.Inputs.Properties,
		}
		if len(functionSpec.Inputs.Required) > 0 {
			result["inputs"].(map[string]any)["required"] = functionSpec.Inputs.Required
		}
	}

	if functionSpec.Outputs != nil {
		result["outputs"] = map[string]any{
			"properties": functionSpec.Outputs.Properties,
		}
		if len(functionSpec.Outputs.Required) > 0 {
			result["outputs"].(map[string]any)["required"] = functionSpec.Outputs.Required
		}
	}

	if functionSpec.DeprecationMessage != "" {
		result["deprecationMessage"] = functionSpec.DeprecationMessage
	}

	if functionSpec.IsOverlay {
		result["isOverlay"] = true
	}

	return result, nil
}

// ParsePackageSpec parses a JSON package schema into a PackageSpec.
func ParsePackageSpec(schemaJSON []byte) (*schema.PackageSpec, error) {
	var spec schema.PackageSpec
	if err := json.Unmarshal(schemaJSON, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse package schema: %w", err)
	}
	return &spec, nil
}

// PackageSpecToJSON converts a PackageSpec to JSON.
func PackageSpecToJSON(spec *schema.PackageSpec) (map[string]any, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal package spec: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}
	return result, nil
}
