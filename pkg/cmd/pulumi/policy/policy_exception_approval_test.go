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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
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

func TestPolicyExceptionApprovalGet(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), gates: gateWith("bob")}
	stdout, _, err := runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID, "--org", "other-org")
	require.NoError(t, err)
	assert.Equal(t, "other-org", c.gotOrgFlag)
	assert.Equal(t, `ID:            `+approvalCRID+`
Policy pack:   aws-security
Status:        pending
Requested:     2026-11-18T09:00:00Z by alice
Expires:       7 days after it's created
Approved:      false
  Exceptions (not satisfied): 1 of 2 approvals (bob)
URL:           http://localhost:3000/other-org/web/prod?exception=`+approvalCRID+`

Policy Pack exceptions configuration:
{
  "reason": "Logs are public by design",
  "match": [
    {
      "policies": [
        "s3-no-public-read"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"
      ]
    }
  ]
}
`, stdout)

	// Once applied, it names the exception it created.
	c = &mockPolicyExceptionClient{
		crCurrent: "applied", draft: sampleExceptionRequest(),
		pages: []cloudapitype.ListPolicyExceptionsResponse{
			{
				Exceptions:        []cloudapitype.PolicyException{{ID: "EXC-3", ChangeRequestID: new("other")}},
				ContinuationToken: new("next"),
			},
			{Exceptions: []cloudapitype.PolicyException{{ID: "EXC-4", ChangeRequestID: new(approvalCRID)}}},
		},
	}
	stdout, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID, "--output", "json")
	require.NoError(t, err)
	var got policyExceptionRequest
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, "EXC-4", got.ExceptionID)
	assert.Equal(t, "applied", got.Status)
	assert.Equal(t, "aws-security", got.Exception.PolicyPack)
	require.Len(t, c.gotListOpts, 2)
	assert.Equal(t, "aws-security", c.gotListOpts[0].PolicyPack)
	assert.Equal(t, []string{"active", "removed"}, c.gotListOpts[0].Status)
	assert.Equal(t, "next", c.gotListOpts[1].ContinuationToken)

	// Change requests for something else aren't shown.
	c = &mockPolicyExceptionClient{crEntityType: "environment"}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "change request "+approvalCRID+" isn't a policy exception request; "+
		"use `pulumi env approval` for environment change requests")
	c = &mockPolicyExceptionClient{noCR: true}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "policy exception request "+approvalCRID+" not found in acme")

	// An organization without the feature.
	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{
		Code: http.StatusConflict, Message: `"acme" does not support policy-exceptions.`,
	}}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "policy exceptions aren't enabled for organization acme")
}

func TestPolicyExceptionApprovalApprove(t *testing.T) {
	t.Parallel()

	// Approving the last approval needed creates the exception, and prints its ID.
	c := &mockPolicyExceptionClient{
		crCurrent: "pending", approveStatus: "applied", draft: sampleExceptionRequest(),
		pages: []cloudapitype.ListPolicyExceptionsResponse{
			{Exceptions: []cloudapitype.PolicyException{{ID: "EXC-9", ChangeRequestID: new(approvalCRID)}}},
		},
	}
	stdout, stderr, err := runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "-m", "lgtm", "--yes")
	require.NoError(t, err)
	assert.Equal(t, []string{"approve " + approvalCRID + "@2 lgtm"}, c.gotReviews)
	assert.Equal(t, "EXC-9\n", stdout)
	assert.Contains(t, stderr, "Policy exception request "+approvalCRID+
		" will create a policy exception for Policy Pack aws-security.\n\nID:")
	assert.Contains(t, stderr, "\nApproved policy exception request "+approvalCRID+"\nCreated policy exception EXC-9\n")

	// More approvals are needed.
	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), gates: gateWith("bob")}
	stdout, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--yes")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.True(t, strings.HasSuffix(stderr, "\nApproved policy exception request "+approvalCRID+"\n"), stderr)

	// Approved, but not created.
	c = &mockPolicyExceptionClient{crCurrent: "pending", approveStatus: "ready", draft: sampleExceptionRequest()}
	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--yes")
	require.NoError(t, err)
	assert.Contains(t, stderr,
		"Approved policy exception request "+approvalCRID+"\nThe request is pending to be applied.\n")

	// --dry-run shows the request without approving it; without --yes, approving needs a terminal.
	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest()}
	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Status:        pending\n")
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID)
	assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.Empty(t, c.gotReviews)

	for _, status := range []string{"applied", "closed"} {
		c = &mockPolicyExceptionClient{crCurrent: status, draft: sampleExceptionRequest()}
		_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--yes")
		assert.EqualError(t, err, "policy exception request "+approvalCRID+" is "+status)
		assert.Empty(t, c.gotReviews)
	}
}

