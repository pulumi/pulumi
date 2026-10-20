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
	"encoding/json"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func pinRawJSONPtr(s string) *json.RawMessage {
	raw := json.RawMessage(s)
	return &raw
}

func pinGroupSummariesFull() []cloudapitype.AppPolicyGroupSummary {
	return []cloudapitype.AppPolicyGroupSummary{
		{
			Name:                  "default-policy-group",
			IsOrgDefault:          true,
			NumStacks:             42,
			NumAccounts:           3,
			EntityType:            cloudapitype.AppEntityTypeStacks,
			Mode:                  cloudapitype.PolicyGroupModePreventative,
			NumEnabledPolicyPacks: 5,
		},
		{
			Name:                  "audit-accounts",
			IsOrgDefault:          false,
			NumStacks:             0,
			NumAccounts:           7,
			EntityType:            cloudapitype.AppEntityTypeAccounts,
			Mode:                  cloudapitype.PolicyGroupModeAudit,
			NumEnabledPolicyPacks: 2,
		},
	}
}

func pinGroupSummariesMinimal() []cloudapitype.AppPolicyGroupSummary {
	return []cloudapitype.AppPolicyGroupSummary{{Name: "bare", EntityType: cloudapitype.AppEntityTypeStacks}}
}

func pinGroupFull() cloudapitype.PolicyGroup {
	return cloudapitype.PolicyGroup{
		Name:         "prod-policies",
		IsOrgDefault: true,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModePreventative,
		Stacks: []cloudapitype.AppPulumiStackReference{
			{Name: "prod", RoutingProject: "web"},
			{Name: "standalone"},
		},
		AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
			{
				Name:        "aws-guardrails",
				DisplayName: "AWS Guardrails",
				Version:     3,
				VersionTag:  "3.1.0",
				Config: map[string]*json.RawMessage{
					"s3-no-public": pinRawJSONPtr(`{"enforcementLevel":"mandatory","nested":{"tags":["a","b"],"limit":2.5}}`),
				},
				Environments: []string{"infra/policy-secrets", "infra/other@v2"},
			},
			{Name: "tagging", Version: 7},
		},
		Accounts:    []string{"prod-aws", "prod-gcp"},
		AgentPoolID: new("pool-123"),
	}
}

func pinGroupMinimal() cloudapitype.PolicyGroup {
	return cloudapitype.PolicyGroup{
		Name:       "bare",
		EntityType: cloudapitype.PolicyGroupEntityTypeAccounts, //nolint:staticcheck
		Mode:       cloudapitype.PolicyGroupModeAudit,
	}
}

func pinIssueFull() apitype.PolicyIssue {
	return apitype.PolicyIssue{
		ID:               "issue-1",
		EntityType:       "stack",
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
		ObservedAt:       "2026-03-04T05:06:07.123456Z",
		LastModified:     "2026-03-05T06:07:08.5Z",
		Status:           "open",
		Kind:             "audit",
		Priority:         "P1",
	}
}

func pinIssueMinimal() apitype.PolicyIssue {
	return apitype.PolicyIssue{
		ID:         "issue-2",
		PolicyPack: "tagging",
		PolicyName: "require-owner",
		Level:      "advisory",
	}
}

func pinPolicyPacks() []apitype.PolicyPackWithVersions {
	return []apitype.PolicyPackWithVersions{
		{
			Name:        "aws-guardrails",
			DisplayName: "AWS Guardrails",
			Versions:    []int{1, 2, 3},
			VersionTags: []string{"1.0.0", "2.0.0", "3.1.0"},
		},
		{Name: "tagging", DisplayName: "Tagging", Versions: []int{7}, VersionTags: []string{"7.0.0"}},
	}
}

func pinComplianceColumns() []string {
	return []string{"default-policy-group", "audit-accounts", "aws-guardrails"}
}

func pinComplianceRows() []apitype.PolicyComplianceResult {
	return []apitype.PolicyComplianceResult{
		{EntityName: "web/prod", Scores: []int{100, 85, -1}},
		{EntityName: "web/dev", Scores: []int{0, -2, 50}},
	}
}

