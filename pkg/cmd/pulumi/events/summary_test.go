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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// encodeStream renders a slice of events as a JSONL stream, mirroring what `pulumi up --json`
// would write to disk or stdout. Kept next to the tests that need it because every test case
// builds its own stream.
func encodeStream(t *testing.T, events []apitype.EngineEvent) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, e := range events {
		require.NoError(t, enc.Encode(e))
	}
	return &buf
}

// rootStackURN is the canonical root-stack URN used in fixtures. The type segment
// (`pulumi:pulumi:Stack`) is what the summary uses to identify the stack's outputs.
const rootStackURN = "urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev"

func TestReduceEvents_EmptyStream(t *testing.T) {
	t.Parallel()

	// An empty stream must still produce a valid document: version set, everything else
	// zero-valued. Consumers can rely on `version` being present even when nothing happened.
	summary, err := reduceEvents(strings.NewReader(""))
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, summarySchemaVersion, summary.Version)
	assert.False(t, summary.IsPreview)
	assert.Empty(t, summary.Steps)
	assert.Empty(t, summary.Diagnostics)
}

func TestReduceEvents_PopulatesConfigFromPrelude(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{PreludeEvent: &apitype.PreludeEvent{Config: map[string]string{"proj:region": "us-west-2"}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"proj:region": "us-west-2"}, summary.Config)
}

func TestReduceEvents_IgnoresOpSameSteps(t *testing.T) {
	t.Parallel()

	// OpSame must never appear in Steps — the summary is about what changed, not about what
	// didn't. The user explicitly confirmed this decision on 2026-04-24.
	events := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpSame, URN: "urn:unchanged",
		}}},
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpSame, URN: "urn:unchanged",
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Empty(t, summary.Steps, "OpSame steps must not appear in the summary")
}

func TestReduceEvents_ResOutputsSupersedesResourcePre(t *testing.T) {
	t.Parallel()

	// A resource typically emits a ResourcePreEvent then a ResOutputsEvent. The latter arrives
	// with the completed step state, so the summary must reflect that — not the pre event.
	urn := "urn:pulumi:dev::proj::pkg:index:Res::r"
	events := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpUpdate, URN: urn, Type: "pkg:index:Res",
			Diffs: []string{"pre"},
		}}},
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpUpdate, URN: urn, Type: "pkg:index:Res",
			Diffs: []string{"post"},
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	require.Len(t, summary.Steps, 1)
	assert.Equal(t, []string{"post"}, summary.Steps[0].Diffs,
		"the later ResOutputsEvent state should win")
}

func TestReduceEvents_StepZerosOldAndNew(t *testing.T) {
	t.Parallel()

	// The summary stays compact by zeroing Old/New — full state is available in the raw
	// event stream for consumers that need it.
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpUpdate, URN: "urn:r", Type: "pkg:index:Res",
			Old: &apitype.StepEventStateMetadata{Inputs: map[string]any{"foo": "old"}},
			New: &apitype.StepEventStateMetadata{Inputs: map[string]any{"foo": "new"}},
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	require.Len(t, summary.Steps, 1)
	assert.Nil(t, summary.Steps[0].Old, "Old must be nil so the summary stays compact")
	assert.Nil(t, summary.Steps[0].New, "New must be nil so the summary stays compact")
}

func TestReduceEvents_CarriesDetailedDiffAndReplaceReasons(t *testing.T) {
	t.Parallel()

	detailed := map[string]apitype.PropertyDiff{
		"foo": {Kind: apitype.DiffUpdateReplace, InputDiff: true},
	}
	events := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpReplace, URN: "urn:r", Type: "pkg:index:Res",
			Diffs:        []string{"foo"},
			Keys:         []string{"foo"},
			DetailedDiff: detailed,
			Provider:     "urn:provider",
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	require.Len(t, summary.Steps, 1)
	s := summary.Steps[0]
	assert.Equal(t, apitype.OpReplace, s.Op)
	assert.Equal(t, []string{"foo"}, s.Keys)
	assert.Equal(t, detailed, s.DetailedDiff)
	assert.Equal(t, "urn:provider", s.Provider)
}

func TestReduceEvents_ResOpFailedSetsStepAndSummaryFailed(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:dev::proj::pkg:index:Res::r"
	events := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpCreate, URN: urn, Type: "pkg:index:Res",
		}}},
		{ResOpFailedEvent: &apitype.ResOpFailedEvent{
			Metadata: apitype.StepEventMetadata{
				Op: apitype.OpCreate, URN: urn, Type: "pkg:index:Res",
			},
		}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.True(t, summary.Failed, "ResOpFailedEvent must flip summary.Failed")
	require.Len(t, summary.Steps, 1)
	assert.True(t, summary.Steps[0].Failed, "the failed URN's step must be flagged")
}

