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

// capturedScanCall records the arguments a single ScanInsightsAccount call
// received, so tests can assert that flags propagate correctly to the cloud
// client.
type capturedScanCall struct {
	org     string
	account string
	req     apitype.InsightsScanRequest
}

// mockScanClient stubs insightsAccountScanClient. Each instance returns a
// fixed response (or error) and records the most recent invocation.
type mockScanClient struct {
	resp     apitype.InsightsScanResponse
	err      error
	captured *capturedScanCall
}

func (m *mockScanClient) ScanInsightsAccount(
	_ context.Context, org, account string, req apitype.InsightsScanRequest,
) (apitype.InsightsScanResponse, error) {
	if m.captured != nil {
		*m.captured = capturedScanCall{org: org, account: account, req: req}
	}
	if m.err != nil {
		return apitype.InsightsScanResponse{}, m.err
	}
	return m.resp, nil
}

// stubScanFactory returns a scanClientFactory that always yields client and
// effectiveOrg. If orgOverride is non-empty, the override wins — matching
// production behaviour so per-call --org assertions still work in the
// cobra-level test.
func stubScanFactory(client insightsAccountScanClient, defaultOrg string) scanClientFactory {
	return func(_ context.Context, orgOverride string) (insightsAccountScanClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

// failingScanFactory returns a scanClientFactory that always errors. Useful to
// cover the not-logged-in / missing-org branches.
func failingScanFactory(err error) scanClientFactory {
	return func(_ context.Context, _ string) (insightsAccountScanClient, string, error) {
		return nil, "", err
	}
}

func sampleScanResponse() apitype.InsightsScanResponse {
	return apitype.InsightsScanResponse{
		ID:            "wf-123",
		OrgID:         "org-1",
		UserID:        "user-1",
		Status:        "running",
		StartedAt:     time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC),
		LastUpdatedAt: time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC),
		JobTimeout:    time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC),
	}
}

func TestInsightsAccountScanCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockScanClient{resp: sampleScanResponse()}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "ID:           wf-123")
	assert.Contains(t, output, "Status:       running")
	assert.Contains(t, output, "Started:      2026-05-13T10:00:00Z")
	assert.Contains(t, output, "Last updated: 2026-05-13T10:00:00Z")
	// FinishedAt is zero in the sample → must not be rendered, to avoid
	// suggesting "0001-01-01" is a real timestamp.
	assert.NotContains(t, output, "Finished:")
	// No jobs in the sample → no "Jobs:" header.
	assert.NotContains(t, output, "Jobs:")
}

func TestInsightsAccountScanCmd_DefaultOutput_Finished(t *testing.T) {
	t.Parallel()

	resp := sampleScanResponse()
	resp.Status = "succeeded"
	resp.FinishedAt = time.Date(2026, 5, 13, 10, 5, 0, 0, time.UTC)
	client := &mockScanClient{resp: resp}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Status:       succeeded")
	assert.Contains(t, output, "Finished:     2026-05-13T10:05:00Z")
}

func TestInsightsAccountScanCmd_DefaultOutput_WithJobs(t *testing.T) {
	t.Parallel()

	resp := sampleScanResponse()
	resp.Jobs = []apitype.InsightsScanJobRun{
		{
			Status:  "running",
			Timeout: int64(time.Hour),
			Steps: []apitype.InsightsScanStepRun{
				{Name: "list", Status: "running"},
				{Name: "read", Status: "not-started"},
			},
		},
		{
			Status:  "not-started",
			Timeout: int64(time.Hour),
			Steps:   []apitype.InsightsScanStepRun{{Name: "list", Status: "not-started"}},
		},
	}
	client := &mockScanClient{resp: resp}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Jobs:")
	assert.Contains(t, output, "Job 1: running")
	assert.Contains(t, output, "    - list (running)")
	assert.Contains(t, output, "    - read (not-started)")
	assert.Contains(t, output, "Job 2: not-started")
}

func TestInsightsAccountScanCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	client := &mockScanClient{resp: sampleScanResponse()}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{output: "json"})
	require.NoError(t, err)

	var got apitype.InsightsScanResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleScanResponse(), got)
}

