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

	"github.com/jedib0t/go-pretty/v6/table"
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
)

// deploymentListClient is the subset of cloud-API operations the list command
// needs. Defined here so tests can stub a thin interface instead of the full
// HTTP client surface.
type deploymentListClient interface {
	ListStackDeployments(
		ctx context.Context, stack client.StackIdentifier, opts client.ListStackDeploymentsOptions,
	) (apitype.ListDeploymentResponseV2, error)
}

// deploymentListClientFactory resolves a cloud client and the StackIdentifier
// to list deployments for. stackFlag carries the raw value of `--stack` (empty
// means "use the current stack"). A non-nil error is returned for any
// resolution failure — not logged in, no stack selected, non-cloud backend.
type deploymentListClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentListClient, client.StackIdentifier, error)

const defaultPageSize = 100

// deploymentListArgs collects the flag values for the list command, in one
// struct so Run can be driven directly from tests.
type deploymentListArgs struct {
	stack  string
	count  int
	all    bool
	sort   string
	asc    bool
	output string
}

// newDeploymentListCmd builds `pulumi deployment list` with the production
// client factory. The factory is overridable via newDeploymentListCmdWith for
// tests.
func newDeploymentListCmd() *cobra.Command {
	return newDeploymentListCmdWith(nil)
}

func newDeploymentListCmdWith(factory deploymentListClientFactory) *cobra.Command {
	var args deploymentListArgs

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List deployments for a stack",
		Long: "[EXPERIMENTAL] List deployments for a stack.\n" +
			"\n" +
			"Returns deployments for the selected stack, showing each deployment's ID,\n" +
			"operation, status, version, initiator, and the time it was last modified.\n" +
			"Default output is a human-readable table; pass --output=json for a JSON\n" +
			"envelope.\n" +
			"\n" +
			"By default, the first 10 results are shown. Use --count to request more,\n" +
			"or --all to fetch every deployment.",
		Example: "  # List the most recent deployments for the current stack.\n" +
			"  pulumi deployment list\n\n" +
			"  # List the last 50 deployments.\n" +
			"  pulumi deployment list --count 50\n\n" +
			"  # Fetch every deployment as JSON.\n" +
			"  pulumi deployment list --all --output json\n\n" +
			"  # Sort ascending by a server-supported field.\n" +
			"  pulumi deployment list --sort created --asc",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if args.all && args.count > 0 {
				return errors.New("--all and --count are mutually exclusive")
			}
			f := factory
			if f == nil {
				f = defaultDeploymentListClientFactory
			}
			return runDeploymentList(cmd.Context(), cmd.OutOrStdout(), f, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"Number of results to display (fetches multiple pages if needed)")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Fetch all results (mutually exclusive with --count)")
	cmd.Flags().StringVar(&args.sort, "sort", "", "The field to sort results by")
	cmd.Flags().BoolVar(&args.asc, "asc", false, "Sort in ascending order (default descending)")
	cmd.Flags().StringVar(&args.output, "output", "default",
		"Output format. One of: default, json")

	return cmd
}

// defaultDeploymentListClientFactory is the production wiring: resolve the
// stack via RequireStack (non-prompting beyond the standard select flow), cast
// to the cloud-backend types, and hand back the underlying *client.Client.
func defaultDeploymentListClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentListClient, client.StackIdentifier, error) {
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
			errors.New("listing deployments requires the Pulumi Cloud backend; run `pulumi login`")
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
			errors.New("listing deployments requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

// runDeploymentList is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runDeploymentList(
	ctx context.Context, w io.Writer, factory deploymentListClientFactory, args deploymentListArgs,
) error {
	render, err := deploymentListRenderer(args.output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	// Determine how many results to fetch.
	want := 10
	if args.count > 0 {
		want = args.count
	} else if args.all {
		want = -1 // sentinel: fetch everything
	}

	var deployments []apitype.ListDeploymentSnapshot
	var total int64
	page := int64(1)

	for {
		pageSize := int64(defaultPageSize)
		if want > 0 && want-len(deployments) < int(pageSize) {
			pageSize = int64(want - len(deployments))
		}

		resp, err := c.ListStackDeployments(ctx, stackID, client.ListStackDeploymentsOptions{
			Page:     page,
			PageSize: pageSize,
			Sort:     args.sort,
			Asc:      args.asc,
		})
		if err != nil {
			return fmt.Errorf("listing stack deployments: %w", err)
		}
		total = resp.Total
		deployments = append(deployments, resp.Deployments...)

		// Stop if we've collected enough, or there are no more results.
		if want > 0 && len(deployments) >= want {
			deployments = deployments[:want]
			break
		}
		if int64(len(deployments)) >= total || len(resp.Deployments) == 0 {
			break
		}
		page++
	}

	return render(w, deployments, total)
}

type deploymentListRenderFunc func(
	w io.Writer, deployments []apitype.ListDeploymentSnapshot, total int64,
) error

func deploymentListRenderer(format string) (deploymentListRenderFunc, error) {
	switch format {
	case "", "default", "table":
		return renderDeploymentListTable, nil
	case "json":
		return renderDeploymentListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderDeploymentListTable(
	w io.Writer, deployments []apitype.ListDeploymentSnapshot, total int64,
) error {
	if len(deployments) == 0 {
		fmt.Fprintln(w, "No deployments found for this stack.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "OPERATION", "VERSION", "STATUS", "INITIATED BY", "MODIFIED"})

	for _, d := range deployments {
		initiatedBy := d.RequestedBy.GitHubLogin
		if initiatedBy == "" {
			initiatedBy = d.RequestedBy.Name
		}
		t.AppendRow(table.Row{
			d.ID,
			string(d.PulumiOperation),
			d.Version,
			d.Status,
			initiatedBy,
			d.Modified,
		})
	}
	t.Render()

	fmt.Fprintf(w, "\nShowing %d of %d deployment(s)\n", len(deployments), total)
	return nil
}

// deploymentListEnvelope is the JSON shape emitted by `pulumi deployment list --output=json`
type deploymentListEnvelope struct {
	Deployments []apitype.ListDeploymentSnapshot `json:"deployments"`
	Count       int                              `json:"count"`
	Total       int64                            `json:"total"`
}

func renderDeploymentListJSON(
	w io.Writer, deployments []apitype.ListDeploymentSnapshot, total int64,
) error {
	// Normalize nil slice to empty so scripts can rely on `.deployments`
	// always being a JSON array.
	if deployments == nil {
		deployments = []apitype.ListDeploymentSnapshot{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(deploymentListEnvelope{
		Deployments: deployments,
		Count:       len(deployments),
		Total:       total,
	})
}
