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

package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// summarySchemaVersion is the version of the EventSummary document this command emits. Bumped
// only on breaking schema changes — additive changes keep the same version.
const summarySchemaVersion = 1

// NewSummaryCmd builds the `pulumi events summary` command. It reads a JSONL engine event
// stream from stdin and emits a single JSON `display.EventSummary` object to stdout,
// capturing the same information a human sees in the CLI output of `pulumi preview`,
// `pulumi up`, `pulumi refresh`, or `pulumi destroy`.
//
// Hidden while the interface is being developed.
func NewSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarise an engine event stream as JSON",
		Long: "Summarise an engine event stream as JSON.\n" +
			"\n" +
			"Reads an engine event stream on stdin and writes a single structured JSON\n" +
			"document on stdout, mirroring the information a human sees in the output of\n" +
			"`pulumi preview`, `pulumi up`, `pulumi refresh`, or `pulumi destroy`:\n" +
			"configuration, per-resource steps, stack outputs, resource change counts,\n" +
			"duration, diagnostics, and policy results.\n",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSummary(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

// runSummary reads JSONL engine events from in, reduces them into a single EventSummary, and
// writes the result as a pretty-printed JSON object to out. Decoding errors surface to the
// caller so that malformed input fails fast.
func runSummary(in io.Reader, out io.Writer) error {
	summary, err := reduceEvents(in)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		return fmt.Errorf("encoding summary: %w", err)
	}
	return nil
}

// reduceEvents folds a JSONL engine event stream into an EventSummary. The reduction preserves
// arrival order for lists (diagnostics, policy violations) and picks the latest observed entry
// per URN for steps. Failure modes (ResOpFailedEvent, ErrorEvent) are surfaced on the summary's
// `Failed` / `Error` fields; they do not abort reduction — a partial stream still yields a
// useful document.
func reduceEvents(in io.Reader) (*display.EventSummary, error) {
	summary := &display.EventSummary{Version: summarySchemaVersion}

	// stepOrder preserves the order URNs first appear in the stream; stepByURN holds the
	// current snapshot for each. At the end we flatten stepByURN in stepOrder into Steps.
	stepOrder := []resource.URN{}
	stepByURN := map[resource.URN]*display.SummaryStep{}

	dec := json.NewDecoder(in)
	first := true
	for {
		var evt apitype.EngineEvent
		if err := dec.Decode(&evt); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding event: %w", err)
		}

		if first {
			summary.StartTime = evt.Timestamp
			first = false
		}
		if evt.Timestamp != 0 {
			summary.EndTime = evt.Timestamp
		}

		switch {
		case evt.PreludeEvent != nil:
			summary.Config = evt.PreludeEvent.Config

		case evt.ResourcePreEvent != nil:
			applyResourceStep(stepByURN, &stepOrder, evt.ResourcePreEvent.Metadata, false)
			captureStackOutputs(summary, evt.ResourcePreEvent.Metadata)

		case evt.ResOutputsEvent != nil:
			applyResourceStep(stepByURN, &stepOrder, evt.ResOutputsEvent.Metadata, false)
			captureStackOutputs(summary, evt.ResOutputsEvent.Metadata)

		case evt.ResOpFailedEvent != nil:
			applyResourceStep(stepByURN, &stepOrder, evt.ResOpFailedEvent.Metadata, true)
			summary.Failed = true

		case evt.DiagnosticEvent != nil:
			if d := toSummaryDiagnostic(evt.DiagnosticEvent); d != nil {
				summary.Diagnostics = append(summary.Diagnostics, *d)
			}

		case evt.PolicyEvent != nil:
			summary.PolicyViolations = append(summary.PolicyViolations, toPolicyViolation(evt.PolicyEvent))

		case evt.PolicyRemediationEvent != nil:
			summary.PolicyViolations = append(summary.PolicyViolations, toPolicyRemediation(evt.PolicyRemediationEvent))

		case evt.ErrorEvent != nil:
			summary.Failed = true
			summary.Error = evt.ErrorEvent.Error

		case evt.SummaryEvent != nil:
			applySummaryEvent(summary, evt.SummaryEvent)
		}
	}

	summary.Steps = flattenSteps(stepByURN, stepOrder)
	return summary, nil
}

