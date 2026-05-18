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

// deploymentCancelClient is the subset of cloud-API operations the cancel
// command needs. Defined here so tests can stub a thin interface rather than
// the full HTTP client surface.
type deploymentCancelClient interface {
	CancelStackDeployment(ctx context.Context, stack client.StackIdentifier, deploymentID string) error
}

// deploymentCancelClientFactory resolves a cloud client and the
// StackIdentifier the deployment lives under. stackFlag carries the raw value
// of `--stack` (empty means "use the current stack"). The returned stackName
// is the human-readable label used in the confirmation prompt and success
// message; it deliberately mirrors what the user passed/selected rather than
// the canonical `org/project/stack` form so error and confirmation text stays
// terse.
type deploymentCancelClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentCancelClient, client.StackIdentifier, string, error)

// deploymentCancelRender renders a successful cancellation to w. It is the
// renderer type plugged into [outputflag.OutputFlag] so `--output` selects
// between human-readable and JSON output.
type deploymentCancelRender func(w io.Writer, deploymentID, stackName string) error

// defaultDeploymentCancelOutputFormat returns the OutputFlag wired up with
// every supported format. Callers (cobra constructors and tests) install this
// on deploymentCancelArgs so `--output` selects between them.
func defaultDeploymentCancelOutputFormat() outputflag.OutputFlag[deploymentCancelRender] {
	return outputflag.OutputFlag[deploymentCancelRender]{
		RenderForTerminal: renderDeploymentCancelText,
		RenderJSON:        renderDeploymentCancelJSON,
	}
}

// deploymentCancelArgs collects the flag values for the cancel command, so
// Run can be driven directly from tests.
type deploymentCancelArgs struct {
	stack  string
	yes    bool
	output outputflag.OutputFlag[deploymentCancelRender]
}

// confirmFunc abstracts the interactive confirmation so tests can inject a
// deterministic answer without driving a fake terminal. A nil value resolves
// to the default production prompt at runDeploymentCancel time —
// terminal-attached sessions ask the user to type `cancel` (matching the
// pattern used for other destructive `remove`-style commands where the
// natural identifier — here, a UUID — would be hostile to ask for verbatim),
// non-interactive sessions (CI, piped stdin, scripts) implicitly accept so
// they don't block on input that can never arrive.
type confirmFunc func(prompt string) bool

// newDeploymentCancelCmd builds `pulumi deployment cancel` with the production
// client factory and confirmation prompt. Test entrypoint is
// [newDeploymentCancelCmdWith].
func newDeploymentCancelCmd() *cobra.Command {
	return newDeploymentCancelCmdWith(nil, nil)
}

func newDeploymentCancelCmdWith(
	factory deploymentCancelClientFactory, confirm confirmFunc,
) *cobra.Command {
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
			return runDeploymentCancel(cmd.Context(), cmd.OutOrStdout(), factory, confirm, posArgs[0], args)
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

// runDeploymentCancel is the cobra-decoupled body so tests can drive it
// without going through the flag parser. Passing nil for factory or confirm
// installs the production defaults — useful for the cobra wiring above, and
// also lets us exercise the default-resolution path from a test without
// stubbing every input.
func runDeploymentCancel(
	ctx context.Context, w io.Writer,
	factory deploymentCancelClientFactory, confirm confirmFunc,
	deploymentID string, args deploymentCancelArgs,
) error {
	render := args.output.Get()

	if factory == nil {
		factory = defaultDeploymentCancelClientFactory
	}
	if confirm == nil {
		// Non-interactive sessions (CI, piped stdin, scripts) implicitly
		// accept so the command doesn't block on input that can never
		// arrive. The interactive branch asks the user to type "cancel" —
		// the deployment ID is a UUID so making them paste that verbatim
		// (the usual ConfirmPrompt pattern) would be hostile.
		confirm = func(prompt string) bool {
			if !cmdutil.Interactive() {
				return true
			}
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}
			return ui.ConfirmPrompt(prompt, "cancel", opts)
		}
	}

	c, stackID, stackName, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	// Ask before doing anything destructive. We skip the prompt only when
	// --yes is set.
	if !args.yes {
		prompt := fmt.Sprintf(
			"Cancel deployment '%s' for stack '%s'?",
			deploymentID, stackName)
		if !confirm(prompt) {
			return errors.New("confirmation declined")
		}
	}

	if err := c.CancelStackDeployment(ctx, stackID, deploymentID); err != nil {
		return fmt.Errorf("canceling deployment: %w", err)
	}

	return render(w, deploymentID, stackName)
}

// defaultDeploymentCancelClientFactory is the production wiring: resolve the
// stack via RequireStack (non-prompting beyond the standard select flow), cast
// to the cloud-backend types, and hand back the underlying *client.Client.
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
	// The Stack→Backend type pair is guaranteed by the cloud backend
	// implementation: any httpstate.Stack's Backend() returns an
	// httpstate.Backend, so a second type assertion would be unreachable
	// defensiveness rather than useful safety.
	return cloudStack.Backend().(httpstate.Backend).Client(), stackID, ref.Name().String(), nil
}

func renderDeploymentCancelText(w io.Writer, deploymentID, _ string) error {
	// The API is fire-and-forget — a 200 OK means the cancellation request was
	// accepted, not that the deployment has finished tearing down. Match the
	// wording to that nuance so users don't expect immediate completion.
	fmt.Fprintf(w, "Cancellation requested for deployment '%s'.\n", deploymentID)
	return nil
}

// deploymentCancelEnvelope is the JSON shape emitted by
// `pulumi deployment cancel --output=json`. Returning the inputs back to the
// caller keeps scripts that pipe one cancel into another from having to
// remember their own state.
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
