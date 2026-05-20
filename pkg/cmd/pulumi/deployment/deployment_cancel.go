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

package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type deploymentCancelClient interface {
	CancelStackDeployment(ctx context.Context, stack client.StackIdentifier, deploymentID string) error
}

type deploymentCancelClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentCancelClient, client.StackIdentifier, string, error)

type deploymentCancelRender func(w io.Writer, deploymentID, stackName string) error

func defaultDeploymentCancelOutputFormat() outputflag.OutputFlag[deploymentCancelRender] {
	return outputflag.OutputFlag[deploymentCancelRender]{
		RenderForTerminal: renderDeploymentCancelText,
		RenderJSON:        renderDeploymentCancelJSON,
	}
}

type deploymentCancelArgs struct {
	stack  string
	yes    bool
	output outputflag.OutputFlag[deploymentCancelRender]
}

func newDeploymentCancelCmd() *cobra.Command {
	return newDeploymentCancelCmdWith(nil)
}

func newDeploymentCancelCmdWith(factory deploymentCancelClientFactory) *cobra.Command {
	args := deploymentCancelArgs{output: defaultDeploymentCancelOutputFormat()}

	cmd := &cobra.Command{
		Use:   "cancel <deployment-id>",
		Short: "[EXPERIMENTAL] Cancel an in-progress deployment",
		Long: "[EXPERIMENTAL] Cancel an in-progress deployment.\n" +
			"\n" +
			"Terminates an in-progress Pulumi Deployments execution. If the deployment is\n" +
			"currently running, it is stopped immediately. If the deployment is queued but\n" +
			"has not yet started, it is removed from the queue.\n" +
			"\n" +
			"Canceling a deployment is a dangerous action and may leave the stack in an\n" +
			"inconsistent state if canceled during the execution of a Pulumi operation.",
		Example: "  # Cancel a deployment of the current stack.\n" +
			"  pulumi deployment cancel dep-abc123\n\n" +
			"  # Cancel a deployment of a different stack.\n" +
			"  pulumi deployment cancel dep-abc123 --stack acme/web/prod\n\n" +
			"  # Skip the confirmation prompt.\n" +
			"  pulumi deployment cancel dep-abc123 --yes\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi deployment cancel dep-abc123 --yes --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentCancel(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "deployment-id"}},
		Required:  1,
	})

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with cancellation anyway")
	outputflag.VarP(cmd.Flags(), &args.output)

	return cmd
}

func runDeploymentCancel(
	ctx context.Context, w io.Writer,
	factory deploymentCancelClientFactory,
	deploymentID string, args deploymentCancelArgs,
) error {
	render := args.output.Get()

	if factory == nil {
		factory = defaultDeploymentCancelClientFactory
	}

	c, stackID, stackName, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	if !args.yes {
		if !cmdutil.Interactive() {
			return backenderr.ErrNonInteractiveRequiresYes
		}
		opts := display.Options{Color: cmdutil.GetGlobalColorization()}
		prompt := fmt.Sprintf(
			"This will cancel deployment '%s' for stack '%s'!",
			deploymentID, stackName)
		if !ui.ConfirmPrompt(prompt, "cancel", opts) {
			return errors.New("confirmation declined")
		}
	}

	if err := c.CancelStackDeployment(ctx, stackID, deploymentID); err != nil {
		return fmt.Errorf("canceling deployment: %w", err)
	}

	return render(w, deploymentID, stackName)
}

func defaultDeploymentCancelClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentCancelClient, client.StackIdentifier, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, "", fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{}, "",
			errors.New("canceling deployments requires the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}
	return cloudStack.Backend().(httpstate.Backend).Client(), stackID, ref.Name().String(), nil
}

func renderDeploymentCancelText(w io.Writer, deploymentID, _ string) error {
	// The API is fire-and-forget — a 200 OK means the cancellation request was
	// accepted, not that the deployment has finished tearing down. Match the
	// wording to that nuance so users don't expect immediate completion.
	fmt.Fprintf(w, "Cancellation requested for deployment '%s'.\n", deploymentID)
	return nil
}

type deploymentCancelEnvelope struct {
	DeploymentID string `json:"deploymentId"`
	Stack        string `json:"stack"`
	Canceled     bool   `json:"canceled"`
}

func renderDeploymentCancelJSON(w io.Writer, deploymentID, stackName string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(deploymentCancelEnvelope{
		DeploymentID: deploymentID,
		Stack:        stackName,
		Canceled:     true,
	})
}