func renderPin(t *testing.T, render func(w *bytes.Buffer) error) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, render(&buf))
	return buf.String()
}

func TestPin_PolicyGroupList_JSON_Full(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return formatPolicyGroupsJSON(w, pinGroupSummariesFull()) })
	assert.JSONEq(t, `[
  {
    "name": "default-policy-group",
    "default": true,
    "numPolicyPacks": 5,
    "numStacks": 42
  },
  {
    "name": "audit-accounts",
    "default": false,
    "numPolicyPacks": 2,
    "numStacks": 0
  }
]
`, got)
}

func TestPin_PolicyGroupList_JSON_Minimal(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return formatPolicyGroupsJSON(w, pinGroupSummariesMinimal()) })
	assert.JSONEq(t, `[
  {
    "name": "bare",
    "default": false,
    "numPolicyPacks": 0,
    "numStacks": 0
  }
]
`, got)
}

func TestPin_PolicyGroupList_Table(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return formatPolicyGroupsConsole(w, pinGroupSummariesFull()) })
	assert.Equal(t, `NAME                  DEFAULT  ENABLED POLICY PACKS  STACKS
default-policy-group  Y        5                     42
audit-accounts        N        2                     0
`, got)
}

func TestPin_PolicyGroupGet_JSON_Full(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyGroupGetJSON(w, pinGroupFull()) })
	assert.JSONEq(t, `{
  "name": "prod-policies",
  "isOrgDefault": true,
  "entityType": "stacks",
  "mode": "preventative",
  "stacks": [
    {
      "name": "prod",
      "routingProject": "web"
    },
    {
      "name": "standalone"
    }
  ],
  "appliedPolicyPacks": [
    {
      "name": "aws-guardrails",
      "displayName": "AWS Guardrails",
      "version": 3,
      "versionTag": "3.1.0",
      "config": {
        "s3-no-public": {
          "enforcementLevel": "mandatory",
          "nested": {
            "tags": [
              "a",
              "b"
            ],
            "limit": 2.5
          }
        }
      },
      "environments": [
        "infra/policy-secrets",
        "infra/other@v2"
      ]
    },
    {
      "name": "tagging",
      "version": 7
    }
  ],
  "accounts": [
    "prod-aws",
    "prod-gcp"
  ],
  "agentPoolId": "pool-123"
}
`, got)
}

func TestPin_PolicyGroupGet_JSON_Minimal(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyGroupGetJSON(w, pinGroupMinimal()) })
	assert.JSONEq(t, `{
  "name": "bare",
  "isOrgDefault": false,
  "entityType": "accounts",
  "mode": "audit",
  "stacks": [],
  "appliedPolicyPacks": [],
  "accounts": []
}
`, got)
}

func TestPin_PolicyGroupGet_Table(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyGroupGetText(w, pinGroupFull()) })
	assert.Equal(t, `Name:                  prod-policies
Org default:           yes
Entity type:           stacks
Mode:                  preventative
Agent pool:            pool-123
Applied policy packs:  2
  - aws-guardrails (3.1.0)
  - tagging (v7)
Stacks:                2
  - web/prod
  - standalone
Accounts:              2
  - prod-aws
  - prod-gcp
`, got)
}

