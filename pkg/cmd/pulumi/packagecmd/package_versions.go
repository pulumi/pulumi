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
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newPackageVersionsCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "versions <package>",
		Short: "List version history for a registry package",
		Long: `List version history for a registry package.

The package argument accepts the same formats as 'pulumi package add':
  aws, pulumi/pulumi/aws`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := validateLimit(limit); err != nil {
				return err
			}

			pkg, err := parseAndResolvePackage(ctx, args[0])
			if err != nil {
				return err
			}

			reg := registryForContext(ctx)
			versions, err := reg.ListPackageVersions(
				ctx, pkg.Source, pkg.Publisher, pkg.Name, limit,
			)
			if err != nil {
				return err
			}

			if len(versions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No versions found.")
				return nil
			}

			rows := make([]cmdutil.TableRow, len(versions))
			for i, v := range versions {
				registryURL := fmt.Sprintf("%s/%s/%s@%s", v.Source, v.Publisher, v.Name, v.Version)
				published := humanize.Time(v.CreatedAt)
				rows[i] = cmdutil.TableRow{
					Columns: []string{v.Version.String(), registryURL, published},
				}
			}

			ui.PrintTable(cmdutil.Table{
				Headers: []string{"VERSION", "REGISTRY URL", "PUBLISHED"},
				Rows:    rows,
			}, nil)

			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package", Usage: "<package>"},
		},
		Required: 1,
	})

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of versions (max 500)")

	return cmd
}
