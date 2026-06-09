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

package neo

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newNeoDebugCmd creates the `pulumi neo debug [id]` subcommand: a structured entry to the same
// interactive Neo experience as `pulumi neo`, seeded to investigate a failed update or preview.
// The CLI suggests this command in failed `up`/`preview` output (see display.PrintNeoLink), so the
// user can drop straight into Neo with the failure in context. It carries its own copy of the
// shared flags so it honors the same --stack/--org/--cwd/--approval-mode/--permission-mode/--print
// options as the parent command.
func newNeoDebugCmd() *cobra.Command {
	flags := &neoFlags{}

	cmd := &cobra.Command{
		Use:   "debug [update-or-preview-id]",
		Short: "Start a Pulumi Neo agent task to debug a failed update or preview",
		Long: "Starts the interactive Pulumi Neo experience seeded to investigate a failed update " +
			"or preview and propose a fix. With no argument, Neo debugs your most recent operation on " +
			"the stack and confirms which one before acting; pass an update version or preview id to " +
			"target a specific one. Neo runs against the current stack, so run this from the same " +
			"project directory as the failed operation.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			approvalMode, permissionMode, err := flags.resolveModes(cmd)
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			return runNeo(
				ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(),
				debugSeedPrompt(id), flags.stackName, flags.orgFlag, flags.cwdFlag,
				approvalMode, permissionMode, flags.printMode,
				flags.disableIntegrations)
		},
	}

	flags.register(cmd)

	return cmd
}

// debugSeedPrompt builds the initial Neo prompt for `pulumi neo debug`. It is deliberately a short
// trigger line, not a procedure: Neo's skill evaluator matches "debug ... failed update/preview" and
// loads the pulumi-debug-failed-operation skill, which carries the actual debugging steps. With no id
// the seed targets the user's most recent operation (the skill confirms which one); with an id it
// targets that specific run. Either way the fix should land locally in the working directory.
func debugSeedPrompt(id string) string {
	if id == "" {
		return "Debug my most recent Pulumi operation on this stack and fix it directly in this working directory.\n"
	}
	operation := "update"
	if !isUpdateID(id) {
		operation = "preview"
	}
	return fmt.Sprintf(
		"Debug the failed %s %s of this stack and fix it directly in this working directory.\n",
		operation, id)
}

// isUpdateID reports whether id looks like an update version rather than a preview id. Update
// versions are sequential integers; preview ids are UUIDs, so a digits-only id is an update.
func isUpdateID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
