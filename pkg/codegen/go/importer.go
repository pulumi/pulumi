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

	// Module path for go.mod
	//
	//   go get github.com/pulumi/pulumi-aws-native/sdk/go/aws@v0.16.0
	//          ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ module path
	//                                                  ~~~~~~ package path - can be any number of path parts
	//                                                         ~~~~~~~ version
	ModulePath string `json:"modulePath,omitempty"`

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

	// Defines the pattern for how import paths should be constructed from the base import path and used module
	// By default, the pattern is "{baseImportPath}/{module}" but can be overridden to support other patterns.
	// for example you can use "{baseImportPath}/{module}/v2" which will replace {module} with the module name
	// and {baseImportPath} with the base import path to compute the full path.
	ImportPathPattern string `json:"importPathPattern,omitempty"`

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

	// When set, the code generator will use this name for the generated internal module
	// instead of "internal" so that functionality within the module can be used by end users.
	InternalModuleName string `json:"internalModuleName,omitempty"`

	// Feature flag to disable generating Pulumi object default functions. This is a
	// space saving measure.
	DisableObjectDefaults bool `json:"disableObjectDefaults,omitempty"`

	// GenerateExtraInputTypes determines whether or not the code generator generates input (and output) types for
	// all plain types, instead of for only types that are used as input/output types.
	GenerateExtraInputTypes bool `json:"generateExtraInputTypes,omitempty"`

	// omitExtraInputTypes determines whether the code generator generates input (and output) types
	// for all plain types, instead of for only types that are used as input/output types.
	OmitExtraInputTypes bool `json:"omitExtraInputTypes,omitempty"`

	// Respect the Pkg.Version field for emitted code.
	RespectSchemaVersion bool `json:"respectSchemaVersion,omitempty"`

	// InternalDependencies are blank imports that are emitted in the SDK so that `go mod tidy` does not remove the
	// associated module dependencies from the SDK's go.mod.
	InternalDependencies []string `json:"internalDependencies,omitempty"`

	// Specifies how to handle generating a variant of the SDK that uses generics.
	// Allowed values are the following:
	// - "none" (default): do not generate a generics variant of the SDK
	// - "side-by-side": generate a side-by-side generics variant of the SDK under the x subdirectory
	// - "only-generics": generate a generics variant of the SDK only
	Generics string `json:"generics,omitempty"`
}

// Importer implements schema.Language for Go.
var Importer schema.Language = importer(0)

type importer int

// ImportDefaultSpec decodes language-specific metadata associated with a DefaultValue.
func (importer) ImportDefaultSpec(raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportPropertySpec decodes language-specific metadata associated with a Property.
func (importer) ImportPropertySpec(raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportObjectTypeSpec decodes language-specific metadata associated with a ObjectType.
func (importer) ImportObjectTypeSpec(raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportResourceSpec decodes language-specific metadata associated with a Resource.
func (importer) ImportResourceSpec(raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportFunctionSpec decodes language-specific metadata associated with a Function.
func (importer) ImportFunctionSpec(raw json.RawMessage) (interface{}, error) {
	return raw, nil
}

// ImportPackageSpec decodes language-specific metadata associated with a Package.
func (importer) ImportPackageSpec(raw json.RawMessage) (interface{}, error) {
	var info GoPackageInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	return info, nil
}
