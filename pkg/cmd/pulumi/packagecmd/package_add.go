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
	"fmt"
	"os"
	"strings"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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
		Short: "Add a package to your Pulumi project",
		Long: `Add a package to your Pulumi project.

This command locally generates an SDK in the currently selected Pulumi language,
adds the package to your project configuration file (Pulumi.yaml), and prints
instructions on how to link it into your project. The SDK is based on a Pulumi
package schema extracted from a given resource plugin or provided
directly.

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
			ws := pkgWorkspace.Instance
			proj, root, err := ws.ReadProject()
			if err != nil {
				return err
			}

			language := proj.Runtime.Name()

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			sink := cmdutil.Diag()
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			pluginSource := args[0]
			parameters := &plugin.ParameterizeArgs{Args: args[1:]}

			pkg, packageSpec, diags, err := packages.InstallPackage(ws, pctx, language, root, pluginSource, parameters,
				cmdCmd.NewDefaultRegistry(cmd.Context(), ws, proj, cmdutil.Diag(), env.Global()))
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				return err
			}

			// Build and add the package spec to the project
			pluginSplit := strings.Split(pluginSource, "@")
			source := pluginSplit[0]
			version := ""
			if pkg.Version != nil {
				version = pkg.Version.String()
			} else if len(pluginSplit) == 2 {
				version = pluginSplit[1]
			}
			if pkg.Parameterization != nil {
				source = pkg.Parameterization.BaseProvider.Name
				version = pkg.Parameterization.BaseProvider.Version.String()
			}
			if len(parameters.Args) > 0 && packageSpec != nil {
				packageSpec.Parameters = parameters.Args
			} else if packageSpec == nil {
				packageSpec = &workspace.PackageSpec{
					Source:     source,
					Version:    version,
					Parameters: parameters.Args,
				}
			}
			proj.AddPackage(pkg.Name, *packageSpec)

			// Save the updated project
			if err := workspace.SaveProject(proj); err != nil {
				return fmt.Errorf("failed to update Pulumi.yaml: %w", err)
			}

			fmt.Printf("Added package %q to Pulumi.yaml\n", pkg.Name)
			return nil
		},
	}

	return cmd
}
