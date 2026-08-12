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
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// resourcePreEvent is a tiny helper for building ResourcePreEvent fixtures in table tests.
func resourcePreEvent(op apitype.OpType, diffs []string, oldIn, newIn map[string]any) apitype.EngineEvent {
	return apitype.EngineEvent{
		Sequence:  7,
		Timestamp: 42,
		ResourcePreEvent: &apitype.ResourcePreEvent{
			Metadata: apitype.StepEventMetadata{
				Op:    op,
				URN:   "urn:pulumi:stack::proj::pkg:index:Res::r",
				Type:  "pkg:index:Res",
				Diffs: diffs,
				Old:   &apitype.StepEventStateMetadata{URN: "urn:old", Inputs: oldIn, Outputs: map[string]any{"o": 1}},
				New:   &apitype.StepEventStateMetadata{URN: "urn:new", Inputs: newIn, Outputs: map[string]any{"o": 2}},
			},
		},
	}
}

func TestFilterChangesOnly_KeepsNonResourceEvents(t *testing.T) {
	t.Parallel()

	// Non-resource events carry context (stdout, diagnostics, summary, ...) that consumers of a
	// filtered stream may still care about, so the filter passes them through unchanged.
	cases := []struct {
		name  string
		event apitype.EngineEvent
	}{
		{"cancel", apitype.EngineEvent{CancelEvent: &apitype.CancelEvent{}}},
		{"stdout", apitype.EngineEvent{StdoutEvent: &apitype.StdoutEngineEvent{Message: "hi"}}},
		{"diagnostic", apitype.EngineEvent{DiagnosticEvent: &apitype.DiagnosticEvent{Message: "warn"}}},
		{"prelude", apitype.EngineEvent{PreludeEvent: &apitype.PreludeEvent{}}},
		{"summary", apitype.EngineEvent{SummaryEvent: &apitype.SummaryEvent{}}},
		{"policy", apitype.EngineEvent{PolicyEvent: &apitype.PolicyEvent{}}},
		{"policy-remediation", apitype.EngineEvent{PolicyRemediationEvent: &apitype.PolicyRemediationEvent{}}},
		{"policy-load", apitype.EngineEvent{PolicyLoadEvent: &apitype.PolicyLoadEvent{}}},
		{"progress", apitype.EngineEvent{ProgressEvent: &apitype.ProgressEvent{}}},
		{"start-debugging", apitype.EngineEvent{StartDebuggingEvent: &apitype.StartDebuggingEvent{}}},
		{"error", apitype.EngineEvent{ErrorEvent: &apitype.ErrorEvent{}}},
		{"empty", apitype.EngineEvent{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := filterChangesOnly(c.event)
			require.NotNil(t, got, "%s events must pass through --changes-only", c.name)
			assert.Equal(t, c.event, *got, "%s events must pass through unchanged", c.name)
		})
	}
}

func TestFilterChangesOnly_DropsOpSame(t *testing.T) {
	t.Parallel()

	// OpSame is the engine's explicit "nothing changed" marker — that's the one op we drop.
	evt := resourcePreEvent(apitype.OpSame, nil, map[string]any{"foo": "a"}, map[string]any{"foo": "a"})
	assert.Nil(t, filterChangesOnly(evt), "OpSame resource events must be dropped")
}

func TestFilterChangesOnly_KeepsReadRefreshDiscardAndReplacements(t *testing.T) {
	t.Parallel()

	// Read, refresh, read-replacement, and read-discard all represent real observable changes:
	// a resource was pulled in, refreshed state diverged, or a resource was dropped from state.
	// They must survive the filter.
	ops := []apitype.OpType{
		apitype.OpRead,
		apitype.OpReadReplacement,
		apitype.OpReadDiscard,
		apitype.OpRefresh,
	}
	for _, op := range ops {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()
			evt := resourcePreEvent(op, []string{"foo"},
				map[string]any{"foo": "old", "bar": "s"},
				map[string]any{"foo": "new", "bar": "s"})
			got := filterChangesOnly(evt)
			require.NotNil(t, got, "%s events must survive --changes-only", op)
			require.NotNil(t, got.ResourcePreEvent)
			md := got.ResourcePreEvent.Metadata
			assert.Equal(t, op, md.Op)
			// Diffs, when present, still restrict inputs regardless of the op.
			assert.Equal(t, map[string]any{"foo": "old"}, md.Old.Inputs)
			assert.Equal(t, map[string]any{"foo": "new"}, md.New.Inputs)
		})
	}
}

