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

package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// SummaryJSON is a one-line JSON summary of a stack operation, intended for
// programmatic / LLM consumers when `--output json` is set.
//
// The shape is intentionally narrower than `apitype.SummaryEvent`: it only
// surfaces the fields that make sense as a final summary of the run (no
// `isPreview`, no `policyPacks`, no `maybeCorrupt`).
type SummaryJSON struct {
	// Result is the high-level outcome of the operation.
	Result apitype.OperationResult `json:"result"`
	// Duration is how long the operation took.
	Duration time.Duration `json:"duration"`
	// Summary is the count per operation kind (create, update, etc).
	Summary display.ResourceChanges `json:"summary,omitempty"`
	// Resources lists each resource the operation acted on, with its planned
	// (or performed) operation. Unchanged (`same`) resources are omitted unless
	// the caller passed `--show-sames`, mirroring the human-readable display.
	Resources []ResourceJSON `json:"resources,omitempty"`
}

// ResourceJSON is the per-resource entry that appears in SummaryJSON.Resources.
// It is intentionally compact: callers that need full property values should
// use `--json` (the streaming event format) instead.
type ResourceJSON struct {
	// URN is the canonical, globally-unique identifier of the resource.
	URN string `json:"urn"`
	// Type is the resource type token (e.g. "aws:s3/bucket:Bucket").
	Type string `json:"type"`
	// Name is the resource's program-assigned name.
	Name string `json:"name"`
	// Op is the planned (preview) or performed (up/destroy/refresh) operation.
	Op apitype.OpType `json:"op"`
	// Parent is the URN of this resource's parent, if any.
	Parent string `json:"parent,omitempty"`
	// Diff maps changed property paths to their old/new values. It is only
	// populated when `--diff` is combined with `--output json`.
	Diff map[string]PropertyDiffJSON `json:"diff,omitempty"`
}

// PropertyDiffJSON describes the change to a single property path within a
// resource's diff.
type PropertyDiffJSON struct {
	// Kind is one of "add", "delete", or "update".
	Kind string `json:"kind"`
	// Old is the value before the operation; unset for adds.
	Old any `json:"old,omitempty"`
	// New is the value after the operation; unset for deletes.
	New any `json:"new,omitempty"`
}

// summaryJSONFromEvent extracts the summary JSON shape from a SummaryEventPayload.
// The Resources field is populated separately by the tap as resource events flow
// past, so this helper only fills the run-level fields.
func summaryJSONFromEvent(p engine.SummaryEventPayload) SummaryJSON {
	return SummaryJSON{
		Result:   p.Result,
		Duration: p.Duration,
		Summary:  p.ResourceChanges,
	}
}

// resourceJSONFromEvent converts a per-resource pre-event into the summary's
// per-resource JSON shape. Returns nil when the event should be skipped:
// internal events never surface to users, and `same` (unchanged) resources are
// omitted unless the display is configured to show them.
func resourceJSONFromEvent(p engine.ResourcePreEventPayload, showSames bool) *ResourceJSON {
	if p.Internal {
		return nil
	}
	if p.Metadata.Op == deploy.OpSame && !showSames {
		return nil
	}

	// Parent lives on the post-step state when there is one, and falls back to
	// the pre-step state for deletes (where New is nil).
	var parent string
	switch {
	case p.Metadata.New != nil:
		parent = string(p.Metadata.New.Parent)
	case p.Metadata.Old != nil:
		parent = string(p.Metadata.Old.Parent)
	}

	r := NewResourceJSON(p.Metadata.URN, apitype.OpType(p.Metadata.Op), parent)
	return &r
}

// NewResourceJSON builds the per-resource summary entry from the fields
// shared by the live display and `pulumi stack history events --summary`.
func NewResourceJSON(urn resource.URN, op apitype.OpType, parent string) ResourceJSON {
	return ResourceJSON{
		URN:    string(urn),
		Type:   string(urn.Type()),
		Name:   urn.Name(),
		Op:     op,
		Parent: parent,
	}
}

