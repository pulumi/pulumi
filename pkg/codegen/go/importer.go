// Copyright 2016-2021, Pulumi Corporation.
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

package gen

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// GoPackageInfo holds information required to generate the Go SDK from a schema.
type GoPackageInfo struct {
	// Base path for package imports
	//
	//    github.com/pulumi/pulumi-kubernetes/sdk/go/kubernetes
	ImportBasePath string `json:"importBasePath,omitempty"`

	// Explicit package name, which may be different to the import path.
	RootPackageName string `json:"rootPackageName,omitempty"`

	// Map from module -> package name
	//
	//    { "flowcontrol.apiserver.k8s.io/v1alpha1": "flowcontrol/v1alpha1" }
	//
	ModuleToPackage map[string]string `json:"moduleToPackage,omitempty"`

	// Map from package name -> package alias
	//
	//    { "github.com/pulumi/pulumi-kubernetes/sdk/go/kubernetes/flowcontrol/v1alpha1": "flowcontrolv1alpha1" }
	//
	PackageImportAliases map[string]string `json:"packageImportAliases,omitempty"`

	// Generate container types (arrays, maps, pointer output types etc.) for each resource.
	// These are typically used to support external references.
	GenerateResourceContainerTypes bool `json:"generateResourceContainerTypes,omitempty"`

	// The version of the Pulumi SDK used with this provider, e.g. 3.
	// Used to generate doc links for pulumi builtin types. If omitted, the latest SDK version is used.
	PulumiSDKVersion int `json:"pulumiSDKVersion,omitempty"`

	// Feature flag to disable generating `$fnOutput` invoke
	// function versions to save space.
	DisableFunctionOutputVersions bool `json:"disableFunctionOutputVersions,omitempty"`

	// Determines whether to make single-return-value methods return an output struct or the value.
	LiftSingleValueMethodReturns bool `json:"liftSingleValueMethodReturns,omitempty"`

	// Feature flag to disable generating input type registration. This is a
	// space saving measure.
	DisableInputTypeRegistrations bool `json:"disableInputTypeRegistrations,omitempty"`

	// Feature flag to disable generating Pulumi object default functions. This is a
	// space saving measure.
	DisableObjectDefaults bool `json:"disableObjectDefaults,omitempty"`

	// GenerateExtraInputTypes determines whether or not the code generator generates input (and output) types for
	// all plain types, instead of for only types that are used as input/output types.
	GenerateExtraInputTypes bool `json:"generateExtraInputTypes,omitempty"`

	// Respect the Pkg.Version field for emitted code.
	RespectSchemaVersion bool `json:"respectSchemaVersion,omitempty"`
}

// Importer implements schema.Language for Go.
var Importer schema.Language = importer(0)

type importer int

// ImportDefaultSpec decodes language-specific metadata associated with a DefaultValue.
func (importer) ImportDefaultSpec(def *schema.DefaultValue, raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportPropertySpec decodes language-specific metadata associated with a Property.
func (importer) ImportPropertySpec(property *schema.Property, raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportObjectTypeSpec decodes language-specific metadata associated with a ObjectType.
func (importer) ImportObjectTypeSpec(object *schema.ObjectType, raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportResourceSpec decodes language-specific metadata associated with a Resource.
func (importer) ImportResourceSpec(resource *schema.Resource, raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportFunctionSpec decodes language-specific metadata associated with a Function.
func (importer) ImportFunctionSpec(function *schema.Function, raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportPackageSpec decodes language-specific metadata associated with a Package.
func (importer) ImportPackageSpec(pkg *schema.Package, raw json.RawMessage) (interface{}, error) {
	var info GoPackageInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	return info, nil
}
