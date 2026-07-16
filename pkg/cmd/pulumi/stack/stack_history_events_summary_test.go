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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func outputsEvent(ts int, op apitype.OpType, urn, typ string, m apitype.StepEventMetadata) apitype.EngineEvent {
	m.Op, m.URN, m.Type = op, urn, typ
	return apitype.EngineEvent{
		Timestamp:       ts,
		ResOutputsEvent: &apitype.ResOutputsEvent{Metadata: m},
	}
}

func TestBuildUpdateSummary(t *testing.T) {
	t.Parallel()

	events := eventSeq(
		apitype.EngineEvent{
			Timestamp:    1000,
			PreludeEvent: &apitype.PreludeEvent{Config: map[string]string{"proj:region": "us-west-2"}},
		},
		outputsEvent(1010, apitype.OpCreate,
			"urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket", "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{}),
		outputsEvent(1020, apitype.OpUpdate,
			"urn:pulumi:dev::proj::aws:lambda/function:Function::f", "aws:lambda/function:Function",
			apitype.StepEventMetadata{
				DetailedDiff: map[string]apitype.PropertyDiff{
					"code":    {Kind: apitype.DiffUpdate},
					"tags":    {Kind: apitype.DiffAdd},
					"handler": {Kind: apitype.DiffDelete},
				},
			}),
		outputsEvent(1025, apitype.OpSame,
			"urn:pulumi:dev::proj::aws:iam/role:Role::r", "aws:iam/role:Role",
			apitype.StepEventMetadata{}),
		outputsEvent(1030, apitype.OpSame,
			"urn:pulumi:dev::proj::pulumi:pulumi:Stack::proj-dev", "pulumi:pulumi:Stack",
			apitype.StepEventMetadata{
				New: &apitype.StepEventStateMetadata{Outputs: map[string]any{"bucketName": "b"}},
			}),
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
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "noisy", Severity: "debug"},
		},
		apitype.EngineEvent{
			Timestamp:       1035,
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "spinner", Severity: "info", Ephemeral: true},
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
			Timestamp: 1037,
			PolicyEvent: &apitype.PolicyEvent{
				ResourceURN:      "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket",
				PolicyName:       "no-public-buckets",
				PolicyPackName:   "aws-guard",
				EnforcementLevel: "mandatory",
				Message:          "bucket must not be public",
			},
		},
		apitype.EngineEvent{
			Timestamp: 1040,
			SummaryEvent: &apitype.SummaryEvent{
				DurationSeconds: 40,
				Result:          apitype.OperationResultFailed,
			},
		},
	)

	s, err := buildUpdateSummary("acme/proj/dev", events)
	require.NoError(t, err)

	assert.Equal(t, 1, s.Version)
	assert.Equal(t, "acme/proj/dev", s.Stack)
	assert.Equal(t, "failed", s.Status)
	assert.Empty(t, s.Operation)
	assert.Equal(t, "1970-01-01T00:16:40Z", s.StartedAt)
	assert.Equal(t, "1970-01-01T00:17:20Z", s.CompletedAt)
	assert.Equal(t, int64(40000), s.DurationMs)

	assert.Equal(t, map[string]int{
		"created":   1,
		"updated":   1,
		"unchanged": 2,
		"failed":    1,
	}, s.Summary)

	// Unchanged resources are counted but not listed.
	require.Len(t, s.Resources, 3)
	assert.Equal(t, "my-bucket", s.Resources[0].Name)
	assert.Equal(t, "created", s.Resources[0].Change)
	assert.Equal(t, map[string]propertyChange{
		"code":    {Kind: "updated"},
		"tags":    {Kind: "added"},
		"handler": {Kind: "deleted"},
	}, s.Resources[1].PropertyChanges)
	assert.Equal(t, "failed", s.Resources[2].Change)

	assert.Equal(t, map[string]any{"bucketName": "b"}, s.Outputs)

	// Debug and ephemeral diagnostics are dropped.
	require.Len(t, s.Diagnostics, 1)
	assert.Equal(t, "error", s.Diagnostics[0].Severity)
	assert.Equal(t, "creation failed: quota exceeded", s.Diagnostics[0].Message)

	require.Len(t, s.PolicyViolations, 1)
	assert.Equal(t, "aws-guard", s.PolicyViolations[0].PolicyPack)
	assert.Equal(t, "no-public-buckets", s.PolicyViolations[0].Policy)
}

