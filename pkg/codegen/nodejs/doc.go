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

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// DocLanguageHelper is the NodeJS-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForPulumiType returns the NodeJS API doc link for a Pulumi type.
func (d DocLanguageHelper) GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string {
	typeName = strings.ReplaceAll(typeName, "?", "")
	return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/pulumi/#%s", typeName)
}

// GetDocLinkForResourceType returns the NodeJS API doc for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(pkg *schema.Package, modName, typeName string) string {
	var path string
	switch {
	case pkg.Name != "" && modName != "":
		path = fmt.Sprintf("%s/%s", pkg.Name, modName)
	case pkg.Name == "" && modName != "":
		path = modName
	case pkg.Name != "" && modName == "":
		path = pkg.Name
	}
	typeName = strings.ReplaceAll(typeName, "?", "")
	return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/#%s", path, typeName)
}

// GetDocLinkForResourceInputOrOutputType returns the doc link for an input or output type of a Resource.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	typeName = strings.TrimSuffix(typeName, "?")
	parts := strings.Split(typeName, ".")
	typeName = parts[len(parts)-1]
	if input {
		return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/types/input/#%s", pkg.Name, typeName)
	}
	return fmt.Sprintf("/docs/reference/pkg/nodejs/pulumi/%s/types/output/#%s", pkg.Name, typeName)
}

// GetDocLinkForFunctionInputOrOutputType returns the doc link for an input or output type of a Function.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	return d.GetDocLinkForResourceInputOrOutputType(pkg, modName, typeName, input)
}

// GetLanguageTypeString returns the language-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input bool) string {
	// Remove the union with `undefined` for optional types,
	// since we will show that information separately anyway.
	if optional, ok := t.(*schema.OptionalType); ok {
		t = optional.ElementType
	}

	modCtx := &modContext{
		pkg: pkg,
		mod: moduleName,
	}
	typeName := modCtx.typeString(t, input, nil)

	// Remove any package qualifiers from the type name.
	typeQualifierPackage := "inputs"
	if !input {
		typeQualifierPackage = "outputs"
	}
	typeName = strings.ReplaceAll(typeName, typeQualifierPackage+".", "")
	typeName = strings.ReplaceAll(typeName, "enums.", "")

	return typeName
}

func (d DocLanguageHelper) GetFunctionName(modName string, f *schema.Function) string {
	return tokenToFunctionName(f.Token)
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(modName string, f *schema.Function) string {
	funcName := d.GetFunctionName(modName, f)
	return title(funcName) + "Result"
}

// GetPropertyName returns the property name specific to NodeJS.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return p.Name, nil
}

// GetEnumName returns the enum name specific to NodeJS.
func (d DocLanguageHelper) GetEnumName(e *schema.Enum, typeName string) (string, error) {
	name := fmt.Sprintf("%v", e.Value)
	if e.Name != "" {
		name = e.Name
	}
	return makeSafeEnumName(name, typeName)
}

// GetModuleDocLink returns the display name and the link for a module.
func (d DocLanguageHelper) GetModuleDocLink(pkg *schema.Package, modName string) (string, string) {
	var displayName string
	var link string
	if modName == "" {
		displayName = fmt.Sprintf("@pulumi/%s", pkg.Name)
	} else {
		displayName = fmt.Sprintf("@pulumi/%s/%s", pkg.Name, strings.ToLower(modName))
	}
	link = d.GetDocLinkForResourceType(pkg, modName, "")
	return displayName, link
}
