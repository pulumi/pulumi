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
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
)

// updateSummary is the document emitted by `pulumi stack history events
// --summary`: the base shape of the live `pulumi up --output json` summary,
// extended with error diagnostics and per-resource failure markers.
type updateSummary struct {
	display.SummaryJSON

	// Shadows the embedded field (shallower fields win during Go's JSON
	// conflict resolution) so entries can carry Failed.
	Resources []summaryResource `json:"resources,omitempty"`

	Diagnostics []diagnosticSummary `json:"diagnostics,omitempty"`
}

type summaryResource struct {
	display.ResourceJSON

	Failed bool `json:"failed,omitempty"`
}

type diagnosticSummary struct {
	Severity string `json:"severity"`
	URN      string `json:"urn,omitempty"`
	Message  string `json:"message"`
}

// buildUpdateSummary reduces an engine event stream to an updateSummary,
// mirroring the live summary tap (display.tapSummaryJSON): one resource entry
// per attempted operation, unchanged (`same`) resources omitted. When
// includeDiff is set, each entry carries its property diff too.
//
// Events arrive in their API shape and are converted back to engine events up
// front so the rest of the reduction can share the display package's helpers.
func buildUpdateSummary(
	events iter.Seq2[apitype.EngineEvent, error], includeDiff bool,
) (*updateSummary, error) {
	s := &updateSummary{}

	var startTs, endTs int
	var summaryEvent *engine.SummaryEventPayload
	anyFailed := false
	anyError := false

	markFailed := func(m engine.StepEventMetadata) {
		for i := len(s.Resources) - 1; i >= 0; i-- {
			if s.Resources[i].URN == string(m.URN) {
				s.Resources[i].Failed = true
				return
			}
		}
		s.Resources = append(s.Resources, summaryResource{ResourceJSON: display.NewResourceJSON(&m), Failed: true})
	}

	for ev, err := range events {
		if err != nil {
			return nil, err
		}
		if ev.Timestamp > 0 {
			if startTs == 0 || ev.Timestamp < startTs {
				startTs = ev.Timestamp
			}
			if ev.Timestamp > endTs {
				endTs = ev.Timestamp
			}
		}
		e, err := display.ConvertJSONEvent(ev)
		if err != nil {
			// An event kind this CLI doesn't know about; skip it, as the switch
			// below would anyway.
			continue
		}
		switch e.Type { //nolint:exhaustive // we only care about a few event types here
		case engine.SummaryEvent:
			p := e.Payload().(engine.SummaryEventPayload)
			summaryEvent = &p
		case engine.ResourcePreEvent:
			if m := e.Payload().(engine.ResourcePreEventPayload).Metadata; m.Op != deploy.OpSame {
				r := summaryResource{ResourceJSON: display.NewResourceJSON(&m)}
				if includeDiff {
					r.Diff = display.NewDiffJSON(&m, false /* refresh */, false /* showSecrets */)
				}
				s.Resources = append(s.Resources, r)
			}
		case engine.ResourceOutputsEvent:
			// Refresh steps only reveal their diff once the provider has read the
			// resource's current state, which arrives on the outputs event.
			if !includeDiff {
				continue
			}
			m := e.Payload().(engine.ResourceOutputsEventPayload).Metadata
			d := display.RefreshDiffJSON(&m, false /* showSecrets */)
			if d == nil {
				continue
			}
			for i := len(s.Resources) - 1; i >= 0; i-- {
				if s.Resources[i].URN == string(m.URN) && s.Resources[i].Op == apitype.OpRefresh {
					s.Resources[i].Diff = d
					break
				}
			}
		case engine.ResourceOperationFailed:
			anyFailed = true
			markFailed(e.Payload().(engine.ResourceOperationFailedPayload).Metadata)
		case engine.DiagEvent:
			d := e.Payload().(engine.DiagEventPayload)
			// A failing program reports through the language host's stderr, which
			// arrives as "info#err" rather than "error" — for a preview that never
			// reached a resource operation it is the only record of what went wrong.
			// It doesn't imply failure on its own (a program can write to stderr and
			// succeed), so only "error" contributes to the result below.
			if d.Ephemeral || (d.Severity != diag.Error && d.Severity != diag.Infoerr) {
				continue
			}
			anyError = anyError || d.Severity == diag.Error
			s.Diagnostics = append(s.Diagnostics, diagnosticSummary{
				Severity: string(d.Severity),
				URN:      string(d.URN),
				Message:  strings.TrimRight(plain(d.Message), "\n"),
			})
		}
	}

	if summaryEvent != nil {
		s.Summary = summaryEvent.ResourceChanges
	}

	switch {
	case summaryEvent != nil && summaryEvent.Result != "":
		s.Result = summaryEvent.Result
	case anyFailed || anyError:
		s.Result = apitype.OperationResultFailed
	case summaryEvent != nil:
		s.Result = apitype.OperationResultSucceeded
	default:
		// Older or interrupted updates may carry no summary event at all.
		s.Result = "unknown"
	}

	switch {
	case summaryEvent != nil && summaryEvent.Duration > 0:
		s.Duration = summaryEvent.Duration
	case endTs > startTs:
		s.Duration = time.Duration(endTs-startTs) * time.Second
	}

	return s, nil
}

type summaryRender func(w io.Writer, s *updateSummary) error

// renderUpdateSummaryJSON emits a single line, like the live summary.
func renderUpdateSummaryJSON(w io.Writer, s *updateSummary) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(s)
}

func renderUpdateSummaryText(w io.Writer, s *updateSummary) error {
	fmt.Fprintf(w, "Result:   %s\n", s.Result)
	if s.Duration > 0 {
		fmt.Fprintf(w, "Duration: %s\n", s.Duration)
	}

	parts := make([]string, 0, len(s.Summary))
	for _, op := range slices.Sorted(maps.Keys(s.Summary)) {
		parts = append(parts, fmt.Sprintf("%d %s", s.Summary[op], op))
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "Changes:  %s\n", strings.Join(parts, ", "))
	}

	if len(s.Resources) > 0 {
		fmt.Fprintln(w)
		t := table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"NAME", "TYPE", "OP"})
		for _, r := range s.Resources {
			op := string(r.Op)
			if r.Failed {
				op += " (failed)"
			}
			t.AppendRow(table.Row{r.Name, r.Type, op})
		}
		t.Render()
	}

	diffed := false
	for _, r := range s.Resources {
		if len(r.Diff) == 0 {
			continue
		}
		if !diffed {
			fmt.Fprintln(w, "\nDiffs:")
			diffed = true
		}
		fmt.Fprintf(w, "  %s (%s)\n", r.Name, r.Type)
		for _, path := range slices.Sorted(maps.Keys(r.Diff)) {
			d := r.Diff[path]
			switch d.Kind {
			case "add":
				fmt.Fprintf(w, "    + %s: %s\n", path, compactJSON(d.New))
			case "delete":
				fmt.Fprintf(w, "    - %s: %s\n", path, compactJSON(d.Old))
			default:
				fmt.Fprintf(w, "    ~ %s: %s => %s\n", path, compactJSON(d.Old), compactJSON(d.New))
			}
		}
	}

	if len(s.Diagnostics) > 0 {
		fmt.Fprintln(w, "\nDiagnostics:")
		for _, d := range s.Diagnostics {
			if d.URN != "" {
				fmt.Fprintf(w, "  %s: %s: %s\n", d.Severity, d.URN, d.Message)
			} else {
				fmt.Fprintf(w, "  %s: %s\n", d.Severity, d.Message)
			}
		}
	}

	return nil
}

// compactJSON renders a diff value on one line for the text view.
func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
