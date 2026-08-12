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
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockHistoryEventsClient returns a fixed sequence of responses, advancing
// through them on each call. This lets tests simulate paginated responses
// from the cloud.
type mockHistoryEventsClient struct {
	pages []apitype.GetUpdateEventsResponse
	err   error

	gotOpts []client.GetUpdateEngineEventsOptions
}

func (m *mockHistoryEventsClient) GetUpdateEngineEvents(
	_ context.Context, _ client.UpdateIdentifier, opts client.GetUpdateEngineEventsOptions,
) (apitype.GetUpdateEventsResponse, error) {
	m.gotOpts = append(m.gotOpts, opts)
	if m.err != nil {
		return apitype.GetUpdateEventsResponse{}, m.err
	}
	if len(m.gotOpts) > len(m.pages) {
		return apitype.GetUpdateEventsResponse{}, nil
	}
	return m.pages[len(m.gotOpts)-1], nil
}

func (m *mockHistoryEventsClient) calls() int { return len(m.gotOpts) }

func singlePage(events []apitype.EngineEvent) *mockHistoryEventsClient {
	return &mockHistoryEventsClient{pages: []apitype.GetUpdateEventsResponse{{Events: events}}}
}

func sampleEvents() []apitype.EngineEvent {
	return []apitype.EngineEvent{
		{
			Sequence:     1,
			Timestamp:    1700000000,
			PreludeEvent: &apitype.PreludeEvent{Config: map[string]string{}},
		},
		{
			Sequence:  2,
			Timestamp: 1700000001,
			DiagnosticEvent: &apitype.DiagnosticEvent{
				URN:      "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket",
				Message:  "boom",
				Severity: "error",
			},
		},
		{
			Sequence:  3,
			Timestamp: 1700000002,
			SummaryEvent: &apitype.SummaryEvent{
				DurationSeconds: 5,
				Result:          apitype.OperationResultSucceeded,
			},
		},
		{
			Sequence:    4,
			Timestamp:   1700000003,
			CancelEvent: &apitype.CancelEvent{},
		},
	}
}

// collectEvents drains an iter.Seq2 into slices for easier assertion.
func collectEvents(seq iter.Seq2[apitype.EngineEvent, error]) ([]apitype.EngineEvent, []error) {
	var events []apitype.EngineEvent
	var errs []error
	for ev, err := range seq {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		events = append(events, ev)
	}
	return events, errs
}

func TestIterateEngineEvents_DefaultStopsAfterOnePage(t *testing.T) {
	t.Parallel()

	tok := "tok-1"
	c := &mockHistoryEventsClient{
		pages: []apitype.GetUpdateEventsResponse{
			{Events: sampleEvents()[:2], ContinuationToken: &tok},
			{Events: sampleEvents()[2:]},
		},
	}

	events, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, client.GetUpdateEngineEventsOptions{},
		false /*all*/, 0 /*count*/))
	require.Empty(t, errs)
	assert.Equal(t, 1, c.calls())
	assert.Equal(t, sampleEvents()[:2], events)
}

func TestIterateEngineEvents_AllFollowsContinuationTokens(t *testing.T) {
	t.Parallel()

	tok1, tok2 := "tok-1", "tok-2"
	c := &mockHistoryEventsClient{
		pages: []apitype.GetUpdateEventsResponse{
			{Events: sampleEvents()[:2], ContinuationToken: &tok1},
			{Events: sampleEvents()[2:3], ContinuationToken: &tok2},
			{Events: sampleEvents()[3:]},
		},
	}

	events, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, client.GetUpdateEngineEventsOptions{},
		true /*all*/, 0 /*count*/))
	require.Empty(t, errs)

	assert.Equal(t, 3, c.calls())
	require.Len(t, c.gotOpts, 3)
	assert.Nil(t, c.gotOpts[0].ContinuationToken)
	require.NotNil(t, c.gotOpts[1].ContinuationToken)
	assert.Equal(t, "tok-1", *c.gotOpts[1].ContinuationToken)
	require.NotNil(t, c.gotOpts[2].ContinuationToken)
	assert.Equal(t, "tok-2", *c.gotOpts[2].ContinuationToken)

	assert.Equal(t, sampleEvents(), events)
}

func TestIterateEngineEvents_CountFetchesEnoughPages(t *testing.T) {
	t.Parallel()

	// Each page yields a single event; --count 3 must trigger three calls.
	tok1, tok2, tok3 := "tok-1", "tok-2", "tok-3"
	c := &mockHistoryEventsClient{
		pages: []apitype.GetUpdateEventsResponse{
			{Events: sampleEvents()[:1], ContinuationToken: &tok1},
			{Events: sampleEvents()[1:2], ContinuationToken: &tok2},
			{Events: sampleEvents()[2:3], ContinuationToken: &tok3},
			{Events: sampleEvents()[3:]},
		},
	}

	events, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, client.GetUpdateEngineEventsOptions{},
		false /*all*/, 3 /*count*/))
	require.Empty(t, errs)
	assert.Equal(t, 3, c.calls())
	assert.Equal(t, sampleEvents()[:3], events)
}

func TestIterateEngineEvents_CountStopsEarlyOnNilToken(t *testing.T) {
	t.Parallel()

	c := singlePage(sampleEvents()[:2])

	events, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, client.GetUpdateEngineEventsOptions{},
		false /*all*/, 10 /*count*/))
	require.Empty(t, errs)
	assert.Equal(t, 1, c.calls())
	assert.Equal(t, sampleEvents()[:2], events)
}

