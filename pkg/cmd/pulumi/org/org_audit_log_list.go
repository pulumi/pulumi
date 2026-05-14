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

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// orgAuditLogListClient is the narrow subset of cloud-API operations the list
// command needs.
type orgAuditLogListClient interface {
	ListAuditLogs(
		ctx context.Context, orgName string, opts client.ListAuditLogsOptions,
	) (apitype.ListAuditLogEventsResponse, error)
}

// orgAuditLogListClientFactory resolves a cloud client and the organization
// to list audit log events for. orgFlag carries the raw value of `--org`
// (empty means "use the default org").
type orgAuditLogListClientFactory func(
	ctx context.Context, orgFlag string,
) (orgAuditLogListClient, string, error)

// orgAuditLogListArgs collects the flag values for the list command, in one
// struct so Run can be driven directly from tests.
type orgAuditLogListArgs struct {
	org               string
	eventType         string
	user              string
	startTime         string
	continuationToken string
	output            string
}

// newOrgAuditLogListCmd builds `pulumi org audit-log list` with the production
// client factory.
func newOrgAuditLogListCmd() *cobra.Command {
	return newOrgAuditLogListCmdWith(defaultOrgAuditLogListClientFactory)
}

func newOrgAuditLogListCmdWith(factory orgAuditLogListClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgAuditLogListClientFactory must not be nil")
	var args orgAuditLogListArgs

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "[EXPERIMENTAL] List audit log events for an organization",
		Long: "[EXPERIMENTAL] List audit log events for an organization.\n" +
			"\n" +
			"Returns a single page of audit log events for the organization.\n" +
			"Results may be filtered by event type and by the user that triggered\n" +
			"the event. Use --start-time to bound the upper end of the time range.\n" +
			"\n" +
			"The endpoint is paginated; the response includes a continuation token\n" +
			"when more results are available. Pass that token via\n" +
			"--continuation-token on a subsequent call to fetch the next page.\n" +
			"\n" +
			"Wraps the `ListAuditLogEvents` Pulumi Cloud REST endpoint. Default\n" +
			"output is a human-readable table; pass --output=json for the full\n" +
			"response as a JSON envelope.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgAuditLogList(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to list audit logs for")
	cmd.Flags().StringVar(&args.eventType, "event-type", "", "Filter by event type")
	cmd.Flags().StringVar(&args.user, "user", "", "Filter by user login")
	cmd.Flags().StringVar(&args.startTime, "start-time", "",
		"The upper bound of the time range (V1 semantics)")
	cmd.Flags().StringVar(&args.continuationToken, "continuation-token", "",
		"The continuation token for paginated retrieval")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// defaultOrgAuditLogListClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultOrgAuditLogListClientFactory(
	ctx context.Context, orgFlag string,
) (orgAuditLogListClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"listing audit logs requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

// runOrgAuditLogList is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runOrgAuditLogList(
	ctx context.Context, w io.Writer,
	factory orgAuditLogListClientFactory, args orgAuditLogListArgs,
) error {
	render, err := orgAuditLogListRenderer(args.output)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.ListAuditLogs(ctx, org, client.ListAuditLogsOptions{
		EventType:         args.eventType,
		User:              args.user,
		StartTime:         args.startTime,
		ContinuationToken: args.continuationToken,
	})
	if err != nil {
		return err
	}

	return render(w, resp)
}

type orgAuditLogListRenderFunc func(w io.Writer, resp apitype.ListAuditLogEventsResponse) error

func orgAuditLogListRenderer(format string) (orgAuditLogListRenderFunc, error) {
	switch format {
	case "", "default", "table":
		return renderOrgAuditLogListTable, nil
	case "json":
		return renderOrgAuditLogListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// formatAuditLogTimestamp renders a Unix-epoch timestamp in RFC3339 UTC. A
// zero timestamp renders as the empty-cell placeholder so the table stays
// aligned.
func formatAuditLogTimestamp(ts int64) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func renderOrgAuditLogListTable(
	w io.Writer, resp apitype.ListAuditLogEventsResponse,
) error {
	if len(resp.AuditLogEvents) == 0 {
		fmt.Fprintln(w, "No audit log events found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"TIMESTAMP", "USER", "EVENT", "NAME", "SOURCE IP"})

	for _, ev := range resp.AuditLogEvents {
		user := ev.User.GitHubLogin
		if user == "" {
			user = "-"
		}
		event := ev.Event
		if event == "" {
			event = "-"
		}
		name := ev.Name
		if name == "" {
			name = "-"
		}
		sourceIP := ev.SourceIP
		if sourceIP == "" {
			sourceIP = "-"
		}
		t.AppendRow(table.Row{
			formatAuditLogTimestamp(ev.Timestamp),
			user,
			event,
			name,
			sourceIP,
		})
	}
	t.Render()

	if resp.ContinuationToken != "" {
		fmt.Fprintf(w,
			"\nMore results available. Re-run with --continuation-token %q to continue.\n",
			resp.ContinuationToken)
	}
	return nil
}

// auditLogListEnvelope is the JSON shape emitted by
// `pulumi org audit-log list --output=json`.
type auditLogListEnvelope struct {
	Events            []apitype.AuditLogEvent `json:"events"`
	ContinuationToken string                  `json:"continuationToken"`
}

func renderOrgAuditLogListJSON(
	w io.Writer, resp apitype.ListAuditLogEventsResponse,
) error {
	events := resp.AuditLogEvents
	if events == nil {
		events = []apitype.AuditLogEvent{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(auditLogListEnvelope{
		Events:            events,
		ContinuationToken: resp.ContinuationToken,
	})
}
