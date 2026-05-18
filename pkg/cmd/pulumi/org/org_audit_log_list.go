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
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
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
	org          string
	eventType    string
	user         string
	startTime    string
	count        int64
	all          bool
	outputFormat outputflag.OutputFlag[orgAuditLogListRenderFunc]
}

// defaultOrgAuditLogListOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultOrgAuditLogListOutputFormat() outputflag.OutputFlag[orgAuditLogListRenderFunc] {
	return outputflag.OutputFlag[orgAuditLogListRenderFunc]{
		RenderForTerminal: renderOrgAuditLogListTable,
		RenderJSON:        renderOrgAuditLogListJSON,
	}
}

// newOrgAuditLogListCmd builds `pulumi org audit-log list` with the production
// client factory.
func newOrgAuditLogListCmd() *cobra.Command {
	return newOrgAuditLogListCmdWith(defaultOrgAuditLogListClientFactory)
}

func newOrgAuditLogListCmdWith(factory orgAuditLogListClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgAuditLogListClientFactory must not be nil")
	var args orgAuditLogListArgs
	args.outputFormat = defaultOrgAuditLogListOutputFormat()

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List audit log events for an organization",
		Long: "[EXPERIMENTAL] List audit log events for an organization.\n" +
			"\n" +
			"Returns audit log events for the organization. Results may be filtered\n" +
			"by event type and by the user that triggered the event. Use\n" +
			"--start-time to bound the upper end of the time range.\n" +
			"\n" +
			"Default output is a human-readable table; pass --output=json for the\n" +
			"full response as a JSON envelope.",
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
	cmd.Flags().Int64Var(&args.count, "count", 0,
		"Maximum number of events to return. Defaults to the size of the first page; "+
			"larger values auto-paginate")
	cmd.Flags().BoolVar(&args.all, "all", false, "Return all matching events; mutually exclusive with --count")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

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
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	// Fetch the first page. Auto-paginate when --all is set or --count
	// exceeds the first page.
	first, err := c.ListAuditLogs(ctx, org, client.ListAuditLogsOptions{
		EventType: args.eventType,
		User:      args.user,
		StartTime: args.startTime,
	})
	if err != nil {
		return err
	}

	events := first.AuditLogEvents
	token := first.ContinuationToken
	want := args.count
	all := args.all

	for token != "" && (all || want > int64(len(events))) {
		next, err := c.ListAuditLogs(ctx, org, client.ListAuditLogsOptions{
			EventType:         args.eventType,
			User:              args.user,
			StartTime:         args.startTime,
			ContinuationToken: token,
		})
		if err != nil {
			return err
		}
		events = append(events, next.AuditLogEvents...)
		token = next.ContinuationToken
	}

	if !all && want > 0 && int64(len(events)) > want {
		events = events[:want]
	}

	return args.outputFormat.Get()(w, apitype.ListAuditLogEventsResponse{
		AuditLogEvents: events,
	})
}

type orgAuditLogListRenderFunc func(w io.Writer, resp apitype.ListAuditLogEventsResponse) error

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
	t.AppendHeader(table.Row{"TIMESTAMP", "USER", "EVENT", "DESCRIPTION", "SOURCE IP"})

	for _, ev := range resp.AuditLogEvents {
		user := ev.User.GitHubLogin
		if user == "" {
			user = "-"
		}
		event := ev.Event
		if event == "" {
			event = "-"
		}
		description := ev.Description
		if description == "" {
			description = "-"
		}
		sourceIP := ev.SourceIP
		if sourceIP == "" {
			sourceIP = "-"
		}
		t.AppendRow(table.Row{
			formatAuditLogTimestamp(ev.Timestamp),
			user,
			event,
			description,
			sourceIP,
		})
	}
	t.Render()
	return nil
}

// auditLogListEnvelope is the JSON shape emitted by
// `pulumi org audit-log list --output=json`.
type auditLogListEnvelope struct {
	Events []apitype.AuditLogEvent `json:"events"`
	Count  int                     `json:"count"`
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
		Events: events,
		Count:  len(events),
	})
}