func TestReduceEvents_CapturesRootStackOutputsFromNew(t *testing.T) {
	t.Parallel()

	outputs := map[string]any{"endpoint": "https://api.example.com"}
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op:   apitype.OpUpdate,
			URN:  rootStackURN,
			Type: string(tokens.RootStackType),
			New:  &apitype.StepEventStateMetadata{Outputs: outputs},
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Equal(t, outputs, summary.Outputs)
}

func TestReduceEvents_CapturesRootStackOutputsFromOldOnDestroy(t *testing.T) {
	t.Parallel()

	// On destroy the final root-stack step has no `New` (the stack is being deleted). The
	// outputs we care about are in `Old` — that's what the human saw printed before destroy.
	outputs := map[string]any{"endpoint": "https://api.example.com"}
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op:   apitype.OpDelete,
			URN:  rootStackURN,
			Type: string(tokens.RootStackType),
			Old:  &apitype.StepEventStateMetadata{Outputs: outputs},
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Equal(t, outputs, summary.Outputs,
		"destroy should surface the pre-destroy outputs from Old")
}

func TestReduceEvents_IgnoresOutputsFromNonRootResources(t *testing.T) {
	t.Parallel()

	// Non-root resource outputs must not end up in the summary's top-level Outputs — that
	// field is reserved for the stack outputs a user explicitly exports.
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op:   apitype.OpCreate,
			URN:  "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
			Type: "aws:s3/bucket:Bucket",
			New:  &apitype.StepEventStateMetadata{Outputs: map[string]any{"arn": "arn:aws:s3:::b"}},
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Nil(t, summary.Outputs, "only root-stack outputs should populate summary.Outputs")
}

