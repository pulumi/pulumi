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

// capturedScanListCall records the most recent call to
// ListInsightsAccountScans so tests can assert that flags propagate correctly
// to the cloud client.
type capturedScanListCall struct {
	org     string
	account string
	params  apitype.ListInsightsAccountScansParams
}

// mockInsightsAccountScanListClient stubs insightsAccountScanListClient.
// Successive invocations return the next entry of `pages`, simulating
// server-side pagination via continuationToken.
type mockInsightsAccountScanListClient struct {
	pages []apitype.ListInsightsAccountScansResponse
	calls []capturedScanListCall
	err   error
}

func (m *mockInsightsAccountScanListClient) ListInsightsAccountScans(
	_ context.Context, org, account string, params apitype.ListInsightsAccountScansParams,
) (apitype.ListInsightsAccountScansResponse, error) {
	m.calls = append(m.calls, capturedScanListCall{org: org, account: account, params: params})
	if m.err != nil {
		return apitype.ListInsightsAccountScansResponse{}, m.err
	}
	if len(m.pages) == 0 {
		return apitype.ListInsightsAccountScansResponse{}, nil
	}
	resp := m.pages[0]
	m.pages = m.pages[1:]
	return resp, nil
}

func stubScanListFactory(
	client insightsAccountScanListClient, defaultOrg string,
) accountScanListClientFactory {
	return func(_ context.Context, orgOverride string) (insightsAccountScanListClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

func failingScanListFactory(err error) accountScanListClientFactory {
	return func(_ context.Context, _ string) (insightsAccountScanListClient, string, error) {
		return nil, "", err
	}
}

func defaultScanListArgs() insightsAccountScanListArgs {
	return insightsAccountScanListArgs{output: defaultAccountScanListOutputFormat()}
}

func withScanListOutput(args insightsAccountScanListArgs, format string) insightsAccountScanListArgs {
	if err := args.output.Set(format); err != nil {
		panic(err)
	}
	return args
}

// sampleScan returns a fully populated scan status. `id` doubles as a sort key
// so tests can assert ordering trivially.
func sampleScan(id, account string) apitype.InsightsAccountScanStatus {
	started := time.Date(2026, 1, 21, 15, 58, 47, 0, time.UTC)
	finished := time.Date(2026, 1, 21, 16, 43, 16, 0, time.UTC)
	return apitype.InsightsAccountScanStatus{
		ID:            id,
		OrgID:         "org-1",
		UserID:        "user-1",
		AccountName:   account,
		Status:        "succeeded",
		StartedAt:     started,
		FinishedAt:    &finished,
		LastUpdatedAt: finished,
		JobTimeout:    started.Add(12 * time.Hour),
		ResourceCount: 42,
	}
}

func TestInsightsAccountScanListCmd_DefaultOutput(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{{
			ScanStatuses: []apitype.InsightsAccountScanStatus{
				sampleScan("scan-1", "prod-aws/us-east-1"),
				sampleScan("scan-2", "prod-aws/us-west-2"),
			},
		}},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", defaultScanListArgs())
	require.NoError(t, err)

	output := out.String()
	// Headers
	assert.Contains(t, output, "Scan ID")
	assert.Contains(t, output, "Account")
	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "Started")
	assert.Contains(t, output, "Duration")
	// Finished column was intentionally dropped — Started + Duration tells the
	// same story without the extra column.
	assert.NotContains(t, output, "Finished")
	// Row contents
	assert.Contains(t, output, "scan-1")
	assert.Contains(t, output, "scan-2")
	assert.Contains(t, output, "prod-aws/us-east-1")
	assert.Contains(t, output, "succeeded")
	assert.Contains(t, output, "2026-01-21T15:58:47Z") // started
	assert.Contains(t, output, "44m29s")               // duration
}

func TestInsightsAccountScanListCmd_DefaultOutput_NoResults(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{{ScanStatuses: nil}},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", defaultScanListArgs())
	require.NoError(t, err)
	assert.Equal(t, "No scans found.\n", out.String())
}

// TestInsightsAccountScanListCmd_InFlight covers the row layout for a scan
// that is still running — Duration renders as `-` because we deliberately do
// not compute "now - StartedAt" (a moving target on a live re-run).
func TestInsightsAccountScanListCmd_InFlight(t *testing.T) {
	t.Parallel()

	inflight := sampleScan("running-scan", "prod-aws/us-east-1")
	inflight.Status = "running"
	inflight.FinishedAt = nil

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{{
			ScanStatuses: []apitype.InsightsAccountScanStatus{inflight},
		}},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", defaultScanListArgs())
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "running-scan")
	assert.Contains(t, output, "running")
	// Started is still concrete, but Duration is `-` because the scan hasn't
	// finished. We assert the literal " - " filler appears so a future change
	// to compute a moving "now-Started" gets noticed.
	assert.Contains(t, output, " - ")
}

func TestInsightsAccountScanListCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	want := []apitype.InsightsAccountScanStatus{
		sampleScan("scan-1", "prod-aws/us-east-1"),
		sampleScan("scan-2", "prod-aws/us-west-2"),
	}
	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{{ScanStatuses: want}},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", withScanListOutput(defaultScanListArgs(), "json"))
	require.NoError(t, err)

	var got struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, want, got.Scans)
}

// TestInsightsAccountScanListCmd_JSONOutput_EmptyList ensures an empty result
// serialises to `[]` rather than `null`, so jq scripting can iterate without
// a nil-check — same contract as `account list`.
func TestInsightsAccountScanListCmd_JSONOutput_EmptyList(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{{ScanStatuses: nil}},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", withScanListOutput(defaultScanListArgs(), "json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"scans":[]}`, out.String())
}

// TestInsightsAccountScanListCmd_DefaultStopsAfterFirstPage: without --count
// or --all the command returns exactly the first server-side page even when
// a continuationToken is available — matches the epic's "default is the size
// of the first page" rule.
func TestInsightsAccountScanListCmd_DefaultStopsAfterFirstPage(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{
			{
				ScanStatuses: []apitype.InsightsAccountScanStatus{
					sampleScan("s1", "prod-aws/us-east-1"),
					sampleScan("s2", "prod-aws/us-east-1"),
				},
				ContinuationToken: "cursor-1",
			},
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s3", "prod-aws/us-east-1")},
				ContinuationToken: "",
			},
		},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", withScanListOutput(defaultScanListArgs(), "json"))
	require.NoError(t, err)

	var got struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Scans, 2)
	assert.Equal(t, "s1", got.Scans[0].ID)
	assert.Equal(t, "s2", got.Scans[1].ID)
	require.Len(t, client.calls, 1, "should not follow continuationToken without --count/--all")
}

// TestInsightsAccountScanListCmd_AllFollowsPagination: --all keeps following
// the continuationToken cursor until the server reports an empty token.
func TestInsightsAccountScanListCmd_AllFollowsPagination(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s1", "prod-aws/us-east-1")},
				ContinuationToken: "cursor-1",
			},
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s2", "prod-aws/us-east-1")},
				ContinuationToken: "cursor-2",
			},
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s3", "prod-aws/us-east-1")},
				ContinuationToken: "",
			},
		},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	args := withScanListOutput(defaultScanListArgs(), "json")
	args.all = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", args)
	require.NoError(t, err)

	var got struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Scans, 3)
	require.Len(t, client.calls, 3)
	assert.Empty(t, client.calls[0].params.ContinuationToken)
	assert.Equal(t, "cursor-1", client.calls[1].params.ContinuationToken)
	assert.Equal(t, "cursor-2", client.calls[2].params.ContinuationToken)
}

// TestInsightsAccountScanListCmd_CountPaginatesUntilSatisfied: --count N
// pages across as many server pages as needed and truncates the last page so
// the user sees exactly N.
func TestInsightsAccountScanListCmd_CountPaginatesUntilSatisfied(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{
			{
				ScanStatuses: []apitype.InsightsAccountScanStatus{
					sampleScan("s1", "prod-aws/us-east-1"),
					sampleScan("s2", "prod-aws/us-east-1"),
				},
				ContinuationToken: "cursor-1",
			},
			{
				ScanStatuses: []apitype.InsightsAccountScanStatus{
					sampleScan("s3", "prod-aws/us-east-1"),
					sampleScan("s4", "prod-aws/us-east-1"),
				},
				ContinuationToken: "cursor-2",
			},
			// The third page must never be requested — --count=3 is satisfied
			// after page 2.
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s5", "prod-aws/us-east-1")},
				ContinuationToken: "",
			},
		},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	args := withScanListOutput(defaultScanListArgs(), "json")
	args.count = 3
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", args)
	require.NoError(t, err)

	var got struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Scans, 3, "result should be truncated to --count")
	assert.Equal(t, []string{"s1", "s2", "s3"},
		[]string{got.Scans[0].ID, got.Scans[1].ID, got.Scans[2].ID})
	require.Len(t, client.calls, 2, "should stop paginating once --count is satisfied")
}

// TestInsightsAccountScanListCmd_CountZeroEqualsAll: --count 0 is a synonym
// for --all per #22959 — paginate to exhaustion.
func TestInsightsAccountScanListCmd_CountZeroEqualsAll(t *testing.T) {
	t.Parallel()

	client := &mockInsightsAccountScanListClient{
		pages: []apitype.ListInsightsAccountScansResponse{
			{
				ScanStatuses: []apitype.InsightsAccountScanStatus{
					sampleScan("s1", "prod-aws/us-east-1"),
					sampleScan("s2", "prod-aws/us-east-1"),
				},
				ContinuationToken: "cursor-1",
			},
			{
				ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s3", "prod-aws/us-east-1")},
				ContinuationToken: "",
			},
		},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	args := withScanListOutput(defaultScanListArgs(), "json")
	args.count = 0
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", args)
	require.NoError(t, err)

	var got struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Scans, 3)
	require.Len(t, client.calls, 2)
}

func TestInsightsAccountScanListCmd_NegativeCountRejected(t *testing.T) {
	t.Parallel()

	c := &insightsAccountScanListCmd{
		clientFactory: stubScanListFactory(&mockInsightsAccountScanListClient{}, "acme"),
	}

	args := defaultScanListArgs()
	args.count = -3
	args.countSet = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--count must be non-negative")
}

func TestInsightsAccountScanListCmd_OrgAndAccountPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(*insightsAccountScanListArgs)
		defaultOrg  string
		account     string
		wantOrg     string
		wantAccount string
	}{
		{
			name:        "default org used when --org omitted",
			defaultOrg:  "acme",
			account:     "prod-aws",
			wantOrg:     "acme",
			wantAccount: "prod-aws",
		},
		{
			name:        "--org overrides default",
			mutate:      func(a *insightsAccountScanListArgs) { a.org = "other-co" },
			defaultOrg:  "acme",
			account:     "prod-aws",
			wantOrg:     "other-co",
			wantAccount: "prod-aws",
		},
		{
			name:        "account names with slashes pass through unmodified",
			defaultOrg:  "acme",
			account:     "prod-aws/us-east-1",
			wantOrg:     "acme",
			wantAccount: "prod-aws/us-east-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInsightsAccountScanListClient{
				pages: []apitype.ListInsightsAccountScansResponse{{ScanStatuses: nil}},
			}
			c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, tc.defaultOrg)}

			args := defaultScanListArgs()
			if tc.mutate != nil {
				tc.mutate(&args)
			}

			var out bytes.Buffer
			require.NoError(t, c.Run(t.Context(), &out, tc.account, args))
			require.Len(t, client.calls, 1)
			assert.Equal(t, tc.wantOrg, client.calls[0].org)
			assert.Equal(t, tc.wantAccount, client.calls[0].account)
		})
	}
}

func TestInsightsAccountScanListCmd_FactoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("not logged in")
	c := &insightsAccountScanListCmd{clientFactory: failingScanListFactory(wantErr)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", defaultScanListArgs())
	require.ErrorIs(t, err, wantErr)
}

func TestInsightsAccountScanListCmd_ClientErrorWrapped(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("[404] Not Found")
	client := &mockInsightsAccountScanListClient{err: wantErr}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", defaultScanListArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing insights scans")
	assert.ErrorIs(t, err, wantErr)
}

// TestInsightsAccountScanListCmd_PageLoopBudget covers the safety net against
// a misbehaving server that returns a non-empty continuationToken forever.
func TestInsightsAccountScanListCmd_PageLoopBudget(t *testing.T) {
	t.Parallel()

	// Looping client: every call returns the same non-empty token.
	client := &endlessScanListClient{
		page: apitype.ListInsightsAccountScansResponse{
			ScanStatuses:      []apitype.InsightsAccountScanStatus{sampleScan("s", "prod-aws/us-east-1")},
			ContinuationToken: "cursor-1",
		},
	}
	c := &insightsAccountScanListCmd{clientFactory: stubScanListFactory(client, "acme")}

	args := defaultScanListArgs()
	args.all = true

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, "prod-aws", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagination exceeded")
}

// endlessScanListClient always returns the same page. Used to exercise the
// pagination safety net without storing 1000+ pages in memory.
type endlessScanListClient struct {
	page  apitype.ListInsightsAccountScansResponse
	calls int
}

func (e *endlessScanListClient) ListInsightsAccountScans(
	_ context.Context, _, _ string, _ apitype.ListInsightsAccountScansParams,
) (apitype.ListInsightsAccountScansResponse, error) {
	e.calls++
	return e.page, nil
}
