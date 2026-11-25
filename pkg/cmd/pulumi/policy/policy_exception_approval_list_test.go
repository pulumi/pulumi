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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

func TestPolicyExceptionApprovalList(t *testing.T) {
	t.Parallel()

	request := func(id, status string) esc_client.ChangeRequest {
		return esc_client.ChangeRequest{
			ID: id, Status: status, Action: "create",
			Entity:    esc_client.ChangeRequestEntity{EntityType: policyExceptionsEntityType},
			CreatedBy: esc_client.ChangeRequestUser{GithubLogin: "alice"},
		}
	}
	c := &mockPolicyExceptionClient{
		draft: sampleExceptionRequest(),
		crs:   []esc_client.ChangeRequest{request("cr-1", "pending"), request("cr-2", "applied"), request("cr-3", "ready")},
	}
	stdout, stderr, err := runApprovalCmd(t, newPolicyExceptionApprovalListCmd, c)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, []string{"policy_exceptions "}, c.gotListedCRs)
	assert.Equal(t, `┌──────┬──────────────┬───────────────────┬─────────┬────────┬───────────────────────────┐
│ ID   │ POLICY PACK  │ POLICIES          │ TARGETS │ AUTHOR │ REASON                    │
├──────┼──────────────┼───────────────────┼─────────┼────────┼───────────────────────────┤
│ cr-1 │ aws-security │ s3-no-public-read │       1 │ alice  │ Logs are public by design │
│ cr-3 │ aws-security │ s3-no-public-read │       1 │ alice  │ Logs are public by design │
└──────┴──────────────┴───────────────────┴─────────┴────────┴───────────────────────────┘
`, stdout)

	stdout, _, err = runApprovalCmd(t, newPolicyExceptionApprovalListCmd, c, "--status", "applied", "--output", "json")
	require.NoError(t, err)
	var got struct {
		ChangeRequests []policyExceptionRequestSummary `json:"changeRequests"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	require.Len(t, got.ChangeRequests, 1)
	assert.Equal(t, "cr-2", got.ChangeRequests[0].ID)
	assert.Equal(t, "aws-security", got.ChangeRequests[0].Exception.PolicyPack)

	c = &mockPolicyExceptionClient{}
	stdout, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalListCmd, c, "--status", "closed")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "No policy exception requests found.\n", stderr)

	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalListCmd, c, "--status", "pending")
	assert.EqualError(t, err, `invalid --status "pending": expected open, applied, closed or all`)
}
