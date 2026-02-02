// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type accountsDeleteArgs struct {
	yes bool
}

// newAccountsDeleteCmd creates the `pulumi insights accounts delete` command.
func newAccountsDeleteCmd() *cobra.Command {
	args := &accountsDeleteArgs{}

	cmd := &cobra.Command{
		Use:   "delete <account-name>",
		Short: "Delete an Insights discovery account",
		Long: "Delete an Insights discovery account.\n" +
			"\n" +
			"This removes the account and stops any scheduled scans.\n" +
			"Previously discovered resources remain in Insights.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			accountName := posArgs[0]

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			// Confirm deletion
			if cmdutil.Interactive() && !args.yes {
				displayOpts := getDisplayOptions()
				option := ui.PromptUser(
					fmt.Sprintf("Delete account %q? This cannot be undone.", accountName),
					[]string{"yes", "no"},
					"no",
					displayOpts.Color,
				)
				if option != "yes" {
					fmt.Println("Deletion cancelled.")
					return nil
				}
			}

			err = cloudBackend.Client().DeleteInsightsAccount(ctx, orgName, accountName)
			if err != nil {
				return fmt.Errorf("deleting account %q: %w", accountName, err)
			}

			fmt.Printf("Account %q deleted.\n", accountName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip confirmation prompt")

	return cmd
}
