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
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// updateSummary is a resource-centric digest of an update's engine events. It
// deliberately avoids engine vocabulary (steps, ops, diff kinds): the target
// consumer is a person — or an agent — diagnosing an update after the fact,
// without access to the original CLI output.
type updateSummary struct {
	Version          int                      `json:"version"`
	Operation        string                   `json:"operation,omitempty"`
	Status           string                   `json:"status"`
	Stack            string                   `json:"stack"`
	StartedAt        string                   `json:"startedAt,omitempty"`
	CompletedAt      string                   `json:"completedAt,omitempty"`
	DurationMs       int64                    `json:"durationMs,omitempty"`
	Summary          map[string]int           `json:"summary"`
	Resources        []resourceSummary        `json:"resources"`
	Outputs          map[string]any           `json:"outputs,omitempty"`
	Diagnostics      []diagnosticSummary      `json:"diagnostics,omitempty"`
	PolicyViolations []policyViolationSummary `json:"policyViolations,omitempty"`
}

// resourceSummary describes what happened to a single resource. Resources
// whose change is "unchanged" are counted in updateSummary.Summary but
// excluded from updateSummary.Resources to keep the document focused on
// what actually changed.
type resourceSummary struct {
	URN             string                    `json:"urn"`
	Name            string                    `json:"name"`
	Type            string                    `json:"type"`
	Change          string                    `json:"change"`
	PropertyChanges map[string]propertyChange `json:"propertyChanges,omitempty"`
}

type propertyChange struct {
	Kind string `json:"kind"`
}

type diagnosticSummary struct {
	Severity string `json:"severity"`
	URN      string `json:"urn,omitempty"`
	Message  string `json:"message"`
}

type policyViolationSummary struct {
	PolicyPack       string `json:"policyPack"`
	Policy           string `json:"policy"`
	URN              string `json:"urn,omitempty"`
	EnforcementLevel string `json:"enforcementLevel"`
	Severity         string `json:"severity,omitempty"`
	Message          string `json:"message"`
}

// changeForOp translates an engine operation into the summary's
// consumer-facing change vocabulary.
func changeForOp(op apitype.OpType) string {
	switch op {
	case apitype.OpCreate:
		return "created"
	case apitype.OpUpdate, apitype.OpRefresh:
		return "updated"
	case apitype.OpSame:
		return "unchanged"
	case apitype.OpReplace, apitype.OpCreateReplacement, apitype.OpDeleteReplaced,
		apitype.OpReadReplacement, apitype.OpDiscardReplaced, apitype.OpImportReplacement:
		return "replaced"
	case apitype.OpDelete, apitype.OpReadDiscard, apitype.OpRemovePendingReplace:
		return "deleted"
	case apitype.OpRead:
		return "read"
	case apitype.OpImport:
		return "imported"
	default:
		return string(op)
	}
}

func kindForDiff(k apitype.DiffKind) string {
	switch k {
	case apitype.DiffAdd, apitype.DiffAddReplace:
		return "added"
	case apitype.DiffDelete, apitype.DiffDeleteReplace:
		return "deleted"
	case apitype.DiffUpdate, apitype.DiffUpdateReplace:
		return "updated"
	default:
		return string(k)
	}
}

// propertyChangesFor extracts per-property changes from a step's metadata,
// preferring the detailed diff and falling back to the plain list of changed
// keys when the detailed diff is absent.
func propertyChangesFor(m apitype.StepEventMetadata) map[string]propertyChange {
	if len(m.DetailedDiff) > 0 {
		out := make(map[string]propertyChange, len(m.DetailedDiff))
		for path, d := range m.DetailedDiff {
			out[path] = propertyChange{Kind: kindForDiff(d.Kind)}
		}
		return out
	}
	if len(m.Diffs) > 0 {
		out := make(map[string]propertyChange, len(m.Diffs))
		for _, k := range m.Diffs {
			out[k] = propertyChange{Kind: "updated"}
		}
		return out
	}
	return nil
}

func resourceName(urn string) string {
	if i := strings.LastIndex(urn, "::"); i >= 0 {
		return urn[i+2:]
	}
	return urn
}

