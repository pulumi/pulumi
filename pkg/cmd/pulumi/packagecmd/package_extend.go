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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Constructs the `pulumi package extend` command.
func newPackageExtendCmd() *cobra.Command {
	var language string
	cmd := &cobra.Command{
		Use:   "extend",
		Short: "Extend a base provider with additional resources via extension parameterization.",
		Long: `Extend a base provider with additional resources via extension parameterization.

  pulumi package extend <provider> [--] [provider-parameter]...
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			target, err := loadEnclosingTarget(cmd.Context(), wd)
			if err != nil && !errors.Is(err, workspace.ErrBaseProjectNotFound) {
				return err
			}
			foundProject := err == nil

			if foundProject && language != "" {
				return fmt.Errorf("--language is for use outside a Pulumi project or "+
					"plugin, but %s was found; remove the flag, or run from a "+
					"directory outside the project", *target.projectFilePath)
			}
			if !foundProject {
				if language == "" {
					return fmt.Errorf("%w; pass --language LANG to run "+
						"outside a Pulumi project or plugin", err)
				}
				target = addTarget{
					installRoot: wd,
					proj: &workspace.PluginProject{
						Runtime: workspace.NewProjectRuntimeInfo(cmdCmd.NormalizeRuntimeName(language), nil),
					},
					reg: cmdCmd.NewDefaultRegistry(
						cmd.Context(), cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global()),
				}
			}

			sink := cmdutil.Diag()
			pctx, err := plugin.NewContext(cmd.Context(),
				sink, sink, nil, nil, target.installRoot, nil, false, nil, schema.NewLoaderServerFromHost)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

			pluginSource := args[0]
			parameters := &plugin.ParameterizeArgs{Args: args[1:]}

			// asExtension promotes the schema's Parameterization to
			// ExtensionParameterization before binding and codegen.
			pkg, _, diags, err := packages.InstallPackage(
				cmd.OutOrStdout(),
				pkgWorkspace.Instance,
				target.proj,
				pctx,
				target.proj.RuntimeInfo().Name(),
				target.installRoot,
				pluginSource,
				parameters,
				target.reg,
				env.Global(),
				0,    /* unbounded concurrency */
				true, /* asExtension */
			)
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				return err
			}

			if pkg.ExtensionParameterization == nil {
				return fmt.Errorf(
					"provider %s did not return an extension-parameterized schema; "+
						"the provider may not support extension parameterization "+
						"(use 'pulumi package add' for replacement parameterization)",
					args[0])
			}

			// File-based schemas have no underlying provider to record.
			source := strings.Split(pluginSource, "@")[0]
			if ext := filepath.Ext(source); ext == ".yaml" || ext == ".yml" || ext == ".json" {
				return nil
			}

			if target.projectFilePath != nil {
				target.proj.AddPackage(pkg.Name, workspace.PackageSpec{
					Base: &workspace.PackageSpec{
						Source:  pkg.ExtensionParameterization.BaseProvider.Name,
						Version: pkg.ExtensionParameterization.BaseProvider.Version.String(),
					},
					Extensions: parameters.Args,
				})
				fileName := filepath.Base(*target.projectFilePath)
				if err := target.proj.Save(*target.projectFilePath); err != nil {
					return fmt.Errorf("failed to update %s: %w", fileName, err)
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Extended package %s\n", schemaDisplayName(pkg))
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "provider", Usage: "<provider|schema|path>"},
			{Name: "provider-parameter"},
		},
		Required: 1,
		Variadic: true,
	})

	cmd.Use = "extend <provider|schema|path> [flags] [--] [provider-parameter]..."

	cmd.Flags().StringVar(&language, "language", "",
		"Run outside a Pulumi project or plugin: [nodejs|python|go|dotnet|java]")

	return cmd
}
