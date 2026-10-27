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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyIssueGetClient stubs policyIssueGetClient. It returns a fixed
// response (or error) and records the arguments it was called with.
type mockPolicyIssueGetClient struct {
	resp       apitype.PolicyIssue
	err        error
	gotOrg     string
	gotIssueID string
}

func (m *mockPolicyIssueGetClient) GetPolicyIssue(
	_ context.Context, org, issueID string,
) (apitype.PolicyIssue, error) {
	m.gotOrg = org
	m.gotIssueID = issueID
	if m.err != nil {
		return apitype.PolicyIssue{}, m.err
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

func samplePolicyIssue() apitype.PolicyIssue {
	return apitype.PolicyIssue{
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
		ObservedAt:    "2026-05-01T12:00:00Z",
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
