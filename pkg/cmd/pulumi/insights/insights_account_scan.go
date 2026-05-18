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

// insightsAccountScanClient is the subset of cloud-API operations the scan
// command needs. Defined inside this package so unit tests can stub it without
// touching the full HTTP client surface.
type insightsAccountScanClient interface {
	ScanInsightsAccount(
		ctx context.Context, org, account string, req apitype.InsightsScanRequest,
	) (apitype.InsightsScanResponse, error)
}

// scanClientFactory resolves the cloud client and the effective org for the
// call. orgOverride wins when non-empty; otherwise the default org from the
// cloud context is used.
type scanClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsAccountScanClient, string, error)

// accountScanRender is the function signature stored in the OutputFlag — one
// per supported output format. Sharing the signature keeps the renderer table
// in [defaultAccountScanOutputFormat] readable.
type accountScanRender func(*insightsAccountScanCmd, io.Writer, apitype.InsightsScanResponse) error

// defaultAccountScanOutputFormat wires the output flag up with every supported
// format. Sharing this between the cobra constructor and the tests keeps the
// renderer table in one place.
func defaultAccountScanOutputFormat() outputflag.OutputFlag[accountScanRender] {
	return outputflag.OutputFlag[accountScanRender]{
		RenderForTerminal: (*insightsAccountScanCmd).renderText,
		RenderJSON:        (*insightsAccountScanCmd).renderJSON,
	}
}

type insightsAccountScanArgs struct {
	org             string
	agentPoolID     string
	listConcurrency int64
	readConcurrency int64
	batchSize       int64
	readTimeout     string
	output          outputflag.OutputFlag[accountScanRender]
}

type insightsAccountScanCmd struct {
	clientFactory scanClientFactory
}

// newInsightsAccountScanCmd builds the `pulumi insights account scan` command.
// factory produces the cloud client and resolves the effective org; pass nil to
// use the production factory backed by [cloud.ResolveContext].
func newInsightsAccountScanCmd(factory scanClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultScanClientFactory
	}

	scan := &insightsAccountScanCmd{clientFactory: factory}
	args := insightsAccountScanArgs{output: defaultAccountScanOutputFormat()}

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "[EXPERIMENTAL] Trigger a resource discovery scan for an Insights account",
		Long: "[EXPERIMENTAL] Trigger a resource discovery scan for an Insights account.\n" +
			"\n" +
			"The positional argument is the Insights account to scan; the organization\n" +
			"defaults to the current default org and can be overridden with --org. For\n" +
			"parent accounts the server fans the scan out across child accounts and\n" +
			"returns the parent workflow run.",
		Example: "  # Trigger a scan in a specific Insights account.\n" +
			"  pulumi insights account scan prod-aws\n\n" +
			"  # Override the organization.\n" +
			"  pulumi insights account scan --org acme prod-aws\n\n" +
			"  # Tune scan concurrency and read timeout.\n" +
			"  pulumi insights account scan prod-aws \\\n" +
			"      --list-concurrency 16 --read-concurrency 32 --read-timeout 1m\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi insights account scan prod-aws --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return scan.Run(cmd.Context(), cmd.OutOrStdout(), posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "account"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization that owns the Insights account (defaults to the current default org)")
	cmd.Flags().StringVar(&args.agentPoolID, "agent-pool", "",
		"Agent pool ID to use for the scan (defaults to the account's default pool)")
	cmd.Flags().Int64Var(&args.listConcurrency, "list-concurrency", 0,
		"Parallelism for list operations during the scan (server default when 0)")
	cmd.Flags().Int64Var(&args.readConcurrency, "read-concurrency", 0,
		"Parallelism for read operations during the scan (server default when 0)")
	cmd.Flags().Int64Var(&args.batchSize, "batch-size", 0,
		"Number of resources processed per batch (server default when 0)")
	cmd.Flags().StringVar(&args.readTimeout, "read-timeout", "",
		"Per-read timeout as a Go duration (e.g. '30s', '5m'); server default when empty")
	outputflag.VarP(cmd.Flags(), &args.output)

	cmd.AddCommand(newInsightsAccountScanLogCmd(nil))

	return cmd
}

