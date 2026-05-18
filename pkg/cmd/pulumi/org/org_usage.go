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

package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// validUsageGranularity enumerates the granularity values accepted by the
// server. Validating up front turns a typo into a clean CLI error rather than
// a 4xx round-trip.
var validUsageGranularity = map[string]struct{}{
	"hourly":  {},
	"daily":   {},
	"monthly": {},
}

func newOrgUsageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "usage",
		Short:  "[EXPERIMENTAL] Inspect organization resource usage",
		Long:   "[EXPERIMENTAL] Inspect organization resource usage.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgUsageGetCmd(nil))
	return cmd
}

// orgUsageGetClient is the subset of cloud-API operations the usage get
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type orgUsageGetClient interface {
	GetOrgUsageSummary(
		ctx context.Context, org string, params apitype.OrgUsageSummaryParams,
	) (apitype.OrgUsageSummaryResponse, error)
}

// orgUsageGetClientFactory resolves the cloud client and the effective org for
// the call. orgOverride wins when non-empty; otherwise the default org from
// the cloud context is used.
type orgUsageGetClientFactory func(
	ctx context.Context, orgOverride string,
) (orgUsageGetClient, string, error)

// usageGetRender renders a usage summary response to a writer.
type usageGetRender func(w io.Writer, resp apitype.OrgUsageSummaryResponse) error

type orgUsageGetArgs struct {
	org           string
	granularity   string
	lookbackDays  int64
	lookbackStart int64
	render        usageGetRender
}

// newOrgUsageGetCmd builds `pulumi org usage get`. factory produces the cloud
// client and resolves the effective org; pass nil to use the production
// factory backed by [cloud.ResolveContext].
func newOrgUsageGetCmd(factory orgUsageGetClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultUsageGetClientFactory
	}

	var args orgUsageGetArgs
	output := outputflag.OutputFlag[usageGetRender]{
		RenderForTerminal: renderUsageGetTable,
		RenderJSON:        renderUsageGetJSON,
	}

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "[EXPERIMENTAL] Fetch the resources-under-management summary for an organization",
		Long: "[EXPERIMENTAL] Fetch the resources-under-management summary for an organization.\n" +
			"\n" +
			"Returns the Resources Under Management (RUM) and Resource Hours Under\n" +
			"Management (RHUM) totals for the organization, bucketed by the requested\n" +
			"granularity. Default output is a human-readable table; pass --output=json\n" +
			"for the full response as a JSON envelope.\n" +
			"\n" +
			"Wraps the `GetUsageSummaryResourceHours` Pulumi Cloud REST endpoint.",
		Example: "  # Summary for the default org, server-chosen granularity and window.\n" +
			"  pulumi org usage get\n\n" +
			"  # Daily summary for the last 30 days.\n" +
			"  pulumi org usage get --granularity daily --lookback-days 30\n\n" +
			"  # Hourly summary ending at a specific time (Unix seconds).\n" +
			"  pulumi org usage get --granularity hourly --lookback-start 1700000000\n\n" +
			"  # JSON output for scripting.\n" +
			"  pulumi org usage get --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			args.render = output.Get()
			return runOrgUsageGet(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization to fetch usage for (defaults to the current default org)")
	cmd.Flags().StringVar(&args.granularity, "granularity", "daily",
		"Time granularity for aggregation. One of: hourly, daily, monthly")
	cmd.Flags().Int64Var(&args.lookbackDays, "lookback-days", 30,
		"Number of days to look back from --lookback-start (or the current time)")
	cmd.Flags().Int64Var(&args.lookbackStart, "lookback-start", 0,
		"Unix timestamp (seconds) marking the end of the lookback window")
	outputflag.Var(cmd.Flags(), &output)

	return cmd
}

func runOrgUsageGet(
	ctx context.Context, w io.Writer, factory orgUsageGetClientFactory, args orgUsageGetArgs,
) error {
	if args.granularity != "" {
		if _, ok := validUsageGranularity[args.granularity]; !ok {
			return fmt.Errorf("invalid --granularity %q (must be one of: hourly, daily, monthly)",
				args.granularity)
		}
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.GetOrgUsageSummary(ctx, org, apitype.OrgUsageSummaryParams{
		Granularity:   args.granularity,
		LookbackDays:  args.lookbackDays,
		LookbackStart: args.lookbackStart,
	})
	if err != nil {
		return fmt.Errorf("fetching organization usage summary: %w", err)
	}

	return args.render(w, resp)
}

// defaultUsageGetClientFactory is the production wiring: resolve the cloud
// context, require a logged-in cloud session, and hand back the underlying
// *client.Client.
func defaultUsageGetClientFactory(
	ctx context.Context, orgOverride string,
) (orgUsageGetClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}

	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}

	return resolved.Client, org, nil
}

// renderUsageGetTable prints the summary as a compact table. The period
// columns are populated according to whichever of year/month/day/week/hour the
// server returned for each row, so the table adapts to the requested
// granularity without the CLI having to know which fields go with which.
func renderUsageGetTable(w io.Writer, resp apitype.OrgUsageSummaryResponse) error {
	if len(resp.Summary) == 0 {
		fmt.Fprintln(w, "No usage data available for this organization and time window.")
		return nil
	}

	hasMonth, hasDay, hasWeek, hasHour := summaryColumnsPresent(resp.Summary)

	header := table.Row{"Year"}
	if hasMonth {
		header = append(header, "Month")
	}
	if hasDay {
		header = append(header, "Day")
	}
	if hasWeek {
		header = append(header, "Week")
	}
	if hasHour {
		header = append(header, "Hour")
	}
	header = append(header, "Resources", "Resource Hours")

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Format.Header = text.FormatDefault
	t.AppendHeader(header)

	for _, s := range resp.Summary {
		row := table.Row{s.Year}
		if hasMonth {
			row = append(row, intPtrCell(s.Month))
		}
		if hasDay {
			row = append(row, intPtrCell(s.Day))
		}
		if hasWeek {
			row = append(row, intPtrCell(s.WeekNumber))
		}
		if hasHour {
			row = append(row, intPtrCell(s.Hour))
		}
		row = append(row, s.Resources, s.ResourceHours)
		t.AppendRow(row)
	}
	t.Render()

	fmt.Fprintf(w, "\n%s summary point(s).\n", strconv.Itoa(len(resp.Summary)))
	return nil
}

// summaryColumnsPresent reports which optional time fields appear in at least
// one row of the summary. We hide columns that are entirely empty so the table
// matches the requested granularity without leaving cosmetic blank columns.
func summaryColumnsPresent(rows []apitype.OrgResourceCountSummary) (month, day, week, hour bool) {
	for _, s := range rows {
		if s.Month != nil {
			month = true
		}
		if s.Day != nil {
			day = true
		}
		if s.WeekNumber != nil {
			week = true
		}
		if s.Hour != nil {
			hour = true
		}
	}
	return month, day, week, hour
}

func intPtrCell(p *int) any {
	if p == nil {
		return ""
	}
	return *p
}

func renderUsageGetJSON(w io.Writer, resp apitype.OrgUsageSummaryResponse) error {
	// Normalize nil slice to empty so scripts can rely on `.summary` always
	// being a JSON array.
	if resp.Summary == nil {
		resp.Summary = []apitype.OrgResourceCountSummary{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
