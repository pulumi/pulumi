// Copyright 2016-2021, Pulumi Corporation.
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

package python

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Generates code to build and regsiter ResourceModule and
// ResourcePackage instances with the Pulumi runtime. This code
// supports deserialization of references into fully hydrated Resource
// and Provider instances.
//
// Implementation scans the given root `modContext` for resource
// packages and modules, passes the info down as JSON literals to the
// generated code, and generates a `_utilities.register` call to do
// the heavy lifting.
//
// Generates code only for the top-level `__init__.py`. Whenever any
// of the sub-modules is imported, Python imports top-level module
// also. This scheme ensures all resource modules are registered
// eagerly even when we apply lazy loading for some of the modules.
func genResourceMappings(root *modContext, w io.Writer) error {
	if root.isTopLevel() {
		rm, err := jsonPythonLiteral(allResourceModuleInfos(root))
		if err != nil {
			return err
		}
		rp, err := jsonPythonLiteral(allResourcePackageInfos(root))
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "_utilities.register(\n    resource_modules=%s,\n    resource_packages=%s\n)\n", rm, rp)
		return nil
	}
	return nil
}

func jsonPythonLiteral(thing interface{}) (string, error) {
	bytes, err := json.MarshalIndent(thing, "", " ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("\"\"\"\n%s\n\"\"\"", string(bytes)), nil
}

// Information about a resource module and how it maps tokens to
// Python classes.
//
// Example:
//
// {
//   "pkg": "azure-native",
//   "mod": "databricks",
//   "fqn": "pulumi_azure_native.databricks"
//   "classes": {
//     "azure-native:databricks:Workspace": "Workspace",
//     "azure-native:databricks:vNetPeering": "VNetPeering"
//   }
// }
type resourceModuleInfo struct {
	Pkg     string            `json:"pkg"`
	Mod     string            `json:"mod"`
	Fqn     string            `json:"fqn"`
	Classes map[string]string `json:"classes"`
}

func (rmi *resourceModuleInfo) Token() string {
	return fmt.Sprintf("%s:%s", rmi.Pkg, rmi.Mod)
}

func makeResourceModuleInfo(pkg, mod, fqn string) resourceModuleInfo {
	return resourceModuleInfo{pkg, mod, fqn, make(map[string]string)}
}

func allResourceModuleInfos(root *modContext) []resourceModuleInfo {
	var result []resourceModuleInfo
	for _, mctx := range root.walkSelfWithDescendants() {
		result = append(result, collectResourceModuleInfos(mctx)...)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Token() < result[j].Token()
	})
	return result
}

func collectResourceModuleInfos(mctx *modContext) []resourceModuleInfo {
	byMod := make(map[string]resourceModuleInfo)

	for _, res := range mctx.resources {
		if !res.IsProvider {
			pkg := mctx.pkg.Name
			mod := mctx.pkg.TokenToRuntimeModule(res.Token)
			fqn := mctx.fullyQualifiedImportName()

			rmi, found := byMod[mod]
			if !found {
				rmi = makeResourceModuleInfo(pkg, mod, fqn)
				byMod[mod] = rmi
			}

			rmi.Classes[res.Token] = tokenToName(res.Token)
		}
	}

	var result []resourceModuleInfo
	for _, rmi := range byMod {
		result = append(result, rmi)
	}

	return result
}

// Information about a package.
//
// Example:
//
// {
//   "pkg": "azure-native",
//   "token": "pulumi:providers:azure-native"
//   "fqn": "pulumi_azure_native"
//   "class": "Provider"
// }

type resourcePackageInfo struct {
	Pkg   string `json:"pkg"`
	Token string `json:"token"`
	Fqn   string `json:"fqn"`
	Class string `json:"class"`
}

func allResourcePackageInfos(root *modContext) []resourcePackageInfo {
	var result []resourcePackageInfo
	for _, mctx := range root.walkSelfWithDescendants() {
		result = append(result, collectResourcePackageInfos(mctx)...)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].Token < result[j].Token
	})
	return result
}

func collectResourcePackageInfos(mctx *modContext) []resourcePackageInfo {
	var out []resourcePackageInfo
	for _, res := range mctx.resources {
		if res.IsProvider {
			pkg := mctx.pkg.Name
			token := res.Token
			fqn := mctx.fullyQualifiedImportName()
			class := "Provider"
			out = append(out, resourcePackageInfo{pkg, token, fqn, class})
		}
	}
	return out
}
