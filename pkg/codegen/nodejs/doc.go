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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: lll, goconst
package nodejs

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

// DocLanguageHelper is the NodeJS-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForResourceType returns the NodeJS API doc for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(packageName, modName, typeName string) string {
	var path string
	if packageName != "" {
		path = fmt.Sprintf("%s/%s", packageName, modName)
	} else {
		path = modName
	}
	typeName = strings.ReplaceAll(typeName, "?", "")
	return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/#%s", path, typeName)
}

// GetDocLinkForResourceInputOrOutputType returns the doc link for an input or output type of a Resource.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(packageName, modName, typeName string, input bool) string {
	typeName = strings.ReplaceAll(typeName, "?", "")
	typeName = strings.ReplaceAll(typeName, modName+".", "")
	if input {
		return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/types/input/#%s", packageName, typeName)
	}
	return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/types/output/#%s", packageName, typeName)
}

// GetDocLinkForFunctionInputOrOutputType returns the doc link for an input or output type of a Function.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(packageName, modName, typeName string, input bool) string {
	return d.GetDocLinkForResourceInputOrOutputType(packageName, modName, typeName, input)
}

// GetDocLinkForBuiltInType returns the URL for a built-in type.
func GetDocLinkForBuiltInType(typeName string) string {
	return fmt.Sprintf("https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/%s", typeName)
}

// GetLanguageTypeString returns the language-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string {
	modCtx := &modContext{
		pkg: pkg,
		mod: moduleName,
	}
	typeName := modCtx.typeString(t, input, false, optional)

	// Remove any package qualifiers from the type name.
	typeQualifierPackage := "inputs"
	if !input {
		typeQualifierPackage = "outputs"
	}
	typeName = strings.ReplaceAll(typeName, typeQualifierPackage+".", "")

	// Remove the union with `undefined` for optional types,
	// since we will show that information separately anyway.
	if optional {
		typeName = strings.ReplaceAll(typeName, " | undefined", "?")
	}
	return typeName
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(resourceName string) string {
	return "Get" + resourceName + "Result"
}

// GetPropertyName returns the property name specific to NodeJS.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return p.Name, nil
}