func TestBuildUpdateSummary_ReplacePairCollapses(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b"
	events := eventSeq(
		outputsEvent(1, apitype.OpCreateReplacement, urn, "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{Diffs: []string{"bucketName"}}),
		outputsEvent(2, apitype.OpDeleteReplaced, urn, "aws:s3/bucket:Bucket",
			apitype.StepEventMetadata{}),
	)

	s, err := buildUpdateSummary("acme/proj/dev", events)
	require.NoError(t, err)

	assert.Equal(t, map[string]int{"replaced": 1}, s.Summary)
	require.Len(t, s.Resources, 1)
	assert.Equal(t, "replaced", s.Resources[0].Change)
	assert.Equal(t, map[string]propertyChange{"bucketName": {Kind: "updated"}}, s.Resources[0].PropertyChanges)
}

func TestBuildUpdateSummary_FailedWinsOverLaterSteps(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b"
	events := eventSeq(
		apitype.EngineEvent{
			Timestamp: 1,
			ResOpFailedEvent: &apitype.ResOpFailedEvent{
				Metadata: apitype.StepEventMetadata{Op: apitype.OpCreate, URN: urn, Type: "aws:s3/bucket:Bucket"},
			},
		},
		outputsEvent(2, apitype.OpCreate, urn, "aws:s3/bucket:Bucket", apitype.StepEventMetadata{}),
	)

	s, err := buildUpdateSummary("acme/proj/dev", events)
	require.NoError(t, err)
	require.Len(t, s.Resources, 1)
	assert.Equal(t, "failed", s.Resources[0].Change)
	assert.Equal(t, "failed", s.Status)
}

func TestBuildUpdateSummary_StatusFallbacks(t *testing.T) {
	t.Parallel()

	// No summary event, no failures: unknown.
	s, err := buildUpdateSummary("acme/proj/dev", eventSeq())
	require.NoError(t, err)
	assert.Equal(t, "unknown", s.Status)
	require.NotNil(t, s.Resources)

	// No summary event, but an error diagnostic: failed.
	s, err = buildUpdateSummary("acme/proj/dev", eventSeq(apitype.EngineEvent{
		DiagnosticEvent: &apitype.DiagnosticEvent{Message: "boom", Severity: "error"},
	}))
	require.NoError(t, err)
	assert.Equal(t, "failed", s.Status)

	// Summary event without a result (older updates): succeeded.
	s, err = buildUpdateSummary("acme/proj/dev", eventSeq(apitype.EngineEvent{
		SummaryEvent: &apitype.SummaryEvent{DurationSeconds: 1},
	}))
	require.NoError(t, err)
	assert.Equal(t, "succeeded", s.Status)

	// Preview bit is the only operation we can derive.
	s, err = buildUpdateSummary("acme/proj/dev", eventSeq(apitype.EngineEvent{
		SummaryEvent: &apitype.SummaryEvent{IsPreview: true, Result: apitype.OperationResultSucceeded},
	}))
	require.NoError(t, err)
	assert.Equal(t, "preview", s.Operation)
}

func TestBuildUpdateSummary_PropagatesIteratorError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	events := func(yield func(apitype.EngineEvent, error) bool) {
		yield(apitype.EngineEvent{}, boom)
	}

	_, err := buildUpdateSummary("acme/proj/dev", events)
	assert.ErrorIs(t, err, boom)
}

func TestRenderUpdateSummaryJSON(t *testing.T) {
	t.Parallel()

	s, err := buildUpdateSummary("acme/proj/dev", eventSeq(apitype.EngineEvent{
		SummaryEvent: &apitype.SummaryEvent{Result: apitype.OperationResultSucceeded},
	}))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, renderUpdateSummaryJSON(&buf, s))

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "succeeded", decoded["status"])
	assert.Equal(t, "acme/proj/dev", decoded["stack"])
	// resources must serialize as [], not null, so consumers can rely on it.
	assert.Equal(t, []any{}, decoded["resources"])
}

func TestRenderUpdateSummaryText(t *testing.T) {
	t.Parallel()

	events := eventSeq(
		outputsEvent(1, apitype.OpUpdate,
			"urn:pulumi:dev::proj::aws:lambda/function:Function::f", "aws:lambda/function:Function",
			apitype.StepEventMetadata{
				DetailedDiff: map[string]apitype.PropertyDiff{"code": {Kind: apitype.DiffUpdate}},
			}),
		apitype.EngineEvent{
			DiagnosticEvent: &apitype.DiagnosticEvent{Message: "boom", Severity: "error"},
		},
	)
	s, err := buildUpdateSummary("acme/proj/dev", events)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, renderUpdateSummaryText(&buf, s))
	out := buf.String()

	assert.Contains(t, out, "Status:   failed")
	assert.Contains(t, out, "1 updated")
	assert.Contains(t, out, "code (updated)")
	assert.Contains(t, out, "error: boom")
}
