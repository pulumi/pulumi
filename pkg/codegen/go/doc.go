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
package gen

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const pulumiSDKVersion = "v3"

// DocLanguageHelper is the Go-specific implementation of the DocLanguageHelper.
type DocLanguageHelper struct {
	packages    map[string]*pkgContext
	topLevelPkg schema.PackageReference
	goPkgInfo   GoPackageInfo
}

var _ codegen.DocLanguageHelper = DocLanguageHelper{}

// GetDocLinkForPulumiType returns the doc link for a Pulumi type.
func (d DocLanguageHelper) GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string {
	version := pulumiSDKVersion
	if info, ok := pkg.Language["go"].(GoPackageInfo); ok {
		if info.PulumiSDKVersion == 1 {
			return "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#" + typeName
		}
		if info.PulumiSDKVersion != 0 {
			version = fmt.Sprintf("v%d", info.PulumiSDKVersion)
		}
	}
	return fmt.Sprintf("https://pkg.go.dev/github.com/pulumi/pulumi/sdk/%s/go/pulumi?tab=doc#%s", version, typeName)
}

// GetDocLinkForResourceType returns the godoc URL for a type belonging to a resource provider.
func (d DocLanguageHelper) GetDocLinkForResourceType(pkg *schema.Package, moduleName string, typeName string) string {
	path := fmt.Sprintf("%s/%s", packageName(pkg), moduleName)
	typeNameParts := strings.Split(typeName, ".")
	typeName = typeNameParts[len(typeNameParts)-1]
	typeName = strings.TrimPrefix(typeName, "*")

	moduleVersion := ""
	if pkg.Version != nil {
		if pkg.Version.Major > 1 {
			moduleVersion = fmt.Sprintf("v%d/", pkg.Version.Major)
		}
	}

	return fmt.Sprintf("https://pkg.go.dev/github.com/pulumi/pulumi-%s/sdk/%sgo/%s?tab=doc#%s", pkg.Name, moduleVersion, path, typeName)
}

// GetDocLinkForResourceInputOrOutputType returns the godoc URL for an input or output type.
func (d DocLanguageHelper) GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string {
	link := d.GetDocLinkForResourceType(pkg, moduleName, typeName)
	if !input {
		return link + "Output"
	}
	return link + "Args"
}

// GetDocLinkForFunctionInputOrOutputType returns the doc link for an input or output type of a Function.
func (d DocLanguageHelper) GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string {
	link := d.GetDocLinkForResourceType(pkg, moduleName, typeName)
	if !input {
		return link
	}
	return link + "Args"
}

// GetLanguageTypeString returns the Go-specific type given a Pulumi schema type.
func (d DocLanguageHelper) GetTypeName(pkg schema.PackageReference, t schema.Type, input bool, relativeToModule string) string {
	goPkg := moduleToPackage(d.goPkgInfo.ModuleToPackage, relativeToModule)
	modPkg, ok := d.packages[goPkg]
	if !ok {
		glog.Fatalf("cannot calculate type string for type %q. could not find a package for module %q",
			t.String(), goPkg)
	}
	return modPkg.typeString(t)
}

// GeneratePackagesMap generates a map of Go packages for resources, functions and types.
func (d *DocLanguageHelper) GeneratePackagesMap(pkg schema.PackageReference, tool string, goInfo GoPackageInfo) {
	var err error
	d.packages, err = generatePackageContextMap(tool, pkg, goInfo, nil)
	d.goPkgInfo = goInfo
	d.topLevelPkg = pkg
	contract.AssertNoErrorf(err, "Could not generate package context map for %q", pkg.Name())
}

// GetPropertyName returns the property name specific to Go.
func (d DocLanguageHelper) GetPropertyName(p *schema.Property) (string, error) {
	return strings.Title(p.Name), nil
}

// GetEnumName returns the enum name specific to Go.
func (d DocLanguageHelper) GetEnumName(e *schema.Enum, typeName string) (string, error) {
	name := fmt.Sprintf("%v", e.Value)
	if e.Name != "" {
		name = e.Name
	}
	return makeSafeEnumName(name, typeName)
}

func (d DocLanguageHelper) GetFunctionName(f *schema.Function) string {
	funcName := tokenToName(f.Token)
	if d.topLevelPkg == nil {
		return funcName
	}
	pkg, ok := d.packages[tokenToPackage(d.topLevelPkg, d.goPkgInfo.ModuleToPackage, f.Token)]
	if !ok {
		return funcName
	}

	if override, ok := pkg.functionNames[f]; ok {
		funcName = override
	}
	return funcName
}

// GetResourceFunctionResultName returns the name of the result type when a function is used to lookup
// an existing resource.
func (d DocLanguageHelper) GetResourceFunctionResultName(modName string, f *schema.Function) string {
	funcName := d.GetFunctionName(f)
	return funcName + "Result"
}

func (d DocLanguageHelper) GetMethodName(m *schema.Method) string {
	return Title(m.Name)
}

func (d DocLanguageHelper) GetMethodResultName(pkg schema.PackageReference, modName string, r *schema.Resource,
	m *schema.Method,
) string {
	var info GoPackageInfo
	if i, err := pkg.Language("go"); err == nil {
		info, _ = i.(GoPackageInfo)
	}
	var objectReturnType *schema.ObjectType
	if m.Function.ReturnType != nil {
		if objectType, ok := m.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			objectReturnType = objectType
		}
	}

	if info.LiftSingleValueMethodReturns && objectReturnType != nil && len(objectReturnType.Properties) == 1 {
		t := objectReturnType.Properties[0].Type
		modPkg, ok := d.packages[modName]
		if !ok {
			glog.Fatalf("cannot calculate type string for type %q. could not find a package for module %q",
				t.String(), modName)
		}
		return modPkg.outputType(t)
	}
	return fmt.Sprintf("%s%sResultOutput", rawResourceName(r), d.GetMethodName(m))
}