func TestPolicyExceptionApprovalReviews(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest()}
	_, stderr, err := runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID, "-m", "fix it", "--yes")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Closed policy exception request "+approvalCRID+"\n")
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID)
	assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)

	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalUnapproveCmd, c, approvalCRID)
	require.NoError(t, err)
	assert.Equal(t, "Removed approval from policy exception request "+approvalCRID+"\n", stderr)

	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalCommentCmd, c, approvalCRID, "-m", "why?")
	require.NoError(t, err)
	assert.Equal(t, "Added comment to policy exception request "+approvalCRID+"\n", stderr)
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalCommentCmd, c, approvalCRID)
	assert.EqualError(t, err, "a comment is required: pass -m <comment>")

	// Closing a closed request does nothing, and says so.
	closed := &mockPolicyExceptionClient{crCurrent: "closed", draft: sampleExceptionRequest()}
	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, closed, approvalCRID, "-m", "x", "--yes")
	require.NoError(t, err)
	assert.Contains(t, stderr,
		"warning: Policy exception request "+approvalCRID+" is already closed. The comment was not posted.")
	assert.Empty(t, closed.gotReviews)

	assert.Equal(t, []string{
		"close " + approvalCRID + " fix it",
		"unapprove " + approvalCRID,
		"comment " + approvalCRID + " why?",
	}, c.gotReviews)

	// None of them act on a change request for something else.
	c = &mockPolicyExceptionClient{crEntityType: "environment"}
	for _, tc := range []struct {
		newCmd func(policyExceptionClientFactory) *cobra.Command
		args   []string
	}{
		{newPolicyExceptionApprovalApproveCmd, []string{"--yes"}},
		{newPolicyExceptionApprovalCloseCmd, []string{"--yes"}},
		{newPolicyExceptionApprovalUnapproveCmd, nil},
		{newPolicyExceptionApprovalCommentCmd, []string{"-m", "x"}},
		{newPolicyExceptionApprovalLogCmd, nil},
	} {
		_, _, err = runApprovalCmd(t, tc.newCmd, c, append([]string{approvalCRID}, tc.args...)...)
		assert.ErrorContains(t, err, "isn't a policy exception request")
	}
	assert.Empty(t, c.gotReviews)
}

func TestPolicyExceptionApprovalLog(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	user := esc_client.ChangeRequestUser{GithubLogin: "bob"}
	c := &mockPolicyExceptionClient{crEvents: []esc_client.ChangeRequestEvent{
		{EventType: "status_changed", NewStatus: "draft", CreatedAt: at, CreatedBy: user},
		{EventType: "commented", Comment: "old", ReplacedBy: "e9", CreatedAt: at, CreatedBy: user},
		{EventType: "commented", Comment: "Which CDN?\nThe new one?", CreatedAt: at, CreatedBy: user},
		{EventType: "approved_by_user", RevisionNumber: 2, CreatedAt: at, CreatedBy: user},
		{EventType: "status_changed", Reason: "applied successfully", NewStatus: "applied", CreatedAt: at, CreatedBy: user},
	}}
	stdout, _, err := runApprovalCmd(t, newPolicyExceptionApprovalLogCmd, c, approvalCRID)
	require.NoError(t, err)
	assert.Equal(t, `2026-11-18T09:00:00Z  bob requested the exception
2026-11-18T09:00:00Z  bob commented
    Which CDN?
    The new one?
2026-11-18T09:00:00Z  bob approved revision 2
2026-11-18T09:00:00Z  bob created the exception
`, stdout)

	stdout, _, err = runApprovalCmd(t, newPolicyExceptionApprovalLogCmd, c, approvalCRID, "--output", "json")
	require.NoError(t, err)
	var got struct {
		Events []esc_client.ChangeRequestEvent `json:"events"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	require.Len(t, got.Events, 4)
}
