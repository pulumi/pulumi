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

package policy

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// capturedPolicyIssueListCall records the inputs to a single ListPolicyIssues
// call so tests can assert on the flag-to-request propagation.
type capturedPolicyIssueListCall struct {
	org  string
	opts client.ListPolicyIssuesOptions
}

// mockPolicyIssueListClient stubs policyIssueListClient. It returns a fixed
// response (or error) and records the most recent invocation.
type mockPolicyIssueListClient struct {
	resp     apitype.ListPolicyIssuesResponse
	err      error
	captured *capturedPolicyIssueListCall
}

func (m *mockPolicyIssueListClient) ListPolicyIssues(
	_ context.Context, org string, opts client.ListPolicyIssuesOptions,
) (apitype.ListPolicyIssuesResponse, error) {
	if m.captured != nil {
		*m.captured = capturedPolicyIssueListCall{org: org, opts: opts}
	}
	if m.err != nil {
		return apitype.ListPolicyIssuesResponse{}, m.err
	}
	return m.resp, nil
}

func stubPolicyIssueListFactory(c policyIssueListClient, org string) policyIssueListClientFactory {
	return func(_ context.Context, _ string) (policyIssueListClient, string, error) {
		return c, org, nil
	}
}

func samplePolicyIssueListResponse() apitype.ListPolicyIssuesResponse {
	return apitype.ListPolicyIssuesResponse{
		RowCount: new(2),
		PolicyIssues: []apitype.PolicyIssue{
			{
				ID:            "issue-1",
				PolicyName:    "no-public-buckets",
				PolicyPack:    "aws-guardrails",
				PolicyPackTag: "1.2.0",
				Level:         string(apitype.Mandatory),
				Severity:      apitype.PolicySeverityHigh,
				ResourceURN:   "urn:pulumi:prod::web::aws:s3/bucket:Bucket::data",
				ResourceType:  "aws:s3/bucket:Bucket",
				EntityID:      "prod",
				EntityProject: "web",
				Message:       "S3 bucket must not allow public access",
				ObservedAt:    new(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)),
			},
			{
				ID:            "issue-2",
				PolicyName:    "required-tags",
				PolicyPack:    "tagging",
				Level:         string(apitype.Advisory),
				EntityID:      "staging",
				EntityProject: "web",
				Message: "Resource is missing required tags: this message is intentionally " +
					"long enough to exercise the truncation path in the table renderer",
				ObservedAt: new(time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)),
			},
		},
	}
}

func TestPolicyIssueList_DefaultOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueListClient{resp: samplePolicyIssueListResponse()}
	err := runPolicyIssueList(t.Context(), buf,
		stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)

	out := buf.String()
	// Table headers.
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "POLICY PACK")
	assert.Contains(t, out, "POLICY")
	assert.Contains(t, out, "ENFORCEMENT")
	assert.Contains(t, out, "STACK")
	assert.Contains(t, out, "MESSAGE")

	// First row content.
	assert.Contains(t, out, "issue-1")
	assert.Contains(t, out, "aws-guardrails@1.2.0")
	assert.Contains(t, out, "no-public-buckets")
	assert.Contains(t, out, "mandatory")
	assert.Contains(t, out, "web/prod")
	assert.Contains(t, out, "S3 bucket must not allow public access")

	// Second row: long message should be truncated with an ellipsis.
	assert.Contains(t, out, "issue-2")
	assert.Contains(t, out, "tagging")
	assert.Contains(t, out, "advisory")
	assert.Contains(t, out, "web/staging")
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, "truncation path in the table renderer")

	// Footer summary (no page number).
	assert.Contains(t, out, "Showing 2 of 2 policy issue(s)")
}

func TestPolicyIssueList_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueListClient{resp: apitype.ListPolicyIssuesResponse{}}
	err := runPolicyIssueList(t.Context(), buf,
		stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, "No policy issues found for this organization.\n", buf.String())
}

func TestPolicyIssueList_JSONOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueListClient{resp: samplePolicyIssueListResponse()}
	args := policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyIssueList(t.Context(), buf,
		stubPolicyIssueListFactory(c, "acme"), args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"issues": [
			{
				"id": "issue-1",
				"policyName": "no-public-buckets",
				"policyPack": "aws-guardrails",
				"policyPackTag": "1.2.0",
				"level": "mandatory",
				"severity": "high",
				"resourceURN": "urn:pulumi:prod::web::aws:s3/bucket:Bucket::data",
				"resourceType": "aws:s3/bucket:Bucket",
				"entityId": "prod",
				"entityProject": "web",
				"message": "S3 bucket must not allow public access",
				"observedAt": "2026-05-01T12:00:00Z",
				"entityType": "",
				"resourceProvider": "",
				"resourceName": "",
				"status": "",
				"kind": "",
				"priority": ""
			},
			{
				"id": "issue-2",
				"policyName": "required-tags",
				"policyPack": "tagging",
				"level": "advisory",
				"entityId": "staging",
				"entityProject": "web",
				"message": "Resource is missing required tags: this message is intentionally `+
		`long enough to exercise the truncation path in the table renderer",
				"observedAt": "2026-04-30T08:00:00Z",
				"entityType": "",
				"policyPackTag": "",
				"severity": "",
				"resourceURN": "",
				"resourceType": "",
				"resourceProvider": "",
				"resourceName": "",
				"status": "",
				"kind": "",
				"priority": ""
			}
		],
		"total": 2
	}`, buf.String())
}

func TestPolicyIssueList_NilRowCountStopsPaging(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policyIssues":[{"id":"issue-1","policyName":"no-public-buckets"}]}`))
	}))
	defer server.Close()

	pc := client.NewClient(server.URL, "", false, nil)
	factory := func(context.Context, string) (policyIssueListClient, string, error) {
		return pc, "acme", nil
	}

	buf := &bytes.Buffer{}
	err := runPolicyIssueList(t.Context(), buf, factory,
		policyIssueListArgs{all: true, outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, int32(1), requests.Load())
	assert.Contains(t, buf.String(), "issue-1")
	assert.Contains(t, buf.String(), "Showing 1 of 1 policy issue(s)")
}