func TestFilterChangesOnly_UpdateRestrictsInputsAndKeepsOutputs(t *testing.T) {
	t.Parallel()

	oldIn := map[string]any{"foo": "old", "bar": "stable", "baz": "also-stable"}
	newIn := map[string]any{"foo": "new", "bar": "stable", "baz": "also-stable"}
	evt := resourcePreEvent(apitype.OpUpdate, []string{"foo"}, oldIn, newIn)

	got := filterChangesOnly(evt)
	require.NotNil(t, got, "update event should survive the filter")
	require.NotNil(t, got.ResourcePreEvent)

	md := got.ResourcePreEvent.Metadata
	assert.Equal(t, apitype.OpUpdate, md.Op)
	// Sequence/timestamp are passed through unchanged.
	assert.Equal(t, 7, got.Sequence)
	assert.Equal(t, 42, got.Timestamp)

	require.NotNil(t, md.Old)
	require.NotNil(t, md.New)
	assert.Equal(t, map[string]any{"foo": "old"}, md.Old.Inputs,
		"Old.Inputs must contain only changed keys")
	assert.Equal(t, map[string]any{"foo": "new"}, md.New.Inputs,
		"New.Inputs must contain only changed keys")
	assert.Equal(t, map[string]any{"o": 1}, md.Old.Outputs,
		"Old.Outputs must be preserved — consumers still want to see resulting state")
	assert.Equal(t, map[string]any{"o": 2}, md.New.Outputs,
		"New.Outputs must be preserved")
}

func TestFilterChangesOnly_ReplaceRestrictsInputsToDiffs(t *testing.T) {
	t.Parallel()

	oldIn := map[string]any{"foo": "old", "bar": "stable"}
	newIn := map[string]any{"foo": "new", "bar": "stable"}
	evt := resourcePreEvent(apitype.OpReplace, []string{"foo"}, oldIn, newIn)

	got := filterChangesOnly(evt)
	require.NotNil(t, got)
	md := got.ResourcePreEvent.Metadata
	assert.Equal(t, map[string]any{"foo": "old"}, md.Old.Inputs)
	assert.Equal(t, map[string]any{"foo": "new"}, md.New.Inputs)
}

func TestFilterChangesOnly_CreateKeepsAllInputsAndOutputs(t *testing.T) {
	t.Parallel()

	// Creates have no "Old" state and typically no Diffs; every property is new, so the full
	// Inputs map is preserved along with the Outputs.
	evt := apitype.EngineEvent{
		ResourcePreEvent: &apitype.ResourcePreEvent{
			Metadata: apitype.StepEventMetadata{
				Op:  apitype.OpCreate,
				URN: "urn:pulumi:stack::proj::pkg:index:Res::r",
				New: &apitype.StepEventStateMetadata{
					Inputs:  map[string]any{"foo": "a", "bar": "b"},
					Outputs: map[string]any{"o": 1},
				},
			},
		},
	}

	got := filterChangesOnly(evt)
	require.NotNil(t, got)
	md := got.ResourcePreEvent.Metadata
	assert.Nil(t, md.Old)
	assert.Equal(t, map[string]any{"foo": "a", "bar": "b"}, md.New.Inputs,
		"create events keep all inputs — every property is new")
	assert.Equal(t, map[string]any{"o": 1}, md.New.Outputs, "outputs must be preserved")
}

func TestFilterChangesOnly_DeleteKeepsAllOldInputsAndOutputs(t *testing.T) {
	t.Parallel()

	evt := apitype.EngineEvent{
		ResourcePreEvent: &apitype.ResourcePreEvent{
			Metadata: apitype.StepEventMetadata{
				Op:  apitype.OpDelete,
				URN: "urn:pulumi:stack::proj::pkg:index:Res::r",
				Old: &apitype.StepEventStateMetadata{
					Inputs:  map[string]any{"foo": "a", "bar": "b"},
					Outputs: map[string]any{"o": 1},
				},
			},
		},
	}

	got := filterChangesOnly(evt)
	require.NotNil(t, got)
	md := got.ResourcePreEvent.Metadata
	assert.Equal(t, map[string]any{"foo": "a", "bar": "b"}, md.Old.Inputs,
		"delete events keep all old inputs — consumers need to know what's gone")
	assert.Equal(t, map[string]any{"o": 1}, md.Old.Outputs, "outputs must be preserved")
	assert.Nil(t, md.New)
}

