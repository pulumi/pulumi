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

	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedScanLogCall struct {
	org     string
	account string
	scanID  string
	params  apitype.InsightsScanLogsParams
}

// mockScanLogClient consumes one response per call, so multi-page tests can
// script a paginated stream.
type mockScanLogClient struct {
	responses []apitype.InsightsScanLogs
	err       error
	calls     []capturedScanLogCall
}

func (m *mockScanLogClient) GetInsightsScanLogs(
	_ context.Context, org, account, scanID string,
	params apitype.InsightsScanLogsParams,
) (apitype.InsightsScanLogs, error) {
	m.calls = append(m.calls, capturedScanLogCall{
		org: org, account: account, scanID: scanID, params: params,
	})
	if m.err != nil {
		return apitype.InsightsScanLogs{}, m.err
	}
	if len(m.calls) > len(m.responses) {
		return apitype.InsightsScanLogs{}, nil
	}
	return m.responses[len(m.calls)-1], nil
}

// stubScanLogFactory mirrors production's orgOverride-wins behaviour so
// per-call --org assertions reflect reality.
func stubScanLogFactory(client insightsScanLogClient, defaultOrg string) scanLogClientFactory {
	return func(_ context.Context, orgOverride string) (insightsScanLogClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return client, org, nil
	}
}

func failingScanLogFactory(err error) scanLogClientFactory {
	return func(_ context.Context, _ string) (insightsScanLogClient, string, error) {
		return nil, "", err
	}
}

// newTestScanLogCmd wires up the same renderer table the production
// constructor installs.
func newTestScanLogCmd() (*insightsAccountScanLogCmd, *bytes.Buffer) {
	var buf bytes.Buffer
	return &insightsAccountScanLogCmd{
		output: outputflag.OutputFlag[scanLogRender]{
			RenderForTerminal: (*insightsAccountScanLogCmd).renderText,
			RenderJSON:        (*insightsAccountScanLogCmd).renderJSON,
		},
		w: &buf,
	}, &buf
}

func continuationPage(lines int, token string) apitype.InsightsScanLogs {
	entries := make([]apitype.InsightsScanLogLine, 0, lines)
	for i := range lines {
		entries = append(entries, apitype.InsightsScanLogLine{
			Header:    "scan",
			Timestamp: time.Date(2026, 5, 1, 14, 30, i, 0, time.UTC),
			Line:      "line " + string(rune('0'+i)),
		})
	}
	return apitype.InsightsScanLogs{
		Type:      "continuation",
		Lines:     entries,
		NextToken: token,
	}
}

// stepPage builds a step-mode response: structured lines plus an optional
// nextOffset cursor mirroring what the live API returns.
func stepPage(lines int, prefix string, nextOffset int64) apitype.InsightsScanLogs {
	entries := make([]apitype.InsightsScanLogLine, 0, lines)
	for i := range lines {
		entries = append(entries, apitype.InsightsScanLogLine{
			Timestamp: time.Date(2026, 5, 1, 14, 30, i, 0, time.UTC),
			Line:      prefix + "-" + string(rune('0'+i)) + "\n",
		})
	}
	return apitype.InsightsScanLogs{
		Type:       "DeploymentLogsStep",
		Lines:      entries,
		NextOffset: nextOffset,
	}
}

func TestScanLog_ContinuationDefault(t *testing.T) {
	t.Parallel()

	// Default: one server page even when more pages are available.
	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{
			continuationPage(3, "next-page"),
		},
	}
	cmd, buf := newTestScanLogCmd()
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	require.Len(t, client.calls, 1)
	assert.Equal(t, "", client.calls[0].params.ContinuationToken)
	assert.Equal(t, 0, client.calls[0].params.Count)

	out := buf.String()
	assert.Contains(t, out, "2026-05-01T14:30:00Z [scan] line 0")
	assert.Contains(t, out, "2026-05-01T14:30:02Z [scan] line 2")
	assert.Contains(t, out, "--count <N> or --all")
}

func TestScanLog_ContinuationEmpty(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{{}},
	}
	cmd, buf := newTestScanLogCmd()
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)
	assert.Equal(t, "No log entries.\n", buf.String())
}

func TestScanLog_ContinuationStripsTrailingNewlines(t *testing.T) {
	t.Parallel()

	// The server's Line field ends with `\n`; the renderer must not
	// double-space the output by adding its own newline on top.
	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{{
			Lines: []apitype.InsightsScanLogLine{
				{
					Timestamp: time.Date(2026, 5, 1, 14, 30, 0, 0, time.UTC),
					Line:      "starting scan\n",
				},
				{
					Timestamp: time.Date(2026, 5, 1, 14, 30, 1, 0, time.UTC),
					Line:      "finished scan\n",
				},
			},
		}},
	}
	cmd, buf := newTestScanLogCmd()
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	assert.Equal(t,
		"2026-05-01T14:30:00Z starting scan\n"+
			"2026-05-01T14:30:01Z finished scan\n",
		buf.String())
}

