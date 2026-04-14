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

package resources

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// NewResourcesCmd creates a new `pulumi resources` command, the parent for resource-oriented
// subcommands such as `pulumi resources list`. The command group is agent-first: every
// subcommand is expected to support a `--output` flag for machine-readable output and to
// preserve a human-friendly default when attached to a TTY.
func NewResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Browse and filter resources across stacks",
		Long: "Browse and filter resources across stacks.\n" +
			"\n" +
			"The `resources` command group provides AG Grid-style browsing of the resources\n" +
			"tracked in a stack's state. Commands in this group are agent-first: they emit\n" +
			"structured JSON by default when stdout is not a TTY, and a formatted table when\n" +
			"invoked interactively. Output can always be forced with `--output`.\n" +
			"\n" +
			"This is a non-destructive, read-only view of state - no mutations are performed.",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newResourcesListCmd())

	return cmd
}