// NewDiffJSON flattens a step's property diff into path → change entries,
// drawing on the same sources as the human diff view: the provider's detailed
// diff when present, otherwise a diff computed from the step's old and new
// inputs.
func NewDiffJSON(m *engine.StepEventMetadata, refresh, showSecrets bool) map[string]PropertyDiffJSON {
	// An OpSame might have a diff due to metadata changes (e.g. protect) but we
	// should never report a property diff. See
	// https://github.com/pulumi/pulumi/issues/15944 for context.
	if m.Op == deploy.OpSame {
		return nil
	}

	var diff *resource.ObjectDiff
	var hidden []resource.PropertyPath
	switch {
	case m.DetailedDiff != nil:
		// TranslateDetailedDiff already excludes hidden (HideDiffs) paths.
		diff, _ = engine.TranslateDetailedDiff(m, refresh)
	case m.Old == nil && m.New != nil:
		diff = resource.PropertyMap{}.Diff(m.New.Inputs, resource.IsInternalPropertyKey)
		hidden = m.New.HideDiffs
	case m.Old != nil && m.New == nil:
		diff = m.Old.Inputs.Diff(resource.PropertyMap{}, resource.IsInternalPropertyKey)
		hidden = m.Old.HideDiffs
	case m.Old != nil && m.New != nil:
		diff = m.Old.Inputs.Diff(m.New.Inputs, resource.IsInternalPropertyKey)
		hidden = m.New.HideDiffs
	}
	if diff == nil {
		return nil
	}

	out := map[string]PropertyDiffJSON{}
	flattenObjectDiff(out, nil, diff, hidden, showSecrets)
	if len(out) == 0 {
		return nil
	}
	return out
}

// NewDiffJSONFromAPI is NewDiffJSON for step metadata that has already been
// serialized to its API shape, as returned by the Pulumi Cloud events
// endpoint. Secret values in such metadata are already blinded, so there is
// no showSecrets option to offer.
func NewDiffJSONFromAPI(md apitype.StepEventMetadata, refresh bool) map[string]PropertyDiffJSON {
	em := convertJSONStepEventMetadata(md)
	return NewDiffJSON(&em, refresh, false /* showSecrets */)
}

func flattenObjectDiff(out map[string]PropertyDiffJSON, path resource.PropertyPath, diff *resource.ObjectDiff,
	hidden []resource.PropertyPath, showSecrets bool,
) {
	at := func(k resource.PropertyKey) resource.PropertyPath {
		return append(slices.Clone(path), string(k))
	}
	for k, v := range diff.Adds {
		emitDiffEntry(out, at(k), PropertyDiffJSON{Kind: "add", New: jsonPropertyValue(v, showSecrets)}, hidden)
	}
	for k, v := range diff.Deletes {
		emitDiffEntry(out, at(k), PropertyDiffJSON{Kind: "delete", Old: jsonPropertyValue(v, showSecrets)}, hidden)
	}
	for k, v := range diff.Updates {
		flattenValueDiff(out, at(k), v, hidden, showSecrets)
	}
}

func flattenValueDiff(out map[string]PropertyDiffJSON, path resource.PropertyPath, diff resource.ValueDiff,
	hidden []resource.PropertyPath, showSecrets bool,
) {
	at := func(i int) resource.PropertyPath {
		return append(slices.Clone(path), i)
	}
	switch {
	case diff.Object != nil:
		flattenObjectDiff(out, path, diff.Object, hidden, showSecrets)
	case diff.Array != nil:
		for i, v := range diff.Array.Adds {
			emitDiffEntry(out, at(i), PropertyDiffJSON{Kind: "add", New: jsonPropertyValue(v, showSecrets)}, hidden)
		}
		for i, v := range diff.Array.Deletes {
			emitDiffEntry(out, at(i), PropertyDiffJSON{Kind: "delete", Old: jsonPropertyValue(v, showSecrets)}, hidden)
		}
		for i, v := range diff.Array.Updates {
			flattenValueDiff(out, at(i), v, hidden, showSecrets)
		}
	default:
		emitDiffEntry(out, path, PropertyDiffJSON{
			Kind: "update",
			Old:  jsonPropertyValue(diff.Old, showSecrets),
			New:  jsonPropertyValue(diff.New, showSecrets),
		}, hidden)
	}
}