func TestScanLog_ContinuationLastPage(t *testing.T) {
	t.Parallel()

	// Empty continuation token → no follow-up hint.
	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{continuationPage(2, "")},
	}
	cmd, buf := newTestScanLogCmd()
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "More entries available")
}

func TestScanLog_ContinuationCount(t *testing.T) {
	t.Parallel()

	// --count 4 spans two pages (3 + 1).
	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{
			continuationPage(3, "tok-1"),
			continuationPage(3, "tok-2"),
		},
	}
	cmd, buf := newTestScanLogCmd()
	cmd.count = 4
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	require.Len(t, client.calls, 2)
	assert.Equal(t, "", client.calls[0].params.ContinuationToken)
	assert.Equal(t, 4, client.calls[0].params.Count)
	// Second call asks only for the remaining 1 entry.
	assert.Equal(t, "tok-1", client.calls[1].params.ContinuationToken)
	assert.Equal(t, 1, client.calls[1].params.Count)

	out := buf.String()
	assert.Contains(t, out, "2026-05-01T14:30:00Z [scan] line 0")
	assert.Equal(t, 4, bytesLineCount(out, "[scan] line"))
}

func TestScanLog_ContinuationAll(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{
			continuationPage(2, "tok-1"),
			continuationPage(2, "tok-2"),
			continuationPage(1, ""),
		},
	}
	cmd, buf := newTestScanLogCmd()
	cmd.all = true
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	require.Len(t, client.calls, 3)
	// --all maxes out per-page count to minimise round-trips.
	for _, call := range client.calls {
		assert.Equal(t, 500, call.params.Count)
	}
	assert.Equal(t, "", client.calls[0].params.ContinuationToken)
	assert.Equal(t, "tok-1", client.calls[1].params.ContinuationToken)
	assert.Equal(t, "tok-2", client.calls[2].params.ContinuationToken)

	out := buf.String()
	assert.Equal(t, 5, bytesLineCount(out, "[scan] line"))
	assert.NotContains(t, out, "More entries available")
}

func TestScanLog_AllAndCountMutuallyExclusive(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{}
	cmd := newInsightsAccountScanLogCmd(stubScanLogFactory(client, "acme"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--all", "--count", "5", "prod-aws", "scan-123"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[all count]")
	assert.Empty(t, client.calls)
}

func TestScanLog_StepDefault(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{stepPage(2, "setup", 1024)},
	}
	cmd, buf := newTestScanLogCmd()
	cmd.jobSet = true
	cmd.job = 0
	cmd.step = 0
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	// Zero values must reach the wire as non-nil pointers.
	require.Len(t, client.calls, 1)
	require.NotNil(t, client.calls[0].params.Job)
	require.NotNil(t, client.calls[0].params.Step)
	assert.Equal(t, 0, *client.calls[0].params.Job)
	assert.Equal(t, 0, *client.calls[0].params.Step)
	assert.Nil(t, client.calls[0].params.Offset, "default mode never sets offset")

	out := buf.String()
	assert.Contains(t, out, "setup-0")
	assert.Contains(t, out, "setup-1")
	assert.Contains(t, out, "More output available. Re-run with --all")
}

func TestScanLog_StepAll(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{
			stepPage(2, "page0", 1024),
			stepPage(2, "page1", 2048),
			stepPage(1, "page2", 0),
		},
	}
	cmd, buf := newTestScanLogCmd()
	cmd.jobSet = true
	cmd.job = 1
	cmd.step = 2
	cmd.all = true
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	require.Len(t, client.calls, 3)
	assert.Nil(t, client.calls[0].params.Offset)
	require.NotNil(t, client.calls[1].params.Offset)
	assert.Equal(t, int64(1024), *client.calls[1].params.Offset)
	require.NotNil(t, client.calls[2].params.Offset)
	assert.Equal(t, int64(2048), *client.calls[2].params.Offset)
	for _, call := range client.calls {
		require.NotNil(t, call.params.Job)
		require.NotNil(t, call.params.Step)
		assert.Equal(t, 1, *call.params.Job)
		assert.Equal(t, 2, *call.params.Step)
	}

	out := buf.String()
	// Every line from every page is accumulated and rendered.
	assert.Contains(t, out, "page0-0")
	assert.Contains(t, out, "page0-1")
	assert.Contains(t, out, "page1-0")
	assert.Contains(t, out, "page1-1")
	assert.Contains(t, out, "page2-0")
	assert.NotContains(t, out, "More output available")
}

