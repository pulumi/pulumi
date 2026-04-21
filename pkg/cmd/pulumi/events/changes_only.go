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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// runChangesOnly reads a JSONL engine event stream from in, applies the `--changes-only` filter
// to each event, and writes the surviving events back as JSONL to out. Decoding errors are
// surfaced to the caller so that malformed input fails fast rather than silently dropping events.
func runChangesOnly(in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)
	for {
		var evt apitype.EngineEvent
		if err := dec.Decode(&evt); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decoding event: %w", err)
		}
		filtered := filterChangesOnly(evt)
		if filtered == nil {
			continue
		}
		if err := enc.Encode(filtered); err != nil {
			return fmt.Errorf("encoding event: %w", err)
		}
	}
}

// filterChangesOnly applies the `--changes-only` transform to a single engine event, returning nil
// if the event should be dropped from the stream or a freshly-constructed, filtered copy otherwise.
//
// Rules:
//
//   - Non-resource events (stdout, diagnostic, prelude, summary, policy, progress, cancel, ...)
//     are passed through unchanged — they carry context the consumer may care about regardless of
//     whether any resource changed.
//   - Resource events whose op is `same` are dropped: `same` is the engine's explicit marker that
//     nothing changed. Every other op (create, update, replace, delete, read, refresh,
//     read-replacement, read-discard) represents a real observable change and is kept.
//   - For kept resource events, if the provider reported a non-empty `Diffs` list, `Old.Inputs`
//     and `New.Inputs` are restricted to those top-level keys. For ops that typically have no
//     diffs (e.g. create and delete, where every property is new or gone) inputs are kept in full.
//   - `Outputs` are preserved: consumers that render these events still want to see the resulting
//     state.
//
// The input event is never mutated; every pointer returned references freshly-allocated structs so
// callers can safely keep and modify the result.
func filterChangesOnly(evt apitype.EngineEvent) *apitype.EngineEvent {
	var md *apitype.StepEventMetadata
	switch {
	case evt.ResourcePreEvent != nil:
		md = &evt.ResourcePreEvent.Metadata
	case evt.ResOutputsEvent != nil:
		md = &evt.ResOutputsEvent.Metadata
	case evt.ResOpFailedEvent != nil:
		md = &evt.ResOpFailedEvent.Metadata
	default:
		// Non-resource event: pass through a shallow copy so callers still own a fresh pointer.
		cp := evt
		return &cp
	}

	if md.Op == apitype.OpSame {
		return nil
	}

	filteredMd := *md
	filteredMd.Old = restrictInputsToDiffs(md.Old, md.Diffs)
	filteredMd.New = restrictInputsToDiffs(md.New, md.Diffs)

	out := apitype.EngineEvent{Sequence: evt.Sequence, Timestamp: evt.Timestamp}
	switch {
	case evt.ResourcePreEvent != nil:
		cp := *evt.ResourcePreEvent
		cp.Metadata = filteredMd
		out.ResourcePreEvent = &cp
	case evt.ResOutputsEvent != nil:
		cp := *evt.ResOutputsEvent
		cp.Metadata = filteredMd
		out.ResOutputsEvent = &cp
	case evt.ResOpFailedEvent != nil:
		cp := *evt.ResOpFailedEvent
		cp.Metadata = filteredMd
		out.ResOpFailedEvent = &cp
	}
	return &out
}

// restrictInputsToDiffs returns a shallow copy of s with `Inputs` narrowed to the top-level keys
// in diffs. If diffs is empty the full `Inputs` map is preserved (every property is, by the
// engine's own report, either unchanged or — for creates and deletes — entirely new or gone).
// `Outputs` are carried over unchanged.
func restrictInputsToDiffs(
	s *apitype.StepEventStateMetadata, diffs []string,
) *apitype.StepEventStateMetadata {
	if s == nil {
		return nil
	}
	out := *s
	if len(diffs) > 0 {
		keep := make(map[string]bool, len(diffs))
		for _, k := range diffs {
			keep[k] = true
		}
		filtered := make(map[string]any, len(keep))
		for k, v := range s.Inputs {
			if keep[k] {
				filtered[k] = v
			}
		}
		out.Inputs = filtered
	}
	return &out
}
