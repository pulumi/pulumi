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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func samplePolicyGroupSummaries() []apitype.PolicyGroupSummary {
	return []apitype.PolicyGroupSummary{
		{
			Name:                  "default-policy-group",
			IsOrgDefault:          true,
			NumStacks:             42,
			NumAccounts:           3,
			EntityType:            apitype.Stacks,
			Mode:                  apitype.PolicyGroupModePreventative,
			NumEnabledPolicyPacks: 5,
		},
		{
			Name:                  "audit-accounts",
			NumAccounts:           7,
			EntityType:            apitype.Accounts,
			Mode:                  apitype.PolicyGroupModeAudit,
			NumEnabledPolicyPacks: 2,
		},
	}
}

func TestPolicyGroupList_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyGroupsConsole(&buf, samplePolicyGroupSummaries()))
	assert.Equal(t, `NAME                  DEFAULT  ENABLED POLICY PACKS  STACKS
default-policy-group  Y        5                     42
audit-accounts        N        2                     0
`, buf.String())
}

func TestPolicyGroupList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyGroupsJSON(&buf, samplePolicyGroupSummaries()))
	assert.JSONEq(t, `[
		{"name": "default-policy-group", "default": true, "numPolicyPacks": 5, "numStacks": 42},
		{"name": "audit-accounts", "default": false, "numPolicyPacks": 2, "numStacks": 0}
	]`, buf.String())
}
