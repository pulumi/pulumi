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

// Pulling out some of the repeated strings tokens into constants would harm readability,
// so we just ignore the goconst linter's warning.
//
//nolint:lll, goconst
package python

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// DocLanguageHelper is the Python-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForPulumiType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string {
	return ""
}

// GetDocLinkForResourceType returns the Python API doc for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(pkg *schema.Package, modName, typeName string) string {
	// The k8s module names contain the domain names. For now we are stripping them off manually so they link correctly.
	if modName != "" {
		modName = strings.ReplaceAll(modName, ".k8s.io", "")
		modName = strings.ReplaceAll(modName, ".apiserver", "")
		modName = strings.ReplaceAll(modName, ".authorization", "")
	}

	var path string
	var fqdnTypeName string
	packageName := PyPack(pkg.Namespace, pkg.Name)
	switch {
	case pkg.Name != "" && modName != "":
		path = fmt.Sprintf("%s/%s", packageName, modName)
		fqdnTypeName = fmt.Sprintf("%s.%s.%s", packageName, modName, typeName)
	case pkg.Name == "" && modName != "":
		path = modName
		fqdnTypeName = fmt.Sprintf("%s.%s", modName, typeName)
	case pkg.Name != "" && modName == "":
		path = packageName
		fqdnTypeName = fmt.Sprintf("%s.%s", packageName, typeName)
	}

	return fmt.Sprintf("/docs/reference/pkg/python/%s/#%s", path, fqdnTypeName)
}

// GetDocLinkForResourceInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	return ""
}

// GetDocLinkForFunctionInputOrOutputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, modName, typeName string, input bool) string {
	return ""
}

// GetLanguageTypeString returns the Python-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input bool) string {
	typeDetails := map[*schema.ObjectType]*typeDetails{}
	mod := &modContext{
		pkg:         pkg.Reference(),
		mod:         moduleName,
		typeDetails: typeDetails,
	}
	typeName := mod.typeString(t, input, false /*acceptMapping*/, false /*forDict*/)

	// Remove any package qualifiers from the type name.
	if !input {
		typeName = strings.ReplaceAll(typeName, "outputs.", "")
	}

	// Remove single quote from type names.
	typeName = strings.ReplaceAll(typeName, "'", "")

	return typeName
}

func (d DocLanguageHelper) GetFunctionName(modName string, f *schema.Function) string {
	return PyName(tokenToName(f.Token))
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(modName string, f *schema.Function) string {
	return title(tokenToName(f.Token)) + "Result"
}

func (d DocLanguageHelper) GetMethodName(m *schema.Method) string {
	return PyName(m.Name)
}

func (d DocLanguageHelper) GetMethodResultName(pkg *schema.Package, modName string, r *schema.Resource,
	m *schema.Method,
) string {
	var returnType *schema.ObjectType
	if m.Function.ReturnType != nil {
		if objectType, ok := m.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			returnType = objectType
		}
	}

	if info, ok := pkg.Language["python"].(PackageInfo); ok {
		if info.LiftSingleValueMethodReturns && returnType != nil && len(returnType.Properties) == 1 {
			typeDetails := map[*schema.ObjectType]*typeDetails{}
			mod := &modContext{
				pkg:         pkg.Reference(),
				mod:         modName,
				typeDetails: typeDetails,
			}
			return mod.typeString(returnType.Properties[0].Type, false, false, false /*forDict*/)
		}
	}
	return fmt.Sprintf("%s.%sResult", resourceName(r), title(d.GetMethodName(m)))
}

// GetPropertyName returns the property name specific to Python.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return PyName(p.Name), nil
}

// GetEnumName returns the enum name specific to Python.
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
		displayName = PyPack(pkg.Namespace, pkg.Name)
	} else {
		displayName = fmt.Sprintf("%s/%s", PyPack(pkg.Namespace, pkg.Name), strings.ToLower(modName))
	}
	link = "/docs/reference/pkg/python/" + displayName
	return displayName, link
}
