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

package codegen

import (
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper interface {
	GetPropertyName(p *schema.Property) (string, error)
	GetDocLinkForResourceType(packageName, moduleName, typeName string) string
	GetDocLinkForResourceInputOrOutputType(packageName, moduleName, typeName string, input bool) string
	GetDocLinkForFunctionInputOrOutputType(packageName, moduleName, typeName string, input bool) string
	GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string
	// GetResourceFunctionResultName returns the name of the result type when a static resource function is used to lookup
	// an existing resource.
	GetResourceFunctionResultName(resourceName string) string
}
