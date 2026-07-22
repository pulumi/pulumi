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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackHistoryEventsClient is the interface iterateEngineEvents needs from the
// API client. Defined locally so tests can drive the iterator with a fake
// without pulling in the full *client.Client surface.
type stackHistoryEventsClient interface {
	GetUpdateEngineEvents(
		ctx context.Context,
		update client.UpdateIdentifier,
		opts client.GetUpdateEngineEventsOptions,
	) (apitype.GetUpdateEventsResponse, error)
}

type historyEventsRender func(
	w io.Writer, events iter.Seq2[apitype.EngineEvent, error],
) error

type stackHistoryEventsArgs struct {
	updateID            string
	eventTypes          []string
	resourceURN         string
	includeNonActivated bool
	count               int
	all                 bool
	render              historyEventsRender
}

func newStackHistoryEventsCmd(
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
) *cobra.Command {
	var (
		stack string
		args  = stackHistoryEventsArgs{}
	)

	outputFormat := outputflag.OutputFlag[historyEventsRender]{
		RenderForTerminal: renderHistoryEventsTable,
		RenderJSON:        renderHistoryEventsJSON,
	}

	cmd := &cobra.Command{
		Use:   "events",
		Short: "[EXPERIMENTAL] Retrieve engine events for an update",
		Long: "[EXPERIMENTAL] Retrieve engine events for a specific update of a stack.\n" +
			"\n" +
			"Engine events represent individual resource operations and diagnostic\n" +
			"messages produced during an update. By default a single page of\n" +
			"events is returned.\n" +
			"\n" +
			"This command requires the Pulumi Cloud backend.",
		Example: "  # Show the first page of events in a human-readable table.\n" +
			"  pulumi stack history events <update-id>\n\n" +
			"  # Return at least 500 events.\n" +
			"  pulumi stack history events <update-id> --count 500\n\n" +
			"  # Return every event for the update.\n" +
			"  pulumi stack history events <update-id> --all\n\n" +
			"  # Emit the raw event stream as JSON for scripting.\n" +
			"  pulumi stack history events <update-id> --all --output json\n\n" +
			"  # Filter to a specific resource URN.\n" +
			"  pulumi stack history events <update-id> \\\n" +
			"      --urn urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket",
		RunE: func(cmd *cobra.Command, positional []string) error {
			args.updateID = positional[0]
			args.render = outputFormat.Get()
			sink := diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
				Color: cmdutil.GetGlobalColorization(),
			})
			return runStackHistoryEvents(cmd.Context(), cmd.OutOrStdout(), sink, ws, lm, stack, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "update-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringArrayVar(&args.eventTypes, "event-type", nil,
		"Filter by Pulumi Cloud engine event type code; numeric, repeatable")
	cmd.Flags().StringVar(&args.resourceURN, "urn", "", "Filter by resource URN")
	cmd.Flags().BoolVar(&args.includeNonActivated, "include-non-activated", false,
		"Include events not yet marked as activated")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"Return at least this many events, fetching additional pages as needed")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Return every event for the update")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.Var(cmd.Flags(), &outputFormat)

	return cmd
}

func runStackHistoryEvents(
	ctx context.Context,
	w io.Writer,
	sink diag.Sink,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	stackFlag string,
	args stackHistoryEventsArgs,
) error {
	c, stackID, err := RequireCloudStack(ctx, sink, ws, lm, stackFlag)
	if err != nil {
		return err
	}

	update := client.UpdateIdentifier{
		StackIdentifier: stackID,
		UpdateKind:      apitype.UpdateUpdate,
		UpdateID:        args.updateID,
	}

	opts := client.GetUpdateEngineEventsOptions{
		EventTypes:          args.eventTypes,
		URN:                 args.resourceURN,
		IncludeNonActivated: args.includeNonActivated,
	}

	events := iterateEngineEvents(ctx, c, update, opts, args.all, args.count)
	return args.render(w, events)
}

