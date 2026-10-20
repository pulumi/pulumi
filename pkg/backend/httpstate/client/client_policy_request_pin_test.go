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

package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi-cloud-sdk/go/apitype/jsonvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type capturedRequest struct {
	method string
	path   string
	body   string
}

func newCapturingClient(t *testing.T, captured *capturedRequest) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		captured.method = r.Method
		captured.path = r.URL.EscapedPath()
		captured.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)
	return newMockClient(server)
}

func rawJSONPtr(s string) *json.RawMessage {
	return new(json.RawMessage(s))
}

func pinCreatePolicyGroupRequest() cloudapitype.NewPolicyGroupRequest {
	return cloudapitype.NewPolicyGroupRequest{
		Name:        "prod-policies",
		EntityType:  new(cloudapitype.PolicyGroupEntityTypeStacks), //nolint:staticcheck
		Mode:        new(cloudapitype.PolicyGroupModePreventative),
		AgentPoolID: jsonvalue.NotNull("pool-123"),
	}
}

func TestPin_CreatePolicyGroup_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	require.NoError(t, c.CreatePolicyGroup(t.Context(), "acme", pinCreatePolicyGroupRequest()))

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/orgs/acme/policygroups", got.path)
	assert.JSONEq(t, `{
		"name": "prod-policies",
		"entityType": "stacks",
		"mode": "preventative",
		"agentPoolId": "pool-123"
	}`, got.body)
}

func pinBatchUpdatePolicyGroupRequests() []cloudapitype.UpdatePolicyGroupRequest {
	newName := "renamed"
	return []cloudapitype.UpdatePolicyGroupRequest{
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{NewName: &newName}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			AddStack: &cloudapitype.AppPulumiStackReference{Name: "dev", RoutingProject: "web"},
		}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			RemoveStack: &cloudapitype.AppPulumiStackReference{Name: "old", RoutingProject: "api"},
		}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			AddPolicyPack: &cloudapitype.AppPolicyPackMetadata{
				Name:       "aws-pack",
				VersionTag: "1.2.3",
				Config: map[string]*json.RawMessage{
					"s3-no-public": rawJSONPtr(`{"enforcementLevel":"mandatory","tags":["a","b"]}`),
				},
			},
		}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			RemovePolicyPack: &cloudapitype.AppPolicyPackMetadata{Name: "legacy-pack", VersionTag: "0.9.0"},
		}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			AddInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: "prod-aws"},
		}},
		{AppUpdatePolicyGroupRequest: cloudapitype.AppUpdatePolicyGroupRequest{
			RemoveInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: "stale-gcp"},
		}},
	}
}

func TestPin_BatchUpdatePolicyGroup_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	require.NoError(t, c.BatchUpdatePolicyGroup(t.Context(), "acme", "prod-policies", pinBatchUpdatePolicyGroupRequests()))

	assert.Equal(t, http.MethodPatch, got.method)
	assert.Equal(t, "/api/orgs/acme/policygroups/prod-policies/batch", got.path)
	assert.JSONEq(t, `[
		{"newName": "renamed"},
		{"addStack": {"name": "dev", "routingProject": "web"}},
		{"removeStack": {"name": "old", "routingProject": "api"}},
		{"addPolicyPack": {
			"name": "aws-pack",
			"displayName": "",
			"version": 0,
			"versionTag": "1.2.3",
			"config": {"s3-no-public": {"enforcementLevel": "mandatory", "tags": ["a", "b"]}}
		}},
		{"removePolicyPack": {"name": "legacy-pack", "displayName": "", "version": 0, "versionTag": "0.9.0"}},
		{"addInsightsAccount": {"name": "prod-aws"}},
		{"removeInsightsAccount": {"name": "stale-gcp"}}
	]`, got.body)
}

