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
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
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
// directly from tests. Sentinel values (-1 for job/step, 0 for offset/count)
// mean "unset" and are not sent to the API.
type deploymentLogArgs struct {
	stack        string
	job          int
	step         int
	offset       int
	count        int
	all          bool
	outputFormat outputflag.OutputFlag[deploymentLogRenderFunc]
}

// defaultDeploymentLogArgs returns the zero-args value with the documented
// sentinel defaults (job/step = -1, offset/count = 0).
func defaultDeploymentLogArgs() deploymentLogArgs {
	return deploymentLogArgs{
		job:          -1,
		step:         -1,
		offset:       0,
		count:        0,
		outputFormat: defaultDeploymentLogOutputFormat(),
	}
}

// defaultDeploymentLogOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultDeploymentLogOutputFormat() outputflag.OutputFlag[deploymentLogRenderFunc] {
	return outputflag.OutputFlag[deploymentLogRenderFunc]{
		RenderForTerminal: renderDeploymentLogText,
		RenderJSON:        renderDeploymentLogJSON,
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
			"Returns log lines from a deployment. Pass --job and --step to scope to a\n" +
			"specific step within a specific job; in step mode --count must be 1-499\n" +
			"(default 100 server-side). Pass --all to fetch every available log line,\n" +
			"following the server's pagination internally; --count and --all are\n" +
			"mutually exclusive.\n" +
			"\n" +
			"Default output prints one log line per row; pass --output=json for a\n" +
			"structured envelope.",
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
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Fetch every available log line, following server-side pagination; mutually exclusive with --count")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

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
	if args.step >= 0 && args.job < 0 {
		return errors.New("--step requires --job to also be set (>= 0)")
	}

	opts := client.GetDeploymentLogsOptions{}
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

	// --all walks the server's continuation tokens internally, accumulating
	// every page into a single response before rendering.
	if args.all {
		for resp.NextToken != "" {
			opts.ContinuationToken = resp.NextToken
			next, err := c.GetDeploymentLogs(ctx, stackID, deploymentID, opts)
			if err != nil {
				return fmt.Errorf("getting deployment logs: %w", err)
			}
			if next == nil {
				break
			}
			resp.Lines = append(resp.Lines, next.Lines...)
			resp.NextToken = next.NextToken
		}
	}

	return args.outputFormat.Get()(w, *resp)
}

type deploymentLogRenderFunc func(w io.Writer, logs apitype.DeploymentLogs) error

// renderDeploymentLogText prints one log line per row, prefixing each line
// with `[header] ` when a header is present.
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
	return nil
}

// deploymentLogJSON is the JSON envelope for
// `pulumi deployment log --output=json`. Nil slices are normalized to `[]` so
// scripts can rely on the `lines` key always being a JSON array.
type deploymentLogJSON struct {
	Lines []apitype.DeploymentLogLine `json:"lines"`
}

func toDeploymentLogJSON(logs apitype.DeploymentLogs) deploymentLogJSON {
	lines := logs.Lines
	if lines == nil {
		lines = []apitype.DeploymentLogLine{}
	}
	return deploymentLogJSON{
		Lines: lines,
	}
}

func renderDeploymentLogJSON(w io.Writer, logs apitype.DeploymentLogs) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toDeploymentLogJSON(logs))
}
