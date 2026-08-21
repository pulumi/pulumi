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
	"maps"
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
	// Diff maps property paths to their changes; only populated when `--diff` is set.
	Diff map[string]PropertyDiffJSON `json:"diff,omitempty"`
}

// PropertyDiffJSON is the change to a single property path in ResourceJSON.Diff.
// Old is absent for adds, New for deletes.
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
	// Imports only have their diff once outputs arrive, mirroring renderDiffResourcePreEvent.
	if opts.Type == DisplayDiff && p.Metadata.Op != deploy.OpImport {
		r.Diff = diffJSONFromStep(&p.Metadata, false /* refresh */, opts.ShowSecrets)
	}
	return &r
}

// diffJSONFromStep renders the diff computed by stepDiff — the same one the
// human-readable display prints — as a flat path → change map.
func diffJSONFromStep(m *engine.StepEventMetadata, refresh, showSecrets bool) map[string]PropertyDiffJSON {
	diff, include, _ := stepDiff(m, refresh)
	if diff == nil {
		return nil
	}
	if include != nil {
		keep := make(map[resource.PropertyKey]bool, len(include))
		for _, k := range include {
			keep[k] = true
		}
		maps.DeleteFunc(diff.Adds, func(k resource.PropertyKey, _ resource.PropertyValue) bool { return !keep[k] })
		maps.DeleteFunc(diff.Deletes, func(k resource.PropertyKey, _ resource.PropertyValue) bool { return !keep[k] })
		maps.DeleteFunc(diff.Updates, func(k resource.PropertyKey, _ resource.ValueDiff) bool { return !keep[k] })
	}

	out := map[string]PropertyDiffJSON{}
	add := func(path resource.PropertyPath, kind string, old, new *resource.PropertyValue) {
		entry := PropertyDiffJSON{Kind: kind}
		if old != nil {
			entry.Old = propertyValueJSON(*old, showSecrets)
		}
		if new != nil {
			entry.New = propertyValueJSON(*new, showSecrets)
		}
		out[path.String()] = entry
	}
	flattenObjectDiff(nil, diff, add)

	if len(out) == 0 {
		return nil
	}
	return out
}

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
			switch e.Type { //nolint:exhaustive
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
				// Import and refresh diffs only arrive on the outputs event,
				// mirroring renderDiffResourceOutputsEvent.
				if payload, ok := e.Payload().(engine.ResourceOutputsEventPayload); ok && opts.Type == DisplayDiff {
					m := payload.Metadata
					refresh := refreshed[m.URN]
					if m.Op == deploy.OpImport || (refresh && m.Op == deploy.OpUpdate) {
						diff := diffJSONFromStep(&m, refresh, opts.ShowSecrets)
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