// Run executes the scan operation. ctx and out are decoupled from cobra so the
// function is straightforward to drive from tests.
func (c *insightsAccountScanCmd) Run(
	ctx context.Context, out io.Writer, account string, args insightsAccountScanArgs,
) error {
	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	req := apitype.InsightsScanRequest{
		AgentPoolID:     args.agentPoolID,
		ListConcurrency: args.listConcurrency,
		ReadConcurrency: args.readConcurrency,
		BatchSize:       args.batchSize,
		ReadTimeout:     args.readTimeout,
	}
	resp, err := client.ScanInsightsAccount(ctx, org, account, req)
	if err != nil {
		return fmt.Errorf("starting insights scan: %w", err)
	}

	return args.output.Get()(c, out, resp)
}

// renderText writes a human-readable view of the workflow run. The shape
// mirrors `insights resource get`'s flat key/value layout — a single record
// reads better than a table here.
//
// When the response is empty — the current 204-No-Content reality, see
// [client.Client.ScanInsightsAccount] — we print a single "scan triggered"
// line instead of a header full of zero fields.
func (c *insightsAccountScanCmd) renderText(w io.Writer, r apitype.InsightsScanResponse) error {
	if scanResponseEmpty(r) {
		fmt.Fprintln(w, "Scan triggered.")
		return nil
	}
	fmt.Fprintf(w, "ID:           %s\n", r.ID)
	fmt.Fprintf(w, "Status:       %s\n", r.Status)
	fmt.Fprintf(w, "Started:      %s\n", r.StartedAt.UTC().Format(time.RFC3339))
	// FinishedAt is required by the schema but zero when the run is still
	// in flight; skip the line in that case so "finished: 0001-01-01" doesn't
	// look like an outage.
	if !r.FinishedAt.IsZero() {
		fmt.Fprintf(w, "Finished:     %s\n", r.FinishedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(w, "Last updated: %s\n", r.LastUpdatedAt.UTC().Format(time.RFC3339))
	if len(r.Jobs) > 0 {
		fmt.Fprintln(w, "Jobs:")
		for i, job := range r.Jobs {
			fmt.Fprintf(w, "  Job %d: %s\n", i+1, job.Status)
			for _, step := range job.Steps {
				fmt.Fprintf(w, "    - %s (%s)\n", step.Name, step.Status)
			}
		}
	}
	return nil
}

// renderJSON writes the workflow run as indented JSON. Indentation matches
// the rest of the cli/cloud commands so jq-style scripting feels consistent.
//
// When the response is empty we emit `{"started": true}` rather than a
// struct full of zero fields, so jq consumers can tell a freshly initiated
// scan apart from a populated workflow run.
func (c *insightsAccountScanCmd) renderJSON(w io.Writer, r apitype.InsightsScanResponse) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if scanResponseEmpty(r) {
		return enc.Encode(struct {
			Started bool `json:"started"`
		}{Started: true})
	}
	return enc.Encode(r)
}

// scanResponseEmpty reports whether the response carries no workflow data.
// The service currently returns 204 No Content for a successful scan trigger,
// so a zero-value [apitype.InsightsScanResponse] means "the call succeeded
// but the server didn't include a workflow run".
func scanResponseEmpty(r apitype.InsightsScanResponse) bool {
	return r.ID == "" && r.Status == "" && r.StartedAt.IsZero() && len(r.Jobs) == 0
}

// defaultScanClientFactory is the production wiring for scanClientFactory. It
// resolves the cloud context via cloud.ResolveContext and surfaces the
// *client.Client directly — *client.Client already satisfies
// insightsAccountScanClient through its ScanInsightsAccount method.
func defaultScanClientFactory(
	ctx context.Context, orgOverride string,
) (insightsAccountScanClient, string, error) {
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
