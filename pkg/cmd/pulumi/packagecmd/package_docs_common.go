// Copyright 2026, Pulumi Corporation.
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

package packagecmd

import (
	"context"
	"strings"

	"github.com/blang/semver"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// resolvedPackage holds the coordinates of a resolved registry package.
type resolvedPackage struct {
	Source    string
	Publisher string
	Name      string
	Version   string
}

// parseAndResolvePackage parses a user-provided package argument (e.g. "aws",
// "aws@7.20.0", "pulumi/pulumi/aws") and resolves it via the registry.
func parseAndResolvePackage(ctx context.Context, pkg string) (resolvedPackage, error) {
	name, versionStr, _ := strings.Cut(pkg, "@")

	var version *semver.Version
	if versionStr != "" {
		v, err := semver.Parse(versionStr)
		if err != nil {
			return resolvedPackage{}, err
		}
		version = &v
	}

	reg := cmdCmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())
	meta, err := registry.ResolvePackageFromName(ctx, reg, name, version)
	if err != nil {
		return resolvedPackage{}, err
	}

	return resolvedPackage{
		Source:    meta.Source,
		Publisher: meta.Publisher,
		Name:      meta.Name,
		Version:   meta.Version.String(),
	}, nil
}

// registryForContext returns a registry suitable for making doc API calls.
func registryForContext(ctx context.Context) registry.Registry {
	return cmdCmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())
}

var runtimeToLang = map[string]string{
	"nodejs": "typescript",
	"dotnet": "csharp",
	"go":     "go",
	"python": "python",
	"yaml":   "yaml",
	"java":   "java",
}

// detectLang returns the docs language inferred from the current Pulumi
// project's runtime, or empty string if detection fails.
func detectLang() string {
	proj, err := workspace.DetectProject()
	if err != nil {
		return ""
	}
	if lang, ok := runtimeToLang[proj.Runtime.Name()]; ok {
		return lang
	}
	return ""
}

const defaultLang = "go"

// effectiveLang returns the explicitly provided language flag, falling back
// to project detection, then to "go" (matching the server-side default).
func effectiveLang(flag string) string {
	if flag != "" {
		return flag
	}
	if detected := detectLang(); detected != "" {
		return detected
	}
	return defaultLang
}

// docsOpts builds a PackageDocsOptions from common flag values.
func docsOpts(lang, os, query string) apitype.PackageDocsOptions {
	return apitype.PackageDocsOptions{
		Lang:  effectiveLang(lang),
		OS:    os,
		Query: query,
	}
}
