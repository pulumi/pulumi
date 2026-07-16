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
	pkgdisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// updateSummary is the document emitted by `pulumi stack history events
// --summary`. It embeds the base shape of the live `pulumi up --output json`
// summary (display.SummaryJSON) so tooling can treat live runs and historical
// lookups the same way, and extends it with the two things a post-hoc
// debugging pass needs that the live summary does not carry: the update's
// error diagnostics and per-resource failure markers.
type updateSummary struct {
	display.SummaryJSON

	// Resources shadows the embedded field (shallower fields win during Go's
	// JSON conflict resolution) so each entry can carry the failure marker
	// while remaining parseable as a display.ResourceJSON.
	Resources []summaryResource `json:"resources,omitempty"`

	// Diagnostics holds the update's error messages. Errors not tied to any
	// resource (program exceptions, provider configuration failures) have no
	// URN, which is why the list lives at the top level rather than on
	// resource entries.
	Diagnostics []diagnosticSummary `json:"diagnostics,omitempty"`
}

type summaryResource struct {
	display.ResourceJSON

	// Failed is set when an operation on this resource failed; the error
	// messages appear in updateSummary.Diagnostics under the same URN.
	Failed bool `json:"failed,omitempty"`
}

type diagnosticSummary struct {
	Severity string `json:"severity"`
	URN      string `json:"urn,omitempty"`
	Message  string `json:"message"`
}

func resourceName(urn string) string {
	if i := strings.LastIndex(urn, "::"); i >= 0 {
		return urn[i+2:]
	}
	return urn
}

func summaryResourceFor(m apitype.StepEventMetadata) summaryResource {
	// Parent lives on the post-step state when there is one, and falls back
	// to the pre-step state for deletes (where New is nil), mirroring the
	// live summary's resourceJSONFromEvent.
	var parent string
	switch {
	case m.New != nil:
		parent = m.New.Parent
	case m.Old != nil:
		parent = m.Old.Parent
	}
	return summaryResource{ResourceJSON: display.ResourceJSON{
		URN:    m.URN,
		Type:   m.Type,
		Name:   resourceName(m.URN),
		Op:     m.Op,
		Parent: parent,
	}}
}

// buildUpdateSummary reduces an engine event stream to an updateSummary. The
// base fields mirror the live summary tap (display.tapSummaryJSON): one
// resource entry per attempted operation from resource pre-events, with
// unchanged (`same`) resources omitted.
func buildUpdateSummary(events iter.Seq2[apitype.EngineEvent, error]) (*updateSummary, error) {
	s := &updateSummary{}

	var startTs, endTs int
	var summaryEvent *apitype.SummaryEvent
	anyFailed := false

	// markFailed flags the most recent entry for a URN, appending one when
	// the failure is the only event we saw for that resource.
	markFailed := func(m apitype.StepEventMetadata) {
		for i := len(s.Resources) - 1; i >= 0; i-- {
			if s.Resources[i].URN == m.URN {
				s.Resources[i].Failed = true
				return
			}
		}
		r := summaryResourceFor(m)
		r.Failed = true
		s.Resources = append(s.Resources, r)
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
		switch {
		case ev.SummaryEvent != nil:
			summaryEvent = ev.SummaryEvent
		case ev.ResourcePreEvent != nil:
			if m := ev.ResourcePreEvent.Metadata; m.Op != apitype.OpSame {
				s.Resources = append(s.Resources, summaryResourceFor(m))
			}
		case ev.ResOpFailedEvent != nil:
			anyFailed = true
			markFailed(ev.ResOpFailedEvent.Metadata)
		case ev.DiagnosticEvent != nil:
			d := ev.DiagnosticEvent
			if d.Severity != "error" || d.Ephemeral {
				continue
			}
			s.Diagnostics = append(s.Diagnostics, diagnosticSummary{
				Severity: d.Severity,
				URN:      d.URN,
				Message:  strings.TrimRight(plain(d.Message), "\n"),
			})
		}
	}

	if summaryEvent != nil {
		s.Summary = make(pkgdisplay.ResourceChanges, len(summaryEvent.ResourceChanges))
		for op, n := range summaryEvent.ResourceChanges {
			s.Summary[pkgdisplay.StepOp(op)] = n
		}
	}

	switch {
	case summaryEvent != nil && summaryEvent.Result != "":
		s.Result = summaryEvent.Result
	case anyFailed || len(s.Diagnostics) > 0:
		s.Result = apitype.OperationResultFailed
	case summaryEvent != nil:
		s.Result = apitype.OperationResultSucceeded
	default:
		// Older or interrupted updates may carry no summary event at all.
		s.Result = "unknown"
	}

	switch {
	case summaryEvent != nil && summaryEvent.DurationSeconds > 0:
		s.Duration = time.Duration(summaryEvent.DurationSeconds) * time.Second
	case endTs > startTs:
		s.Duration = time.Duration(endTs-startTs) * time.Second
	}

	return s, nil
}

type summaryRender func(w io.Writer, s *updateSummary) error

// renderUpdateSummaryJSON emits the summary as a single line, matching the
// live `pulumi up --output json` output format.
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
