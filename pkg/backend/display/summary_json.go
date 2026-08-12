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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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
// It is intentionally compact: property-level detail only appears when the
// caller opts in with `--diff`.
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
	// Diff maps property paths to their changes. Only populated when `--diff`
	// is combined with `--output json`.
	Diff map[string]PropertyDiffJSON `json:"diff,omitempty"`
}

// PropertyDiffJSON describes the change to a single property path within
// ResourceJSON.Diff. Old is absent for adds and New is absent for deletes;
// secret values are masked unless the caller passed `--show-secrets`.
type PropertyDiffJSON struct {
	Kind string `json:"kind"`
	Old  any    `json:"old,omitzero"`
	New  any    `json:"new,omitzero"`
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
func resourceJSONFromEvent(p engine.ResourcePreEventPayload, opts Options) *ResourceJSON {
	if p.Internal {
		return nil
	}
	if p.Metadata.Op == deploy.OpSame && !opts.ShowSameResources {
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
	if opts.Type == DisplayDiff {
		r.Diff = diffJSONFromStep(&p.Metadata, false /* refresh */, opts.ShowSecrets)
	}
	return &r
}

// diffJSONFromStep flattens the property changes carried by a step event into
// the summary's path → change map, drawing on the same sources as the
// human-readable diff display: the provider's detailed diff when present, and
// an old-vs-new inputs comparison otherwise. Creates report every input as an
// add and deletes report every input as a delete. Returns nil when the step
// carries no property changes.
func diffJSONFromStep(m *engine.StepEventMetadata, refresh, showSecrets bool) map[string]PropertyDiffJSON {
	// An OpSame can carry a metadata-only diff (e.g. protect); never report a
	// property diff for it. See https://github.com/pulumi/pulumi/issues/15944.
	if m.Op == deploy.OpSame {
		return nil
	}

	var hidden []resource.PropertyPath
	if m.New != nil {
		hidden = m.New.HideDiffs
	} else if m.Old != nil {
		hidden = m.Old.HideDiffs
	}

	out := map[string]PropertyDiffJSON{}
	add := func(path resource.PropertyPath, kind string, old, new *resource.PropertyValue) {
		for _, h := range hidden {
			if h.Contains(path) {
				return
			}
		}
		entry := PropertyDiffJSON{Kind: kind}
		if old != nil {
			entry.Old = propertyValueJSON(*old, showSecrets)
		}
		if new != nil {
			entry.New = propertyValueJSON(*new, showSecrets)
		}
		out[path.String()] = entry
	}

	switch {
	case m.DetailedDiff != nil && m.Old != nil && m.New != nil:
		if diff, _ := engine.TranslateDetailedDiff(m, refresh); diff != nil {
			flattenObjectDiff(nil, diff, add)
		}
	case m.Old != nil && m.New != nil:
		if diff := m.Old.Inputs.Diff(m.New.Inputs, resource.IsInternalPropertyKey); diff != nil {
			flattenObjectDiff(nil, diff, add)
		}
	case m.New != nil:
		for k, v := range m.New.Inputs {
			if !resource.IsInternalPropertyKey(k) {
				add(resource.PropertyPath{string(k)}, "add", nil, &v)
			}
		}
	case m.Old != nil:
		for k, v := range m.Old.Inputs {
			if !resource.IsInternalPropertyKey(k) {
				add(resource.PropertyPath{string(k)}, "delete", &v, nil)
			}
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// propertyValueJSON prepares a property value for JSON output: secrets are
// masked (unless showSecrets), and unknowns, assets, and resource references
// are rendered the same way the preview digest renders resource states.
func propertyValueJSON(v resource.PropertyValue, showSecrets bool) any {
	serialized, err := stack.SerializePropertyValue(
		context.TODO(), massagePropertyValue(v, showSecrets), config.NewPanicCrypter(), showSecrets)
	if err != nil {
		logging.V(7).Infof("not adding property value as there was an error serializing: %s", err)
		return nil
	}
	return serialized
}

type addDiffEntry func(path resource.PropertyPath, kind string, old, new *resource.PropertyValue)

func flattenObjectDiff(prefix resource.PropertyPath, d *resource.ObjectDiff, add addDiffEntry) {
	for k, vd := range d.Updates {
		flattenValueDiff(append(prefix, string(k)), vd, add)
	}
	for k, v := range d.Adds {
		add(append(prefix, string(k)), "add", nil, &v)
	}
	for k, v := range d.Deletes {
		add(append(prefix, string(k)), "delete", &v, nil)
	}
}

func flattenValueDiff(path resource.PropertyPath, vd resource.ValueDiff, add addDiffEntry) {
	switch {
	case vd.Object != nil:
		flattenObjectDiff(path, vd.Object, add)
	case vd.Array != nil:
		for i, ed := range vd.Array.Updates {
			flattenValueDiff(append(path, i), ed, add)
		}
		for i, v := range vd.Array.Adds {
			add(append(path, i), "add", nil, &v)
		}
		for i, v := range vd.Array.Deletes {
			add(append(path, i), "delete", &v, nil)
		}
	case vd.Old.V == nil && vd.New.V != nil:
		add(path, "add", nil, &vd.New)
	case vd.Old.V != nil && vd.New.V == nil:
		add(path, "delete", &vd.Old, nil)
	default:
		add(path, "update", &vd.Old, &vd.New)
	}
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
	go func() {
		defer close(out)
		var resources []ResourceJSON
		refreshed := map[resource.URN]bool{}
		for e := range in {
			switch e.Type { //nolint:exhaustive // we only care about a few event types here
			case engine.ResourcePreEvent:
				if payload, ok := e.Payload().(engine.ResourcePreEventPayload); ok {
					if payload.Metadata.Op == deploy.OpRefresh {
						refreshed[payload.Metadata.URN] = true
					}
					if r := resourceJSONFromEvent(payload, opts); r != nil {
						resources = append(resources, *r)
					}
				}
			case engine.ResourceOutputsEvent:
				// Refreshes only discover their changes after reading the
				// resource: the outputs event carries them, rewritten as an
				// update or delete. Attach the diff to the entry recorded by
				// the pre-event.
				if payload, ok := e.Payload().(engine.ResourceOutputsEventPayload); ok && opts.Type == DisplayDiff {
					m := payload.Metadata
					if refreshed[m.URN] && ((m.Op == deploy.OpUpdate && m.DetailedDiff != nil) || m.Op == deploy.OpDelete) {
						diff := diffJSONFromStep(&m, true /* refresh */, opts.ShowSecrets)
						for i := range resources {
							if resources[i].URN == string(m.URN) {
								resources[i].Diff = diff
							}
						}
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