func TestReduceEvents_DiagnosticsKeepsNonEphemeralNonDebug(t *testing.T) {
	t.Parallel()

	// Ephemeral diagnostics are progress-spinner text meant for live rendering only; they
	// must not show up in a persisted summary. Debug messages are just as noisy.
	events := []apitype.EngineEvent{
		{DiagnosticEvent: &apitype.DiagnosticEvent{
			URN: "urn:a", Message: "heads up", Severity: "warning",
		}},
		{DiagnosticEvent: &apitype.DiagnosticEvent{
			Message: "transient", Severity: "info", Ephemeral: true,
		}},
		{DiagnosticEvent: &apitype.DiagnosticEvent{Message: "verbose", Severity: "debug"}},
		{DiagnosticEvent: &apitype.DiagnosticEvent{
			URN: "urn:b", Message: "boom", Severity: "error",
		}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	require.Len(t, summary.Diagnostics, 2)
	assert.Equal(t, diag.Severity("warning"), summary.Diagnostics[0].Severity)
	assert.Equal(t, "heads up", summary.Diagnostics[0].Message)
	assert.Equal(t, resource.URN("urn:a"), summary.Diagnostics[0].URN)
	assert.Equal(t, diag.Severity("error"), summary.Diagnostics[1].Severity)
}

func TestReduceEvents_PolicyViolationsAndRemediationsGoToSeparateLists(t *testing.T) {
	t.Parallel()

	// Keeping the two event types in separate fields preserves the PolicyRemediationEvent's
	// before/after payload — a single fused list would've forced us to drop it.
	events := []apitype.EngineEvent{
		{PolicyEvent: &apitype.PolicyEvent{
			ResourceURN: "urn:a", PolicyName: "require-tag",
			PolicyPackName: "aws-best", PolicyPackVersion: "1.0.0",
			EnforcementLevel: "mandatory", Message: "missing tag",
		}},
		{PolicyRemediationEvent: &apitype.PolicyRemediationEvent{
			ResourceURN: "urn:b", PolicyName: "default-tag",
			PolicyPackName: "aws-best", PolicyPackVersion: "1.0.0",
			Before: map[string]any{"tag": nil},
			After:  map[string]any{"tag": "auto"},
		}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	require.Len(t, summary.PolicyViolations, 1)
	assert.Equal(t, "missing tag", summary.PolicyViolations[0].Message)
	require.Len(t, summary.PolicyRemediations, 1)
	assert.Equal(t, map[string]any{"tag": nil}, summary.PolicyRemediations[0].Before,
		"Before payload must be preserved on remediations")
	assert.Equal(t, map[string]any{"tag": "auto"}, summary.PolicyRemediations[0].After)
}

func TestReduceEvents_ErrorEventSetsFailedAndError(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{ErrorEvent: &apitype.ErrorEvent{Error: "engine exploded"}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.True(t, summary.Failed)
	assert.Equal(t, "engine exploded", summary.Error)
	assert.False(t, summary.Completed,
		"an ErrorEvent without a SummaryEvent means the run didn't reach the end-of-update handshake")
}

func TestReduceEvents_CompletedFlagTracksSummaryEvent(t *testing.T) {
	t.Parallel()

	// Without a SummaryEvent the run is "interrupted" — no positive signal of a clean end.
	noSummary := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpCreate, URN: "urn:r", Type: "pkg:index:Res",
		}}},
	}
	got, err := reduceEvents(encodeStream(t, noSummary))
	require.NoError(t, err)
	assert.False(t, got.Completed,
		"a stream without SummaryEvent must leave Completed=false — we can't tell success from interruption")

	// With a SummaryEvent, Completed is true. `Failed` stays independent so the consumer can
	// distinguish "completed cleanly and succeeded" from "completed with failures".
	withSummary := append(noSummary, apitype.EngineEvent{SummaryEvent: &apitype.SummaryEvent{}})
	got, err = reduceEvents(encodeStream(t, withSummary))
	require.NoError(t, err)
	assert.True(t, got.Completed,
		"a SummaryEvent in the stream must flip Completed — it's the engine's end-of-update handshake")
	assert.False(t, got.Failed, "SummaryEvent alone does not imply failure")
}

func TestReduceEvents_SummaryEventPopulatesScalarsAndChanges(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{SummaryEvent: &apitype.SummaryEvent{
			IsPreview:       false,
			DurationSeconds: 42,
			MaybeCorrupt:    true,
			ResourceChanges: map[apitype.OpType]int{
				apitype.OpCreate: 3,
				apitype.OpUpdate: 1,
				apitype.OpSame:   5,
			},
			PolicyPacks: map[string]string{"z-pack": "2", "a-pack": "1"},
		}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.False(t, summary.IsPreview)
	assert.True(t, summary.MaybeCorrupt)
	assert.Equal(t, 42*time.Second, summary.Duration)
	assert.Equal(t, display.ResourceChanges{
		display.StepOp(apitype.OpCreate): 3,
		display.StepOp(apitype.OpUpdate): 1,
		display.StepOp(apitype.OpSame):   5,
	}, summary.ChangeSummary)
	assert.Equal(t, map[string]string{"a-pack": "1", "z-pack": "2"}, summary.PolicyPacks)
}

func TestReduceEvents_PreviewLeavesOutputsEmpty(t *testing.T) {
	t.Parallel()

	// A preview's SummaryEvent has IsPreview=true and DurationSeconds=0. The stack hasn't been
	// realised, so there are no applied Outputs to capture.
	events := []apitype.EngineEvent{
		{SummaryEvent: &apitype.SummaryEvent{IsPreview: true, ResourceChanges: map[apitype.OpType]int{
			apitype.OpCreate: 1,
		}}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.True(t, summary.IsPreview)
	assert.Zero(t, summary.Duration)
	assert.Empty(t, summary.Outputs)
}

func TestReduceEvents_RecordsStartAndEndTimestamps(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{Sequence: 1, Timestamp: 1000, PreludeEvent: &apitype.PreludeEvent{}},
		{Sequence: 2, Timestamp: 1050, StdoutEvent: &apitype.StdoutEngineEvent{Message: "hi"}},
		{Sequence: 3, Timestamp: 1100, SummaryEvent: &apitype.SummaryEvent{}},
	}
	summary, err := reduceEvents(encodeStream(t, events))
	require.NoError(t, err)
	assert.Equal(t, 1000, summary.StartTime)
	assert.Equal(t, 1100, summary.EndTime)
}

func TestReduceEvents_MalformedJSONReturnsError(t *testing.T) {
	t.Parallel()

	_, err := reduceEvents(strings.NewReader("{not json}\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding event")
}

func TestRunSummary_EndToEndForUp(t *testing.T) {
	t.Parallel()

	// A realistic-ish stream for a small `pulumi up`: prelude, one create, one update, the
	// root stack's outputs, and a summary. Exercises the full pipeline including JSON
	// encoding.
	events := []apitype.EngineEvent{
		{Timestamp: 1000, PreludeEvent: &apitype.PreludeEvent{
			Config: map[string]string{"proj:region": "us-west-2"},
		}},
		{Timestamp: 1010, ResOutputsEvent: &apitype.ResOutputsEvent{
			Metadata: apitype.StepEventMetadata{
				Op: apitype.OpCreate, URN: "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
				Type: "aws:s3/bucket:Bucket",
			},
		}},
		{Timestamp: 1020, ResOutputsEvent: &apitype.ResOutputsEvent{
			Metadata: apitype.StepEventMetadata{
				Op: apitype.OpUpdate, URN: "urn:pulumi:dev::proj::aws:lambda/function:Function::f",
				Type:  "aws:lambda/function:Function",
				Diffs: []string{"code"},
			},
		}},
		{Timestamp: 1030, ResOutputsEvent: &apitype.ResOutputsEvent{
			Metadata: apitype.StepEventMetadata{
				Op: apitype.OpSame, URN: rootStackURN, Type: string(tokens.RootStackType),
				New: &apitype.StepEventStateMetadata{
					Outputs: map[string]any{"bucketName": "b"},
				},
			},
		}},
		{Timestamp: 1040, SummaryEvent: &apitype.SummaryEvent{
			DurationSeconds: 30,
			ResourceChanges: map[apitype.OpType]int{
				apitype.OpCreate: 1, apitype.OpUpdate: 1, apitype.OpSame: 1,
			},
		}},
	}

	var out bytes.Buffer
	require.NoError(t, runSummary(encodeStream(t, events), &out))

	// The wire format is a single indented JSON object — decode it and spot-check fields.
	var got display.EventSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, summarySchemaVersion, got.Version)
	assert.False(t, got.IsPreview)
	assert.Equal(t, map[string]string{"proj:region": "us-west-2"}, got.Config)
	assert.Equal(t, 30*time.Second, got.Duration)
	assert.Equal(t, map[string]any{"bucketName": "b"}, got.Outputs)
	// Even though the root-stack step is `OpSame`, the non-stack steps should appear.
	require.Len(t, got.Steps, 2)
	assert.Equal(t, apitype.OpCreate, got.Steps[0].Op)
	assert.Equal(t, apitype.OpUpdate, got.Steps[1].Op)
	assert.Equal(t, 1000, got.StartTime)
	assert.Equal(t, 1040, got.EndTime)
}

func TestRunSummary_EndToEndForPreview(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpCreate, URN: "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
			Type: "aws:s3/bucket:Bucket",
		}}},
		{SummaryEvent: &apitype.SummaryEvent{
			IsPreview:       true,
			ResourceChanges: map[apitype.OpType]int{apitype.OpCreate: 1},
		}},
	}

	var out bytes.Buffer
	require.NoError(t, runSummary(encodeStream(t, events), &out))

	var got display.EventSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.True(t, got.IsPreview)
	assert.Zero(t, got.Duration, "preview must not report a duration")
	assert.Empty(t, got.Outputs, "preview must not report outputs — nothing was applied")
	require.Len(t, got.Steps, 1)
}

