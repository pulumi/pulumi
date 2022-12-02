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
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper interface {
	GetPropertyName(p *schema.Property) (string, error)
	GetEnumName(e *schema.Enum, typeName string) (string, error)
	GetDocLinkForResourceType(pkg *schema.Package, moduleName, typeName string) string
	GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string
	GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input bool) string

	GetFunctionName(modName string, f *schema.Function) string
	// GetResourceFunctionResultName returns the name of the result type when a static resource function is used to lookup
	// an existing resource.
	GetResourceFunctionResultName(modName string, f *schema.Function) string

	GetMethodName(m *schema.Method) string
	GetMethodResultName(pkg *schema.Package, modName string, r *schema.Resource, m *schema.Method) string

	// GetModuleDocLink returns the display name and the link for a module (including root modules) in a given package.
	GetModuleDocLink(pkg *schema.Package, modName string) (string, string)
}

// FilterExamples filters the code snippets in a schema docstring to include only those that target the given language.
func FilterExamples(description string, lang string) string {
	if description == "" {
		return ""
	}

	source := []byte(description)
	parsed := schema.ParseDocs(source)
	schema.LegacyFilterExamples(source, parsed, lang)
	return schema.RenderDocsToString(source, parsed)
}