func TestIterateEngineEvents_PropagatesFilterOptions(t *testing.T) {
	t.Parallel()

	c := singlePage(sampleEvents())

	opts := client.GetUpdateEngineEventsOptions{
		EventTypes:          []string{"3", "5"},
		URN:                 "urn:pulumi:dev::p::pkg:m:Type::name",
		IncludeNonActivated: true,
	}
	_, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, opts, false, 0))
	require.Empty(t, errs)
	require.Equal(t, 1, c.calls())

	assert.Equal(t, []string{"3", "5"}, c.gotOpts[0].EventTypes)
	assert.Equal(t, "urn:pulumi:dev::p::pkg:m:Type::name", c.gotOpts[0].URN)
	assert.True(t, c.gotOpts[0].IncludeNonActivated)
	assert.Nil(t, c.gotOpts[0].ContinuationToken)
}

func TestIterateEngineEvents_PropagatesClientError(t *testing.T) {
	t.Parallel()

	c := &mockHistoryEventsClient{err: errors.New("server error")}
	_, errs := collectEvents(iterateEngineEvents(
		t.Context(), c, client.UpdateIdentifier{}, client.GetUpdateEngineEventsOptions{},
		false, 0))
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "getting update engine events")
	assert.Contains(t, errs[0].Error(), "server error")
}

// eventsFromSlice builds an iter.Seq2 yielding the given events with no
// errors so renderer tests don't need a mock client.
func eventsFromSlice(events []apitype.EngineEvent) iter.Seq2[apitype.EngineEvent, error] {
	return func(yield func(apitype.EngineEvent, error) bool) {
		for _, ev := range events {
			if !yield(ev, nil) {
				return
			}
		}
	}
}

func TestRenderHistoryEventsTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderHistoryEventsTable(&buf, eventsFromSlice(sampleEvents()))
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "SEQUENCE", "GET endpoint does not expose sequence")
	assert.Contains(t, out, "TIMESTAMP")
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "DETAILS")
	assert.Contains(t, out, "prelude")
	assert.Contains(t, out, "diagnostic/error")
	// Details column may wrap on narrow terminals; assert on the URN prefix
	// rather than the full diagnostic message.
	assert.Contains(t, out, "urn:pulumi:dev")
	assert.Contains(t, out, "summary")
	assert.Contains(t, out, "cancel")
	assert.Contains(t, out, "4 event(s)")
}

func TestRenderHistoryEventsTable_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderHistoryEventsTable(&buf, eventsFromSlice(nil))
	require.NoError(t, err)
	assert.Equal(t, "No events found for this update.\n", buf.String())
}

func TestRenderHistoryEventsJSON(t *testing.T) {
	t.Parallel()

	events := []apitype.EngineEvent{
		{
			Sequence:  1,
			Timestamp: 1700000000,
			DiagnosticEvent: &apitype.DiagnosticEvent{
				URN:      "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket",
				Message:  "boom",
				Severity: "error",
			},
		},
	}

	var buf bytes.Buffer
	err := renderHistoryEventsJSON(&buf, eventsFromSlice(events))
	require.NoError(t, err)

	assert.JSONEq(t, `[
		{
			"timestamp": 1700000000,
			"diagnosticEvent": {
				"urn": "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket",
				"message": "boom",
				"color": "",
				"severity": "error"
			}
		}
	]`, buf.String())
}

func TestRenderHistoryEventsJSON_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := renderHistoryEventsJSON(&buf, eventsFromSlice(nil))
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, buf.String())
}

func TestRenderHistoryEventsJSON_PropagatesIteratorError(t *testing.T) {
	t.Parallel()

	seq := iter.Seq2[apitype.EngineEvent, error](func(yield func(apitype.EngineEvent, error) bool) {
		yield(apitype.EngineEvent{}, assert.AnError)
	})

	var buf bytes.Buffer
	err := renderHistoryEventsJSON(&buf, seq)
	require.ErrorIs(t, err, assert.AnError)
}

func TestDescribeEngineEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		event       apitype.EngineEvent
		wantKind    string
		wantDetails string
	}{
		{
			name:     "cancel",
			event:    apitype.EngineEvent{CancelEvent: &apitype.CancelEvent{}},
			wantKind: "cancel",
		},
		{
			name:        "stdout",
			event:       apitype.EngineEvent{StdoutEvent: &apitype.StdoutEngineEvent{Message: "hello"}},
			wantKind:    "stdout",
			wantDetails: "hello",
		},
		{
			name: "diagnostic with urn",
			event: apitype.EngineEvent{DiagnosticEvent: &apitype.DiagnosticEvent{
				URN: "urn:x", Message: "bad", Severity: "warning",
			}},
			wantKind:    "diagnostic/warning",
			wantDetails: "urn:x: bad",
		},
		{
			name: "diagnostic without severity or urn",
			event: apitype.EngineEvent{DiagnosticEvent: &apitype.DiagnosticEvent{
				Message: "bad",
			}},
			wantKind:    "diagnostic",
			wantDetails: "bad",
		},
		{
			name: "error",
			event: apitype.EngineEvent{ErrorEvent: &apitype.ErrorEvent{
				Error: "engine exploded",
			}},
			wantKind:    "error",
			wantDetails: "engine exploded",
		},
		{
			name:     "unknown",
			event:    apitype.EngineEvent{},
			wantKind: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, details := describeEngineEvent(tc.event)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantDetails, details)
		})
	}
}
