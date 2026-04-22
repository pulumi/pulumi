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
//   - For kept resource events, if the provider reported a non-empty `Diffs` list, the same set
//     of top-level keys is used to restrict both `Inputs` and `Outputs` on `Old` and `New`. For
//     ops that typically have no diffs (e.g. create and delete, where every property is new or
//     gone) both maps are kept in full.
//
// Using the input-level `Diffs` to narrow `Outputs` is deliberately pragmatic: the engine only
// exposes input-side diffs, and in practice Pulumi providers export outputs whose keys mirror
// inputs, so filtering by `Diffs` keeps the properties a reader of a changes-only stream actually
// cares about while dropping stable derived outputs (ids, arns, ...). Output keys that don't
// overlap with `Diffs` are dropped — consumers that need the full post-state can work from the
// unfiltered stream.
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
	filteredMd.Old = restrictToDiffs(md.Old, md.Diffs)
	filteredMd.New = restrictToDiffs(md.New, md.Diffs)

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

// restrictToDiffs returns a shallow copy of s with `Inputs` and `Outputs` narrowed to the
// top-level keys in diffs. If diffs is empty the full `Inputs` and `Outputs` maps are preserved
// (every property is, by the engine's own report, either unchanged or — for creates and deletes —
// entirely new or gone).
func restrictToDiffs(
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
		out.Inputs = pickKeys(s.Inputs, keep)
		out.Outputs = pickKeys(s.Outputs, keep)
	}
	return &out
}

// pickKeys returns a fresh map containing only the entries of m whose keys are present in keep.
// A nil input yields a nil output.
func pickKeys(m map[string]any, keep map[string]bool) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(keep))
	for k, v := range m {
		if keep[k] {
			out[k] = v
		}
	}
	return out
}
