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
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPolicyInstallCmd() *cobra.Command {
	var policyInstallCmd policyInstallCmd
	var stack string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install required policy packs for a stack",
		Long: "Install required policy packs for a stack.\n" +
			"\n" +
			"This command installs the policy packs required by the stack's organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return policyInstallCmd.Run(ctx, stack)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

type policyInstallCmd struct {
	getwd  func() (string, error)
	diag   diag.Sink
	stderr io.Writer // defaults to os.Stderr

	// requireStack is a function that returns the stack to operate on.
	// If nil, the default implementation is used.
	requireStack func(ctx context.Context, stackName string) (backend.Stack, error)
}

func (cmd *policyInstallCmd) Run(
	ctx context.Context,
	stackName string,
) error {
	if cmd.getwd == nil {
		cmd.getwd = os.Getwd
	}
	if cmd.diag == nil {
		cmd.diag = cmdutil.Diag()
	}
	if cmd.stderr == nil {
		cmd.stderr = os.Stderr
	}

	if cmd.requireStack == nil {
		cmd.requireStack = func(ctx context.Context, stackName string) (backend.Stack, error) {
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			return cmdStack.RequireStack(ctx, cmd.diag, pkgWorkspace.Instance,
				cmdBackend.DefaultLoginManager, stackName, cmdStack.LoadOnly, displayOpts)
		}
	}

	// Get the stack (uses current stack if stackName is empty).
	s, err := cmd.requireStack(ctx, stackName)
	if err != nil {
		if stackName == "" && errors.Is(err, workspace.ErrProjectNotFound) {
			return errors.New("could not find a Pulumi project in the current working directory; " +
				"please specify a stack using the --stack flag.")
		}
		return err
	}

	// Fetch the required policy packs for the stack.
	policyPacks, err := s.Backend().GetStackPolicyPacks(ctx, s.Ref())
	if err != nil {
		return fmt.Errorf("getting stack policy packs: %w", err)
	}

	if len(policyPacks) == 0 {
		fmt.Fprintf(cmd.stderr, "No policy packs to install for stack %s\n", s.Ref().String())
		return nil
	}

	cwd, err := cmd.getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	pctx, err := plugin.NewContext(ctx, cmd.diag, cmd.diag, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return fmt.Errorf("creating plugin context: %w", err)
	}
	defer func() {
		contract.IgnoreError(pctx.Close())
	}()

	// Install the required policy packs.
	if err := engine.EnsurePoliciesAreInstalled(ctx, pctx, nil, policyPacks); err != nil {
		return err
	}

	var plural string
	if len(policyPacks) > 1 {
		plural = "s"
	}
	fmt.Fprintf(cmd.stderr,
		"Successfully installed %d policy pack%s for stack %s\n", len(policyPacks), plural, s.Ref().String())
	return nil
}
