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

// newScansCmd creates the `pulumi insights scans` command.
func newScansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scans",
		Short: "Manage Insights discovery scans",
		Long: "Manage Insights discovery scans.\n" +
			"\n" +
			"Discovery scans enumerate cloud resources and import them into Pulumi Insights.",
		Args: cobra.NoArgs,
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newScansCreateCmd())
	cmd.AddCommand(newScansListCmd())
	cmd.AddCommand(newScansShowCmd())

	return cmd
}
