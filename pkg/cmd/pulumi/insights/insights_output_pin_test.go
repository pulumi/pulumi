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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

func pinInsightsAccountFull() cloudapitype.InsightsAccount {
	finished := time.Date(2026, 3, 5, 6, 7, 8, 500_000_000, time.UTC)
	return cloudapitype.InsightsAccount{
		ID:                   "acct-1",
		Name:                 "prod-aws",
		Provider:             "aws",
		ProviderVersion:      "6.1.0", //nolint:staticcheck
		Environment:          "infra/aws-prod@v3",
		ScheduledScanEnabled: true,
		AgentPoolID:          "pool-123",
		ProviderConfig:       json.RawMessage(`{"regions":["us-west-2","eu-west-1"],"options":{"deep":true,"limit":2.5}}`),
		OwnedBy: cloudapitype.UserInfo{
			Name:        "Ada Lovelace",
			GitHubLogin: "ada",
			AvatarURL:   "https://avatars.example.com/ada.png",
		},
		ScanStatus: cloudapitype.ScanStatus{
			WorkflowRun: cloudapitype.WorkflowRun{
				ID:            "scan-9",
				OrgID:         "org-1",
				UserID:        "user-1",
				Status:        "succeeded",
				StartedAt:     time.Date(2026, 3, 5, 6, 0, 0, 123_456_000, time.UTC),
				FinishedAt:    &finished,
				LastUpdatedAt: time.Date(2026, 3, 5, 6, 7, 9, 250_000_000, time.UTC),
				JobTimeout:    time.Date(2026, 3, 5, 7, 0, 0, 0, time.UTC),
			},
			AccountName:   "prod-aws",
			ResourceCount: 1234,
		},
	}
}

func pinInsightsAccountNoScanStatus() cloudapitype.InsightsAccount {
	return cloudapitype.InsightsAccount{
		ID:       "acct-2",
		Name:     "fresh-gcp",
		Provider: "gcp",
		OwnedBy:  cloudapitype.UserInfo{Name: "Grace Hopper", GitHubLogin: "grace"},
	}
}

func TestPin_InsightsAccountList_JSON_Full(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &insightsAccountListCmd{}
	require.NoError(t, c.renderJSON(&buf, []cloudapitype.InsightsAccount{pinInsightsAccountFull()}))
	assert.JSONEq(t, `{
  "accounts": [
    {
      "id": "acct-1",
      "name": "prod-aws",
      "provider": "aws",
      "providerVersion": "6.1.0",
      "providerEnvRef": "infra/aws-prod@v3",
      "scheduledScanEnabled": true,
      "agentPoolID": "pool-123",
      "providerConfig": {
        "regions": [
          "us-west-2",
          "eu-west-1"
        ],
        "options": {
          "deep": true,
          "limit": 2.5
        }
      },
      "ownedBy": {
        "name": "Ada Lovelace",
        "githubLogin": "ada",
        "avatarUrl": "https://avatars.example.com/ada.png"
      },
      "scanStatus": {
        "accountName": "prod-aws",
        "id": "scan-9",
        "orgId": "org-1",
        "resourceCount": 1234,
        "userId": "user-1",
        "status": "succeeded",
        "startedAt": "2026-03-05T06:00:00.123456Z",
        "finishedAt": "2026-03-05T06:07:08.5Z",
        "lastUpdatedAt": "2026-03-05T06:07:09.25Z",
        "jobTimeout": "2026-03-05T07:00:00Z"
      }
    }
  ]
}
`, buf.String())
}

func TestPin_InsightsAccountList_JSON_NoScanStatus(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &insightsAccountListCmd{}
	require.NoError(t, c.renderJSON(&buf, []cloudapitype.InsightsAccount{pinInsightsAccountNoScanStatus()}))
	assert.JSONEq(t, `{
  "accounts": [
    {
      "id": "acct-2",
      "name": "fresh-gcp",
      "provider": "gcp",
      "scheduledScanEnabled": false,
      "ownedBy": {
        "name": "Grace Hopper",
        "githubLogin": "grace",
        "avatarUrl": ""
      }
    }
  ]
}
`, buf.String())
}

func TestPin_InsightsAccountList_Table(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &insightsAccountListCmd{}
	require.NoError(t, c.renderTable(&buf, []cloudapitype.InsightsAccount{
		pinInsightsAccountFull(), pinInsightsAccountNoScanStatus(),
	}))
	assert.Equal(t, `┌───────────┬──────────┬───────┬────────────────┬────────────────────────┬───────────┐
│ Name      │ Provider │ Owner │ Scheduled Scan │ Last Scan              │ Resources │
├───────────┼──────────┼───────┼────────────────┼────────────────────────┼───────────┤
│ prod-aws  │ aws      │ ada   │ yes            │ succeeded (2026-03-05) │ 1234      │
│ fresh-gcp │ gcp      │ grace │ no             │ -                      │ -         │
└───────────┴──────────┴───────┴────────────────┴────────────────────────┴───────────┘
`, buf.String())
}

func TestPin_InsightsAccountNew_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, renderInsightsAccountJSON(&buf, pinInsightsAccountFull()))
	assert.JSONEq(t, `{
  "id": "acct-1",
  "name": "prod-aws",
  "provider": "aws",
  "providerVersion": "6.1.0",
  "providerEnvRef": "infra/aws-prod@v3",
  "scheduledScanEnabled": true,
  "agentPoolID": "pool-123",
  "providerConfig": {
    "regions": [
      "us-west-2",
      "eu-west-1"
    ],
    "options": {
      "deep": true,
      "limit": 2.5
    }
  },
  "ownedBy": {
    "name": "Ada Lovelace",
    "githubLogin": "ada",
    "avatarUrl": "https://avatars.example.com/ada.png"
  },
  "scanStatus": {
    "accountName": "prod-aws",
    "id": "scan-9",
    "orgId": "org-1",
    "resourceCount": 1234,
    "userId": "user-1",
    "status": "succeeded",
    "startedAt": "2026-03-05T06:00:00.123456Z",
    "finishedAt": "2026-03-05T06:07:08.5Z",
    "lastUpdatedAt": "2026-03-05T06:07:09.25Z",
    "jobTimeout": "2026-03-05T07:00:00Z"
  }
}
`, buf.String())
}

func TestPin_InsightsAccountNew_Table(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, renderInsightsAccountText(&buf, pinInsightsAccountFull()))
	assert.Equal(t, `ID:                    acct-1
Name:                  prod-aws
Owner:                 Ada Lovelace (ada)
Provider:              aws (v6.1.0)
Environment:           infra/aws-prod@v3
Scheduled scan:        enabled
Agent pool:            pool-123
Provider config:
{
  "regions": [
    "us-west-2",
    "eu-west-1"
  ],
  "options": {
    "deep": true,
    "limit": 2.5
  }
}
Scan status:           succeeded
Last scan finished:    2026-03-05T06:07:08Z
Resources discovered:  1234
`, buf.String())
}
