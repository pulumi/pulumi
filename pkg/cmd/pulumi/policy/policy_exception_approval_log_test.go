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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

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
