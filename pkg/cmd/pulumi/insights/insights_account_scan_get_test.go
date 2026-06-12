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

package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedScanGetCall struct {
	org     string
	account string
	scanID  string
}

type mockInsightsAccountScanGetClient struct {
	resp  apitype.InsightsScanResponse
	calls []capturedScanGetCall
	err   error
}

func (m *mockInsightsAccountScanGetClient) GetInsightsScan(
	_ context.Context, org, account, scanID string,
) (apitype.InsightsScanResponse, error) {
	m.calls = append(m.calls, capturedScanGetCall{org: org, account: account, scanID: scanID})
	if m.err != nil {
		return apitype.InsightsScanResponse{}, m.err
	}
	return m.resp, nil
}

func stubScanGetFactory(
	client insightsAccountScanGetClient, defaultOrg string,
) accountScanGetClientFactory {
	return func(_ context.Context, orgOverride string) (insightsAccountScanGetClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

func failingScanGetFactory(err error) accountScanGetClientFactory {
	return func(_ context.Context, _ string) (insightsAccountScanGetClient, string, error) {
		return nil, "", err
	}
}

func defaultScanGetArgs() insightsAccountScanGetArgs {
	return insightsAccountScanGetArgs{output: defaultAccountScanGetOutputFormat()}
}

func withScanGetOutput(args insightsAccountScanGetArgs, format string) insightsAccountScanGetArgs {
	if err := args.output.Set(format); err != nil {
		panic(err)
	}
	return args
}

// fullScanResponse returns a populated InsightsScanResponse with a single job
// and a few steps — the typical shape for a completed account scan.
func fullScanResponse() apitype.InsightsScanResponse {
	started := time.Date(2026, 1, 21, 15, 58, 47, 0, time.UTC)
	finished := time.Date(2026, 1, 21, 16, 43, 16, 0, time.UTC)
	return apitype.InsightsScanResponse{
		ID:            "scan-1",
		OrgID:         "org-1",
		UserID:        "user-1",
		Status:        "succeeded",
		StartedAt:     started,
		FinishedAt:    finished,
		LastUpdatedAt: finished,
		Jobs: []apitype.InsightsScanJobRun{{
			Status:      "succeeded",
			Started:     started,
			LastUpdated: finished,
			Timeout:     int64(12 * time.Hour),
			Steps: []apitype.InsightsScanStepRun{
				{Name: "Setup", Status: "succeeded"},
				{Name: "Scan account", Status: "succeeded"},
			},
		}},
	}
}

// TestInsightsAccountScanGetCmd_DefaultOutput covers the human-readable
// summary. We deliberately mirror `deployment get`'s "Jobs: <count>" style:
// the per-job/per-step breakdown lives in --output json so the text view
// stays scan-summary at-a-glance.
func TestInsightsAccountScanGetCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanGetClient{resp: fullScanResponse()}
	c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws/us-east-1", "scan-1", defaultScanGetArgs())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "ID:")
	assert.Contains(t, output, "scan-1")
	assert.Contains(t, output, "Status:")
	assert.Contains(t, output, "succeeded")
	assert.Contains(t, output, "Started:")
	assert.Contains(t, output, "2026-01-21T15:58:47Z")
	assert.Contains(t, output, "Finished:")
	assert.Contains(t, output, "2026-01-21T16:43:16Z")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "44m29s")
	assert.Contains(t, output, "Last updated:")
	assert.Contains(t, output, "Jobs:")
	assert.Contains(t, output, " 1")

	// Per-step breakdown is intentionally NOT in the text view — match
	// `deployment get`. Catches accidental re-introduction of step lines.
	assert.NotContains(t, output, "Setup")
	assert.NotContains(t, output, "Scan account")
	assert.NotContains(t, output, "Job 1:")
}

// TestInsightsAccountScanGetCmd_InFlight covers an in-flight scan: FinishedAt
// is zero, so the Finished / Duration lines are omitted (avoids
// "Finished: 0001-01-01" rendering as if the run completed in 1AD).
func TestInsightsAccountScanGetCmd_InFlight(t *testing.T) {
	t.Parallel()

	resp := fullScanResponse()
	resp.Status = "running"
	resp.FinishedAt = time.Time{}
	resp.Jobs[0].Status = "running"

	client := &mockInsightsAccountScanGetClient{resp: resp}
	c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, "acme")}

	var out bytes.Buffer
	require.NoError(t, c.Run(t.Context(), &out, "prod-aws/us-east-1", "scan-1", defaultScanGetArgs()))

	output := out.String()
	assert.Contains(t, output, "running")
	assert.NotContains(t, output, "Finished:")
	assert.NotContains(t, output, "Duration:")
	assert.NotContains(t, output, "0001-01-01")
}

func TestInsightsAccountScanGetCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	want := fullScanResponse()
	client := &mockInsightsAccountScanGetClient{resp: want}
	c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out,
		"prod-aws/us-east-1", "scan-1",
		withScanGetOutput(defaultScanGetArgs(), "json"))
	require.NoError(t, err)

	// Decode into the envelope so we can assert the normalised shape.
	var got scanGetJSON
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Status, got.Status)
	assert.Equal(t, want.StartedAt.UTC(), got.StartedAt.UTC())
	require.Len(t, got.Jobs, 1)
	require.Len(t, got.Jobs[0].Steps, 2)
	assert.Equal(t, "Setup", got.Jobs[0].Steps[0].Name)
}

// TestInsightsAccountScanGetCmd_JSONOutput_EmptyJobs ensures a nil Jobs slice
// serialises to `[]` rather than `null` — matches the contract that other
// insights commands keep so jq scripts can iterate without a nil-check.
func TestInsightsAccountScanGetCmd_JSONOutput_EmptyJobs(t *testing.T) {
	t.Parallel()

	resp := fullScanResponse()
	resp.Jobs = nil

	client := &mockInsightsAccountScanGetClient{resp: resp}
	c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, "acme")}

	var out bytes.Buffer
	require.NoError(t, c.Run(t.Context(), &out,
		"prod-aws/us-east-1", "scan-1",
		withScanGetOutput(defaultScanGetArgs(), "json")))

	// Round-trip through generic map to catch `"jobs": null` regressions.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &raw))
	jobs, ok := raw["jobs"]
	require.True(t, ok)
	require.NotNil(t, jobs)
	assert.Empty(t, jobs)
}

func TestInsightsAccountScanGetCmd_OrgAndArgsPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(*insightsAccountScanGetArgs)
		defaultOrg  string
		account     string
		scanID      string
		wantOrg     string
		wantAccount string
		wantScanID  string
	}{
		{
			name:        "default org used when --org omitted",
			defaultOrg:  "acme",
			account:     "prod-aws",
			scanID:      "scan-1",
			wantOrg:     "acme",
			wantAccount: "prod-aws",
			wantScanID:  "scan-1",
		},
		{
			name:        "--org overrides default",
			mutate:      func(a *insightsAccountScanGetArgs) { a.org = "other-co" },
			defaultOrg:  "acme",
			account:     "prod-aws",
			scanID:      "scan-1",
			wantOrg:     "other-co",
			wantAccount: "prod-aws",
			wantScanID:  "scan-1",
		},
		{
			name:        "account names with slashes pass through unmodified",
			defaultOrg:  "acme",
			account:     "prod-aws/us-east-1",
			scanID:      "scan-1",
			wantOrg:     "acme",
			wantAccount: "prod-aws/us-east-1",
			wantScanID:  "scan-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInsightsAccountScanGetClient{resp: fullScanResponse()}
			c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, tc.defaultOrg)}

			args := defaultScanGetArgs()
			if tc.mutate != nil {
				tc.mutate(&args)
			}

			var out bytes.Buffer
			require.NoError(t, c.Run(t.Context(), &out, tc.account, tc.scanID, args))
			require.Len(t, client.calls, 1)
			assert.Equal(t, tc.wantOrg, client.calls[0].org)
			assert.Equal(t, tc.wantAccount, client.calls[0].account)
			assert.Equal(t, tc.wantScanID, client.calls[0].scanID)
		})
	}
}

func TestInsightsAccountScanGetCmd_FactoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("not logged in")
	c := &insightsAccountScanGetCmd{clientFactory: failingScanGetFactory(wantErr)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", "scan-1", defaultScanGetArgs())
	require.ErrorIs(t, err, wantErr)
}

func TestInsightsAccountScanGetCmd_ClientErrorWrapped(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("[404] Not Found")
	client := &mockInsightsAccountScanGetClient{err: wantErr}
	c := &insightsAccountScanGetCmd{clientFactory: stubScanGetFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", "scan-1", defaultScanGetArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting insights scan")
	assert.ErrorIs(t, err, wantErr)
}
