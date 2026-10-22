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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

func TestPin_CreateInsightsAccount_Body(t *testing.T) {
	t.Parallel()

	var got capturedRequest
	c := newCapturingClient(t, &got)
	err := c.CreateInsightsAccount(t.Context(), "acme", "prod-aws", cloudapitype.CreateInsightsAccountRequest{
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
