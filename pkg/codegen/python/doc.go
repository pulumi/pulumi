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
func (d DocLanguageHelper) GetTypeName(pkg schema.PackageReference, t schema.Type, input bool, relativeToModule string) string {
	var info PackageInfo
	if a, err := pkg.Language("python"); err == nil {
		info, _ = a.(PackageInfo)
	}

	mod := &modContext{
		pkg:              pkg,
		mod:              moduleToPythonModule(relativeToModule, info.ModuleNameOverrides),
		modNameOverrides: info.ModuleNameOverrides,
		typeDetails:      map[*schema.ObjectType]*typeDetails{},
	}
	typeName := mod.typeString(t, typeStringOpts{input: input, forDocs: true})

	// Remove any package qualifiers from the type name.
	if !input {
		typeName = strings.ReplaceAll(typeName, "outputs.", "")
	}

	// Remove single quote from type names.
	typeName = strings.ReplaceAll(typeName, "'", "")

	return typeName
}

func (d DocLanguageHelper) GetFunctionName(f *schema.Function) string {
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

func (d DocLanguageHelper) GetMethodResultName(pkg schema.PackageReference, modName string, r *schema.Resource,
	m *schema.Method,
) string {
	var returnType *schema.ObjectType
	if m.Function.ReturnType != nil {
		if objectType, ok := m.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			returnType = objectType
		}
	}

	var info PackageInfo
	if i, err := pkg.Language("python"); err == nil {
		info, _ = i.(PackageInfo)
	}

	if info.LiftSingleValueMethodReturns && returnType != nil && len(returnType.Properties) == 1 {
		typeDetails := map[*schema.ObjectType]*typeDetails{}
		mod := &modContext{
			pkg:         pkg,
			mod:         modName,
			typeDetails: typeDetails,
		}
		return mod.typeString(returnType.Properties[0].Type, typeStringOpts{})
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
