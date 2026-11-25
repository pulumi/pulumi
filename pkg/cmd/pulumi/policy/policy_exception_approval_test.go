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

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const approvalCRID = "fa8258ae-1124-4cdc-98c8-6a6a2f6a2fc5"

func sampleExceptionRequest() cloudapitype.CreatePolicyExceptionRequest {
	return cloudapitype.CreatePolicyExceptionRequest{
		PolicyPack: "aws-security",
		Reason:     "Logs are public by design",
		Match: []apitype.PolicyExceptionMatch{
			{Policies: []string{"s3-no-public-read"}, Targets: []string{logsURN}},
		},
		ExpiresInSeconds: new(int64(7 * 24 * 3600)),
	}
}

// gateWith returns a gate that needs two approvals and has the given approvers.
func gateWith(approvers ...string) esc_client.ChangeRequestGateEvaluation {
	users := make([]esc_client.ChangeRequestUser, len(approvers))
	for i, a := range approvers {
		users[i] = esc_client.ChangeRequestUser{GithubLogin: a}
	}
	return esc_client.ChangeRequestGateEvaluation{
		Satisfied: len(approvers) >= 2,
		ApplicableGates: []esc_client.ChangeGateEvaluation{{
			Name:      "Exceptions",
			Satisfied: len(approvers) >= 2,
			RuleDetails: esc_client.ChangeGateRuleEvaluation{
				RuleType: "approval_required", RequiredApprovals: 2, Approvers: users,
			},
		}},
	}
}

// runApprovalCmd runs a `policy exception approval` subcommand against the mock.
func runApprovalCmd(
	t *testing.T, newCmd func(policyExceptionClientFactory) *cobra.Command, c *mockPolicyExceptionClient,
	args ...string,
) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newCmd(stubPolicyExceptionFactory(c, "acme"))
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

// None of the commands act on a change request for something else.
func TestPolicyExceptionApprovalOtherEntityTypes(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crEntityType: "environment"}
	for _, tc := range []struct {
		newCmd func(policyExceptionClientFactory) *cobra.Command
		args   []string
	}{
		{newPolicyExceptionApprovalGetCmd, nil},
		{newPolicyExceptionApprovalApproveCmd, []string{"--yes"}},
		{newPolicyExceptionApprovalCloseCmd, []string{"--yes"}},
		{newPolicyExceptionApprovalUnapproveCmd, nil},
		{newPolicyExceptionApprovalCommentCmd, []string{"-m", "x"}},
		{newPolicyExceptionApprovalLogCmd, nil},
	} {
		_, _, err := runApprovalCmd(t, tc.newCmd, c, append([]string{approvalCRID}, tc.args...)...)
		assert.EqualError(t, err, "change request "+approvalCRID+" isn't a policy exception request; "+
			"use `pulumi env approval` for environment change requests")
	}
	assert.Empty(t, c.gotReviews)
}