func TestRunSummary_EndToEndForDestroy(t *testing.T) {
	t.Parallel()

	// Destroy streams delete the root stack; outputs come from Old. Every step is OpDelete.
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpDelete, URN: "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
			Type: "aws:s3/bucket:Bucket",
		}}},
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpDelete, URN: rootStackURN, Type: string(tokens.RootStackType),
			Old: &apitype.StepEventStateMetadata{
				Outputs: map[string]any{"bucketName": "b"},
			},
		}}},
		{SummaryEvent: &apitype.SummaryEvent{
			DurationSeconds: 5,
			ResourceChanges: map[apitype.OpType]int{apitype.OpDelete: 2},
		}},
	}

	var out bytes.Buffer
	require.NoError(t, runSummary(encodeStream(t, events), &out))

	var got display.EventSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, map[string]any{"bucketName": "b"}, got.Outputs,
		"destroy surfaces the pre-destroy outputs so a human can see what was there")
	assert.Equal(t, display.ResourceChanges{
		display.StepOp(apitype.OpDelete): 2,
	}, got.ChangeSummary)
}

func TestRunSummary_EndToEndForRefresh(t *testing.T) {
	t.Parallel()

	// Refresh streams consist of OpRefresh steps — they must be represented in the summary.
	events := []apitype.EngineEvent{
		{ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: apitype.StepEventMetadata{
			Op: apitype.OpRefresh, URN: "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
			Type:  "aws:s3/bucket:Bucket",
			Diffs: []string{"tags"},
		}}},
		{SummaryEvent: &apitype.SummaryEvent{
			DurationSeconds: 2,
			ResourceChanges: map[apitype.OpType]int{apitype.OpRefresh: 1},
		}},
	}

	var out bytes.Buffer
	require.NoError(t, runSummary(encodeStream(t, events), &out))

	var got display.EventSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Steps, 1)
	assert.Equal(t, apitype.OpRefresh, got.Steps[0].Op)
	assert.Equal(t, []string{"tags"}, got.Steps[0].Diffs)
}

func TestRunSummary_WritesIndentedJSON(t *testing.T) {
	t.Parallel()

	// The command's contract is "a single JSON document" — pretty-printed so that a human
	// redirecting stdout to a file gets readable output.
	var out bytes.Buffer
	require.NoError(t, runSummary(strings.NewReader(""), &out))
	assert.True(t, strings.Contains(out.String(), "\n  \""),
		"expected indented JSON, got: %s", out.String())
}