func TestInsightsAccountScanCmd_RequestBodyPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args insightsAccountScanArgs
		want apitype.InsightsScanRequest
	}{
		{
			name: "no tuning",
			args: insightsAccountScanArgs{},
			want: apitype.InsightsScanRequest{},
		},
		{
			name: "all tuning flags",
			args: insightsAccountScanArgs{
				agentPoolID:     "pool-1",
				listConcurrency: 8,
				readConcurrency: 16,
				batchSize:       100,
				readTimeout:     "30s",
			},
			want: apitype.InsightsScanRequest{
				AgentPoolID:     "pool-1",
				ListConcurrency: 8,
				ReadConcurrency: 16,
				BatchSize:       100,
				ReadTimeout:     "30s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured capturedScanCall
			client := &mockScanClient{resp: sampleScanResponse(), captured: &captured}
			c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, "prod-aws", tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, captured.req)
		})
	}
}

func TestInsightsAccountScanCmd_OrgOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       insightsAccountScanArgs
		defaultOrg string
		wantOrg    string
	}{
		{
			name:       "default org used when --org omitted",
			args:       insightsAccountScanArgs{},
			defaultOrg: "acme",
			wantOrg:    "acme",
		},
		{
			name:       "--org overrides default",
			args:       insightsAccountScanArgs{org: "other-co"},
			defaultOrg: "acme",
			wantOrg:    "other-co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured capturedScanCall
			client := &mockScanClient{resp: sampleScanResponse(), captured: &captured}
			c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, tt.defaultOrg)}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, "prod-aws", tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, captured.org)
			assert.Equal(t, "prod-aws", captured.account)
		})
	}
}

func TestInsightsAccountScanCmd_InvalidOutput(t *testing.T) {
	t.Parallel()

	client := &mockScanClient{resp: sampleScanResponse()}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{output: "yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestInsightsAccountScanCmd_TuningValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    insightsAccountScanArgs
		wantErr string
	}{
		{
			name:    "negative list-concurrency rejected",
			args:    insightsAccountScanArgs{listConcurrency: -1},
			wantErr: "--list-concurrency",
		},
		{
			name:    "negative read-concurrency rejected",
			args:    insightsAccountScanArgs{readConcurrency: -1},
			wantErr: "--read-concurrency",
		},
		{
			name:    "negative batch-size rejected",
			args:    insightsAccountScanArgs{batchSize: -1},
			wantErr: "--batch-size",
		},
		{
			name:    "malformed read-timeout rejected",
			args:    insightsAccountScanArgs{readTimeout: "not-a-duration"},
			wantErr: "invalid --read-timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &mockScanClient{resp: sampleScanResponse()}
			c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, "prod-aws", tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInsightsAccountScanCmd_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockScanClient{err: errors.New("404 not found")}
	c := &insightsAccountScanCmd{clientFactory: stubScanFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "missing", insightsAccountScanArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "starting insights scan")
	assert.Contains(t, err.Error(), "404 not found")
}

func TestInsightsAccountScanCmd_FactoryError(t *testing.T) {
	t.Parallel()

	c := &insightsAccountScanCmd{clientFactory: failingScanFactory(errors.New("not logged in"))}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", insightsAccountScanArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestNewInsightsAccountScanCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedScanCall
	client := &mockScanClient{resp: sampleScanResponse(), captured: &captured}
	cmd := newInsightsAccountScanCmd(stubScanFactory(client, "acme"))
	assert.Equal(t, "scan", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--agent-pool", "pool-1",
		"--list-concurrency", "8",
		"--read-concurrency", "16",
		"--batch-size", "100",
		"--read-timeout", "30s",
		"--output", "json",
		"prod-aws",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, capturedScanCall{
		org:     "other-co",
		account: "prod-aws",
		req: apitype.InsightsScanRequest{
			AgentPoolID:     "pool-1",
			ListConcurrency: 8,
			ReadConcurrency: 16,
			BatchSize:       100,
			ReadTimeout:     "30s",
		},
	}, captured)

	var got apitype.InsightsScanResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleScanResponse(), got)
}

func TestNewInsightsAccountScanCmd_RegistersScanLog(t *testing.T) {
	t.Parallel()

	cmd := newInsightsAccountScanCmd(stubScanFactory(&mockScanClient{}, "acme"))
	// The scan-log scaffold lives under `scan`; verify the wiring survives the
	// switch from scaffold to real implementation.
	logCmd, _, err := cmd.Find([]string{"log"})
	require.NoError(t, err)
	require.NotNil(t, logCmd)
	assert.Equal(t, "log", logCmd.Name())
}

func TestNewInsightsAccountScanCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory; we only check the command is
	// well-formed without invoking it, since the default factory would hit the
	// real cloud context.
	cmd := newInsightsAccountScanCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "scan", cmd.Name())
}
