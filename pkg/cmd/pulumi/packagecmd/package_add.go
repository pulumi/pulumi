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

package packagecmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider|schema|path> [provider-parameter...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a package to your Pulumi project or plugin",
		Long: `Add a package to your Pulumi project or plugin.

This command locally generates an SDK in the currently selected Pulumi language,
adds the package to your project configuration file (Pulumi.yaml or
PulumiPlugin.yaml), and prints instructions on how to use it in your project.
The SDK is based on a Pulumi package schema extracted from a given resource
plugin or provided directly.

The <provider> argument can be specified in one of the following ways:

- When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to
  resolve a resource plugin first, installing it on-demand, similarly to:

    pulumi plugin install resource PLUGIN [VERSION]

- When <provider> is specified as a local path, Pulumi executes the provider
  binary to extract its package schema:

    pulumi package add ./my-provider

- When <provider> is a path to a local file with a '.json', '.yml' or '.yaml'
  extension, Pulumi package schema is read from it directly:

    pulumi package add ./my/schema.json

- When <provider> is a reference to a Git repo, Pulumi clones the repo and
  executes the source. Optionally a version can be specified.  It can either
  be a tag (in semver format), or a Git commit hash.  By default the latest
  tag (by semver version), or if not available the latest commit on the
  default branch is used. Paths can be disambiguated from the repo name by
  appending '.git' to the repo URL, followed by the path to the package:

    pulumi package add example.org/org/repo.git/path[@<version>]

For parameterized providers, parameters may be specified as additional
arguments. The exact format of parameters is provider-specific; consult the
provider's documentation for more information. If the parameters include flags
that begin with dashes, you may need to use '--' to separate the provider name
from the parameters, as in:

  pulumi package add <provider> -- --provider-parameter-flag value
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			pluginOrProject, err := detectEnclosingPluginOrProject(cmd.Context(), wd)
			if err != nil {
				return err
			}

			sink := cmdutil.Diag()
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, pluginOrProject.installRoot, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			pluginSource := args[0]
			parameters := &plugin.ParameterizeArgs{Args: args[1:]}

			pkg, packageSpec, diags, err := packages.InstallPackage(
				pluginOrProject.proj,
				pctx,
				pluginOrProject.proj.RuntimeInfo().Name(),
				pluginOrProject.installRoot,
				pluginSource,
				parameters,
				pluginOrProject.reg,
				env.Global(),
				0, /* unbounded concurrency */
			)
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				return err
			}

			// Build and add the package spec to the project
			pluginSplit := strings.Split(pluginSource, "@")
			source := pluginSplit[0]

			if ext := filepath.Ext(source); ext == ".yaml" || ext == ".yml" || ext == ".json" {
				// We don't add file based schemas to the project's packages, since there is no actual underlying
				// provider for them.
				return nil
			}

			// TODO[#21349]:  We can't bake  a path into  Pulumi.yaml until we  use [packageresolution.Resolve]
			// when loading a new context, so condense local paths to the name of the package.
			//
			// This is wrong, but its less wrong then producing a Pulumi.yaml that `pulumi` can't process
			// (#21348).
			if plugin.IsLocalPluginPath(cmd.Context(), packageSpec.Source) {
				f, err := os.Stat(packageSpec.Source)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if !f.IsDir() {
					if pkg.Parameterization == nil {
						packageSpec.Source = pkg.Name
						if pkg.Version != nil {
							packageSpec.Version = pkg.Version.String()
						}
					} else {
						packageSpec.Source = pkg.Parameterization.BaseProvider.Name
						packageSpec.Version = pkg.Parameterization.BaseProvider.Version.String()
					}
				}
			}

			contract.Assertf(packageSpec != nil, "packageSpec should be nil if & only if source is file based")
			packageSpec.Parameters = parameters.Args

			pluginOrProject.proj.AddPackage(pkg.Name, *packageSpec)

			fileName := filepath.Base(pluginOrProject.projectFilePath)
			// Save the updated project
			if err := pluginOrProject.proj.Save(pluginOrProject.projectFilePath); err != nil {
				return fmt.Errorf("failed to update %s: %w", fileName, err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Added package %s\n", schemaDisplayName(pkg))
			return nil
		},
	}

	return cmd
}

type pluginOrProject struct {
	installRoot, projectFilePath string
	reg                          registry.Registry
	proj                         workspace.BaseProject
}

func schemaDisplayName(schema *schema.Package) string {
	name := schema.DisplayName
	if name == "" {
		name = schema.Name
	}
	if schema.Namespace != "" {
		name = schema.Namespace + "/" + name
	}
	return name
}

// Detect the nearest enclosing Pulumi Project or Pulumi Plugin root directory.
func detectEnclosingPluginOrProject(ctx context.Context, wd string) (pluginOrProject, error) {
	baseProject, filePath, err := workspace.LoadBaseProjectFrom(wd)
	if err != nil {
		return pluginOrProject{}, err
	}

	switch baseProject := baseProject.(type) {
	case *workspace.Project:
		return pluginOrProject{
			installRoot:     filepath.Dir(filePath),
			projectFilePath: filePath,
			reg:             cmdCmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, baseProject, cmdutil.Diag(), env.Global()),
			proj:            baseProject,
		}, nil
	case *workspace.PluginProject:
		return pluginOrProject{
			installRoot:     filepath.Dir(filePath),
			projectFilePath: filePath,
			proj:            baseProject,
			// Cloud registry is linked to a backend, but we don't have one
			// available in a plugin. Use the unauthenticated registry.
			reg: unauthenticatedregistry.New(cmdutil.Diag(), env.Global()),
		}, nil
	default:
		panic(fmt.Sprintf("workspace.LoadBaseProjectFrom promises that it will return "+
			"either *workspace.Project or *workspace.PluginProject, found %T", baseProject))
	}
}
