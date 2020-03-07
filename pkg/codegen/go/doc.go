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
package gen

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

// DocLanguageHelper is the Go-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct{}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForResourceType returns the godoc URL for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(packageName string, moduleName string, typeName string) string {
	path := fmt.Sprintf("%s/%s", packageName, moduleName)
	typeNameParts := strings.Split(typeName, ".")
	typeName = typeNameParts[len(typeNameParts)-1]
	return fmt.Sprintf("https://pkg.go.dev/github.com/pulumi/pulumi-%s/sdk/go/%s?tab=doc#%s", packageName, path, typeName)
}

// GetDocLinkForInputType returns the godoc URL for an input type.
func (d DocLanguageHelper) GetDocLinkForInputType(packageName, moduleName, typeName string) string {
	name := d.GetDocLinkForResourceType(packageName, moduleName, typeName)
	return name + "Args"
}

// GetDocLinkForBuiltInType returns the godoc URL for a built-in type.
func GetDocLinkForBuiltInType(typeName string) string {
	return fmt.Sprintf("https://golang.org/pkg/builtin/#%s", typeName)
}

// GetLanguageType returns the Go-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetLanguageType(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string {
	mod := &pkgContext{
		pkg: pkg,
	}
	return mod.plainType(t, optional)
}