func emitDiffEntry(out map[string]PropertyDiffJSON, path resource.PropertyPath, d PropertyDiffJSON,
	hidden []resource.PropertyPath,
) {
	for _, h := range hidden {
		if h.Contains(path) {
			return
		}
	}
	out[path.String()] = d
}

// jsonPropertyValue renders a property value as a plain JSON-marshalable
// value, masking secrets and unknowns the same way the human diff display
// does.
func jsonPropertyValue(v resource.PropertyValue, showSecrets bool) any {
	switch {
	case v.IsSecret():
		if !showSecrets {
			return "[secret]"
		}
		return jsonPropertyValue(v.SecretValue().Element, showSecrets)
	case v.IsComputed():
		return "[unknown]"
	case v.IsOutput():
		o := v.OutputValue()
		switch {
		case !o.Known:
			return "[unknown]"
		case o.Secret && !showSecrets:
			return "[secret]"
		default:
			return jsonPropertyValue(o.Element, showSecrets)
		}
	case v.IsString() && !utf8.ValidString(v.StringValue()):
		return byteStringDisplay(v.StringValue())
	case v.IsArray():
		arr := make([]any, len(v.ArrayValue()))
		for i, e := range v.ArrayValue() {
			arr[i] = jsonPropertyValue(e, showSecrets)
		}
		return arr
	case v.IsObject():
		obj := make(map[string]any, len(v.ObjectValue()))
		for k, e := range v.ObjectValue() {
			obj[string(k)] = jsonPropertyValue(e, showSecrets)
		}
		return obj
	default:
		return v.Mappable()
	}
}

// writeSummaryJSON encodes a SummaryJSON to w as a single line.
func writeSummaryJSON(w io.Writer, s SummaryJSON) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	// json.Encoder always appends a trailing newline; that's the line break we want.
	_, err := io.Copy(w, &buf)
	return err
}

// tapSummaryJSON returns a copy of the input channel that, in addition to
// forwarding every event, watches for per-resource events to build up a list
// of affected resources, and for the SummaryEvent to flush the combined
// summary JSON to stdout as a single line.
//
// The tap is only attached when Options.SummaryJSON is set; the rest of the
// display pipeline is otherwise unaffected.
func tapSummaryJSON(in <-chan engine.Event, opts Options) <-chan engine.Event {
	out := make(chan engine.Event)
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	// `--diff` composed with `--output json`: include each resource's property
	// diff in the summary.
	includeDiff := opts.Type == DisplayDiff
	go func() {
		defer close(out)
		var resources []ResourceJSON
		indexByURN := map[resource.URN]int{}
		for e := range in {
			switch e.Type { //nolint:exhaustive // we only care about a few event types here
			case engine.ResourcePreEvent:
				if payload, ok := e.Payload().(engine.ResourcePreEventPayload); ok {
					if r := resourceJSONFromEvent(payload, opts.ShowSameResources); r != nil {
						if includeDiff {
							r.Diff = NewDiffJSON(&payload.Metadata, false /* refresh */, opts.ShowSecrets)
						}
						indexByURN[payload.Metadata.URN] = len(resources)
						resources = append(resources, *r)
					}
				}
			case engine.ResourceOutputsEvent:
				// Refresh steps only reveal their diff once the provider has read
				// the resource's current state, which arrives on the outputs event
				// as an update (with a detailed diff) or a delete.
				if !includeDiff {
					break
				}
				if payload, ok := e.Payload().(engine.ResourceOutputsEventPayload); ok {
					m := payload.Metadata
					if i, ok := indexByURN[m.URN]; ok && resources[i].Op == apitype.OpRefresh &&
						((m.Op == deploy.OpUpdate && m.DetailedDiff != nil) || m.Op == deploy.OpDelete) {
						resources[i].Diff = NewDiffJSON(&m, true /* refresh */, opts.ShowSecrets)
					}
				}
			case engine.SummaryEvent:
				if payload, ok := e.Payload().(engine.SummaryEventPayload); ok {
					s := summaryJSONFromEvent(payload)
					s.Resources = resources
					if err := writeSummaryJSON(stdout, s); err != nil {
						fmt.Fprintf(stderr, "warning: failed to write summary JSON: %v\n", err)
					}
				}
			}
			out <- e
			if e.Type == engine.CancelEvent {
				return
			}
		}
	}()
	return out
}