func TestScanLog_StepWithoutJobRejectsStep(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{}
	cmd, _ := newTestScanLogCmd()
	cmd.step = 2
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--step requires --job")
	assert.Empty(t, client.calls)
}

func TestScanLog_JSONOutput(t *testing.T) {
	t.Parallel()

	resp := continuationPage(2, "next-page")
	client := &mockScanLogClient{responses: []apitype.InsightsScanLogs{resp}}
	cmd, buf := newTestScanLogCmd()
	require.NoError(t, cmd.output.Set("json"))
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "scan-123")
	require.NoError(t, err)

	var got apitype.InsightsScanLogs
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, resp, got)
}

func TestScanLog_OrgOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       insightsAccountScanLogCmd
		defaultOrg string
		wantOrg    string
	}{
		{
			name:       "default org used when --org omitted",
			defaultOrg: "acme",
			wantOrg:    "acme",
		},
		{
			name:       "--org overrides default",
			args:       insightsAccountScanLogCmd{org: "other-co"},
			defaultOrg: "acme",
			wantOrg:    "other-co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &mockScanLogClient{
				responses: []apitype.InsightsScanLogs{{}},
			}
			cmd, _ := newTestScanLogCmd()
			cmd.org = tt.args.org
			err := cmd.run(t.Context(), stubScanLogFactory(client, tt.defaultOrg),
				"prod-aws", "scan-123")
			require.NoError(t, err)
			require.Len(t, client.calls, 1)
			assert.Equal(t, tt.wantOrg, client.calls[0].org)
			assert.Equal(t, "prod-aws", client.calls[0].account)
			assert.Equal(t, "scan-123", client.calls[0].scanID)
		})
	}
}

func TestScanLog_InvalidOutput(t *testing.T) {
	t.Parallel()

	cmd, _ := newTestScanLogCmd()
	err := cmd.output.Set("yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"yaml" not supported`)
}

func TestScanLog_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{err: errors.New("404 not found")}
	cmd, _ := newTestScanLogCmd()
	err := cmd.run(t.Context(), stubScanLogFactory(client, "acme"),
		"prod-aws", "missing-scan")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading insights scan logs")
	assert.Contains(t, err.Error(), "404 not found")
}

func TestScanLog_FactoryError(t *testing.T) {
	t.Parallel()

	cmd, _ := newTestScanLogCmd()
	err := cmd.run(t.Context(), failingScanLogFactory(errors.New("not logged in")),
		"prod-aws", "scan-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestNewInsightsAccountScanLogCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{continuationPage(2, "")},
	}
	cmd := newInsightsAccountScanLogCmd(stubScanLogFactory(client, "acme"))
	assert.Equal(t, "log", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--org", "other-co",
		"--count", "5",
		"--output", "json",
		"prod-aws", "scan-123",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Len(t, client.calls, 1)
	assert.Equal(t, "other-co", client.calls[0].org)
	assert.Equal(t, "prod-aws", client.calls[0].account)
	assert.Equal(t, "scan-123", client.calls[0].scanID)
	assert.Equal(t, 5, client.calls[0].params.Count)
	assert.Empty(t, client.calls[0].params.ContinuationToken)

	var got apitype.InsightsScanLogs
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, continuationPage(2, ""), got)
}

func TestNewInsightsAccountScanLogCmd_FlagBinding_StepMode(t *testing.T) {
	t.Parallel()

	client := &mockScanLogClient{
		responses: []apitype.InsightsScanLogs{stepPage(1, "data", 0)},
	}
	cmd := newInsightsAccountScanLogCmd(stubScanLogFactory(client, "acme"))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Zero values must reach the wire, not be conflated with "unset".
	cmd.SetArgs([]string{
		"--job", "0",
		"--step", "0",
		"prod-aws", "scan-123",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Len(t, client.calls, 1)
	require.NotNil(t, client.calls[0].params.Job)
	require.NotNil(t, client.calls[0].params.Step)
	assert.Equal(t, 0, *client.calls[0].params.Job)
	assert.Equal(t, 0, *client.calls[0].params.Step)
}

func TestNewInsightsAccountScanLogCmd_RequiresAccountAndScanID(t *testing.T) {
	t.Parallel()

	cmd := newInsightsAccountScanLogCmd(stubScanLogFactory(&mockScanLogClient{}, "acme"))
	cmd.SetArgs([]string{"prod-aws"}) // scan-id missing

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
}

func TestNewInsightsAccountScanLogCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Don't invoke — the default factory would hit the real cloud context.
	cmd := newInsightsAccountScanLogCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "log", cmd.Name())
}

func bytesLineCount(s, needle string) int {
	count := 0
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			count++
			i += len(needle) - 1
		}
	}
	return count
}
