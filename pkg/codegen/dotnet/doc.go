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
type DocLanguageHelper struct{}

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

// GetDocLinkForInputType is not implemented at this time for Python.
func (d DocLanguageHelper) GetDocLinkForInputType(packageName, moduleName, typeName string) string {
	return d.GetDocLinkForResourceType(packageName, moduleName, typeName)
}

// GetLanguageTypeString returns the DotNet-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string {
	typeDetails := map[*schema.ObjectType]*typeDetails{}
	mod := &modContext{
		pkg:         pkg,
		typeDetails: typeDetails,
	}
	return mod.typeString(t, "", input, false /*state*/, false /*wrapInput*/, true /*requireInitializers*/, optional)
}