func TestPin_PolicyGroupNew_JSON(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: pinGroupFull()}
	args := policyGroupNewArgs{
		entityType:   "stacks",
		mode:         "preventative",
		agentPoolID:  "pool-123",
		yes:          true,
		outputFormat: defaultPolicyGroupGetOutputFormat(),
	}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	require.NoError(t, runPolicyGroupNew(t.Context(), &buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies", args))
	assert.JSONEq(t, `{
  "name": "prod-policies",
  "isOrgDefault": true,
  "entityType": "stacks",
  "mode": "preventative",
  "stacks": [
    {
      "name": "prod",
      "routingProject": "web"
    },
    {
      "name": "standalone"
    }
  ],
  "appliedPolicyPacks": [
    {
      "name": "aws-guardrails",
      "displayName": "AWS Guardrails",
      "version": 3,
      "versionTag": "3.1.0",
      "config": {
        "s3-no-public": {
          "enforcementLevel": "mandatory",
          "nested": {
            "tags": [
              "a",
              "b"
            ],
            "limit": 2.5
          }
        }
      },
      "environments": [
        "infra/policy-secrets",
        "infra/other@v2"
      ]
    },
    {
      "name": "tagging",
      "version": 7
    }
  ],
  "accounts": [
    "prod-aws",
    "prod-gcp"
  ],
  "agentPoolId": "pool-123"
}
`, buf.String())
}

func TestPin_PolicyGroupEdit_JSON(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: pinGroupFull()}
	args := policyGroupEditArgs{
		outputFormat: defaultPolicyGroupGetOutputFormat(),
		rename:       "prod-policies",
		changed:      map[string]bool{"name": true},
	}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	require.NoError(t, runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "old-name", args))
	assert.JSONEq(t, `{
  "name": "prod-policies",
  "isOrgDefault": true,
  "entityType": "stacks",
  "mode": "preventative",
  "stacks": [
    {
      "name": "prod",
      "routingProject": "web"
    },
    {
      "name": "standalone"
    }
  ],
  "appliedPolicyPacks": [
    {
      "name": "aws-guardrails",
      "displayName": "AWS Guardrails",
      "version": 3,
      "versionTag": "3.1.0",
      "config": {
        "s3-no-public": {
          "enforcementLevel": "mandatory",
          "nested": {
            "tags": [
              "a",
              "b"
            ],
            "limit": 2.5
          }
        }
      },
      "environments": [
        "infra/policy-secrets",
        "infra/other@v2"
      ]
    },
    {
      "name": "tagging",
      "version": 7
    }
  ],
  "accounts": [
    "prod-aws",
    "prod-gcp"
  ],
  "agentPoolId": "pool-123"
}
`, buf.String())
}

func TestPin_PolicyGroupRemove_JSON(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyGroupRemoveJSON(w, "acme", "prod-policies") })
	assert.JSONEq(t, `{
  "organizationName": "acme",
  "name": "prod-policies"
}
`, got)
}

func TestPin_PolicyGroupRemove_Table(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyGroupRemoveText(w, "acme", "prod-policies") })
	assert.Equal(t, `Removed policy group prod-policies from organization acme.
`, got)
}

func TestPin_PolicyIssueList_JSON_Full(t *testing.T) {
	t.Parallel()
	resp := apitype.ListPolicyIssuesResponse{
		Issues: []apitype.PolicyIssue{pinIssueFull(), pinIssueMinimal()},
		Total:  250,
	}
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyIssueListJSON(w, resp) })
	assert.JSONEq(t, `{
  "issues": [
    {
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
      "observedAt": "2026-03-04T05:06:07.123456Z",
      "lastModified": "2026-03-05T06:07:08.5Z",
      "status": "open",
      "kind": "audit",
      "priority": "P1"
    },
    {
      "id": "issue-2",
      "policyPack": "tagging",
      "policyName": "require-owner",
      "level": "advisory"
    }
  ],
  "total": 250
}
`, got)
}

func TestPin_PolicyIssueList_JSON_Minimal(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error {
		return renderPolicyIssueListJSON(w, apitype.ListPolicyIssuesResponse{})
	})
	assert.JSONEq(t, `{
  "issues": [],
  "total": 0
}
`, got)
}

func TestPin_PolicyIssueList_Table(t *testing.T) {
	t.Parallel()
	resp := apitype.ListPolicyIssuesResponse{
		Issues: []apitype.PolicyIssue{pinIssueFull(), pinIssueMinimal()},
		Total:  250,
	}
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyIssueListTable(w, resp) })
	assert.Equal(t, `┌─────────┬──────────────────────┬───────────────┬─────────────┬──────────┬────────────────────────────────────────────────────────────┐
│ ID      │ POLICY PACK          │ POLICY        │ ENFORCEMENT │ STACK    │ MESSAGE                                                    │
├─────────┼──────────────────────┼───────────────┼─────────────┼──────────┼────────────────────────────────────────────────────────────┤
│ issue-1 │ aws-guardrails@3.1.0 │ s3-no-public  │ mandatory   │ web/prod │ Bucket must not allow public access & <must> be "private". │
│ issue-2 │ tagging              │ require-owner │ advisory    │ -        │                                                            │
└─────────┴──────────────────────┴───────────────┴─────────────┴──────────┴────────────────────────────────────────────────────────────┘

Showing 2 of 250 policy issue(s)
`, got)
}

