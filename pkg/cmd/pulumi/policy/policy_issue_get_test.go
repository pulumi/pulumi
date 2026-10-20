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
	resp       cloudapitype.PolicyIssue
	err        error
	gotOrg     string
	gotIssueID string
}

func (m *mockPolicyIssueGetClient) GetPolicyIssue(
	_ context.Context, org, issueID string,
) (cloudapitype.PolicyIssue, error) {
	m.gotOrg = org
	m.gotIssueID = issueID
	if m.err != nil {
		return cloudapitype.PolicyIssue{}, m.err
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
	c := &mockPolicyIssueGetClient{resp: samplePolicyIssue()}
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
	c := &mockPolicyIssueGetClient{resp: samplePolicyIssue()}
	args := policyIssueGetArgs{outputFormat: defaultPolicyIssueGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyIssueGet(t.Context(), buf,
		stubPolicyIssueGetFactory(c, "acme"), "issue-1", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
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
		"observedAt": "2026-05-01T12:00:00Z"
	}`, buf.String())
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
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(runPolicyIssueGetAgainstServer(t, unknownEnumIssueJSON, jsonArgs)), &got))
	assert.Equal(t, "future_status", got["status"])
	assert.Equal(t, "future", got["severity"])

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
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-10-01T12:34:56.789123Z
Message:             Bucket must not allow public access & <must> be "private".
`, text)
}
