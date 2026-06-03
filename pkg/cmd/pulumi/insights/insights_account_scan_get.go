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
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// insightsAccountScanGetClient is the subset of cloud-API operations the
// scan get command needs. Defined inside this package so unit tests can stub
// it without touching the full HTTP client surface.
type insightsAccountScanGetClient interface {
	GetInsightsScan(
		ctx context.Context, org, account, scanID string,
	) (apitype.InsightsScanResponse, error)
}

// accountScanGetClientFactory resolves the cloud client and the effective
// org for the call.
type accountScanGetClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsAccountScanGetClient, string, error)

type accountScanGetRender func(io.Writer, apitype.InsightsScanResponse) error

func defaultAccountScanGetOutputFormat() outputflag.OutputFlag[accountScanGetRender] {
	return outputflag.OutputFlag[accountScanGetRender]{
		RenderForTerminal: renderScanGetText,
		RenderJSON:        renderScanGetJSON,
	}
}

type insightsAccountScanGetArgs struct {
	org    string
	output outputflag.OutputFlag[accountScanGetRender]
}

type insightsAccountScanGetCmd struct {
	clientFactory accountScanGetClientFactory
}

// newInsightsAccountScanGetCmd builds the `pulumi insights account scan get`
// command. factory produces the cloud client and resolves the effective org;
// pass nil to use the production factory backed by [cloud.ResolveContext].
func newInsightsAccountScanGetCmd(factory accountScanGetClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultScanGetClientFactory
	}

	get := &insightsAccountScanGetCmd{clientFactory: factory}
	args := insightsAccountScanGetArgs{output: defaultAccountScanGetOutputFormat()}

	cmd := &cobra.Command{
		Use:   "get <account> <scan-id>",
		Short: "[EXPERIMENTAL] Get details for a specific Insights scan",
		Long: "[EXPERIMENTAL] Get details for a specific Pulumi Insights scan.\n" +
			"\n" +
			"Returns the workflow run for a single scan: status, timing, and the\n" +
			"list of jobs (with their step-level status). Use `account scan list`\n" +
			"to discover recent scan IDs, and `account scan log` to fetch the raw\n" +
			"log output for a step.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for\n" +
			"the full response as JSON.",
		Example: "  # Get the details of a specific scan.\n" +
			"  pulumi insights account scan get prod-aws/us-east-1 7bb80f16-…\n\n" +
			"  # Pipe a recent scan ID into `get` for the full workflow run.\n" +
			"  pulumi insights account scan list prod-aws --output json |\n" +
			"    jq -r '.scans[0].id' | xargs pulumi insights account scan get prod-aws/us-east-1\n\n" +
			"  # Emit JSON for scripting (includes the per-step status).\n" +
			"  pulumi insights account scan get prod-aws/us-east-1 7bb80f16-… --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return get.Run(cmd.Context(), cmd.OutOrStdout(), posArgs[0], posArgs[1], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "account"},
			{Name: "scan-id"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization that owns the Insights account (defaults to the current default org)")
	outputflag.Var(cmd.Flags(), &args.output)

	return cmd
}

// Run executes the get operation. ctx and out are decoupled from cobra so
// the function is straightforward to drive from tests.
func (c *insightsAccountScanGetCmd) Run(
	ctx context.Context, out io.Writer, account, scanID string, args insightsAccountScanGetArgs,
) error {
	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := client.GetInsightsScan(ctx, org, account, scanID)
	if err != nil {
		return fmt.Errorf("getting insights scan: %w", err)
	}

	return args.output.Get()(out, resp)
}

// renderScanGetText prints a human-readable summary of a single scan as
// aligned key/value pairs. The shape mirrors `deployment get` so the two
// workflow-detail views feel like the same family of commands — only summary
// metadata in text; the full per-job/per-step structure is in `--output json`.
func renderScanGetText(w io.Writer, r apitype.InsightsScanResponse) error {
	fmt.Fprintf(w, "%-18s %s\n", "ID:", r.ID)
	fmt.Fprintf(w, "%-18s %s\n", "Status:", r.Status)
	if !r.StartedAt.IsZero() {
		fmt.Fprintf(w, "%-18s %s\n", "Started:", r.StartedAt.UTC().Format(time.RFC3339))
	}
	// FinishedAt is required by the schema but zero when the run is still in
	// flight; skip the line in that case so "Finished: 0001-01-01" doesn't
	// look like an outage.
	if !r.FinishedAt.IsZero() {
		fmt.Fprintf(w, "%-18s %s\n", "Finished:", r.FinishedAt.UTC().Format(time.RFC3339))
		if !r.StartedAt.IsZero() {
			d := r.FinishedAt.Sub(r.StartedAt).Round(time.Second)
			fmt.Fprintf(w, "%-18s %s\n", "Duration:", d)
		}
	}
	if !r.LastUpdatedAt.IsZero() {
		fmt.Fprintf(w, "%-18s %s\n", "Last updated:", r.LastUpdatedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(w, "%-18s %d\n", "Jobs:", len(r.Jobs))
	return nil
}

// scanGetJSON is the JSON envelope emitted by
// `pulumi insights account scan get --output=json`. Nil slices are normalized
// to `[]` so consumers can rely on `jobs` being a JSON array — matching the
// shape used by `deployment get`.
type scanGetJSON struct {
	ID            string                       `json:"id"`
	OrgID         string                       `json:"orgId"`
	UserID        string                       `json:"userId"`
	Status        string                       `json:"status"`
	StartedAt     time.Time                    `json:"startedAt"`
	FinishedAt    time.Time                    `json:"finishedAt"`
	LastUpdatedAt time.Time                    `json:"lastUpdatedAt"`
	JobTimeout    time.Time                    `json:"jobTimeout"`
	Jobs          []apitype.InsightsScanJobRun `json:"jobs"`
}

func toScanGetJSON(r apitype.InsightsScanResponse) scanGetJSON {
	jobs := r.Jobs
	if jobs == nil {
		jobs = []apitype.InsightsScanJobRun{}
	}
	return scanGetJSON{
		ID:            r.ID,
		OrgID:         r.OrgID,
		UserID:        r.UserID,
		Status:        r.Status,
		StartedAt:     r.StartedAt,
		FinishedAt:    r.FinishedAt,
		LastUpdatedAt: r.LastUpdatedAt,
		JobTimeout:    r.JobTimeout,
		Jobs:          jobs,
	}
}

func renderScanGetJSON(w io.Writer, r apitype.InsightsScanResponse) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toScanGetJSON(r))
}

func defaultScanGetClientFactory(
	ctx context.Context, orgOverride string,
) (insightsAccountScanGetClient, string, error) {
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
