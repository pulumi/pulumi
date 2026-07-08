// Copyright 2016, Pulumi Corporation.
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

package deployment

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
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

const (
	optYes = "Yes"
	optNo  = "No"
)

func newDeploymentSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage stack deployment settings",
		Long: "Manage stack deployment settings\n" +
			"\n" +
			"Use this command to manage a stack's deployment settings\n" +
			"directly in Pulumi Cloud.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	// Manage the stack's deployment settings directly in Pulumi Cloud.
	cmd.AddCommand(newDeploymentSettingsDestroyCmd())
	cmd.AddCommand(newDeploymentSettingsGetCmd())
	cmd.AddCommand(newDeploymentSettingsEditCmd())

	return cmd
}

type deploymentSettingsCommandDependencies struct {
	DisplayOptions *display.Options
	Stack          backend.Stack
	Backend        backend.Backend
	Interactive    bool
	Ctx            context.Context
	WorkDir        string
}

func initializeDeploymentSettingsCmd(
	ctx context.Context, stdout io.Writer, ws pkgWorkspace.Context, stack string,
) (*deploymentSettingsCommandDependencies, error) {
	interactive := cmdutil.Interactive()

	displayOpts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
	}

	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return nil, err
	}

	if !be.SupportsDeployments() {
		unsupportedBackendMsg := fmt.Sprintf("Backends of type %q do not support managed deployments.\n\n"+
			"Create a Pulumi Cloud account to get started, learn more about pulumi deployments here: "+
			"https://www.pulumi.com/docs/pulumi-cloud/deployments/",
			be.Name())

		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg,
			fmt.Sprintf("Backends of type %q do not support managed deployments", be.Name()),
			colors.SpecError+colors.Bold)
		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg, "Pulumi Cloud", colors.BrightCyan+colors.Bold)
		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg,
			"https://www.pulumi.com/docs/pulumi-cloud/deployments/", colors.BrightBlue+colors.Underline+colors.Bold)

		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, displayOpts.Color.Colorize(unsupportedBackendMsg))
		fmt.Fprintln(stdout)

		return nil, fmt.Errorf("unable to manage stack deployments for backend type: %s",
			be.Name())
	}

	s, err := cmdStack.RequireStack(
		ctx,
		cmdutil.Diag(),
		ws,
		cmdBackend.DefaultLoginManager,
		stack,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		displayOpts,
		"",
	)
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &deploymentSettingsCommandDependencies{
		DisplayOptions: &displayOpts,
		Stack:          s,
		Backend:        be,
		Interactive:    interactive,
		Ctx:            ctx,
		WorkDir:        wd,
	}, nil
}
