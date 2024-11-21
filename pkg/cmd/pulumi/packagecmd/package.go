// Copyright 2016-2024, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Work with Pulumi packages",
		Long: `Work with Pulumi packages

Install and configure Pulumi packages and their plugins and SDKs.`,
		Args: cmdutil.NoArgs,
	}
	cmd.AddCommand(
		newExtractSchemaCommand(),
		newExtractMappingCommand(),
		newGenSdkCommand(),
		newPackagePublishCmd(),
		newPackagePackCmd(),
		newPackageAddCmd(),
	)
	return cmd
}

// schemaFromSchemaSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
//
//	FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
func schemaFromSchemaSource(ctx context.Context, packageSource string, args []string) (*schema.Package, error) {
	var spec schema.PackageSpec
	bind := func(spec schema.PackageSpec) (*schema.Package, error) {
		pkg, diags, err := schema.BindSpec(spec, nil)
		if err != nil {
			return nil, err
		}
		if diags.HasErrors() {
			return nil, diags
		}
		return pkg, nil
	}
	if ext := filepath.Ext(packageSource); ext == ".yaml" || ext == ".yml" {
		if len(args) > 0 {
			return nil, errors.New("parameterization arguments are not supported for yaml files")
		}
		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	} else if ext == ".json" {
		if len(args) > 0 {
			return nil, errors.New("parameterization arguments are not supported for json files")
		}

		f, err := os.ReadFile(packageSource)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(f, &spec)
		if err != nil {
			return nil, err
		}
		return bind(spec)
	}

	p, err := providerFromSource(packageSource)
	if err != nil {
		return nil, err
	}
	defer p.Close()

	var request plugin.GetSchemaRequest
	if len(args) > 0 {
		resp, err := p.Parameterize(ctx, plugin.ParameterizeRequest{Parameters: &plugin.ParameterizeArgs{Args: args}})
		if err != nil {
			return nil, fmt.Errorf("parameterize: %w", err)
		}

		request = plugin.GetSchemaRequest{
			SubpackageName:    resp.Name,
			SubpackageVersion: &resp.Version,
		}
	}

	schema, err := p.GetSchema(ctx, request)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(schema.Schema, &spec)
	if err != nil {
		return nil, err
	}
	return bind(spec)
}

// providerFromSource takes a plugin name or path.
//
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func providerFromSource(packageSource string) (plugin.Provider, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := cmdutil.Diag()
	pCtx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
	if err != nil {
		return nil, err
	}

	pluginSpec, err := workspace.NewPluginSpec(packageSource, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, err
	}

	descriptor := workspace.PackageDescriptor{PluginSpec: pluginSpec}
	isExecutable := func(info fs.FileInfo) bool {
		// Windows doesn't have executable bits to check
		if runtime.GOOS == "windows" {
			return !info.IsDir()
		}
		return info.Mode()&0o111 != 0 && !info.IsDir()
	}

	// No file separators, so we try to look up the schema
	// On unix, these checks are identical. On windows, filepath.Separator is '\\'
	if !strings.ContainsRune(descriptor.Name, filepath.Separator) && !strings.ContainsRune(descriptor.Name, '/') {
		host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, "")
		if err != nil {
			return nil, err
		}
		// We assume this was a plugin and not a path, so load the plugin.
		provider, err := host.Provider(descriptor)
		if err != nil {
			// There is an executable with the same name, so suggest that
			if info, statErr := os.Stat(descriptor.Name); statErr == nil && isExecutable(info) {
				return nil, fmt.Errorf("could not find installed plugin %s, did you mean ./%[1]s: %w", descriptor.Name, err)
			}

			// Try and install the plugin if it was missing and try again, unless auto plugin installs are turned off.
			var missingError *workspace.MissingError
			if !errors.As(err, &missingError) || env.DisableAutomaticPluginAcquisition.Value() {
				return nil, err
			}

			log := func(sev diag.Severity, msg string) {
				host.Log(sev, "", msg, 0)
			}

			_, err = pkgWorkspace.InstallPlugin(pCtx.Base(), descriptor.PluginSpec, log)
			if err != nil {
				return nil, err
			}

			p, err := host.Provider(descriptor)
			if err != nil {
				return nil, err
			}

			return p, nil
		}
		return provider, nil
	}

	// We were given a path to a binary or folder, so invoke that.
	info, err := os.Stat(packageSource)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("could not find file %s", packageSource)
	} else if err != nil {
		return nil, err
	} else if info.IsDir() {
		// If it's a directory we need to add a fake provider binary to the path because that's what NewProviderFromPath
		// expects.
		packageSource = filepath.Join(packageSource, "pulumi-resource-"+info.Name())
	} else {
		if !isExecutable(info) {
			if p, err := filepath.Abs(packageSource); err == nil {
				packageSource = p
			}
			return nil, fmt.Errorf("plugin at path %q not executable", packageSource)
		}
	}

	host, err := plugin.NewDefaultHost(pCtx, nil, false, nil, nil, nil, "")
	if err != nil {
		return nil, err
	}

	p, err := plugin.NewProviderFromPath(host, pCtx, packageSource)
	if err != nil {
		return nil, err
	}
	return p, nil
}
