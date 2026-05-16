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
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// orgAuditLogExportClient is the narrow subset of cloud-API operations the
// export command needs.
type orgAuditLogExportClient interface {
	ExportAuditLogs(
		ctx context.Context, orgName string, opts client.ExportAuditLogsOptions,
	) (io.ReadCloser, error)
}

// orgAuditLogExportClientFactory resolves a cloud client and the organization
// to export audit logs from. orgFlag carries the raw value of `--org`
// (empty means "use the default org").
type orgAuditLogExportClientFactory func(
	ctx context.Context, orgFlag string,
) (orgAuditLogExportClient, string, error)

// orgAuditLogExportArgs collects the flag values for the export command, in
// one struct so Run can be driven directly from tests.
type orgAuditLogExportArgs struct {
	org          string
	format       string
	eventType    string
	user         string
	startTime    string
	count        int64
	outputFormat outputflag.OutputFlag[orgAuditLogExportRenderFunc]
}

// orgAuditLogExportRenderFunc renders the accumulated export bytes (and the
// requested wire format) to the writer.
type orgAuditLogExportRenderFunc func(w io.Writer, data []byte, format string) error

// defaultOrgAuditLogExportOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultOrgAuditLogExportOutputFormat() outputflag.OutputFlag[orgAuditLogExportRenderFunc] {
	return outputflag.OutputFlag[orgAuditLogExportRenderFunc]{
		RenderForTerminal: renderOrgAuditLogExportRaw,
		RenderJSON:        renderOrgAuditLogExportJSON,
	}
}

// newOrgAuditLogExportCmd builds `pulumi org audit-log export` with the
// production client factory.
func newOrgAuditLogExportCmd() *cobra.Command {
	return newOrgAuditLogExportCmdWith(defaultOrgAuditLogExportClientFactory)
}

func newOrgAuditLogExportCmdWith(factory orgAuditLogExportClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgAuditLogExportClientFactory must not be nil")
	var args orgAuditLogExportArgs
	args.outputFormat = defaultOrgAuditLogExportOutputFormat()

	cmd := &cobra.Command{
		Use:   "export",
		Short: "[EXPERIMENTAL] Export audit log events for an organization",
		Long: "[EXPERIMENTAL] Export audit log events for an organization.\n" +
			"\n" +
			"Streams an export of audit log events for the organization in the\n" +
			"requested format. Results may be filtered by event type and by the\n" +
			"user that triggered the event. Use --start-time to bound the upper\n" +
			"end of the time range.\n" +
			"\n" +
			"Default output writes the raw response body (CSV or CEF) verbatim;\n" +
			"pass --output=json to wrap the body in a JSON envelope with the\n" +
			"response format and base64-encoded data.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgAuditLogExport(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to export audit logs for")
	cmd.Flags().StringVar(&args.format, "format", "csv", "The export format. One of: csv, cef")
	cmd.Flags().StringVar(&args.eventType, "event-type", "", "Filter by event type")
	cmd.Flags().StringVar(&args.user, "user", "", "Filter by user login")
	cmd.Flags().StringVar(&args.startTime, "start-time", "",
		"The upper bound of the time range (V1 semantics)")
	cmd.Flags().Int64Var(&args.count, "count", 0,
		"Truncate the exported response to the given number of bytes (0 returns the full response)")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultOrgAuditLogExportClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultOrgAuditLogExportClientFactory(
	ctx context.Context, orgFlag string,
) (orgAuditLogExportClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"exporting audit logs requires the Pulumi Cloud backend; run `pulumi login`")
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

// runOrgAuditLogExport is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runOrgAuditLogExport(
	ctx context.Context, w io.Writer,
	factory orgAuditLogExportClientFactory, args orgAuditLogExportArgs,
) error {
	contract.Assertf(factory != nil, "orgAuditLogExportClientFactory must not be nil")

	format := args.format
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "cef" {
		return fmt.Errorf("invalid --format value %q (must be 'csv' or 'cef')", format)
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	body, err := c.ExportAuditLogs(ctx, org, client.ExportAuditLogsOptions{
		Format:    format,
		EventType: args.eventType,
		User:      args.user,
		StartTime: args.startTime,
	})
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(body)

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("exporting audit logs: %w", err)
	}

	// Bound output to --count bytes when --count is positive.
	if args.count > 0 && int64(len(data)) > args.count {
		data = data[:args.count]
	}

	return args.outputFormat.Get()(w, data, format)
}

// renderOrgAuditLogExportRaw writes the raw response body verbatim.
func renderOrgAuditLogExportRaw(w io.Writer, data []byte, _ string) error {
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("exporting audit logs: %w", err)
	}
	return nil
}

// renderOrgAuditLogExportJSON parses the export data and emits structured JSON.
// For CSV exports, each row becomes a JSON object keyed by the CSV header.
// For CEF exports, the raw lines are emitted as a string array.
func renderOrgAuditLogExportJSON(w io.Writer, data []byte, format string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if format == "csv" {
		return renderCSVAsJSON(w, data, enc)
	}
	// CEF or unknown: emit raw lines.
	lines := splitLines(data)
	return enc.Encode(struct {
		Format string   `json:"format"`
		Lines  []string `json:"lines"`
		Count  int      `json:"count"`
	}{
		Format: format,
		Lines:  lines,
		Count:  len(lines),
	})
}

func renderCSVAsJSON(w io.Writer, data []byte, enc *json.Encoder) error {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("parsing CSV export: %w", err)
	}
	if len(records) == 0 {
		return enc.Encode(struct {
			Events []map[string]string `json:"events"`
			Count  int                 `json:"count"`
		}{
			Events: []map[string]string{},
			Count:  0,
		})
	}

	headers := records[0]
	events := make([]map[string]string, 0, len(records)-1)
	for _, row := range records[1:] {
		event := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				event[h] = row[i]
			}
		}
		events = append(events, event)
	}

	return enc.Encode(struct {
		Events []map[string]string `json:"events"`
		Count  int                 `json:"count"`
	}{
		Events: events,
		Count:  len(events),
	})
}

func splitLines(data []byte) []string {
	s := string(bytes.TrimRight(data, "\n\r"))
	if s == "" {
		return []string{}
	}
	lines := bytes.Split([]byte(s), []byte("\n"))
	result := make([]string, len(lines))
	for i, l := range lines {
		result[i] = string(bytes.TrimRight(l, "\r"))
	}
	return result
}