func TestFilterChangesOnly_KeepsResOutputsAndResOpFailed(t *testing.T) {
	t.Parallel()

	// ResOutputsEvent with an update op.
	outputs := apitype.EngineEvent{
		ResOutputsEvent: &apitype.ResOutputsEvent{
			Metadata: apitype.StepEventMetadata{
				Op:    apitype.OpUpdate,
				URN:   "urn",
				Diffs: []string{"foo"},
				New: &apitype.StepEventStateMetadata{
					Inputs:  map[string]any{"foo": "n", "bar": "s"},
					Outputs: map[string]any{"o": 1},
				},
			},
		},
	}
	gotOut := filterChangesOnly(outputs)
	require.NotNil(t, gotOut)
	require.NotNil(t, gotOut.ResOutputsEvent)
	assert.Equal(t, map[string]any{"foo": "n"}, gotOut.ResOutputsEvent.Metadata.New.Inputs)
	assert.Equal(t, map[string]any{"o": 1}, gotOut.ResOutputsEvent.Metadata.New.Outputs,
		"ResOutputsEvent outputs must be preserved")

	// ResOpFailedEvent with a create op retains Status/Steps on the copy.
	failed := apitype.EngineEvent{
		ResOpFailedEvent: &apitype.ResOpFailedEvent{
			Status: 3,
			Steps:  5,
			Metadata: apitype.StepEventMetadata{
				Op:  apitype.OpCreate,
				URN: "urn",
				New: &apitype.StepEventStateMetadata{
					Inputs: map[string]any{"foo": "a"},
				},
			},
		},
	}
	gotFail := filterChangesOnly(failed)
	require.NotNil(t, gotFail)
	require.NotNil(t, gotFail.ResOpFailedEvent)
	assert.Equal(t, 3, gotFail.ResOpFailedEvent.Status)
	assert.Equal(t, 5, gotFail.ResOpFailedEvent.Steps)
}

func TestFilterChangesOnly_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	oldIn := map[string]any{"foo": "old", "bar": "stable"}
	newIn := map[string]any{"foo": "new", "bar": "stable"}
	originalOldOutputs := map[string]any{"o": 1}
	originalNewOutputs := map[string]any{"o": 2}

	evt := apitype.EngineEvent{
		Sequence:  1,
		Timestamp: 2,
		ResourcePreEvent: &apitype.ResourcePreEvent{
			Metadata: apitype.StepEventMetadata{
				Op:    apitype.OpUpdate,
				URN:   "urn",
				Diffs: []string{"foo"},
				Old:   &apitype.StepEventStateMetadata{Inputs: oldIn, Outputs: originalOldOutputs},
				New:   &apitype.StepEventStateMetadata{Inputs: newIn, Outputs: originalNewOutputs},
			},
		},
	}

	_ = filterChangesOnly(evt)

	// The originals must be untouched: filter callers expect a pure function.
	assert.Equal(t, map[string]any{"foo": "old", "bar": "stable"}, oldIn)
	assert.Equal(t, map[string]any{"foo": "new", "bar": "stable"}, newIn)
	assert.Equal(t, map[string]any{"o": 1}, originalOldOutputs)
	assert.Equal(t, map[string]any{"o": 2}, originalNewOutputs)
	assert.Equal(t, oldIn, evt.ResourcePreEvent.Metadata.Old.Inputs)
	assert.Equal(t, newIn, evt.ResourcePreEvent.Metadata.New.Inputs)
}

func TestRunChangesOnly_FiltersJSONLStream(t *testing.T) {
	t.Parallel()

	// Build an input stream that mixes non-resource events, a same-op resource event (the only
	// one that should be dropped), and a real update.
	events := []apitype.EngineEvent{
		{StdoutEvent: &apitype.StdoutEngineEvent{Message: "boot"}},
		resourcePreEvent(apitype.OpSame, nil, map[string]any{"x": 1}, map[string]any{"x": 1}),
		resourcePreEvent(apitype.OpUpdate, []string{"foo"},
			map[string]any{"foo": "old", "bar": "s"},
			map[string]any{"foo": "new", "bar": "s"}),
		{SummaryEvent: &apitype.SummaryEvent{}},
	}

	var input bytes.Buffer
	enc := json.NewEncoder(&input)
	for _, e := range events {
		require.NoError(t, enc.Encode(e))
	}

	var out bytes.Buffer
	require.NoError(t, runChangesOnly(&input, &out))

	// The stdout, update, and summary events survive; only the OpSame event is dropped.
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Len(t, lines, 3, "expected 3 surviving events, got: %s", out.String())

	var stdoutEvt apitype.EngineEvent
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &stdoutEvt))
	require.NotNil(t, stdoutEvt.StdoutEvent, "stdout event must pass through")
	assert.Equal(t, "boot", stdoutEvt.StdoutEvent.Message)

	var updateEvt apitype.EngineEvent
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &updateEvt))
	require.NotNil(t, updateEvt.ResourcePreEvent)
	assert.Equal(t, apitype.OpUpdate, updateEvt.ResourcePreEvent.Metadata.Op)
	assert.Equal(t, map[string]any{"foo": "old"}, updateEvt.ResourcePreEvent.Metadata.Old.Inputs)
	assert.Equal(t, map[string]any{"foo": "new"}, updateEvt.ResourcePreEvent.Metadata.New.Inputs)

	var summaryEvt apitype.EngineEvent
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &summaryEvt))
	require.NotNil(t, summaryEvt.SummaryEvent, "summary event must pass through")
}

func TestRunChangesOnly_EmptyStream(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, runChangesOnly(strings.NewReader(""), &out))
	assert.Empty(t, out.String())
}

func TestRunChangesOnly_MalformedJSONReturnsError(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := runChangesOnly(strings.NewReader("{not json}\n"), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding event")
}
