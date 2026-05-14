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

// AI Generated - needs human review

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
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// deploymentLogClient is the narrow API surface the log command depends on.
type deploymentLogClient interface {
	GetDeploymentLogs(
		ctx context.Context, stack client.StackIdentifier, id string,
		opts client.GetDeploymentLogsOptions,
	) (*apitype.DeploymentLogs, error)
}

// deploymentLogClientFactory resolves a cloud client and StackIdentifier for
// the log command. stackFlag carries the raw value of `--stack` (empty means
// "use the current stack").
type deploymentLogClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentLogClient, client.StackIdentifier, error)

// deploymentLogArgs collects the resolved flag values so Run can be driven
// directly from tests. Sentinel values (-1 for job/step, 0 for offset/count,
// "" for continuationToken) mean "unset" and are not sent to the API.
type deploymentLogArgs struct {
	stack             string
	job               int
	step              int
	offset            int
	count             int
	continuationToken string
	output            string
}

// defaultDeploymentLogArgs returns the zero-args value with the documented
// sentinel defaults (job/step = -1, offset/count = 0, output = "default").
func defaultDeploymentLogArgs() deploymentLogArgs {
	return deploymentLogArgs{
		job:    -1,
		step:   -1,
		offset: 0,
		count:  0,
		output: "default",
	}
}

func newDeploymentLogCmd() *cobra.Command {
	return newDeploymentLogCmdWith(defaultDeploymentLogClientFactory)
}

func newDeploymentLogCmdWith(factory deploymentLogClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentLogClientFactory must not be nil")
	args := defaultDeploymentLogArgs()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "log <deployment-id>",
		Short:  "[EXPERIMENTAL] Retrieve execution logs for a deployment",
		Long: "[EXPERIMENTAL] Retrieve execution logs for a deployment.\n" +
			"\n" +
			"Supports two retrieval modes. In streaming mode (default), omit --job and\n" +
			"--step and use --continuation-token to incrementally fetch logs from the\n" +
			"beginning through completion; each response includes a next-token hint in\n" +
			"the default output. In step mode, pass --job and --step (and optionally\n" +
			"--offset/--count) to retrieve logs for a specific step within a specific\n" +
			"job. In step mode --count must be 1-499 (default 100 server-side).\n" +
			"\n" +
			"Wraps the `GetDeploymentLogs` Pulumi Cloud REST endpoint. Default output\n" +
			"prints one log line per row; pass --output=json for a structured envelope.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentLog(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&args.job, "job", -1,
		"The job index to fetch step-level logs for (-1 to leave unset)")
	cmd.Flags().IntVar(&args.step, "step", -1,
		"The step index within the job; requires --job (-1 to leave unset)")
	cmd.Flags().IntVar(&args.offset, "offset", 0,
		"The offset within the step's logs (0 to leave unset)")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"The number of log lines to fetch, 1-499 in step mode (0 to leave unset)")
	cmd.Flags().StringVar(&args.continuationToken, "continuation-token", "",
		"The continuation token for streaming mode")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// defaultDeploymentLogClientFactory mirrors the production wiring used by
// `pulumi deployment get`: resolve the stack, ensure we're on the Pulumi Cloud
// backend, and hand back the underlying *client.Client.
func defaultDeploymentLogClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentLogClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("getting deployment logs requires the Pulumi Cloud backend; run `pulumi login`")
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

	be, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("getting deployment logs requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

// runDeploymentLog is the cobra-decoupled command body so tests can drive it
// directly with a buffer instead of parsing flags.
func runDeploymentLog(
	ctx context.Context, w io.Writer,
	factory deploymentLogClientFactory, deploymentID string, args deploymentLogArgs,
) error {
	render, err := deploymentLogRenderer(args.output)
	if err != nil {
		return err
	}

	if args.step >= 0 && args.job < 0 {
		return errors.New("--step requires --job to also be set (>= 0)")
	}

	opts := client.GetDeploymentLogsOptions{
		ContinuationToken: args.continuationToken,
	}
	if args.job >= 0 {
		j := args.job
		opts.Job = &j
	}
	if args.step >= 0 {
		s := args.step
		opts.Step = &s
	}
	if args.offset > 0 {
		o := args.offset
		opts.Offset = &o
	}
	if args.count > 0 {
		c := args.count
		opts.Count = &c
	}

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	resp, err := c.GetDeploymentLogs(ctx, stackID, deploymentID, opts)
	if err != nil {
		return fmt.Errorf("getting deployment logs: %w", err)
	}
	if resp == nil {
		resp = &apitype.DeploymentLogs{}
	}

	return render(w, *resp)
}

type deploymentLogRenderFunc func(w io.Writer, logs apitype.DeploymentLogs) error

func deploymentLogRenderer(format string) (deploymentLogRenderFunc, error) {
	switch format {
	case "", "default":
		return renderDeploymentLogText, nil
	case "json":
		return renderDeploymentLogJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// renderDeploymentLogText prints one log line per row, prefixing each line
// with `[header] ` when a header is present. After the lines, if NextToken is
// set, prints a hint pointing users at --continuation-token.
func renderDeploymentLogText(w io.Writer, logs apitype.DeploymentLogs) error {
	if len(logs.Lines) == 0 {
		fmt.Fprintln(w, "No log lines available.")
		return nil
	}
	for _, l := range logs.Lines {
		if l.Header != "" {
			fmt.Fprintf(w, "[%s] %s\n", l.Header, l.Line)
		} else {
			fmt.Fprintf(w, "%s\n", l.Line)
		}
	}
	if logs.NextToken != "" {
		fmt.Fprintf(w,
			"\nMore log lines available. Re-run with --continuation-token %q to continue.\n",
			logs.NextToken)
	}
	return nil
}

// deploymentLogJSON is the JSON envelope for
// `pulumi deployment log --output=json`. Nil slices are normalized to `[]` so
// scripts can rely on the `lines` key always being a JSON array.
type deploymentLogJSON struct {
	Lines     []apitype.DeploymentLogLine `json:"lines"`
	NextToken string                      `json:"nextToken"`
}

func toDeploymentLogJSON(logs apitype.DeploymentLogs) deploymentLogJSON {
	lines := logs.Lines
	if lines == nil {
		lines = []apitype.DeploymentLogLine{}
	}
	return deploymentLogJSON{
		Lines:     lines,
		NextToken: logs.NextToken,
	}
}

func renderDeploymentLogJSON(w io.Writer, logs apitype.DeploymentLogs) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toDeploymentLogJSON(logs))
}