// buildUpdateSummary reduces an engine event stream to an updateSummary.
func buildUpdateSummary(
	stack string, events iter.Seq2[apitype.EngineEvent, error],
) (*updateSummary, error) {
	s := &updateSummary{
		Version:   1,
		Stack:     stack,
		Summary:   map[string]int{},
		Resources: []resourceSummary{},
	}

	var startTs, endTs int
	var summaryEvent *apitype.SummaryEvent
	byURN := map[string]*resourceSummary{}
	var order []string

	// record folds a step into the per-resource map. A URN can appear more
	// than once (a replacement is a create-replacement plus a delete-replaced
	// step); "failed" always wins, otherwise the latest step's change wins and
	// property changes accumulate.
	record := func(m apitype.StepEventMetadata, change string) {
		r, ok := byURN[m.URN]
		if !ok {
			r = &resourceSummary{URN: m.URN, Name: resourceName(m.URN), Type: m.Type}
			byURN[m.URN] = r
			order = append(order, m.URN)
		}
		if r.Change != "failed" {
			r.Change = change
		}
		for path, pc := range propertyChangesFor(m) {
			if r.PropertyChanges == nil {
				r.PropertyChanges = map[string]propertyChange{}
			}
			r.PropertyChanges[path] = pc
		}
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
		case ev.ResOutputsEvent != nil:
			m := ev.ResOutputsEvent.Metadata
			record(m, changeForOp(m.Op))
			if m.Type == "pulumi:pulumi:Stack" && m.New != nil && len(m.New.Outputs) > 0 {
				s.Outputs = m.New.Outputs
			}
		case ev.ResOpFailedEvent != nil:
			record(ev.ResOpFailedEvent.Metadata, "failed")
		case ev.DiagnosticEvent != nil:
			d := ev.DiagnosticEvent
			if d.Ephemeral || d.Severity == "debug" {
				continue
			}
			s.Diagnostics = append(s.Diagnostics, diagnosticSummary{
				Severity: d.Severity,
				URN:      d.URN,
				Message:  strings.TrimRight(plain(d.Message), "\n"),
			})
		case ev.PolicyEvent != nil:
			p := ev.PolicyEvent
			s.PolicyViolations = append(s.PolicyViolations, policyViolationSummary{
				PolicyPack:       p.PolicyPackName,
				Policy:           p.PolicyName,
				URN:              p.ResourceURN,
				EnforcementLevel: p.EnforcementLevel,
				Severity:         p.Severity,
				Message:          strings.TrimRight(plain(p.Message), "\n"),
			})
		}
	}

	for _, urn := range order {
		r := byURN[urn]
		s.Summary[r.Change]++
		if r.Change != "unchanged" {
			s.Resources = append(s.Resources, *r)
		}
	}

	hasErrorDiag := false
	for _, d := range s.Diagnostics {
		if d.Severity == "error" {
			hasErrorDiag = true
			break
		}
	}
	switch {
	case summaryEvent != nil && summaryEvent.Result != "":
		s.Status = string(summaryEvent.Result)
	case s.Summary["failed"] > 0 || hasErrorDiag:
		s.Status = "failed"
	case summaryEvent != nil:
		s.Status = "succeeded"
	default:
		s.Status = "unknown"
	}

	// The engine events do not record which operation produced them beyond
	// the preview bit, so operation is omitted unless it is determinable.
	if summaryEvent != nil && summaryEvent.IsPreview {
		s.Operation = "preview"
	}

	if startTs > 0 {
		s.StartedAt = time.Unix(int64(startTs), 0).UTC().Format(time.RFC3339)
		s.CompletedAt = time.Unix(int64(endTs), 0).UTC().Format(time.RFC3339)
	}
	switch {
	case summaryEvent != nil && summaryEvent.DurationSeconds > 0:
		s.DurationMs = int64(summaryEvent.DurationSeconds) * 1000
	case endTs > startTs:
		s.DurationMs = int64(endTs-startTs) * 1000
	}

	return s, nil
}

type summaryRender func(w io.Writer, s *updateSummary) error

func renderUpdateSummaryJSON(w io.Writer, s *updateSummary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func renderUpdateSummaryText(w io.Writer, s *updateSummary) error {
	fmt.Fprintf(w, "Stack:    %s\n", s.Stack)
	fmt.Fprintf(w, "Status:   %s\n", s.Status)
	if s.Operation != "" {
		fmt.Fprintf(w, "Operation: %s\n", s.Operation)
	}
	if s.DurationMs > 0 {
		fmt.Fprintf(w, "Duration: %s\n", time.Duration(s.DurationMs)*time.Millisecond)
	}

	changes := make([]string, 0, len(s.Summary))
	for change := range s.Summary {
		changes = append(changes, change)
	}
	sort.Strings(changes)
	parts := make([]string, 0, len(changes))
	for _, change := range changes {
		parts = append(parts, fmt.Sprintf("%d %s", s.Summary[change], change))
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "Changes:  %s\n", strings.Join(parts, ", "))
	}

	if len(s.Resources) > 0 {
		fmt.Fprintln(w)
		t := table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"RESOURCE", "TYPE", "CHANGE", "PROPERTIES"})
		for _, r := range s.Resources {
			props := make([]string, 0, len(r.PropertyChanges))
			for path := range r.PropertyChanges {
				props = append(props, path)
			}
			sort.Strings(props)
			for i, path := range props {
				props[i] = fmt.Sprintf("%s (%s)", path, r.PropertyChanges[path].Kind)
			}
			t.AppendRow(table.Row{r.Name, r.Type, r.Change, strings.Join(props, ", ")})
		}
		t.SetColumnConfigs([]table.ColumnConfig{
			{Name: "PROPERTIES", WidthMax: 40, WidthMaxEnforcer: text.WrapText},
		})
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

	if len(s.PolicyViolations) > 0 {
		fmt.Fprintln(w, "\nPolicy violations:")
		for _, p := range s.PolicyViolations {
			fmt.Fprintf(w, "  %s/%s (%s): %s\n", p.PolicyPack, p.Policy, p.EnforcementLevel, p.Message)
		}
	}

	return nil
}
