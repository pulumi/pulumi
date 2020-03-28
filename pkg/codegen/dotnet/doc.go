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

// nolint: lll
package dotnet

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

// DocLanguageHelper is the DotNet-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct {
	// Namespaces is a map of Pulumi schema module names to their
	// C# equivalent names, to be used when creating fully-qualified
	// property type strings.
	Namespaces map[string]string
}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForResourceType returns the .NET API doc URL for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(packageName, _, typeName string) string {
	typeName = strings.ReplaceAll(typeName, "?", "")
	var packageNamespace string
	if packageName != "" {
		packageNamespace = "." + title(packageName)
	}
	return fmt.Sprintf("/docs/reference/pkg/dotnet/Pulumi%s/%s.html", packageNamespace, typeName)
}

// GetDocLinkForResourceInputOrOutputType returns the doc link for an input or output type of a Resource.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(packageName, moduleName, typeName string, input bool) string {
	return d.GetDocLinkForResourceType(packageName, moduleName, typeName)
}

// GetDocLinkForFunctionInputOrOutputType returns the doc link for an input or output type of a Function.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(packageName, moduleName, typeName string, input bool) string {
	return d.GetDocLinkForResourceType(packageName, moduleName, typeName)
}

// GetLanguageTypeString returns the DotNet-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string {
	typeDetails := map[*schema.ObjectType]*typeDetails{}
	mod := &modContext{
		pkg:         pkg,
		mod:         moduleName,
		typeDetails: typeDetails,
		namespaces:  d.Namespaces,
	}
	qualifier := "Inputs"
	if !input {
		qualifier = "Outputs"
	}
	return mod.typeString(t, qualifier, input, false /*state*/, false /*wrapInput*/, true /*requireInitializers*/, optional)
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(resourceName string) string {
	return "Get" + resourceName + "Result"
}

// GetPropertyName uses the property's csharp-specific language info, if available, to generate
// the property name. Otherwise, returns the PascalCase as the default.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	propLangName := strings.Title(p.Name)

	names := map[*schema.Property]string{}
	properties := []*schema.Property{p}
	if err := computePropertyNames(properties, names); err != nil {
		return "", err
	}

	if name, ok := names[p]; ok {
		return name, nil
	}
	return propLangName, nil
}
