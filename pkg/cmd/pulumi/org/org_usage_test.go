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

package org

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type capturedUsageCall struct {
	org    string
	params apitype.OrgUsageSummaryParams
}

type mockUsageGetClient struct {
	resp     apitype.OrgUsageSummaryResponse
	err      error
	captured *capturedUsageCall
}

func (m *mockUsageGetClient) GetOrgUsageSummary(
	_ context.Context, org string, params apitype.OrgUsageSummaryParams,
) (apitype.OrgUsageSummaryResponse, error) {
	if m.captured != nil {
		*m.captured = capturedUsageCall{org: org, params: params}
	}
	if m.err != nil {
		return apitype.OrgUsageSummaryResponse{}, m.err
	}
	return m.resp, nil
}

func stubUsageFactory(c orgUsageGetClient, resolvedOrg string) orgUsageGetClientFactory {
	return func(_ context.Context, orgOverride string) (orgUsageGetClient, string, error) {
		if orgOverride != "" {
			return c, orgOverride, nil
		}
		return c, resolvedOrg, nil
	}
}

func ptr[T any](v T) *T { return &v }

func sampleDailyUsage() apitype.OrgUsageSummaryResponse {
	return apitype.OrgUsageSummaryResponse{
		Summary: []apitype.OrgResourceCountSummary{
			{
				Year:          2026,
				Month:         ptr(5),
				Day:           ptr(1),
				Resources:     1200,
				ResourceHours: 28800,
			},
			{
				Year:          2026,
				Month:         ptr(5),
				Day:           ptr(2),
				Resources:     1300,
				ResourceHours: 31200,
			},
		},
	}
}

func defaultArgs() orgUsageGetArgs {
	return orgUsageGetArgs{render: renderUsageGetTable}
}

func TestOrgUsageGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockUsageGetClient{resp: sampleDailyUsage()}
	err := runOrgUsageGet(t.Context(), &buf, stubUsageFactory(c, "acme"), defaultArgs())
	require.NoError(t, err)

	out := buf.String()
	// Headers — only the columns we asked for should appear. "Resource Hours"
	// contains "Hour" so we match exact " Hour " cells to keep the assertion
	// targeted.
	assert.Contains(t, out, "Year")
	assert.Contains(t, out, "Month")
	assert.Contains(t, out, "Day")
	assert.Contains(t, out, "Resources")
	assert.Contains(t, out, "Resource Hours")
	assert.NotContains(t, out, "Week")
	assert.NotContains(t, out, " Hour ")

	// Row values.
	assert.Contains(t, out, "2026")
	assert.Contains(t, out, "1200")
	assert.Contains(t, out, "28800")
	assert.Contains(t, out, "1300")
	assert.Contains(t, out, "31200")

	assert.Contains(t, out, "2 summary point(s).")
}

func TestOrgUsageGet_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockUsageGetClient{resp: apitype.OrgUsageSummaryResponse{}}
	err := runOrgUsageGet(t.Context(), &buf, stubUsageFactory(c, "acme"), defaultArgs())
	require.NoError(t, err)

	assert.Equal(t, "No usage data available for this organization and time window.\n", buf.String())
}

func TestOrgUsageGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockUsageGetClient{resp: sampleDailyUsage()}
	args := defaultArgs()
	args.render = renderUsageGetJSON
	err := runOrgUsageGet(t.Context(), &buf, stubUsageFactory(c, "acme"), args)
	require.NoError(t, err)

	var decoded apitype.OrgUsageSummaryResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, sampleDailyUsage(), decoded)
}

func TestOrgUsageGet_JSONOutput_EmptyNormalizesToArray(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockUsageGetClient{resp: apitype.OrgUsageSummaryResponse{}}
	args := defaultArgs()
	args.render = renderUsageGetJSON
	err := runOrgUsageGet(t.Context(), &buf, stubUsageFactory(c, "acme"), args)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	summary, ok := raw["summary"].([]any)
	require.True(t, ok, "expected summary to be an array, got %T", raw["summary"])
	assert.Empty(t, summary)
}

func TestOrgUsageGet_HourlyColumns(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockUsageGetClient{resp: apitype.OrgUsageSummaryResponse{
		Summary: []apitype.OrgResourceCountSummary{{
			Year:          2026,
			Month:         ptr(5),
			Day:           ptr(14),
			Hour:          ptr(13),
			Resources:     50,
			ResourceHours: 50,
		}},
	}}
	err := runOrgUsageGet(t.Context(), &buf, stubUsageFactory(c, "acme"), defaultArgs())
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Hour")
	assert.NotContains(t, out, "Week")
}

func TestOrgUsageGet_InvalidGranularity(t *testing.T) {
	t.Parallel()

	c := &mockUsageGetClient{resp: sampleDailyUsage()}
	args := defaultArgs()
	args.granularity = "yearly"

	err := runOrgUsageGet(t.Context(), &bytes.Buffer{}, stubUsageFactory(c, "acme"), args)
	require.ErrorContains(t, err, `invalid --granularity "yearly"`)
}

func TestOrgUsageGet_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockUsageGetClient{err: errors.New("boom")}
	err := runOrgUsageGet(t.Context(), &bytes.Buffer{}, stubUsageFactory(c, "acme"), defaultArgs())
	require.ErrorContains(t, err, "fetching organization usage summary")
	require.ErrorContains(t, err, "boom")
}

func TestOrgUsageGet_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	captured := &capturedUsageCall{}
	c := &mockUsageGetClient{resp: sampleDailyUsage(), captured: captured}
	cmd := newOrgUsageGetCmd(stubUsageFactory(c, "default-org"))

	cmd.SetArgs([]string{
		"--org", "explicit-org",
		"--granularity", "monthly",
		"--lookback-days", "30",
		"--lookback-start", "1700000000",
		"--output", "json",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "explicit-org", captured.org)
	assert.Equal(t, apitype.OrgUsageSummaryParams{
		Granularity:   "monthly",
		LookbackDays:  30,
		LookbackStart: 1_700_000_000,
	}, captured.params)

	var decoded apitype.OrgUsageSummaryResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, sampleDailyUsage(), decoded)
}
