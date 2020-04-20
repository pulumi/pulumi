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

package nodejs

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
)

// NodePackageInfo contains NodeJS specific overrides for a package.
type NodePackageInfo struct {
	PackageName        string            `json:"packageName,omitempty"`        // Custom name for the NPM package.
	PackageDescription string            `json:"packageDescription,omitempty"` // Description for the NPM package.
	Dependencies       map[string]string `json:"dependencies,omitempty"`       // NPM dependencies to add to package.json.
	DevDependencies    map[string]string `json:"devDependencies,omitempty"`    // NPM dev-dependencies to add to package.json.
	PeerDependencies   map[string]string `json:"peerDependencies,omitempty"`   // NPM peer-dependencies to add to package.json.
	TypeScriptVersion  string            `json:"typescriptVersion,omitempty"`  // A specific version of TypeScript to include in package.json.
	ModuleToPackage    map[string]string `json:"moduleToPackage,omitempty"`    // A map containing overrides for module names to package names.
}

// Importer implements schema.Language for NodeJS.
var Importer schema.Language = importer(0)

type importer int

// ImportDefaultSpec decodes language-specific metadata associated with a Default.
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
	var info NodePackageInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	return info, nil
}