func TestPin_ApplyPolicyPack_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	config := map[string]*json.RawMessage{
		"s3-no-public":   rawJSONPtr(`{"enforcementLevel":"mandatory"}`),
		"ec2-no-ssh-any": rawJSONPtr(`{"enforcementLevel":"advisory","allowedCidrs":["10.0.0.0/8"]}`),
	}
	require.NoError(t, c.ApplyPolicyPack(t.Context(), "acme", "prod-policies", "aws-pack", "1.2.3", config))

	assert.Equal(t, http.MethodPatch, got.method)
	assert.Equal(t, "/api/orgs/acme/policygroups/prod-policies", got.path)
	assert.JSONEq(t, `{
		"addPolicyPack": {
			"name": "aws-pack",
			"displayName": "",
			"version": 0,
			"versionTag": "1.2.3",
			"config": {
				"s3-no-public": {"enforcementLevel": "mandatory"},
				"ec2-no-ssh-any": {"enforcementLevel": "advisory", "allowedCidrs": ["10.0.0.0/8"]}
			}
		}
	}`, got.body)
}

func TestPin_DisablePolicyPack_Body(t *testing.T) {
	t.Parallel()

	t.Run("with versionTag", func(t *testing.T) {
		t.Parallel()

		var got capturedRequest
		c := newCapturingClient(t, &got)
		require.NoError(t, c.DisablePolicyPack(t.Context(), "acme", "prod-policies", "aws-pack", "1.2.3"))

		assert.Equal(t, http.MethodPatch, got.method)
		assert.Equal(t, "/api/orgs/acme/policygroups/prod-policies", got.path)
		assert.JSONEq(t, `{"removePolicyPack": {
			"name": "aws-pack", "displayName": "", "version": 0, "versionTag": "1.2.3"}}`, got.body)
	})

	t.Run("without versionTag", func(t *testing.T) {
		t.Parallel()

		var got capturedRequest
		c := newCapturingClient(t, &got)
		require.NoError(t, c.DisablePolicyPack(t.Context(), "acme", "prod-policies", "aws-pack", ""))

		assert.JSONEq(t, `{"removePolicyPack": {
			"name": "aws-pack", "displayName": "", "version": 0, "versionTag": ""}}`, got.body)
	})
}

func TestPin_ListPolicyIssues_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	_, err := c.ListPolicyIssues(t.Context(), "acme", cloudapitype.AngularGridGetRowsRequest{
		StartRow: 100,
		EndRow:   200,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/orgs/acme/policyresults/issues", got.path)
	assert.JSONEq(t, `{
		"startRow": 100, "endRow": 200, "sortModel": null,
		"rowGroupCols": null, "valueCols": null, "groupKeys": null, "filterModel": null}`, got.body)
}

func TestPin_GetPolicyComplianceResults_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	size := 50
	token := "next-page-token"
	_, err := c.GetPolicyComplianceResults(t.Context(), "acme", cloudapitype.GetPolicyComplianceResultsRequest{
		Entity:            cloudapitype.PolicyComplianceEntityTypeStack,
		Size:              &size,
		ContinuationToken: &token,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/orgs/acme/policyresults/compliance", got.path)
	assert.JSONEq(t, `{"entity": "stack", "size": 50, "continuationToken": "next-page-token"}`, got.body)
}

func TestPin_CreateInsightsAccount_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	err := c.CreateInsightsAccount(t.Context(), "acme", "prod-aws", apitype.CreateInsightsAccountRequest{
		Provider:       "aws",
		Environment:    "infra/aws-prod@v3",
		ScanSchedule:   "daily",
		AgentPoolID:    "pool-123",
		ProviderConfig: json.RawMessage(`{"regions":["us-west-2","eu-west-1"],"options":{"deep":true}}`),
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/preview/insights/acme/accounts/prod-aws", got.path)
	assert.JSONEq(t, `{
		"provider": "aws",
		"environment": "infra/aws-prod@v3",
		"scanSchedule": "daily",
		"agentPoolID": "pool-123",
		"providerConfig": {"regions": ["us-west-2", "eu-west-1"], "options": {"deep": true}}
	}`, got.body)
}
