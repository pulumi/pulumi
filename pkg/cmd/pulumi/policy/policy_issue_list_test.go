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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
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
	var captured capturedPolicyIssueListCall
	c := &mockPolicyIssueListClient{resp: samplePolicyIssueListResponse(), captured: &captured}
	err := runPolicyIssueList(t.Context(), buf,
		stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)
	assert.False(t, captured.opts.Closed)

	out := buf.String()
	// Table headers.
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "STATUS")
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
  "policyIssues": [
    {
      "id": "issue-1",
      "entityType": "",
      "entityProject": "web",
      "entityId": "prod",
      "policyPack": "aws-guardrails",
      "policyPackTag": "1.2.0",
      "policyName": "no-public-buckets",
      "resourceURN": "urn:pulumi:prod::web::aws:s3/bucket:Bucket::data",
      "resourceProvider": "",
      "resourceType": "aws:s3/bucket:Bucket",
      "resourceName": "",
      "message": "S3 bucket must not allow public access",
      "observedAt": "2026-05-01T12:00:00Z",
      "level": "mandatory",
      "severity": "high",
      "status": "",
      "kind": "",
      "priority": ""
    },
    {
      "id": "issue-2",
      "entityType": "",
      "entityProject": "web",
      "entityId": "staging",
      "policyPack": "tagging",
      "policyPackTag": "",
      "policyName": "required-tags",
      "resourceURN": "",
      "resourceProvider": "",
      "resourceType": "",
      "resourceName": "",
      "message": "Resource is missing required tags: this message is intentionally `+
		`long enough to exercise the truncation path in the table renderer",
      "observedAt": "2026-04-30T08:00:00Z",
      "level": "advisory",
      "severity": "",
      "status": "",
      "kind": "",
      "priority": ""
    }
  ],
  "groupData": null,
  "rowCount": 2
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

func mustParseTime(t string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func newPolicyIssueServerClient(t *testing.T, respond func(r *http.Request) string) *client.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respond(r)))
	}))
	t.Cleanup(server.Close)
	return client.NewClient(server.URL, "token", false, nil)
}

const unknownEnumIssueJSON = `{
	"id": "issue-9",
	"entityType": "stack",
	"entityProject": "web",
	"entityId": "prod",
	"policyPack": "aws-guardrails",
	"policyName": "s3-no-public",
	"level": "mandatory",
	"severity": "future",
	"status": "future_status",
	"kind": "audit",
	"priority": "p9",
	"observedAt": "2026-10-01T12:34:56Z"
}`

const fractionalTimestampIssueJSON = `{
	"id": "issue-1",
	"entityType": "stack",
	"entityProject": "web",
	"entityId": "prod",
	"policyPack": "aws-guardrails",
	"policyPackTag": "3.1.0",
	"policyName": "s3-no-public",
	"level": "mandatory",
	"severity": "critical",
	"resourceURN": "urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets",
	"resourceProvider": "aws",
	"resourceType": "aws:s3/bucket:Bucket",
	"resourceName": "assets",
	"message": "Bucket must not allow public access & <must> be \"private\".",
	"observedAt": "2026-10-01T12:34:56.789123Z",
	"status": "open",
	"kind": "audit",
	"priority": "P1"
}`

const minimalIssueJSON = `{
	"id": "issue-2",
	"policyPack": "tagging",
	"policyName": "require-owner",
	"level": "advisory"
}`

func runPolicyIssueListAgainstServer(
	t *testing.T, body string, args policyIssueListArgs,
) string {
	t.Helper()
	c := newPolicyIssueServerClient(t, func(*http.Request) string { return body })
	buf := &bytes.Buffer{}
	require.NoError(t, runPolicyIssueList(t.Context(), buf, stubPolicyIssueListFactory(c, "acme"), args))
	return buf.String()
}

func TestPolicyIssueList_UnknownEnumValues(t *testing.T) {
	t.Parallel()

	body := `{"policyIssues": [` + unknownEnumIssueJSON + `], "rowCount": 1}`

	jsonArgs := policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()}
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	var envelope struct {
		Issues []map[string]any `json:"policyIssues"`
	}
	require.NoError(t, json.Unmarshal([]byte(runPolicyIssueListAgainstServer(t, body, jsonArgs)), &envelope))
	require.Len(t, envelope.Issues, 1)
	assert.Equal(t, "future_status", envelope.Issues[0]["status"])
	assert.Equal(t, "p9", envelope.Issues[0]["priority"])
	assert.Equal(t, "future", envelope.Issues[0]["severity"])

	table := runPolicyIssueListAgainstServer(t, body,
		policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()})
	assert.Contains(t, table, "issue-9")
	assert.Contains(t, table, "Showing 1 of 1 policy issue(s)")
}

func TestPolicyIssueList_Timestamps(t *testing.T) {
	t.Parallel()

	body := `{"policyIssues": [` + fractionalTimestampIssueJSON + `, ` + minimalIssueJSON + `], "rowCount": 250}`

	jsonArgs := policyIssueListArgs{count: 2, outputFormat: defaultPolicyIssueListOutputFormat()}
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	out := runPolicyIssueListAgainstServer(t, body, jsonArgs)
	assert.Contains(t, out, `"observedAt": "2026-10-01T12:34:56.789123Z"`)
	assert.NotContains(t, out, "lastModified")

	table := runPolicyIssueListAgainstServer(t, body,
		policyIssueListArgs{count: 2, outputFormat: defaultPolicyIssueListOutputFormat()})
	var want bytes.Buffer
	require.NoError(t, renderPolicyIssueListTable(&want, apitype.ListPolicyIssuesResponse{
		PolicyIssues: []apitype.PolicyIssue{samplePolicyIssueFull(), samplePolicyIssueMinimal()},
		RowCount:     new(250),
	}, false))
	assert.Equal(t, want.String(), table)
}

func samplePolicyIssueFull() apitype.PolicyIssue {
	return apitype.PolicyIssue{
		ID:               "issue-1",
		EntityType:       cloudapitype.IssueEntityTypeStack,
		EntityProject:    "web",
		EntityID:         "prod",
		PolicyPack:       "aws-guardrails",
		PolicyPackTag:    "3.1.0",
		PolicyName:       "s3-no-public",
		Level:            "mandatory",
		Severity:         apitype.PolicySeverityCritical,
		ResourceURN:      "urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets",
		ResourceProvider: "aws",
		ResourceType:     "aws:s3/bucket:Bucket",
		ResourceName:     "assets",
		Message:          "Bucket must not allow public access & <must> be \"private\".",
		ObservedAt:       mustParseTime("2026-03-04T05:06:07.123456Z"),
		LastModified:     mustParseTime("2026-03-05T06:07:08.5Z"),
		Status:           cloudapitype.PolicyIssueStatusOpen,
		Kind:             cloudapitype.PolicyIssueKindAudit,
		Priority:         "P1",
	}
}

func samplePolicyIssueMinimal() apitype.PolicyIssue {
	return apitype.PolicyIssue{
		ID:         "issue-2",
		PolicyPack: "tagging",
		PolicyName: "require-owner",
		Level:      "advisory",
	}
}

func TestPolicyIssueList_JSON_Full(t *testing.T) {
	t.Parallel()
	resp := apitype.ListPolicyIssuesResponse{
		PolicyIssues: []apitype.PolicyIssue{samplePolicyIssueFull(), samplePolicyIssueMinimal()},
		RowCount:     new(250),
	}
	var buf bytes.Buffer
	require.NoError(t, renderPolicyIssueListJSON(&buf, resp, false))
	got := buf.String()
	assert.JSONEq(t, `{
  "policyIssues": [
    {
      "id": "issue-1",
      "entityType": "stack",
      "entityProject": "web",
      "entityId": "prod",
      "policyPack": "aws-guardrails",
      "policyPackTag": "3.1.0",
      "policyName": "s3-no-public",
      "resourceURN": "urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets",
      "resourceProvider": "aws",
      "resourceType": "aws:s3/bucket:Bucket",
      "resourceName": "assets",
      "message": "Bucket must not allow public access & <must> be \"private\".",
      "observedAt": "2026-03-04T05:06:07.123456Z",
      "lastModified": "2026-03-05T06:07:08.5Z",
      "level": "mandatory",
      "severity": "critical",
      "status": "open",
      "kind": "audit",
      "priority": "P1"
    },
    {
      "id": "issue-2",
      "entityType": "",
      "entityProject": "",
      "entityId": "",
      "policyPack": "tagging",
      "policyPackTag": "",
      "policyName": "require-owner",
      "resourceURN": "",
      "resourceProvider": "",
      "resourceType": "",
      "resourceName": "",
      "observedAt": null,
      "level": "advisory",
      "severity": "",
      "status": "",
      "kind": "",
      "priority": ""
    }
  ],
  "groupData": null,
  "rowCount": 250
}
`, got)
}

func TestPolicyIssueList_JSON_Minimal(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, renderPolicyIssueListJSON(&buf, apitype.ListPolicyIssuesResponse{}, false))
	got := buf.String()
	assert.JSONEq(t, `{
  "policyIssues": [],
  "groupData": null
}
`, got)
}

func TestPolicyIssueList_Table(t *testing.T) {
	t.Parallel()
	resp := apitype.ListPolicyIssuesResponse{
		PolicyIssues: []apitype.PolicyIssue{samplePolicyIssueFull(), samplePolicyIssueMinimal()},
		RowCount:     new(250),
	}
	var buf bytes.Buffer
	require.NoError(t, renderPolicyIssueListTable(&buf, resp, false))
	got := buf.String()
	for _, want := range []string{
		"ID", "POLICY PACK", "POLICY", "ENFORCEMENT", "STACK", "MESSAGE",
		"issue-1", "aws-guardrails@3.1.0", "s3-no-public", "mandatory", "web/prod",
		`Bucket must not allow public access & <must> be "private".`,
		"issue-2", "tagging", "require-owner", "advisory",
		"Showing 2 of 250 policy issue(s)",
	} {
		assert.Contains(t, got, want)
	}
}

func TestPolicyIssueList_TableShowsClosedStatuses(t *testing.T) {
	t.Parallel()

	statuses := []cloudapitype.PolicyIssueStatus{
		cloudapitype.PolicyIssueStatusFixed,
		cloudapitype.PolicyIssueStatusIgnored,
		cloudapitype.PolicyIssueStatusByDesign,
	}
	issues := make([]apitype.PolicyIssue, 0, len(statuses))
	for i, status := range statuses {
		issues = append(issues, apitype.PolicyIssue{ID: fmt.Sprintf("issue-%d", i), Status: status})
	}
	var buf bytes.Buffer
	require.NoError(t, renderPolicyIssueListTable(&buf, apitype.ListPolicyIssuesResponse{
		PolicyIssues: issues,
		RowCount:     new(len(issues)),
	}, true))
	got := buf.String()
	for _, want := range []string{"fixed", "ignored", "by_design"} {
		assert.Contains(t, got, want)
	}
}

func TestPolicyIssueList_ClosedFlagPassedToClient(t *testing.T) {
	t.Parallel()

	for _, closed := range []bool{false, true} {
		t.Run(fmt.Sprintf("closed=%t", closed), func(t *testing.T) {
			t.Parallel()

			var closedByPage []bool
			c := &pagingPolicyIssueListClient{
				onCall: func(closed bool) { closedByPage = append(closedByPage, closed) },
			}
			err := runPolicyIssueList(t.Context(), &bytes.Buffer{},
				stubPolicyIssueListFactory(c, "acme"),
				policyIssueListArgs{closed: closed, all: true, outputFormat: defaultPolicyIssueListOutputFormat()})
			require.NoError(t, err)
			assert.Equal(t, []bool{closed, closed}, closedByPage)
		})
	}
}

func TestPolicyIssueList_ClosedEmptyMessage(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueListClient{resp: apitype.ListPolicyIssuesResponse{}}
	err := runPolicyIssueList(t.Context(), buf,
		stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{closed: true, outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, "No closed policy issues found for this organization.\n", buf.String())
}

type pagingPolicyIssueListClient struct {
	onCall func(closed bool)
	calls  int
}

func (p *pagingPolicyIssueListClient) ListPolicyIssues(
	_ context.Context, _ string, opts client.ListPolicyIssuesOptions,
) (apitype.ListPolicyIssuesResponse, error) {
	p.onCall(opts.Closed)
	p.calls++
	return apitype.ListPolicyIssuesResponse{
		PolicyIssues: []apitype.PolicyIssue{{ID: fmt.Sprintf("issue-%d", p.calls)}},
		RowCount:     new(2),
	}, nil
}

type recordingPolicyIssueListClient struct {
	reqs []client.ListPolicyIssuesOptions
	all  []apitype.PolicyIssue
	err  error
}

func (r *recordingPolicyIssueListClient) ListPolicyIssues(
	_ context.Context, _ string, req client.ListPolicyIssuesOptions,
) (apitype.ListPolicyIssuesResponse, error) {
	if r.err != nil {
		return apitype.ListPolicyIssuesResponse{}, r.err
	}
	r.reqs = append(r.reqs, req)
	end := min(req.EndRow, len(r.all))
	return apitype.ListPolicyIssuesResponse{
		PolicyIssues: r.all[min(req.StartRow, end):end],
		RowCount:     new(len(r.all)),
	}, nil
}

func TestPolicyIssueList_NoFiltersBodyUnchanged(t *testing.T) {
	t.Parallel()

	c := &recordingPolicyIssueListClient{all: []apitype.PolicyIssue{{ID: "issue-1"}}}
	err := runPolicyIssueList(t.Context(), &bytes.Buffer{}, stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{outputFormat: defaultPolicyIssueListOutputFormat()})
	require.NoError(t, err)
	require.Len(t, c.reqs, 1)
	assert.Nil(t, c.reqs[0].FilterModel)
	assert.Nil(t, c.reqs[0].SortModel)
}

func TestPolicyIssueList_FiltersOnEveryPage(t *testing.T) {
	t.Parallel()

	issues := make([]apitype.PolicyIssue, 250)
	for i := range issues {
		issues[i] = apitype.PolicyIssue{ID: fmt.Sprintf("issue-%d", i)}
	}
	c := &recordingPolicyIssueListClient{all: issues}
	err := runPolicyIssueList(t.Context(), &bytes.Buffer{}, stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{
			all:          true,
			filters:      policyIssueFilters{severities: []string{"high"}, sort: []string{"observedAt:desc"}},
			outputFormat: defaultPolicyIssueListOutputFormat(),
		})
	require.NoError(t, err)
	require.Len(t, c.reqs, 3)
	require.NotNil(t, c.reqs[0].FilterModel)
	require.NotEmpty(t, c.reqs[0].SortModel)
	for _, req := range c.reqs {
		assert.Equal(t, c.reqs[0].FilterModel, req.FilterModel)
		assert.Equal(t, c.reqs[0].SortModel, req.SortModel)
	}
}

func TestPolicyIssueList_InvalidFilterFailsBeforeRequest(t *testing.T) {
	t.Parallel()

	c := &recordingPolicyIssueListClient{}
	err := runPolicyIssueList(t.Context(), &bytes.Buffer{}, stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{
			filters:      policyIssueFilters{severities: []string{"urgent"}},
			outputFormat: defaultPolicyIssueListOutputFormat(),
		})
	require.ErrorContains(t, err, "urgent")
	assert.Empty(t, c.reqs)
}

func TestPolicyIssueList_InvalidFilterFailsBeforeFactory(t *testing.T) {
	t.Parallel()

	factory := func(context.Context, string) (policyIssueListClient, string, error) {
		t.Fatal("factory must not be called when a filter is invalid")
		return nil, "", nil
	}
	err := runPolicyIssueList(t.Context(), &bytes.Buffer{}, factory,
		policyIssueListArgs{
			filters:      policyIssueFilters{severities: []string{"urgent"}},
			outputFormat: defaultPolicyIssueListOutputFormat(),
		})
	require.ErrorContains(t, err, "critical")
}

func TestPolicyIssueList_ServerRejectsFilter(t *testing.T) {
	t.Parallel()

	c := &recordingPolicyIssueListClient{err: errors.New(`[400] Invalid input: unknown field "x"`)}
	err := runPolicyIssueList(t.Context(), &bytes.Buffer{}, stubPolicyIssueListFactory(c, "acme"),
		policyIssueListArgs{
			filters:      policyIssueFilters{packs: []string{"p"}},
			outputFormat: defaultPolicyIssueListOutputFormat(),
		})
	require.ErrorContains(t, err, "unknown field")
}
