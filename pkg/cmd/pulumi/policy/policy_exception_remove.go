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

package policy

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type policyExceptionRemoveArgs struct {
	org    string
	force  bool
	dryRun bool
}

func newPolicyExceptionRemoveCmd(factory policyExceptionEnvFactory) *cobra.Command {
	var args policyExceptionRemoveArgs

	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "[EXPERIMENTAL] Remove a policy exception",
		Long: "[EXPERIMENTAL] Remove a policy exception.\n" +
			"\n" +
			"Removes an active policy exception, so the violations it allowed fail operations again. Pulumi Cloud\n" +
			"keeps the removed exception for auditing. Removing an exception that's already removed succeeds.",
		Example: "  pulumi policy exception remove EXC-12\n" +
			"  pulumi policy exception remove EXC-12 --force",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			env, err := factory(cmd.Context())
			if err != nil {
				return err
			}
			return runPolicyExceptionRemove(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), env, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "id"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization of the exception. Defaults to the default org")
	cmd.Flags().BoolVarP(&args.force, "force", "f", false, "Remove the exception without asking for confirmation")
	cmd.Flags().BoolVar(&args.force, "yes", false, "Alias for --force")
	cmd.Flags().BoolVar(&args.dryRun, "dry-run", false, "Show the exception that would be removed, without removing it")
	return cmd
}

func runPolicyExceptionRemove(
	ctx context.Context, stdout, stderr io.Writer, env policyExceptionEnv, id string, args policyExceptionRemoveArgs,
) error {
	org := cmp.Or(args.org, env.org)
	if args.dryRun {
		e, err := env.client.GetPolicyException(ctx, org, id)
		if err != nil {
			return fmt.Errorf("getting policy exception: %w", err)
		}
		fmt.Fprintf(stderr, "Would remove %s (%s): %s\n", e.ID, e.Status, e.Reason)
		return nil
	}

	if !args.force && !env.interactive {
		return cmdCmd.ConfigurationError{Message: "--force is required when not running interactively"}
	}
	opts := display.Options{Color: cmdutil.GetGlobalColorization(), Stdout: stderr}
	prompt := fmt.Sprintf("This will remove policy exception %s, so the violations it allows fail operations again.", id)
	if err := ui.ConfirmDeletion(args.force, env.interactive, prompt, id, stderr, opts); err != nil {
		return err
	}

	_, err := env.client.RemovePolicyException(ctx, org, id)
	switch {
	case apiErrorCode(err) == http.StatusConflict:
		fmt.Fprintf(stderr, "%s is already removed\n", id)
	case err != nil:
		return fmt.Errorf("removing policy exception: %w", err)
	default:
		fmt.Fprintf(stderr, "Removed %s\n", id)
	}
	fmt.Fprintln(stdout, id)
	return nil
}
