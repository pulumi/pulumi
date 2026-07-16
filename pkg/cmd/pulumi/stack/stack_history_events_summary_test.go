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

package stack

import (
	"bytes"
	"encoding/json"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	pkgdisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func eventSeq(events ...apitype.EngineEvent) iter.Seq2[apitype.EngineEvent, error] {
	return func(yield func(apitype.EngineEvent, error) bool) {
		for _, ev := range events {
			if !yield(ev, nil) {
				return
			}
		}
	}
}

func preEvent(ts int, op apitype.OpType, urn, typ string, m apitype.StepEventMetadata) apitype.EngineEvent {
	m.Op, m.URN, m.Type = op, urn, typ
	return apitype.EngineEvent{
		Timestamp:        ts,
		ResourcePreEvent: &apitype.ResourcePreEvent{Metadata: m},
	}
}

func TestBuildUpdateSummary(t *testing.T) {
	t.Parallel()

	events := eventSeq(
		apitype.EngineEvent{
			Timestamp:    1000,
			PreludeEvent: &apitype.PreludeEvent{Config: map[string]string{"proj:region": "us-west-2"}},
		},
		preEvent(1005, apitype.OpSame,
			"urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev", "pulumi:pulumi:Stack",
			apitype.StepEventMetadata{}),
		preEvent(1010, apitype.OpCreate,
			"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket", "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{
				New: &apitype.StepEventStateMetadata{Parent: "urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev"},
			}),
		preEvent(1020, apitype.OpUpdate,
			"urn:pulumi:dev::proj::aws:lambda/function:Function::f", "aws:lambda/function:Function",
			apitype.StepEventMetadata{}),
		apitype.EngineEvent{
			Timestamp: 1035,
			DiagnosticEvent: &apitype.DiagnosticEvent{
				URN:      "urn:pulumi:dev::proj::aws:rds/instance:Instance::db",
				Message:  "creation failed: quota exceeded\n",
				Severity: "error",
			},
		},
		apitype.EngineEvent{
			Timestamp:       1035,
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "deprecation notice", Severity: "warning"},
		},
		apitype.EngineEvent{
			Timestamp:       1035,
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "transient", Severity: "error", Ephemeral: true},
		},
		apitype.EngineEvent{
			Timestamp: 1036,
			ResOpFailedEvent: &apitype.ResOpFailedEvent{
				Metadata: apitype.StepEventMetadata{
					Op:   apitype.OpCreate,
					URN:  "urn:pulumi:dev::proj::aws:rds/instance:Instance::db",
					Type: "aws:rds/instance:Instance",
				},
			},
		},
		apitype.EngineEvent{
			Timestamp: 1040,
			SummaryEvent: &apitype.SummaryEvent{
				DurationSeconds: 40,
				Result:          apitype.OperationResultFailed,
				ResourceChanges: map[apitype.OpType]int{
					apitype.OpCreate: 1,
					apitype.OpUpdate: 1,
					apitype.OpSame:   1,
				},
			},
		},
	)

	s, err := buildUpdateSummary(events)
	require.NoError(t, err)

	assert.Equal(t, apitype.OperationResultFailed, s.Result)
	assert.Equal(t, 40*time.Second, s.Duration)
	assert.Equal(t, pkgdisplay.ResourceChanges{
		"create": 1,
		"update": 1,
		"same":   1,
	}, s.Summary)

	// Sames are omitted; the failed resource with no pre-event is appended.
	require.Len(t, s.Resources, 3)
	assert.Equal(t, "my-bucket", s.Resources[0].Name)
	assert.Equal(t, apitype.OpCreate, s.Resources[0].Op)
	assert.Equal(t, "urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev", s.Resources[0].Parent)
	assert.False(t, s.Resources[0].Failed)
	assert.Equal(t, "f", s.Resources[1].Name)
	assert.Equal(t, "db", s.Resources[2].Name)
	assert.True(t, s.Resources[2].Failed)

	// Only non-ephemeral errors survive; warnings are dropped.
	require.Len(t, s.Diagnostics, 1)
	assert.Equal(t, "error", s.Diagnostics[0].Severity)
	assert.Equal(t, "creation failed: quota exceeded", s.Diagnostics[0].Message)
	assert.Equal(t, s.Resources[2].URN, s.Diagnostics[0].URN)
}

