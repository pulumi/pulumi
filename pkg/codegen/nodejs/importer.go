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

package nodejs

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Compatibility mode for Kubernetes 2.0 SDK
const kubernetes20 = "kubernetes20"

// Compatibility mode for tfbridge 2.x SDKs
const tfbridge20 = "tfbridge20"

// NodePackageInfo contains NodeJS-specific information for a package.
type NodePackageInfo struct {
	// Custom name for the NPM package.
	PackageName string `json:"packageName,omitempty"`
	// Description for the NPM package.
	PackageDescription string `json:"packageDescription,omitempty"`
	// Readme contains the text for the package's README.md files.
	Readme string `json:"readme,omitempty"`
	// NPM dependencies to add to package.json.
	Dependencies map[string]string `json:"dependencies,omitempty"`
	// NPM dev-dependencies to add to package.json.
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	// NPM peer-dependencies to add to package.json.
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	// NPM resolutions to add to package.json
	Resolutions map[string]string `json:"resolutions,omitempty"`
	// A specific version of TypeScript to include in package.json.
	TypeScriptVersion string `json:"typescriptVersion,omitempty"`
	// A map containing overrides for module names to package names.
	ModuleToPackage map[string]string `json:"moduleToPackage,omitempty"`
	// Toggle compatibility mode for a specified target.
	Compatibility string `json:"compatibility,omitempty"`
	// Disable support for unions in output types.
	DisableUnionOutputTypes bool `json:"disableUnionOutputTypes,omitempty"`
	// An indicator for whether the package contains enums.
	ContainsEnums bool `json:"containsEnums,omitempty"`
	// A map allowing you to map the name of a provider to the name of the module encapsulating the provider.
	ProviderNameToModuleName map[string]string `json:"providerNameToModuleName,omitempty"`
	// Additional files to include in TypeScript compilation.
	// These paths are added to the `files` section of the
	// generated `tsconfig.json`. A typical use case for this is
	// compiling hand-authored unit test files that check the
	// generated code.
	ExtraTypeScriptFiles []string `json:"extraTypeScriptFiles,omitempty"`
	// Determines whether to make single-return-value methods return an output object or the single value.
	LiftSingleValueMethodReturns bool `json:"liftSingleValueMethodReturns,omitempty"`

	// Respect the Pkg.Version field in the schema
	RespectSchemaVersion bool `json:"respectSchemaVersion,omitempty"`

	// Experimental flag that permits `import type *` style code
	// to be generated to optimize startup time of programs
	// consuming the provider by minimizing the set of Node
	// modules loaded at startup. Turning this on may currently
	// generate non-compiling code for some providers; but if the
	// code compiles it is safe to use. Also, turning this on
	// requires TypeScript 3.8 or higher to compile the generated
	// code.
	UseTypeOnlyReferences bool `json:"useTypeOnlyReferences,omitempty"`
}

// NodeObjectInfo contains NodeJS-specific information for an object.
type NodeObjectInfo struct {
	// List of properties that are required on the input side of a type.
	RequiredInputs []string `json:"requiredInputs"`
	// List of properties that are required on the output side of a type.
	RequiredOutputs []string `json:"requiredOutputs"`
}

// Importer implements schema.Language for NodeJS.
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
	var info NodeObjectInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	return info, nil
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
	var info NodePackageInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	return info, nil
}