// applyResourceStep records or updates the SummaryStep for md.URN. `OpSame` is ignored —
// consumers asked for a summary, not a full transcript. A previously-seen step for the same
// URN is overwritten, so a `ResOutputsEvent` supersedes its earlier `ResourcePreEvent` (the
// former is emitted strictly later and carries the final step state). The `failed` flag sticks:
// once a resource has failed, a later successful step for the same URN shouldn't erase that.
func applyResourceStep(
	byURN map[resource.URN]*display.SummaryStep,
	order *[]resource.URN,
	md apitype.StepEventMetadata,
	failed bool,
) {
	if md.Op == apitype.OpSame {
		return
	}
	urn := resource.URN(md.URN)
	step, seen := byURN[urn]
	if !seen {
		step = &display.SummaryStep{URN: urn}
		byURN[urn] = step
		*order = append(*order, urn)
	}
	step.Op = display.StepOp(md.Op)
	step.Type = md.Type
	step.Provider = md.Provider
	step.Diffs = md.Diffs
	step.ReplaceReasons = md.Keys
	step.DetailedDiff = md.DetailedDiff
	if failed {
		step.Failed = true
	}
}

// captureStackOutputs picks up the root-stack's outputs from a resource step. The last
// ResOutputsEvent / ResourcePreEvent for the root stack wins. Outputs are taken from the New
// state if present, falling back to the Old state (useful for destroys, where the final step
// has no New).
func captureStackOutputs(summary *display.EventSummary, md apitype.StepEventMetadata) {
	if md.Type != string(tokens.RootStackType) {
		return
	}
	switch {
	case md.New != nil && md.New.Outputs != nil:
		summary.Outputs = md.New.Outputs
	case md.Old != nil && md.Old.Outputs != nil:
		summary.Outputs = md.Old.Outputs
	}
}

// toSummaryDiagnostic converts a DiagnosticEvent into a SummaryDiagnostic, dropping entries
// that are meant for live display only. Ephemeral messages are transient progress spinners and
// must not pollute a persisted summary; `debug`-level messages are noisy and typically hidden
// from humans too.
func toSummaryDiagnostic(d *apitype.DiagnosticEvent) *display.SummaryDiagnostic {
	if d.Ephemeral || d.Severity == "debug" {
		return nil
	}
	return &display.SummaryDiagnostic{
		URN:      resource.URN(d.URN),
		Message:  d.Message,
		Severity: d.Severity,
	}
}

// toPolicyViolation flattens a PolicyEvent into a SummaryPolicyViolation.
func toPolicyViolation(p *apitype.PolicyEvent) display.SummaryPolicyViolation {
	return display.SummaryPolicyViolation{
		URN:               resource.URN(p.ResourceURN),
		PolicyName:        p.PolicyName,
		PolicyPackName:    p.PolicyPackName,
		PolicyPackVersion: p.PolicyPackVersion,
		EnforcementLevel:  p.EnforcementLevel,
		Severity:          p.Severity,
		Message:           p.Message,
		Remediated:        false,
	}
}

// toPolicyRemediation flattens a PolicyRemediationEvent into a SummaryPolicyViolation. The
// before/after payloads carried on remediations are intentionally dropped from the summary —
// consumers that need them should read the raw event stream.
func toPolicyRemediation(p *apitype.PolicyRemediationEvent) display.SummaryPolicyViolation {
	return display.SummaryPolicyViolation{
		URN:               resource.URN(p.ResourceURN),
		PolicyPackName:    p.PolicyPackName,
		PolicyPackVersion: p.PolicyPackVersion,
		PolicyName:        p.PolicyName,
		Remediated:        true,
	}
}

// applySummaryEvent copies the scalar fields of a SummaryEvent onto the summary and converts
// `PolicyPacks` into a deterministic sorted list. `DurationSeconds` is expanded to a
// `time.Duration` to match the convention used by `PreviewDigest.Duration`.
func applySummaryEvent(summary *display.EventSummary, s *apitype.SummaryEvent) {
	summary.IsPreview = s.IsPreview
	summary.MaybeCorrupt = s.MaybeCorrupt
	summary.Duration = time.Duration(s.DurationSeconds) * time.Second
	if len(s.ResourceChanges) > 0 {
		changes := make(display.ResourceChanges, len(s.ResourceChanges))
		for op, n := range s.ResourceChanges {
			changes[display.StepOp(op)] = n
		}
		summary.ChangeSummary = changes
	}
	if len(s.PolicyPacks) > 0 {
		packs := make([]display.SummaryPolicyPack, 0, len(s.PolicyPacks))
		for name, version := range s.PolicyPacks {
			packs = append(packs, display.SummaryPolicyPack{Name: name, Version: version})
		}
		sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })
		summary.PolicyPacks = packs
	}
}

// flattenSteps returns the per-URN SummaryStep snapshots in first-seen order. `nil` for an
// empty map so the field is omitted from JSON output rather than serialised as `[]`.
func flattenSteps(byURN map[resource.URN]*display.SummaryStep, order []resource.URN) []*display.SummaryStep {
	if len(order) == 0 {
		return nil
	}
	out := make([]*display.SummaryStep, 0, len(order))
	for _, urn := range order {
		out = append(out, byURN[urn])
	}
	return out
}
