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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
)

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

	// A failed approval fails; failing to read the request after it only warns, since the approval went through.
	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), reviewErr: errors.New("boom")}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--yes")
	assert.EqualError(t, err, "approving policy exception request: boom")
	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), rereadErr: errors.New("boom")}
	stdout, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalApproveCmd, c, approvalCRID, "--yes")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.True(t, strings.HasSuffix(stderr, "Approved policy exception request "+approvalCRID+"\n"+
		"warning: couldn't read policy exception request "+approvalCRID+" after approving it: boom\n"), stderr)
}
