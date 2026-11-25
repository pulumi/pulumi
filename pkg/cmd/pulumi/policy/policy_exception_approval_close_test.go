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

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
)

func TestPolicyExceptionApprovalClose(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest()}
	_, stderr, err := runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID, "-m", "fix it", "--yes")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(stderr, "Policy exception request "+approvalCRID+
		" will be closed without creating a policy exception.\n\nID:"), stderr)
	assert.Contains(t, stderr, "Closed policy exception request "+approvalCRID+"\n")
	assert.Equal(t, []string{"close " + approvalCRID + " fix it"}, c.gotReviews)
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID)
	assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)

	// Closing a closed request does nothing, and says so.
	c = &mockPolicyExceptionClient{crCurrent: "closed", draft: sampleExceptionRequest()}
	_, stderr, err = runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID, "-m", "x", "--yes")
	require.NoError(t, err)
	assert.Contains(t, stderr,
		"warning: Policy exception request "+approvalCRID+" is already closed. The comment was not posted.")
	assert.Empty(t, c.gotReviews)

	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), reviewErr: errors.New("boom")}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalCloseCmd, c, approvalCRID, "--yes")
	assert.EqualError(t, err, "closing policy exception request: boom")
}