// TestBuildUpdateSummary_BaseShapeMatchesLive is the compatibility guarantee
// the issue asks for: the emitted JSON must parse as a display.SummaryJSON,
// so tooling can treat live runs and historical lookups the same way.
func TestBuildUpdateSummary_BaseShapeMatchesLive(t *testing.T) {
	t.Parallel()

	s, err := buildUpdateSummary(eventSeq(
		preEvent(1, apitype.OpCreate,
			"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b", "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{}),
		apitype.EngineEvent{
			SummaryEvent: &apitype.SummaryEvent{
				DurationSeconds: 5,
				Result:          apitype.OperationResultSucceeded,
				ResourceChanges: map[apitype.OpType]int{apitype.OpCreate: 1},
			},
		},
	))
	require.NoError(t, err)

	encoded, err := json.Marshal(s)
	require.NoError(t, err)

	var live display.SummaryJSON
	require.NoError(t, json.Unmarshal(encoded, &live))
	assert.Equal(t, apitype.OperationResultSucceeded, live.Result)
	assert.Equal(t, 5*time.Second, live.Duration)
	assert.Equal(t, pkgdisplay.ResourceChanges{"create": 1}, live.Summary)
	require.Len(t, live.Resources, 1)
	assert.Equal(t, "b", live.Resources[0].Name)
	assert.Equal(t, apitype.OpType("create"), live.Resources[0].Op)
}

func TestBuildUpdateSummary_FailureMarksLastEntryForURN(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b"
	s, err := buildUpdateSummary(eventSeq(
		preEvent(1, apitype.OpCreate, urn, "aws:s3/bucket:Bucket", apitype.StepEventMetadata{}),
		apitype.EngineEvent{
			ResOpFailedEvent: &apitype.ResOpFailedEvent{
				Metadata: apitype.StepEventMetadata{Op: apitype.OpCreate, URN: urn, Type: "aws:s3/bucket:Bucket"},
			},
		},
	))
	require.NoError(t, err)

	// The failure marks the existing entry rather than appending a second one.
	require.Len(t, s.Resources, 1)
	assert.True(t, s.Resources[0].Failed)
	assert.Equal(t, apitype.OperationResultFailed, s.Result)
}

func TestBuildUpdateSummary_ResultFallbacks(t *testing.T) {
	t.Parallel()

	// No summary event, no failures: unknown.
	s, err := buildUpdateSummary(eventSeq())
	require.NoError(t, err)
	assert.Equal(t, apitype.OperationResult("unknown"), s.Result)

	// No summary event, but an error diagnostic: failed.
	s, err = buildUpdateSummary(eventSeq(apitype.EngineEvent{
		DiagnosticEvent: &apitype.DiagnosticEvent{Message: "boom", Severity: "error"},
	}))
	require.NoError(t, err)
	assert.Equal(t, apitype.OperationResultFailed, s.Result)

	// Summary event without a result (older updates): succeeded.
	s, err = buildUpdateSummary(eventSeq(apitype.EngineEvent{
		SummaryEvent: &apitype.SummaryEvent{DurationSeconds: 1},
	}))
	require.NoError(t, err)
	assert.Equal(t, apitype.OperationResultSucceeded, s.Result)
}

func TestBuildUpdateSummary_DurationFallsBackToTimestamps(t *testing.T) {
	t.Parallel()

	s, err := buildUpdateSummary(eventSeq(
		preEvent(1000, apitype.OpCreate,
			"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b", "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{}),
		apitype.EngineEvent{
			Timestamp:       1030,
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "boom", Severity: "error"},
		},
	))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, s.Duration)
}

func TestBuildUpdateSummary_PropagatesIteratorError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	events := func(yield func(apitype.EngineEvent, error) bool) {
		yield(apitype.EngineEvent{}, boom)
	}

	_, err := buildUpdateSummary(events)
	assert.ErrorIs(t, err, boom)
}

func TestRenderUpdateSummaryJSON_SingleLine(t *testing.T) {
	t.Parallel()

	s, err := buildUpdateSummary(eventSeq(apitype.EngineEvent{
		SummaryEvent: &apitype.SummaryEvent{Result: apitype.OperationResultSucceeded},
	}))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, renderUpdateSummaryJSON(&buf, s))

	// One line, like the live summary.
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("\n")))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "succeeded", decoded["result"])
}

func TestRenderUpdateSummaryText(t *testing.T) {
	t.Parallel()

	events := eventSeq(
		preEvent(1, apitype.OpCreate,
			"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b", "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{}),
		apitype.EngineEvent{
			ResOpFailedEvent: &apitype.ResOpFailedEvent{
				Metadata: apitype.StepEventMetadata{
					Op:   apitype.OpCreate,
					URN:  "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
					Type: "aws:s3/bucket:Bucket",
				},
			},
		},
		apitype.EngineEvent{
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "boom", Severity: "error"},
		},
		apitype.EngineEvent{
			SummaryEvent: &apitype.SummaryEvent{
				Result:          apitype.OperationResultFailed,
				ResourceChanges: map[apitype.OpType]int{apitype.OpCreate: 1},
			},
		},
	)
	s, err := buildUpdateSummary(events)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, renderUpdateSummaryText(&buf, s))
	out := buf.String()

	assert.Contains(t, out, "Result:   failed")
	assert.Contains(t, out, "1 create")
	assert.Contains(t, out, "create (failed)")
	assert.Contains(t, out, "error: boom")
}
