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

func samplePolicyGroupSummaries() []cloudapitype.AppPolicyGroupSummary {
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
			NumAccounts:           7,
			EntityType:            cloudapitype.AppEntityTypeAccounts,
			Mode:                  cloudapitype.PolicyGroupModeAudit,
			NumEnabledPolicyPacks: 2,
		},
	}
}

func TestPolicyGroupList_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyGroupsConsole(&buf, samplePolicyGroupSummaries()))
	out := buf.String()

	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "DEFAULT")
	assert.Contains(t, out, "ENABLED POLICY PACKS")
	assert.Contains(t, out, "STACKS")
	assert.Contains(t, out, "default-policy-group")
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "audit-accounts")
	assert.Contains(t, out, "ACCOUNTS")
	assert.Contains(t, out, "MODE")
	assert.Contains(t, out, "preventative")
	assert.Contains(t, out, "audit")
}

func TestPolicyGroupList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyGroupsJSON(&buf, samplePolicyGroupSummaries()))
	assert.JSONEq(t, `[
		{
			"name": "default-policy-group", "default": true, "numPolicyPacks": 5, "numStacks": 42,
			"numAccounts": 3, "mode": "preventative"
		},
		{
			"name": "audit-accounts", "default": false, "numPolicyPacks": 2, "numStacks": 0,
			"numAccounts": 7, "mode": "audit"
		}
	]`, buf.String())
}
