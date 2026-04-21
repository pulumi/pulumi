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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// filterChangesOnly applies the `--changes-only` transform to a single engine event, returning nil
// if the event should be dropped from the stream or a freshly-constructed, filtered copy otherwise.
//
// The filter matches the behaviour of `pulumi preview --json --changes-only` (see pulumi/pulumi
// #22068): only resource-lifecycle events for real changes are kept, and their state metadata is
// restricted to the properties that actually changed.
//
//   - Events that aren't resource-lifecycle events (stdout, diagnostic, prelude, progress, policy,
//     summary, ...) are dropped entirely.
//   - Resource events whose op is `same`, `read`, `read-replacement`, `discard`, or `refresh` are
//     dropped: these are observations, not changes.
//   - For `update` and `replace` ops the `Inputs` map on `Old` and `New` is restricted to the top
//     level keys the provider reported via `Diffs`. For creates, deletes, and other ops every
//     property is, by definition, new or gone so inputs are kept in full.
//   - `Outputs` are dropped from `Old` and `New` for every kept event: they describe post-state,
//     not the change itself.
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
		return nil
	}

	//exhaustive:ignore
	switch md.Op {
	case apitype.OpSame, apitype.OpRead, apitype.OpReadReplacement, apitype.OpReadDiscard, apitype.OpRefresh:
		return nil
	}

	filteredMd := *md
	filteredMd.Old = filterStateChangesOnly(md.Old, md.Op, md.Diffs)
	filteredMd.New = filterStateChangesOnly(md.New, md.Op, md.Diffs)

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

// filterStateChangesOnly returns a filtered copy of s suitable for `--changes-only` output.
// Outputs are always dropped; inputs are kept in full for every op except update and replace, where
// they are restricted to the top-level keys in diffs.
func filterStateChangesOnly(
	s *apitype.StepEventStateMetadata, op apitype.OpType, diffs []string,
) *apitype.StepEventStateMetadata {
	if s == nil {
		return nil
	}
	out := *s
	out.Outputs = map[string]any{}
	//exhaustive:ignore
	switch op {
	case apitype.OpUpdate, apitype.OpReplace:
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
