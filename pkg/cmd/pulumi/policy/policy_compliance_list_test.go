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
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleComplianceColumns() []string {
	return []string{"default-policy-group", "audit-accounts", "aws-guardrails"}
}

func sampleComplianceRows() []cloudapitype.PolicyComplianceResult {
	return []cloudapitype.PolicyComplianceResult{
		{EntityName: "web/prod", Scores: []int{100, 85, -1}},
		{EntityName: "web/dev", Scores: []int{0, -2, 50}},
	}
}

func TestComplianceList_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &complianceListCmd{w: &buf}
	require.NoError(t, c.renderJSON(sampleComplianceColumns(), sampleComplianceRows()))
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

func TestComplianceList_Table(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := &complianceListCmd{w: &buf}
	require.NoError(t, c.renderTable(sampleComplianceColumns(), sampleComplianceRows()))
	out := buf.String()
	for _, want := range []string{
		"ENTITY", "DEFAULT-POLICY-GROUP", "AUDIT-ACCOUNTS", "AWS-GUARDRAILS",
		"web/prod", "100%", "85%", "N/A",
		"web/dev", "0%", "ERR", "50%",
		"2 result(s)",
	} {
		assert.Contains(t, out, want)
	}
}
