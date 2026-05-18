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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// driftListClient is the interface the drift list command needs.
type driftListClient interface {
	ListDriftRuns(
		ctx context.Context, stackID client.StackIdentifier,
		page, pageSize int,
	) (apitype.ListDriftRunsResponse, error)
}

type driftListClientFactory func(
	ctx context.Context, stackFlag string,
) (driftListClient, client.StackIdentifier, error)

// driftListRender renders a drift list response.
type driftListRender func(
	cmd *driftListCmd, runs []apitype.DriftRun, total int,
) error

// defaultPageSize is the number of items fetched per API call.
const defaultPageSize = 100

type driftListCmd struct {
	stack  string
	count  int
	all    bool
	output outputflag.OutputFlag[driftListRender]
	w      io.Writer
}

func newStackDriftListCmd() *cobra.Command {
	return newStackDriftListCmdWith(nil)
}

func newStackDriftListCmdWith(factory driftListClientFactory) *cobra.Command {
	dlcmd := &driftListCmd{
		output: outputflag.OutputFlag[driftListRender]{
			RenderForTerminal: (*driftListCmd).renderTable,
			RenderJSON:        (*driftListCmd).renderJSON,
		},
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List drift detection runs for a stack",
		Long: "List drift detection runs for a stack.\n" +
			"\n" +
			"Returns drift detection runs for the specified stack. Each run\n" +
			"includes whether drift was detected, the run status, and details\n" +
			"about detection and remediation phases.\n" +
			"\n" +
			"By default, the first 10 results are shown. Use --count to request\n" +
			"more, or --all to fetch every run.",
		Example: "  # List drift runs for the current stack\n" +
			"  pulumi stack drift list\n\n" +
			"  # Show the last 50 drift runs\n" +
			"  pulumi stack drift list --count 50\n\n" +
			"  # Fetch all drift runs as JSON\n" +
			"  pulumi stack drift list --all --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultDriftListClientFactory
			}
			if dlcmd.all && dlcmd.count > 0 {
				return errors.New("--all and --count are mutually exclusive")
			}
			dlcmd.w = cmd.OutOrStdout()
			return dlcmd.run(cmd.Context(), factory)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&dlcmd.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&dlcmd.count, "count", 0,
		"Number of results to display (fetches multiple pages if needed)")
	cmd.Flags().BoolVar(&dlcmd.all, "all", false,
		"Fetch all results (mutually exclusive with --count)")
	outputflag.Var(cmd.Flags(), &dlcmd.output)

	return cmd
}

func defaultDriftListClientFactory(
	ctx context.Context, stackFlag string,
) (driftListClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("drift commands require the Pulumi Cloud backend; run `pulumi login`")
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

	be := cloudStack.Backend().(httpstate.Backend)
	return be.Client(), stackID, nil
}

func (c *driftListCmd) run(ctx context.Context, factory driftListClientFactory) error {
	cl, stackID, err := factory(ctx, c.stack)
	if err != nil {
		return err
	}

	// Determine how many results to fetch.
	want := 10 // default: one page worth
	if c.count > 0 {
		want = c.count
	} else if c.all {
		want = -1 // sentinel: fetch everything
	}

	var allRuns []apitype.DriftRun
	var total int
	page := 1

	for {
		pageSize := defaultPageSize
		if want > 0 && want-len(allRuns) < pageSize {
			pageSize = want - len(allRuns)
		}

		resp, err := cl.ListDriftRuns(ctx, stackID, page, pageSize)
		if err != nil {
			return fmt.Errorf("listing drift runs: %w", err)
		}
		total = resp.Total
		allRuns = append(allRuns, resp.DriftRuns...)

		// Stop if we've collected enough, or there are no more results.
		if want > 0 && len(allRuns) >= want {
			allRuns = allRuns[:want]
			break
		}
		if len(allRuns) >= total || len(resp.DriftRuns) == 0 {
			break
		}
		page++
	}

	return c.output.Get()(c, allRuns, total)
}

func (c *driftListCmd) renderJSON(runs []apitype.DriftRun, total int) error {
	if runs == nil {
		runs = []apitype.DriftRun{}
	}
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		DriftRuns []apitype.DriftRun `json:"driftRuns"`
		Count     int                `json:"count"`
		Total     int                `json:"total"`
	}{
		DriftRuns: runs,
		Count:     len(runs),
		Total:     total,
	})
}

func (c *driftListCmd) renderTable(runs []apitype.DriftRun, total int) error {
	if len(runs) == 0 {
		fmt.Fprintln(c.w, "No drift detection runs found for this stack.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(c.w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{
		"ID", "CREATED", "STATUS", "DRIFT", "DETECT", "REMEDIATE",
	})

	for _, run := range runs {
		drift := "no"
		if run.DriftDetected {
			drift = "yes"
		}
		detect := formatUpdateStatus(run.DetectUpdate)
		remediate := formatUpdateStatus(run.RemediateUpdate)

		t.AppendRow(table.Row{
			run.ID, run.Created, run.Status, drift, detect, remediate,
		})
	}

	// Let go-pretty wrap columns to fit the terminal. Fixed-width columns
	// (CREATED, STATUS, DRIFT) are short enough to never need wrapping.
	// ID, DETECT, and REMEDIATE get max widths so they wrap if needed.
	cols := cmdCmd.StdoutWidth()
	// CREATED(25) + STATUS(11) + DRIFT(5) + borders(~22) = 63 fixed
	flexible := cols - 63
	if flexible < 30 {
		flexible = 30
	}
	// Split flexible space: ~40% ID, ~30% DETECT, ~30% REMEDIATE
	idWidth := flexible * 2 / 5
	detWidth := flexible * 3 / 10
	remWidth := flexible - idWidth - detWidth
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "ID", WidthMax: idWidth, WidthMaxEnforcer: text.WrapText},
		{Name: "DETECT", WidthMax: detWidth, WidthMaxEnforcer: text.WrapText},
		{Name: "REMEDIATE", WidthMax: remWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(c.w, "\nShowing %d of %d drift run(s)\n", len(runs), total)
	return nil
}

func formatUpdateStatus(u *apitype.DriftRunUpdate) string {
	if u == nil {
		return "-"
	}
	parts := []string{u.Status}
	if len(u.ResourceChanges) > 0 {
		var changes []string
		for op, count := range u.ResourceChanges {
			if op != "same" && count > 0 {
				changes = append(changes, fmt.Sprintf("%d %s", count, op))
			}
		}
		if len(changes) > 0 {
			parts = append(parts, "("+strings.Join(changes, ", ")+")")
		}
	}
	return strings.Join(parts, " ")
}
