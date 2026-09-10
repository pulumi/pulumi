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
	"time"

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
// It is intentionally compact: callers that need full diffs / property values
// should use `--json` (the streaming event format) instead.
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
		for e := range in {
			switch e.Type { //nolint:exhaustive // we only care about two event types here
			case engine.ResourcePreEvent:
				if payload, ok := e.Payload().(engine.ResourcePreEventPayload); ok {
					if r := resourceJSONFromEvent(payload, opts.ShowSameResources); r != nil {
						resources = append(resources, *r)
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