// iterateEngineEvents returns an iterator over engine events. The pagination
// behavior depends on (all, count):
//   - all=true: follow continuation tokens until the cloud reports no more.
//   - count>0: keep fetching pages until at least count events have been yielded
//     or the cloud reports no more.
//   - otherwise: fetch exactly one page.
func iterateEngineEvents(
	ctx context.Context,
	c stackHistoryEventsClient,
	update client.UpdateIdentifier,
	opts client.GetUpdateEngineEventsOptions,
	all bool,
	count int,
) iter.Seq2[apitype.EngineEvent, error] {
	return func(yield func(apitype.EngineEvent, error) bool) {
		page := opts
		yielded := 0
		for {
			resp, err := c.GetUpdateEngineEvents(ctx, update, page)
			if err != nil {
				yield(apitype.EngineEvent{}, fmt.Errorf("getting update engine events: %w", err))
				return
			}
			for _, ev := range resp.Events {
				if !yield(ev, nil) {
					return
				}
				yielded++
			}
			done := resp.ContinuationToken == nil ||
				(!all && count <= 0) ||
				(count > 0 && yielded >= count)
			if done {
				return
			}
			page.ContinuationToken = resp.ContinuationToken
		}
	}
}

// historyEventJSON wraps apitype.EngineEvent for JSON output, hiding the
// embedded Sequence field. The cloud's GET .../events endpoint does not
// populate sequence (the docs are explicit: "Events are returned sorted by
// their internal sequence number (not exposed to the API)"), so emitting it
// would surface a misleading constant 0.
//
// The outer Sequence field shadows the embedded one (shallower fields win
// during Go's JSON conflict resolution) and `omitempty` plus its zero value
// makes the encoder drop the key entirely. `json:"-"` would not work here
// because it removes the outer field from consideration, leaving the
// embedded field as the only candidate.
type historyEventJSON struct {
	apitype.EngineEvent
	Sequence int `json:"sequence,omitempty"`
}

// renderHistoryEventsJSON streams events as an indented JSON array so
// consumers see output as it arrives rather than having to wait for the
// last page.
func renderHistoryEventsJSON(
	w io.Writer, events iter.Seq2[apitype.EngineEvent, error],
) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if _, err := bw.WriteString("[\n"); err != nil {
		return err
	}
	first := true
	for ev, err := range events {
		if err != nil {
			return err
		}
		if !first {
			if _, err := bw.WriteString(",\n"); err != nil {
				return err
			}
		}
		first = false
		b, err := json.MarshalIndent(historyEventJSON{EngineEvent: ev}, "  ", "  ")
		if err != nil {
			return err
		}
		if _, err := bw.WriteString("  "); err != nil {
			return err
		}
		if _, err := bw.Write(b); err != nil {
			return err
		}
		// Flush after each event so streamed consumers see incremental output.
		if err := bw.Flush(); err != nil {
			return err
		}
	}
	if !first {
		if _, err := bw.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString("]\n"); err != nil {
		return err
	}
	return nil
}

const historyEventsFixedColsWidth = 50

// renderHistoryEventsTable buffers all events before rendering because the
// box-style table needs to know its full contents to draw consistent
// borders. Use --output json (with --all or --count) to stream very large
// updates.
func renderHistoryEventsTable(
	w io.Writer, events iter.Seq2[apitype.EngineEvent, error],
) error {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	// The cloud GET endpoint does not expose the internal sequence number, so
	// we omit the column entirely rather than rendering a constant 0.
	t.AppendHeader(table.Row{"TIMESTAMP", "TYPE", "DETAILS"})

	count := 0
	for ev, err := range events {
		if err != nil {
			return err
		}
		ts := ""
		if ev.Timestamp > 0 {
			ts = time.Unix(int64(ev.Timestamp), 0).UTC().Format(time.RFC3339)
		}
		kind, details := describeEngineEvent(ev)
		t.AppendRow(table.Row{ts, kind, details})
		count++
	}

	if count == 0 {
		fmt.Fprintln(w, "No events found for this update.")
		return nil
	}

	cols := cmdCmd.WriterWidth(w)
	// Borders and separators take ~3 chars per column plus 1 outer border each side.
	borderWidth := 3*3 + 1
	detailsWidth := max(cols-borderWidth-historyEventsFixedColsWidth, 20)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "DETAILS", WidthMax: detailsWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(w, "\n%d event(s)\n", count)
	return nil
}

