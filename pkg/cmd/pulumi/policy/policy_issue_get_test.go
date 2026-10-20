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
	"net/http"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyIssueGetClient stubs policyIssueGetClient. It returns a fixed
// response (or error) and records the arguments it was called with.
type mockPolicyIssueGetClient struct {
	resp       cloudapitype.GetPolicyIssueResponse
	err        error
	gotOrg     string
	gotIssueID string
}

func (m *mockPolicyIssueGetClient) GetPolicyIssue(
	_ context.Context, org, issueID string,
) (cloudapitype.GetPolicyIssueResponse, error) {
	m.gotOrg = org
	m.gotIssueID = issueID
	if m.err != nil {
		return cloudapitype.GetPolicyIssueResponse{}, m.err
	}
	return m.resp, nil
}

func stubPolicyIssueGetFactory(c policyIssueGetClient, org string) policyIssueGetClientFactory {
	return func(_ context.Context, _ string) (policyIssueGetClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyIssueGetFactory(err error) policyIssueGetClientFactory {
	return func(_ context.Context, _ string) (policyIssueGetClient, string, error) {
		return nil, "", err
	}
}

func samplePolicyIssue() cloudapitype.PolicyIssue {
	return cloudapitype.PolicyIssue{
		ID:            "issue-1",
		PolicyName:    "no-public-buckets",
		PolicyPack:    "aws-guardrails",
		PolicyPackTag: "1.2.0",
		Level:         string(apitype.Mandatory),
		Severity:      cloudapitype.AppPolicySeverityHigh,
		ResourceURN:   "urn:pulumi:prod::web::aws:s3/bucket:Bucket::data",
		ResourceType:  "aws:s3/bucket:Bucket",
		EntityID:      "prod",
		EntityProject: "web",
		Message:       "S3 bucket must not allow public access",
		ObservedAt:    mustParseTime("2026-05-01T12:00:00Z"),
	}
}

func TestPolicyIssueGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueGetClient{resp: cloudapitype.GetPolicyIssueResponse{PolicyIssue: samplePolicyIssue()}}
	err := runPolicyIssueGet(t.Context(), buf,
		stubPolicyIssueGetFactory(c, "acme"), "issue-1",
		policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `ID:                  issue-1
Policy pack:         aws-guardrails@1.2.0
Policy:              no-public-buckets
Enforcement level:   mandatory
Severity:            high
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::data
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-05-01T12:00:00Z
Message:             S3 bucket must not allow public access
`, buf.String())
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "issue-1", c.gotIssueID)
}

