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

package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// insightsScanLogClient narrows the *client.Client surface so tests can stub
// it without faking the whole HTTP client.
type insightsScanLogClient interface {
	GetInsightsScanLogs(
		ctx context.Context, org, account, scanID string,
		params apitype.InsightsScanLogsParams,
	) (apitype.InsightsScanLogs, error)
}

// scanLogClientFactory resolves the client and effective org. orgOverride
// wins when non-empty.
type scanLogClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsScanLogClient, string, error)

type scanLogRender func(c *insightsAccountScanLogCmd, logs apitype.InsightsScanLogs) error

type insightsAccountScanLogCmd struct {
	org    string
	job    int
	jobSet bool
	step   int
	count  int
	all    bool
	output outputflag.OutputFlag[scanLogRender]

	w io.Writer
}

// newInsightsAccountScanLogCmd builds the `pulumi insights account scan log`
// command. Pass nil for factory to use the production wiring.
func newInsightsAccountScanLogCmd(factory scanLogClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultScanLogClientFactory
	}

	logCmd := &insightsAccountScanLogCmd{
		output: outputflag.OutputFlag[scanLogRender]{
			RenderForTerminal: (*insightsAccountScanLogCmd).renderText,
			RenderJSON:        (*insightsAccountScanLogCmd).renderJSON,
		},
	}

	cmd := &cobra.Command{
		Use:   "log <account> <scan-id>",
		Short: "[EXPERIMENTAL] Retrieve log output for an Insights scan",
		Long: "[EXPERIMENTAL] Retrieve log output for an Insights scan.\n" +
			"\n" +
			"By default, a single page of log entries is returned. Use --count\n" +
			"to request more, or --all to fetch every entry.\n" +
			"\n" +
			"Passing --job switches to step mode and returns the raw text\n" +
			"output of a single step. --step selects the step within the job,\n" +
			"and --all fetches the full step output.",
		Example: "  # Show the first page of a scan's logs.\n" +
			"  pulumi insights account scan log prod-aws scan-123\n\n" +
			"  # Show the last 100 entries.\n" +
			"  pulumi insights account scan log prod-aws scan-123 --count 100\n\n" +
			"  # Fetch every entry as JSON.\n" +
			"  pulumi insights account scan log prod-aws scan-123 --all --output json\n\n" +
			"  # Read step 0 of job 0 in full.\n" +
			"  pulumi insights account scan log prod-aws scan-123 --job 0 --step 0 --all",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			// Zero is a legitimate job/step index, so we can't use it as a
			// sentinel for "unset".
			logCmd.jobSet = cmd.Flags().Changed("job")
			logCmd.w = cmd.OutOrStdout()
			return logCmd.run(cmd.Context(), factory, posArgs[0], posArgs[1])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "account"},
			{Name: "scan-id"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&logCmd.org, "org", "",
		"Organization that owns the Insights account (defaults to the current default org)")
	cmd.Flags().IntVar(&logCmd.count, "count", 0,
		"Number of log entries to display")
	cmd.Flags().BoolVar(&logCmd.all, "all", false,
		"Fetch every entry (mutually exclusive with --count)")
	cmd.Flags().IntVar(&logCmd.job, "job", 0,
		"Switch to step mode and select this job index (combine with --step)")
	cmd.Flags().IntVar(&logCmd.step, "step", 0,
		"Step index within --job whose log output to fetch")
	outputflag.VarP(cmd.Flags(), &logCmd.output)

	cmd.MarkFlagsMutuallyExclusive("all", "count")

	return cmd
}

// run is decoupled from cobra so tests can drive it directly.
func (c *insightsAccountScanLogCmd) run(
	ctx context.Context, factory scanLogClientFactory, account, scanID string,
) error {
	// --all and --count are handled by MarkFlagsMutuallyExclusive on the
	// cobra command; --step requires --job is one-way so we catch it here.
	if !c.jobSet && c.step != 0 {
		return errors.New("--step requires --job")
	}

	client, org, err := factory(ctx, c.org)
	if err != nil {
		return err
	}

	logs, err := c.fetch(ctx, client, org, account, scanID)
	if err != nil {
		return fmt.Errorf("reading insights scan logs: %w", err)
	}
	return c.output.Get()(c, logs)
}

func (c *insightsAccountScanLogCmd) fetch(
	ctx context.Context, client insightsScanLogClient,
	org, account, scanID string,
) (apitype.InsightsScanLogs, error) {
	if c.jobSet {
		return c.fetchStepMode(ctx, client, org, account, scanID)
	}
	return c.fetchContinuationMode(ctx, client, org, account, scanID)
}

