// Copyright 2025, Pulumi Corporation.
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

package loadpackage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// SchemaFromSchemaSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//	FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
func SchemaFromSchemaSource(
	ctx context.Context, packageSource string, parameters plugin.ParameterizeParameters,
	registry registry.Registry, relativeTo string,
	host plugin.Host, sink diag.Sink,
) (*schema.PackageSpec, *workspace.PackageSpec, error) {
	var pkgSpec schema.PackageSpec
	if ext := filepath.Ext(packageSource); slices.Contains(encoding.Exts, ext) {
		if !parameters.Empty() {
			return nil, nil, fmt.Errorf("parameterization arguments are not supported for %s files", ext)
		}
		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, nil, err
		}
		encoder, _ := encoding.Detect(packageSource)
		return &pkgSpec, nil, encoder.Unmarshal(f, &pkgSpec)
	}

	p, spec, err := ProviderFromSource(ctx, packageSource, relativeTo, registry, host, sink)
	if err != nil {
		return nil, nil, err
	}
	defer func() { contract.IgnoreError(host.CloseProvider(p.Provider)) }()

	var request plugin.GetSchemaRequest
	if !parameters.Empty() {
		if p.AlreadyParameterized {
			return nil, nil, fmt.Errorf("cannot specify parameters since %s is already parameterized", packageSource)
		}
		resp, err := p.Provider.Parameterize(ctx, plugin.ParameterizeRequest{
			Parameters: parameters,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("parameterize: %w", err)
		}

		request = plugin.GetSchemaRequest{
			SubpackageName:    resp.Name,
			SubpackageVersion: &resp.Version,
		}
	}

	schema, err := p.Provider.GetSchema(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	err = json.Unmarshal(schema.Schema, &pkgSpec)
	if err != nil {
		return nil, nil, err
	}
	pluginSpec, err := workspace.NewPluginSpec(ctx, packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if pluginSpec.PluginDownloadURL != "" {
		pkgSpec.PluginDownloadURL = pluginSpec.PluginDownloadURL
	}
	setSpecNamespace(&pkgSpec, pluginSpec)
	return &pkgSpec, spec, nil
}

func setSpecNamespace(spec *schema.PackageSpec, pluginSpec workspace.PluginSpec) {
	if spec.Namespace == "" && pluginSpec.IsGitPlugin() {
		namespaceRegex := regexp.MustCompile(`git://[^/]+/([^/]+)/`)
		matches := namespaceRegex.FindStringSubmatch(pluginSpec.PluginDownloadURL)
		if len(matches) == 2 {
			spec.Namespace = strings.ToLower(matches[1])
		}
	}
}

type Provider struct {
	plugin.Provider

	AlreadyParameterized bool
}

// ProviderFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(
	ctx context.Context, packageSource string,
	relativeTo string, reg registry.Registry,
	host plugin.Host, sink diag.Sink,
) (Provider, *workspace.PackageSpec, error) {
	pluginSpec, err := workspace.NewPluginSpec(ctx, packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return Provider{}, nil, err
	}
	descriptor := workspace.PackageDescriptor{
		PluginSpec: pluginSpec,
	}

	hostProvider := func(descriptor workspace.PackageDescriptor) (Provider, error) {
		p, err := host.Provider(descriptor)
		if err == nil {
			return Provider{
				Provider:             p,
				AlreadyParameterized: descriptor.Parameterization != nil,
			}, nil
		}
		return Provider{}, err
	}

	setupProvider := func(
		descriptor workspace.PackageDescriptor, specOverride *workspace.PackageSpec,
	) (Provider, *workspace.PackageSpec, error) {
		p, err := hostProvider(descriptor)
		if err != nil {
			return Provider{}, nil, err
		}
		if descriptor.Parameterization != nil {
			_, err := p.Provider.Parameterize(ctx, plugin.ParameterizeRequest{
				Parameters: &plugin.ParameterizeValue{
					Name:    descriptor.Parameterization.Name,
					Version: descriptor.Parameterization.Version,
					Value:   descriptor.Parameterization.Value,
				},
			})
			if err != nil {
				return Provider{}, nil, fmt.Errorf("failed to parameterize %s: %w", p.Provider.Pkg().Name(), err)
			}
		}
		return p, specOverride, nil
	}

	result, err := packageresolution.Resolve(
		ctx,
		reg,
		packageresolution.DefaultWorkspace(),
		pluginSpec,
		packageresolution.Options{
			DisableRegistryResolve:      env.DisableRegistryResolve.Value(),
			Experimental:                env.Experimental.Value(),
			IncludeInstalledInWorkspace: true,
		},
		relativeTo,
	)
	if err != nil {
		var packageNotFoundErr *packageresolution.PackageNotFoundError
		if errors.As(err, &packageNotFoundErr) {
			for _, suggested := range packageNotFoundErr.Suggestions() {
				sink.Infof(diag.Message("", "%s/%s/%s@%s is a similar package"),
					suggested.Source, suggested.Publisher, suggested.Name, suggested.Version)
			}
		}
		return Provider{}, nil, fmt.Errorf("Unable to resolve package from name: %w", err)
	}

	switch res := result.(type) {
	case packageresolution.LocalPathResult:
		pctx, err := plugin.NewContext(ctx, sink, sink, host, nil, relativeTo, nil, false, nil)
		if err != nil {
			return Provider{}, nil, err
		}
		// We don't close pctx, since that would close the host passed in.
		return setupProviderFromPath(pctx, res.LocalPluginPathAbs)
	case packageresolution.ExternalSourceResult, packageresolution.InstalledInWorkspaceResult:
		return setupProvider(descriptor, nil)
	case packageresolution.RegistryResult:
		return setupProviderFromRegistryMeta(res.Metadata, setupProvider)
	default:
		contract.Failf("Unexpected result type: %T", result)
		return Provider{}, nil, nil
	}
}

func setupProviderFromRegistryMeta(
	meta apitype.PackageMetadata,
	setupProvider func(workspace.PackageDescriptor, *workspace.PackageSpec) (Provider, *workspace.PackageSpec, error),
) (Provider, *workspace.PackageSpec, error) {
	spec := workspace.PluginSpec{
		Name:              meta.Name,
		Kind:              apitype.ResourcePlugin,
		Version:           &meta.Version,
		PluginDownloadURL: meta.PluginDownloadURL,
	}
	var params *workspace.Parameterization
	if meta.Parameterization != nil {
		spec.Name = meta.Parameterization.BaseProvider.Name
		spec.Version = &meta.Parameterization.BaseProvider.Version
		params = &workspace.Parameterization{
			Name:    meta.Name,
			Version: meta.Version,
			Value:   meta.Parameterization.Parameter,
		}
	}
	return setupProvider(workspace.NewPackageDescriptor(spec, params), &workspace.PackageSpec{
		Source:  meta.Source + "/" + meta.Publisher + "/" + meta.Name,
		Version: meta.Version.String(),
	})
}

func setupProviderFromPath(pctx *plugin.Context, packageSource string) (Provider, *workspace.PackageSpec, error) {
	info, err := os.Stat(packageSource)
	if os.IsNotExist(err) {
		return Provider{}, nil, fmt.Errorf("could not find file %s", packageSource)
	} else if err != nil {
		return Provider{}, nil, err
	} else if !info.IsDir() && !isExecutable(info) {
		if p, err := filepath.Abs(packageSource); err == nil {
			packageSource = p
		}
		return Provider{}, nil, fmt.Errorf("plugin at path %q not executable", packageSource)
	}

	p, err := plugin.NewProviderFromPath(pctx.Host, pctx, packageSource)
	if err != nil {
		return Provider{}, nil, err
	}
	return Provider{Provider: p}, nil, nil
}

func isExecutable(info fs.FileInfo) bool {
	// Windows doesn't have executable bits to check
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}
	return info.Mode()&0o111 != 0 && !info.IsDir()
}