func TestPolicyIssueGet_JSONOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueGetClient{resp: cloudapitype.GetPolicyIssueResponse{PolicyIssue: samplePolicyIssue()}}
	args := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyIssueGet(t.Context(), buf,
		stubPolicyIssueGetFactory(c, "acme"), "issue-1", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{"policyIssue": {
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
}}`, buf.String())
}

func TestPolicyIssueGet_ClientError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyIssueGetClient{err: errors.New("not found")}
	err := runPolicyIssueGet(t.Context(), buf,
		stubPolicyIssueGetFactory(c, "acme"), "missing",
		policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "getting policy issue: not found", err.Error())
}

func TestPolicyIssueGet_FactoryError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	err := runPolicyIssueGet(t.Context(), buf,
		failingPolicyIssueGetFactory(errors.New("not logged in")),
		"issue-1", policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}

func runPolicyIssueGetAgainstServer(t *testing.T, issueJSON string, args policyIssueGetArgs) string {
	t.Helper()
	c := newPolicyIssueServerClient(t, func(*http.Request) string {
		return `{"policyIssue": ` + issueJSON + `}`
	})
	buf := &bytes.Buffer{}
	require.NoError(t, runPolicyIssueGet(t.Context(), buf, stubPolicyIssueGetFactory(c, "acme"), "issue-1", args))
	return buf.String()
}

func TestPolicyIssueGet_UnknownEnumValues(t *testing.T) {
	t.Parallel()

	jsonArgs := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	var got struct {
		PolicyIssue map[string]any `json:"policyIssue"`
	}
	require.NoError(t, json.Unmarshal([]byte(runPolicyIssueGetAgainstServer(t, unknownEnumIssueJSON, jsonArgs)), &got))
	assert.Equal(t, "future_status", got.PolicyIssue["status"])
	assert.Equal(t, "future", got.PolicyIssue["severity"])

	text := runPolicyIssueGetAgainstServer(t, unknownEnumIssueJSON,
		policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()})
	assert.Contains(t, text, "Severity:            future\n")
	assert.Contains(t, text, "Status:              future_status\n")
}

func TestPolicyIssueGet_Timestamps(t *testing.T) {
	t.Parallel()

	jsonArgs := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	out := runPolicyIssueGetAgainstServer(t, fractionalTimestampIssueJSON, jsonArgs)
	assert.Contains(t, out, `"observedAt": "2026-10-01T12:34:56.789123Z"`)
	assert.NotContains(t, out, "lastModified")

	text := runPolicyIssueGetAgainstServer(t, fractionalTimestampIssueJSON,
		policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()})
	assert.Equal(t, `ID:                  issue-1
Policy pack:         aws-guardrails@3.1.0
Policy:              s3-no-public
Enforcement level:   mandatory
Severity:            critical
Status:              open
Priority:            P1
Kind:                audit
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-10-01T12:34:56.789123Z
Message:             Bucket must not allow public access & <must> be "private".
`, text)
}

func TestPolicyIssueGet_JSON_Full(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	resp := cloudapitype.GetPolicyIssueResponse{PolicyIssue: samplePolicyIssueFull()}
	require.NoError(t, renderPolicyIssueGetJSON(&buf, resp))
	got := buf.String()
	assert.JSONEq(t, `{"policyIssue": {
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
}}
`, got)
}

func TestPolicyIssueGet_JSON_Minimal(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	resp := cloudapitype.GetPolicyIssueResponse{PolicyIssue: samplePolicyIssueMinimal()}
	require.NoError(t, renderPolicyIssueGetJSON(&buf, resp))
	got := buf.String()
	assert.JSONEq(t, `{"policyIssue": {
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
}}
`, got)
}

func TestPolicyIssueGet_Table(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	resp := cloudapitype.GetPolicyIssueResponse{PolicyIssue: samplePolicyIssueFull()}
	require.NoError(t, renderPolicyIssueGetText(&buf, resp))
	got := buf.String()
	assert.Equal(t, `ID:                  issue-1
Policy pack:         aws-guardrails@3.1.0
Policy:              s3-no-public
Enforcement level:   mandatory
Severity:            critical
Status:              open
Priority:            P1
Kind:                audit
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-03-04T05:06:07.123456Z
Message:             Bucket must not allow public access & <must> be "private".
`, got)
}

func sampleFullPolicyIssueResponse() cloudapitype.GetPolicyIssueResponse {
	issue := samplePolicyIssue()
	issue.Status = cloudapitype.PolicyIssueStatusOpen
	issue.Kind = cloudapitype.PolicyIssueKindAudit
	issue.Priority = cloudapitype.PolicyIssuePriorityP1
	issue.AssignedTo = &cloudapitype.UserInfo{GitHubLogin: "alice", Name: "Alice"}
	issue.PolicyGroupName = new("prod")
	issue.PolicyGroupType = new(cloudapitype.PolicyGroupModeAudit)
	return cloudapitype.GetPolicyIssueResponse{
		PolicyIssue: issue,
		Policy: &cloudapitype.AppPolicy{
			Name:             "no-public-buckets",
			DisplayName:      "No public buckets",
			Description:      "d",
			RemediationSteps: "r",
			URL:              "https://example.com/no-public-buckets",
			Tags:             []string{"s3", "security"},
			Framework: &cloudapitype.AppPolicyComplianceFramework{
				Name: "CIS", Version: "1.4", Reference: "2.1.5",
			},
		},
	}
}

func renderPolicyIssueGetWith(t *testing.T, resp cloudapitype.GetPolicyIssueResponse, format string) string {
	t.Helper()
	args := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
	if format != "" {
		require.NoError(t, args.outputFormat.Set(format))
	}
	buf := &bytes.Buffer{}
	require.NoError(t, runPolicyIssueGet(t.Context(), buf,
		stubPolicyIssueGetFactory(&mockPolicyIssueGetClient{resp: resp}, "acme"), "issue-1", args))
	return buf.String()
}

func TestPolicyIssueGet_JSONFullDetails(t *testing.T) {
	t.Parallel()

	out := renderPolicyIssueGetWith(t, sampleFullPolicyIssueResponse(), "json")

	var got struct {
		PolicyIssue map[string]json.RawMessage `json:"policyIssue"`
		Policy      json.RawMessage            `json:"policy"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.JSONEq(t, `"p1"`, string(got.PolicyIssue["priority"]))
	assert.JSONEq(t, `"audit"`, string(got.PolicyIssue["kind"]))
	assert.JSONEq(t, `{"name": "Alice", "githubLogin": "alice", "avatarUrl": ""}`, string(got.PolicyIssue["assignedTo"]))
	assert.JSONEq(t, `"prod"`, string(got.PolicyIssue["policyGroupName"]))
	assert.JSONEq(t, `"audit"`, string(got.PolicyIssue["policyGroupType"]))
	assert.JSONEq(t, `{
		"name": "no-public-buckets",
		"displayName": "No public buckets",
		"description": "d",
		"enforcementLevel": "",
		"message": "",
		"remediationSteps": "r",
		"url": "https://example.com/no-public-buckets",
		"tags": ["s3", "security"],
		"framework": {"name": "CIS", "version": "1.4", "reference": "2.1.5"}
	}`, string(got.Policy))
}

