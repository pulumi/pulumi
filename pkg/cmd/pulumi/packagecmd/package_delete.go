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
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPackageDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <source>/<publisher>/<name>@<version>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Delete a package version from the registry",
		Long: `Delete a package version from the Pulumi Registry.

The package version must be specified in the format:
  [[<source>/]<publisher>/]<name>[@<version>]

Example:
  pulumi package delete private/myorg/my-package@1.0.0

Warning: If this is the only version of the package, the entire package
will be removed. This action cannot be undone.

You must have publish permissions for the package to delete it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			yes = yes || env.SkipConfirmations.Value()

			opts := display.Options{
				Color:  cmdutil.GetGlobalColorization(),
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			}

			if !cmdutil.Interactive() && !yes {
				return errors.New("non-interactive mode requires --yes flag")
			}

			project, _, err := pkgWorkspace.Instance.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return fmt.Errorf("failed to determine current project: %w", err)
			}

			b, err := login(ctx, project)
			if err != nil {
				return err
			}

			r, err := b.GetCloudRegistry()
			if err != nil {
				return fmt.Errorf("failed to get the registry backend: %w", err)
			}

			packageName, packageVersion := args[0], (*semver.Version)(nil)
			if parts := strings.SplitN(packageName, "@", 2); len(parts) > 1 {
				pv, err := semver.ParseTolerant(parts[1])
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", parts[1], err)
				}
				packageName = parts[0]
				packageVersion = &pv
			}

			formatPkg := func(pkg apitype.PackageMetadata) string {
				return fmt.Sprintf("%s/%s/%s@%s", pkg.Source, pkg.Publisher, pkg.Name, pkg.Version)
			}

			pkg, err := registry.ResolvePackageFromName(ctx, r, packageName, packageVersion)
			if err != nil {
				filterPrivate := func(arr []apitype.PackageMetadata) []apitype.PackageMetadata {
					return slices.DeleteFunc(arr, func(pkg apitype.PackageMetadata) bool {
						return pkg.Source != "private"
					})
				}
				if suggested := filterPrivate(registry.GetSuggestedPackages(err)); len(suggested) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "No matching package was found. Did you mean")
					if len(suggested) == 1 {
						fmt.Println(cmd.ErrOrStderr(), opts.Color.Colorize(fmt.Sprintf(" %s%s%s?",
							colors.SpecInfo, formatPkg(suggested[0]), colors.Reset,
						)))
					}
					fmt.Fprintln(cmd.ErrOrStderr(), " one of:")
					for _, pkg := range suggested {
						fmt.Println(cmd.ErrOrStderr(), opts.Color.Colorize(fmt.Sprintf("- %s%s%s",
							colors.SpecInfo, formatPkg(pkg), colors.Reset,
						)))
					}
				}
				return err
			}

			prompt := opts.Color.Colorize(fmt.Sprintf("This will permanently delete package version %s%s%s!",
				colors.SpecAttention, formatPkg(pkg), colors.Reset,
			))
			if !yes && !ui.ConfirmPrompt(prompt, packageVersion.String(), opts) {
				return result.FprintBailf(cmd.ErrOrStderr(), "confirmation declined")
			}

			if err := r.DeletePackageVersion(ctx,
				pkg.Source,
				pkg.Publisher,
				pkg.Name,
				pkg.Version,
			); err != nil {
				return fmt.Errorf("failed to delete package version: %w", err)
			}

			fmt.Fprintln(cmd.ErrOrStderr(), opts.Color.Colorize(fmt.Sprintf(
				"%sPackage version '%s' has been deleted!%s",
				colors.SpecAttention, packageVersion, colors.Reset,
			)))
			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with deletion anyway")

	return cmd
}
