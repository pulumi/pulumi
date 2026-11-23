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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyExceptionApprovalUnapprove(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest()}
	_, stderr, err := runApprovalCmd(t, newPolicyExceptionApprovalUnapproveCmd, c, approvalCRID)
	require.NoError(t, err)
	assert.Equal(t, "Removed approval from policy exception request "+approvalCRID+"\n", stderr)
	assert.Equal(t, []string{"unapprove " + approvalCRID}, c.gotReviews)

	c = &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), reviewErr: errors.New("boom")}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalUnapproveCmd, c, approvalCRID)
	assert.EqualError(t, err, "removing approval: boom")
}
