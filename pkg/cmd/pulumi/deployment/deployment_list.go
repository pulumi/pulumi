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

// deploymentListArgs collects the flag values for the list command, in one
// struct so Run can be driven directly from tests.
type deploymentListArgs struct {
	stack    string
	page     int64
	pageSize int64
	sort     string
	asc      bool
	output   string
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
		Short: "List deployments for a stack",
		Long: "[EXPERIMENTAL] List deployments for a stack.\n" +
			"\n" +
			"Returns a paginated list of Pulumi Deployments executions for the selected\n" +
			"stack, showing each deployment's ID, operation, status, version, initiator,\n" +
			"and the time it was last modified. Default output is a human-readable table;\n" +
			"pass --output=json for the full response as a JSON envelope.\n" +
			"\n" +
			"Wraps the `ListStackDeploymentsHandlerV2` Pulumi Cloud REST endpoint.",
		Example: "  # List deployments for the current stack.\n" +
			"  pulumi deployment list\n\n" +
			"  # List deployments for a different stack.\n" +
			"  pulumi deployment list --stack acme/web/prod\n\n" +
			"  # Page through results, 25 at a time.\n" +
			"  pulumi deployment list --page 2 --page-size 25\n\n" +
			"  # Sort ascending by a server-supported field.\n" +
			"  pulumi deployment list --sort created --asc\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi deployment list --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
	cmdStack.RegisterCompleteStack(cmd)
	cmd.Flags().Int64Var(&args.page, "page", 1, "The page of results to return (min 1)")
	cmd.Flags().Int64Var(&args.pageSize, "page-size", 10, "The number of results per page (1-100)")
	cmd.Flags().StringVar(&args.sort, "sort", "", "The field to sort results by")
	cmd.Flags().BoolVar(&args.asc, "asc", false, "Sort in ascending order (default descending)")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
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

	resp, err := c.ListStackDeployments(ctx, stackID, client.ListStackDeploymentsOptions{
		Page:     args.page,
		PageSize: args.pageSize,
		Sort:     args.sort,
		Asc:      args.asc,
	})
	if err != nil {
		return fmt.Errorf("listing stack deployments: %w", err)
	}

	return render(w, args, resp)
}

type deploymentListRenderFunc func(w io.Writer, args deploymentListArgs, resp apitype.ListDeploymentResponseV2) error

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

// renderDeploymentListTable prints a compact table of deployments plus a
// pagination footer. Column names are uppercase to match other Cloud-Ready CLI
// list commands (e.g. `pulumi stack webhook list`, `pulumi api list`).
func renderDeploymentListTable(
	w io.Writer, args deploymentListArgs, resp apitype.ListDeploymentResponseV2,
) error {
	if len(resp.Deployments) == 0 {
		fmt.Fprintln(w, "No deployments found for this stack.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "OPERATION", "VERSION", "STATUS", "INITIATED BY", "MODIFIED"})

	for _, d := range resp.Deployments {
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

	fmt.Fprintf(w, "\nShowing %d of %d deployment(s)", len(resp.Deployments), resp.Total)
	if args.page > 0 {
		fmt.Fprintf(w, " (page %d)", args.page)
	}
	fmt.Fprintln(w)
	return nil
}

// deploymentListEnvelope is the JSON shape emitted by
// `pulumi deployment list --output=json`. It mirrors the API response but
// adds the page number the client asked for, which the server doesn't echo
// back, so scripts can keep paginating without remembering their own state.
type deploymentListEnvelope struct {
	Deployments  []apitype.ListDeploymentSnapshot `json:"deployments"`
	Page         int64                            `json:"page"`
	ItemsPerPage int64                            `json:"itemsPerPage"`
	Total        int64                            `json:"total"`
}

func renderDeploymentListJSON(
	w io.Writer, args deploymentListArgs, resp apitype.ListDeploymentResponseV2,
) error {
	// Normalize nil slice to empty so scripts can rely on `.deployments`
	// always being a JSON array.
	deployments := resp.Deployments
	if deployments == nil {
		deployments = []apitype.ListDeploymentSnapshot{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(deploymentListEnvelope{
		Deployments:  deployments,
		Page:         args.page,
		ItemsPerPage: resp.ItemsPerPage,
		Total:        resp.Total,
	})
}
