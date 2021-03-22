//go:generate go run bundler.go

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
package docs

import (
	"path"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func isKubernetesPackage(pkg *schema.Package) bool {
	return pkg.Name == "kubernetes"
}

func (mod *modContext) isKubernetesOverlayModule() bool {
	// The CustomResource overlay resource is directly under the apiextensions module
	// and not under a version, so we include that. The Directory overlay resource is directly under the
	// kustomize module. The resources under helm and yaml are always under a version.
	return mod.mod == "apiextensions" || mod.mod == "kustomize" ||
		strings.HasPrefix(mod.mod, "helm") || strings.HasPrefix(mod.mod, "yaml")
}

func (mod *modContext) isComponentResource() bool {
	// TODO: Support this more generally. For now, only the Helm, Kustomize, and YAML overlays use ComponentResources.
	return strings.HasPrefix(mod.mod, "helm") ||
		strings.HasPrefix(mod.mod, "kustomize") ||
		strings.HasPrefix(mod.mod, "yaml")
}

// getKubernetesOverlayPythonFormalParams returns the formal params to render
// for a Kubernetes overlay resource. These resources do not follow convention
// that other resources do, so it is best to manually set these.
func getKubernetesOverlayPythonFormalParams(modName string) []formalParam {
	var params []formalParam
	switch modName {
	case "helm/v2", "helm/v3":
		params = []formalParam{
			{
				Name: "config",
			},
			{
				Name:         "opts",
				DefaultValue: "=None",
			},
		}
	case "kustomize":
		params = []formalParam{
			{
				Name: "directory",
			},
			{
				Name:         "opts",
				DefaultValue: "=None",
			},
			{
				Name:         "transformations",
				DefaultValue: "=None",
			},
			{
				Name:         "resource_prefix",
				DefaultValue: "=None",
			},
		}
	case "yaml":
		params = []formalParam{
			{
				Name: "file",
			},
			{
				Name:         "opts",
				DefaultValue: "=None",
			},
			{
				Name:         "transformations",
				DefaultValue: "=None",
			},
			{
				Name:         "resource_prefix",
				DefaultValue: "=None",
			},
		}
	case "apiextensions":
		params = []formalParam{
			{
				Name: "api_version",
			},
			{
				Name: "kind",
			},
			{
				Name:         "metadata",
				DefaultValue: "=None",
			},
			{
				Name:         "opts",
				DefaultValue: "=None",
			},
		}
	}
	return params
}

func getKubernetesMod(pkg *schema.Package, token string, modules map[string]*modContext, tool string) *modContext {
	modName := pkg.TokenToModule(token)
	// Kubernetes' moduleFormat in the schema will match everything
	// in the token. So strip some well-known domain name parts from the module
	// names.
	modName = strings.TrimSuffix(modName, ".k8s.io")
	modName = strings.TrimSuffix(modName, ".apiserver")
	modName = strings.TrimSuffix(modName, ".authorization")

	mod, ok := modules[modName]
	if !ok {
		mod = &modContext{
			pkg:  pkg,
			mod:  modName,
			tool: tool,
		}

		if modName != "" {
			parentName := path.Dir(modName)
			// If the parent name is blank, it means this is the package-level.
			if parentName == "." || parentName == "" {
				parentName = ":index:"
			} else {
				parentName = ":" + parentName + ":"
			}
			parent := getKubernetesMod(pkg, parentName, modules, tool)
			parent.children = append(parent.children, mod)
		}

		modules[modName] = mod
	}
	return mod
}