func TestPin_PolicyIssueGet_JSON_Full(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyIssueGetJSON(w, pinIssueFull()) })
	assert.JSONEq(t, `{
  "id": "issue-1",
  "policyName": "s3-no-public",
  "policyPack": "aws-guardrails",
  "policyPackTag": "3.1.0",
  "level": "mandatory",
  "severity": "critical",
  "status": "open",
  "resourceURN": "urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets",
  "resourceType": "aws:s3/bucket:Bucket",
  "entityProject": "web",
  "entityId": "prod",
  "message": "Bucket must not allow public access & <must> be \"private\".",
  "observedAt": "2026-03-04T05:06:07.123456Z"
}
`, got)
}

func TestPin_PolicyIssueGet_JSON_Minimal(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyIssueGetJSON(w, pinIssueMinimal()) })
	assert.JSONEq(t, `{
  "id": "issue-2",
  "policyName": "require-owner",
  "policyPack": "tagging",
  "level": "advisory"
}
`, got)
}

func TestPin_PolicyIssueGet_Table(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return renderPolicyIssueGetText(w, pinIssueFull()) })
	assert.Equal(t, `ID:                  issue-1
Policy pack:         aws-guardrails@3.1.0
Policy:              s3-no-public
Enforcement level:   mandatory
Severity:            critical
Status:              open
Stack:               web/prod
Resource URN:        urn:pulumi:prod::web::aws:s3/bucket:Bucket::assets
Resource type:       aws:s3/bucket:Bucket
Observed at:         2026-03-04T05:06:07.123456Z
Message:             Bucket must not allow public access & <must> be "private".
`, got)
}

func TestPin_ComplianceList_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &complianceListCmd{w: &buf}
	require.NoError(t, c.renderJSON(pinComplianceColumns(), pinComplianceRows()))
	assert.JSONEq(t, `{
  "results": [
    {
      "entity": "web/prod",
      "scores": {
        "audit-accounts": 85,
        "aws-guardrails": -1,
        "default-policy-group": 100
      }
    },
    {
      "entity": "web/dev",
      "scores": {
        "audit-accounts": -2,
        "aws-guardrails": 50,
        "default-policy-group": 0
      }
    }
  ],
  "count": 2
}
`, buf.String())
}

func TestPin_ComplianceList_Table(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &complianceListCmd{w: &buf}
	require.NoError(t, c.renderTable(pinComplianceColumns(), pinComplianceRows()))
	assert.Equal(t, `┌──────────┬──────────────────────┬────────────────┬────────────────┐
│ ENTITY   │ DEFAULT-POLICY-GROUP │ AUDIT-ACCOUNTS │ AWS-GUARDRAILS │
├──────────┼──────────────────────┼────────────────┼────────────────┤
│ web/prod │ 100%                 │ 85%            │ N/A            │
│ web/dev  │ 0%                   │ ERR            │ 50%            │
└──────────┴──────────────────────┴────────────────┴────────────────┘

2 result(s)
`, buf.String())
}

func TestPin_PolicyList_JSON(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return formatPolicyPacksJSON(w, pinPolicyPacks()) })
	assert.JSONEq(t, `[
  {
    "name": "aws-guardrails",
    "versions": [
      "1.0.0",
      "2.0.0",
      "3.1.0"
    ]
  },
  {
    "name": "tagging",
    "versions": [
      "7.0.0"
    ]
  }
]
`, got)
}

func TestPin_PolicyList_Table(t *testing.T) {
	t.Parallel()
	got := renderPin(t, func(w *bytes.Buffer) error { return formatPolicyPacksConsole(w, pinPolicyPacks()) })
	assert.Equal(t, `NAME            VERSIONS
aws-guardrails  1.0.0, 2.0.0, 3.1.0
tagging         7.0.0
`, got)
}
