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
//nolint:lll, goconst
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
	return "/docs/reference/pkg/nodejs/pulumi/pulumi/#" + typeName
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
func (d DocLanguageHelper) GetTypeName(pkg schema.PackageReference, t schema.Type, input bool, relativeToModule string) string {
	// Remove the union with `undefined` for optional types,
	// since we will show that information separately anyway.
	if optional, ok := t.(*schema.OptionalType); ok {
		t = optional.ElementType
	}

	var info NodePackageInfo
	if a, err := pkg.Language("nodejs"); err == nil {
		info, _ = a.(NodePackageInfo)
	}

	modCtx := &modContext{
		pkg:      pkg,
		modToPkg: info.ModuleToPackage,
		mod:      moduleName(relativeToModule, pkg),
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

func (d DocLanguageHelper) GetFunctionName(f *schema.Function) string {
	return tokenToFunctionName(f.Token)
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(modName string, f *schema.Function) string {
	funcName := d.GetFunctionName(f)
	return title(funcName) + "Result"
}

func (d DocLanguageHelper) GetMethodName(m *schema.Method) string {
	return camel(m.Name)
}

func (d DocLanguageHelper) GetMethodResultName(pkg schema.PackageReference, modName string, r *schema.Resource,
	m *schema.Method,
) string {
	var objectReturnType *schema.ObjectType
	if m.Function.ReturnType != nil {
		if objectType, ok := m.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			objectReturnType = objectType
		} else {
			modCtx := &modContext{
				pkg: pkg,
				mod: modName,
			}
			return modCtx.typeString(m.Function.ReturnType, false, nil)
		}
	}

	var info NodePackageInfo
	if i, err := pkg.Language("nodejs"); err == nil {
		info, _ = i.(NodePackageInfo)
	}

	if info.LiftSingleValueMethodReturns && objectReturnType != nil && len(objectReturnType.Properties) == 1 {
		modCtx := &modContext{
			pkg: pkg,
			mod: modName,
		}
		return modCtx.typeString(objectReturnType.Properties[0].Type, false, nil)
	}
	return fmt.Sprintf("%s.%sResult", resourceName(r), title(d.GetMethodName(m)))
}

// GetPropertyName returns the property name specific to NodeJS.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return p.Name, nil
}

// GetEnumName returns the enum name specific to NodeJS.
func (d DocLanguageHelper) GetEnumName(e *schema.Enum, typeName string) (string, error) {
	return enumMemberName(typeName, e)
}
