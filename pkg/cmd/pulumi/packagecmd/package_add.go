// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// InstallPackage installs a package to the project by generating an SDK and linking it.
// It returns the path to the installed package.
func InstallPackage(ws pkgWorkspace.Context, pctx *plugin.Context, language, root,
	schemaSource string, parameters []string,
) (*schema.Package, error) {
	pkg, err := SchemaFromSchemaSource(pctx, schemaSource, parameters)
	if err != nil {
		var diagErr hcl.Diagnostics
		if errors.As(err, &diagErr) {
			return nil, fmt.Errorf("failed to get schema. Diagnostics: %w", errors.Join(diagErr.Errs()...))
		}
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	tempOut, err := os.MkdirTemp("", "pulumi-package-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempOut)

	local := true

	err = GenSDK(
		language,
		tempOut,
		pkg,
		"",    /*overlays*/
		local, /*local*/
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SDK: %w", err)
	}

	out := filepath.Join(root, "sdks")
	err = os.MkdirAll(out, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory for SDK: %w", err)
	}

	outName := pkg.Name
	if pkg.Namespace != "" {
		outName = pkg.Namespace + "-" + outName
	}
	out = filepath.Join(out, outName)

	// If directory already exists, remove it completely before copying new files
	if _, err := os.Stat(out); err == nil {
		if err := os.RemoveAll(out); err != nil {
			return nil, fmt.Errorf("failed to clean existing SDK directory: %w", err)
		}
	}

	err = CopyAll(out, filepath.Join(tempOut, language))
	if err != nil {
		return nil, fmt.Errorf("failed to move SDK to project: %w", err)
	}

	// Link the package to the project
	if err := LinkPackage(ws, language, root, pkg, out); err != nil {
		return nil, err
	}

	return pkg, nil
}

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider|schema|path> [provider-parameter...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a package to your Pulumi project",
		Long: `Add a package to your Pulumi project.

This command locally generates an SDK in the currently selected Pulumi language
and prints instructions on how to link it into your project. The SDK is based on
a Pulumi package schema extracted from a given resource plugin or provided
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
			pctx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			plugin := args[0]
			parameters := args[1:]

			pkg, err := InstallPackage(ws, pctx, language, root, plugin, parameters)
			if err != nil {
				return err
			}

			// Build and add the package spec to the project
			pluginSplit := strings.Split(plugin, "@")
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
			proj.AddPackage(pkg.Name, workspace.PackageSpec{
				Source:     source,
				Version:    version,
				Parameters: parameters,
			})

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
