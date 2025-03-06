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

	"github.com/hashicorp/hcl/v2"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider|schema> [provider-parameter...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a package to your Pulumi project",
		Long: `Add a package to your Pulumi project.

This command locally generates an SDK in the currently selected Pulumi language
and prints instructions on how to link it into your project. The SDK is based on
a Pulumi package schema extracted from a given resource plugin or provided
directly.

When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to
resolve a resource plugin first, installing it on-demand, similarly to:

  pulumi plugin install resource PLUGIN [VERSION]

When <provider> is specified as a local path, Pulumi executes the provider
binary to extract its package schema:

  pulumi package add ./my-provider

For parameterized providers, parameters may be specified as additional
arguments. The exact format of parameters is provider-specific; consult the
provider's documentation for more information. If the parameters include flags
that begin with dashes, you may need to use '--' to separate the provider name
from the parameters, as in:

  pulumi package add <provider> -- --provider-parameter-flag value

When <schema> is a path to a local file with a '.json', '.yml' or '.yaml'
extension, Pulumi package schema is read from it directly:

  pulumi package add ./my/schema.json`,
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

			pkg, err := SchemaFromSchemaSource(pctx, plugin, parameters)
			if err != nil {
				var diagErr hcl.Diagnostics
				if errors.As(err, &diagErr) {
					return fmt.Errorf("failed to get schema.  Diagnostics: %w", errors.Join(diagErr.Errs()...))
				}
				return fmt.Errorf("failed to get schema: %w", err)
			}

			tempOut, err := os.MkdirTemp("", "pulumi-package-add-")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}

			local := true

			err = GenSDK(
				language,
				tempOut,
				pkg,
				"",    /*overlays*/
				local, /*local*/
			)
			if err != nil {
				return fmt.Errorf("failed to generate SDK: %w", err)
			}

			out := filepath.Join(root, "sdks")
			err = os.MkdirAll(out, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create directory for SDK: %w", err)
			}

			out = filepath.Join(out, pkg.Name)
			err = CopyAll(out, filepath.Join(tempOut, language))
			if err != nil {
				return fmt.Errorf("failed to move SDK to project: %w", err)
			}

			err = os.RemoveAll(tempOut)
			if err != nil {
				return fmt.Errorf("failed to remove temporary directory: %w", err)
			}

			return LinkPackage(ws, language, root, pkg, out)
		},
	}

	return cmd
}
