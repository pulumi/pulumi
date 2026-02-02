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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// newAccountsCmd creates the `pulumi insights accounts` command.
func newAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage Insights discovery accounts",
		Long: "Manage Insights discovery accounts.\n" +
			"\n" +
			"Discovery accounts connect your cloud providers to Pulumi Insights, " +
			"enabling automated infrastructure discovery and scanning.",
		Args: cobra.NoArgs,
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newAccountsCreateCmd())
	cmd.AddCommand(newAccountsListCmd())
	cmd.AddCommand(newAccountsShowCmd())
	cmd.AddCommand(newAccountsDeleteCmd())

	return cmd
}