// describeEngineEvent returns a short (kind, details) summary for an engine event.
// It is intentionally lossy; callers wanting full fidelity should use --output=json.
//
// Diagnostic messages contain Pulumi color directives (e.g. "<{%reset%}>") which
// would otherwise render as literal text in the table. We strip them via the
// Never colorization mode.
func describeEngineEvent(ev apitype.EngineEvent) (string, string) {
	switch {
	case ev.CancelEvent != nil:
		return "cancel", ""
	case ev.StdoutEvent != nil:
		return "stdout", strings.TrimRight(plain(ev.StdoutEvent.Message), "\n")
	case ev.DiagnosticEvent != nil:
		d := ev.DiagnosticEvent
		details := strings.TrimRight(plain(d.Message), "\n")
		if d.URN != "" {
			details = d.URN + ": " + details
		}
		if d.Severity != "" {
			return "diagnostic/" + d.Severity, details
		}
		return "diagnostic", details
	case ev.PreludeEvent != nil:
		return "prelude", ""
	case ev.SummaryEvent != nil:
		s := ev.SummaryEvent
		result := string(s.Result)
		if result == "" {
			// Older updates may not include Result on the summary event.
			result = "unknown"
		}
		return "summary", fmt.Sprintf("result=%s duration=%ds", result, s.DurationSeconds)
	case ev.ResourcePreEvent != nil:
		m := ev.ResourcePreEvent.Metadata
		return "resource-pre", fmt.Sprintf("%s %s", m.Op, m.URN)
	case ev.ResOutputsEvent != nil:
		m := ev.ResOutputsEvent.Metadata
		return "resource-outputs", fmt.Sprintf("%s %s", m.Op, m.URN)
	case ev.ResOpFailedEvent != nil:
		m := ev.ResOpFailedEvent.Metadata
		return "resource-op-failed", fmt.Sprintf("%s %s", m.Op, m.URN)
	case ev.PolicyEvent != nil:
		p := ev.PolicyEvent
		return "policy", fmt.Sprintf("%s/%s: %s", p.PolicyPackName, p.PolicyName, plain(p.Message))
	case ev.PolicyRemediationEvent != nil:
		p := ev.PolicyRemediationEvent
		return "policy-remediation", fmt.Sprintf("%s/%s on %s", p.PolicyPackName, p.PolicyName, p.ResourceURN)
	case ev.PolicyLoadEvent != nil:
		return "policy-load", ""
	case ev.PolicyAnalyzeSummaryEvent != nil:
		return "policy-analyze-summary", ev.PolicyAnalyzeSummaryEvent.PolicyPackName
	case ev.PolicyRemediateSummaryEvent != nil:
		return "policy-remediate-summary", ev.PolicyRemediateSummaryEvent.PolicyPackName
	case ev.PolicyAnalyzeStackSummaryEvent != nil:
		return "policy-analyze-stack-summary", ev.PolicyAnalyzeStackSummaryEvent.PolicyPackName
	case ev.StartDebuggingEvent != nil:
		return "start-debugging", ""
	case ev.ProgressEvent != nil:
		p := ev.ProgressEvent
		return "progress", fmt.Sprintf("%s %s (%d/%d)", p.Type, plain(p.Message), p.Completed, p.Total)
	case ev.ErrorEvent != nil:
		return "error", plain(ev.ErrorEvent.Error)
	default:
		return "unknown", ""
	}
}

func plain(s string) string {
	return colors.Never.Colorize(s)
}