func (c *insightsAccountScanLogCmd) fetchContinuationMode(
	ctx context.Context, client insightsScanLogClient,
	org, account, scanID string,
) (apitype.InsightsScanLogs, error) {
	// Target entry count: positive = exact, -1 = unlimited (--all), 0 =
	// one page only (default).
	want := 0
	switch {
	case c.count > 0:
		want = c.count
	case c.all:
		want = -1
	}

	// API limit per request, documented 1-500.
	const maxPageCount = 500

	var acc apitype.InsightsScanLogs
	var token string
	for {
		params := apitype.InsightsScanLogsParams{ContinuationToken: token}
		switch {
		case want == -1:
			// Max out the page to minimise round-trips.
			params.Count = maxPageCount
		case want > 0:
			// Cap to the remaining count so the last page doesn't
			// over-fetch.
			remaining := want - len(acc.Lines)
			if remaining < maxPageCount {
				params.Count = remaining
			} else {
				params.Count = maxPageCount
			}
		}

		resp, err := client.GetInsightsScanLogs(ctx, org, account, scanID, params)
		if err != nil {
			return apitype.InsightsScanLogs{}, err
		}
		if acc.Type == "" {
			acc.Type = resp.Type
		}
		acc.Lines = append(acc.Lines, resp.Lines...)
		acc.ContinuationToken = resp.ContinuationToken

		if want > 0 && len(acc.Lines) >= want {
			acc.Lines = acc.Lines[:want]
			break
		}
		if resp.ContinuationToken == "" || len(resp.Lines) == 0 || want == 0 {
			break
		}
		token = resp.ContinuationToken
	}
	return acc, nil
}

func (c *insightsAccountScanLogCmd) fetchStepMode(
	ctx context.Context, client insightsScanLogClient,
	org, account, scanID string,
) (apitype.InsightsScanLogs, error) {
	job, step := c.job, c.step
	params := apitype.InsightsScanLogsParams{Job: &job, Step: &step}
	resp, err := client.GetInsightsScanLogs(ctx, org, account, scanID, params)
	if err != nil {
		return apitype.InsightsScanLogs{}, err
	}

	// --count doesn't apply in step mode (the API paginates by bytes, not
	// by entries), so default and --count behave identically.
	if !c.all {
		return resp, nil
	}

	acc := resp
	for acc.NextOffset > 0 {
		offset := acc.NextOffset
		more, err := client.GetInsightsScanLogs(ctx, org, account, scanID,
			apitype.InsightsScanLogsParams{Job: &job, Step: &step, Offset: &offset})
		if err != nil {
			return apitype.InsightsScanLogs{}, err
		}
		acc.Output += more.Output
		acc.NextOffset = more.NextOffset
		if more.Output == "" && more.NextOffset == 0 {
			break
		}
	}
	return acc, nil
}

// renderText dispatches by mode: step mode emits a raw text blob, continuation
// mode emits structured records.
func (c *insightsAccountScanLogCmd) renderText(r apitype.InsightsScanLogs) error {
	if c.jobSet {
		return c.renderStepText(r)
	}
	return c.renderContinuationText(r)
}

func (c *insightsAccountScanLogCmd) renderStepText(r apitype.InsightsScanLogs) error {
	if _, err := io.WriteString(c.w, r.Output); err != nil {
		return err
	}
	// Force a trailing newline so the hint below sits on its own line.
	if len(r.Output) > 0 && r.Output[len(r.Output)-1] != '\n' {
		fmt.Fprintln(c.w)
	}
	if r.NextOffset > 0 {
		fmt.Fprintf(c.w,
			"More output available. Re-run with --all to fetch the rest.\n")
	}
	return nil
}

func (c *insightsAccountScanLogCmd) renderContinuationText(r apitype.InsightsScanLogs) error {
	if len(r.Lines) == 0 {
		fmt.Fprintln(c.w, "No log entries.")
		return nil
	}

	for _, line := range r.Lines {
		// The server's Line field already ends in a newline; strip it so we
		// don't double-space the output when we add our own.
		text := strings.TrimRight(line.Line, "\n")
		ts := ""
		if !line.Timestamp.IsZero() {
			ts = line.Timestamp.UTC().Format(time.RFC3339)
		}
		switch {
		case ts != "" && line.Header != "":
			fmt.Fprintf(c.w, "%s [%s] %s\n", ts, line.Header, text)
		case ts != "":
			fmt.Fprintf(c.w, "%s %s\n", ts, text)
		case line.Header != "":
			fmt.Fprintf(c.w, "[%s] %s\n", line.Header, text)
		default:
			fmt.Fprintln(c.w, text)
		}
	}
	if r.ContinuationToken != "" {
		fmt.Fprintln(c.w,
			"\nMore entries available. Re-run with --count <N> or --all to fetch more.")
	}
	return nil
}

func (c *insightsAccountScanLogCmd) renderJSON(r apitype.InsightsScanLogs) error {
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func defaultScanLogClientFactory(
	ctx context.Context, orgOverride string,
) (insightsScanLogClient, string, error) {
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
