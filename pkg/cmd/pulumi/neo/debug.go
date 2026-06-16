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
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
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
			return runNeo(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), neoRunOptions{
				prompt:              debugSeedPrompt(id),
				stackName:           flags.stackName,
				orgFlag:             flags.orgFlag,
				cwdFlag:             flags.cwdFlag,
				approvalMode:        approvalMode,
				permissionMode:      permissionMode,
				printMode:           flags.printMode,
				includeStackContext: true,
				disableIntegrations: flags.disableIntegrations,
			})
		},
	}

	flags.register(cmd)

	return cmd
}

// debugSeedPrompt builds the initial Neo prompt for `pulumi neo debug`. It is deliberately a short
// trigger line, not a procedure: Neo's skill evaluator matches "debug ... failed operation" and
// loads the pulumi-debug-failed-operation skill, which carries the actual debugging steps. With no id
// the seed targets the user's most recent operation (the skill confirms which one); with an id it
// targets that specific run. The id itself tells Neo whether it is an update version (a sequential
// integer) or a preview (a UUID), so the seed doesn't classify it. Either way the fix should land
// locally in the working directory.
func debugSeedPrompt(id string) string {
	if id == "" {
		return "Debug my most recent Pulumi operation on this stack and fix it directly in this working directory.\n"
	}
	return fmt.Sprintf(
		"Debug the failed Pulumi operation %s of this stack and fix it directly in this working directory.\n",
		id)
}

// debugStackContext builds a short, human-readable block describing where the debug session is
// running — the organization, user, project, and stack — plus the stack's most recent operation
// (its kind, version/id, and result). `pulumi neo debug` appends this to the seed prompt so Neo
// starts with the failure already in context instead of rediscovering it. Every lookup is
// best-effort: anything that errors or is unavailable is simply omitted so debug still works when,
// for example, the backend can't return history or no stack is selected.
func debugStackContext(
	ctx context.Context,
	be httpstate.Backend,
	stackRef backend.StackReference,
	org, project string,
) string {
	var b strings.Builder
	b.WriteString("Context for this debug session:\n")
	if org != "" {
		fmt.Fprintf(&b, "- Organization: %s\n", org)
	}
	if user, _, _, err := be.CurrentUser(); err == nil && user != "" {
		fmt.Fprintf(&b, "- User: %s\n", user)
	}
	if project != "" {
		fmt.Fprintf(&b, "- Project: %s\n", project)
	}
	// The stack name and most recent operation both come from the resolved reference, so they
	// share the same nil guard: with no stack selected we emit neither.
	if stackRef != nil {
		fmt.Fprintf(&b, "- Stack: %s\n", stackRef.Name())
		if op := mostRecentOperation(ctx, be, stackRef); op != "" {
			fmt.Fprintf(&b, "- Most recent operation: %s\n", op)
		}
	}
	return b.String()
}

// mostRecentOperation describes the stack's most recent operation across both updates and
// previews, or "" if neither is available. Updates and previews are tracked separately — previews
// never appear in GetHistory — so we look at both and report whichever ran most recently (by start
// time). This is what lets `pulumi neo debug` target a failed preview that is newer than the last
// deployment, rather than always reporting the latest update. Both lookups are best-effort.
func mostRecentOperation(ctx context.Context, be httpstate.Backend, stackRef backend.StackReference) string {
	var (
		bestStart int64 = -1
		best      string
	)
	// Most recent entry from update history (updates/refreshes/destroys/imports — never previews).
	if updates, err := be.GetHistory(ctx, stackRef, 1, 1); err == nil && len(updates) > 0 {
		u := updates[0]
		bestStart = u.StartTime
		best = fmt.Sprintf("%s (version %d, result: %s)", u.Kind, u.Version, u.Result)
	}
	// Most recent preview, which is tracked separately from history.
	if p, err := be.GetLatestStackPreview(ctx, stackRef); err == nil && p != nil && p.Info.StartTime > bestStart {
		best = fmt.Sprintf("preview %s (result: %s)", p.UpdateID, p.Info.Result)
	}
	return best
}
