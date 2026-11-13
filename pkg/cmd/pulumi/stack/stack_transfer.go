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

package stack

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackTransferClient interface {
	TransferStack(ctx context.Context, stackID client.StackIdentifier, toOrg string) error
}

func newStackTransferCmd() *cobra.Command {
	var (
		stack string
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "transfer <organization>",
		Short: "Transfer a stack to another organization",
		Long: "Transfer a stack to another organization.\n" +
			"\n" +
			"The stack keeps its project, name, and update history. You must be a member of both\n" +
			"organizations, and the stack must have no update in progress.\n" +
			"\n" +
			"You will be prompted to confirm by typing the stack name unless --yes is passed.",
		Example: "  # Transfer the current stack to the 'acme' organization\n" +
			"  pulumi stack transfer acme\n" +
			"\n" +
			"  # Transfer a specific stack without confirmation\n" +
			"  pulumi stack transfer acme --stack my-org/my-project/dev --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}

			s, err := RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager, stack, LoadOnly, opts, "")
			if err != nil {
				return err
			}
			cloudStack, ok := s.(httpstate.Stack)
			if !ok {
				return errors.New("transferring a stack requires the Pulumi Cloud backend; run `pulumi login`")
			}
			be := cloudStack.Backend().(httpstate.Backend)

			newID, err := runStackTransfer(ctx, cmd.OutOrStdout(), be.Client(), cloudStack.StackIdentifier(),
				args[0], yes || env.SkipConfirmations.Value(), cmdutil.Interactive(), opts)
			if err != nil {
				return err
			}

			if err := state.SetCurrentStack(ws, state.BackendURLKey(be), newID.String()); err != nil {
				return fmt.Errorf("setting current stack: %w", err)
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "organization"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip confirmation prompts")

	return cmd
}

func runStackTransfer(
	ctx context.Context,
	w io.Writer,
	c stackTransferClient,
	stackID client.StackIdentifier,
	toOrg string,
	yes, interactive bool,
	opts display.Options,
) (client.StackIdentifier, error) {
	if toOrg == stackID.Owner {
		return client.StackIdentifier{}, fmt.Errorf("stack %s is already in organization %s", stackID, toOrg)
	}

	prompt := fmt.Sprintf("This will transfer the stack '%s' to the organization '%s'.\n"+
		"Members of '%s' will lose access unless they also belong to '%s'.",
		stackID, toOrg, stackID.Owner, toOrg)
	if err := ui.ConfirmDeletion(yes, interactive, prompt, stackID.Stack.String(), w, opts); err != nil {
		return client.StackIdentifier{}, err
	}

	if err := c.TransferStack(ctx, stackID, toOrg); err != nil {
		return client.StackIdentifier{}, fmt.Errorf("transferring stack: %w", err)
	}

	newID := stackID
	newID.Owner = toOrg
	fmt.Fprintf(w, "Transferred %s to %s\n", stackID, newID)
	return newID, nil
}