func TestPolicyIssueGet_TextFullDetails(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `ID:                  issue-1
Policy pack:         aws-guardrails@1.2.0
Policy:              no-public-buckets
Enforcement level:   mandatory
Severity:            high
Status:              open
Priority:            p1
Assignee:            alice
Policy group:        prod (audit)
Kind:                audit
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::data
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-05-01T12:00:00Z
Message:             S3 bucket must not allow public access
Description:         d
Remediation:         r
Framework:           CIS 1.4 — 2.1.5
More info:           https://example.com/no-public-buckets
`, renderPolicyIssueGetWith(t, sampleFullPolicyIssueResponse(), ""))
}

func TestPolicyIssueGet_NullPolicy(t *testing.T) {
	t.Parallel()

	c := newPolicyIssueServerClient(t, func(*http.Request) string {
		return `{"policyIssue": ` + fractionalTimestampIssueJSON + `, "policy": null, "policyPack": null}`
	})
	run := func(format string) string {
		args := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
		if format != "" {
			require.NoError(t, args.outputFormat.Set(format))
		}
		buf := &bytes.Buffer{}
		require.NoError(t, runPolicyIssueGet(t.Context(), buf, stubPolicyIssueGetFactory(c, "acme"), "issue-1", args))
		return buf.String()
	}

	var got map[string]map[string]any
	require.NoError(t, json.Unmarshal([]byte(run("json")), &got))
	assert.NotContains(t, got, "policy")
	assert.Equal(t, "s3-no-public", got["policyIssue"]["policyName"])

	text := run("")
	assert.NotContains(t, text, "Description:")
	assert.NotContains(t, text, "Remediation:")
	assert.NotContains(t, text, "Framework:")
	assert.NotContains(t, text, "More info:")
}

func TestPolicyIssueGet_Unassigned(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		assignedTo *cloudapitype.UserInfo
	}{
		{"nil", nil},
		{"empty login", &cloudapitype.UserInfo{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := sampleFullPolicyIssueResponse()
			resp.PolicyIssue.AssignedTo = tt.assignedTo

			assert.NotContains(t, renderPolicyIssueGetWith(t, resp, ""), "Assignee:")
		})
	}
}

func TestFormatPolicyFramework(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		framework cloudapitype.AppPolicyComplianceFramework
		want      string
	}{
		{cloudapitype.AppPolicyComplianceFramework{Name: "CIS", Version: "1.4", Reference: "2.1.5"}, "CIS 1.4 — 2.1.5"},
		{cloudapitype.AppPolicyComplianceFramework{Name: "CIS", Version: "1.4"}, "CIS 1.4"},
		{cloudapitype.AppPolicyComplianceFramework{Name: "CIS", Reference: "2.1.5"}, "CIS — 2.1.5"},
		{cloudapitype.AppPolicyComplianceFramework{Reference: "2.1.5"}, "2.1.5"},
		{cloudapitype.AppPolicyComplianceFramework{}, ""},
	} {
		assert.Equal(t, tt.want, formatPolicyFramework(&tt.framework))
	}
	assert.Empty(t, formatPolicyFramework(nil))
}
