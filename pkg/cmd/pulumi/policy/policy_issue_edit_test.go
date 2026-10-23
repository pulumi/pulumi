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
	"context"
	"encoding/json"
	"errors"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
)

type mockPolicyIssueEditClient struct {
	factoryCalled bool
	calls         []string
	requests      []cloudapitype.UpdatePolicyIssueRequest
	err           error
}

func (m *mockPolicyIssueEditClient) UpdatePolicyIssue(
	_ context.Context, _, issueID string, req cloudapitype.UpdatePolicyIssueRequest,
) (cloudapitype.GetPolicyIssueResponse, error) {
	m.calls = append(m.calls, issueID)
	m.requests = append(m.requests, req)
	if m.err != nil {
		return cloudapitype.GetPolicyIssueResponse{}, m.err
	}
	issue := cloudapitype.PolicyIssue{ID: issueID, Kind: cloudapitype.PolicyIssueKindAudit}
	if req.Status != nil {
		issue.Status = *req.Status
	}
	if req.Priority != nil {
		issue.Priority = *req.Priority
	}
	if req.AssignedTo != nil && *req.AssignedTo != "" {
		issue.AssignedTo = &cloudapitype.UserInfo{GitHubLogin: *req.AssignedTo}
	}
	return cloudapitype.GetPolicyIssueResponse{PolicyIssue: issue}, nil
}

func runPolicyIssueEditCmd(
	t *testing.T, c *mockPolicyIssueEditClient, cmdArgs ...string,
) (string, error) {
	t.Helper()
	factory := func(context.Context, string) (policyIssueEditClient, string, error) {
		c.factoryCalled = true
		return c, "acme", nil
	}
	cmd := newPolicyIssueEditCmdWith(factory)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs(cmdArgs)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), err
}

func TestBuildUpdatePolicyIssueRequest(t *testing.T) {
	t.Parallel()

	t.Run("status and priority", func(t *testing.T) {
		t.Parallel()
		req := buildUpdatePolicyIssueRequest(policyIssueEditArgs{
			status: "in_progress", priority: "p1",
			changed: map[string]bool{"status": true, "priority": true},
		})
		require.NotNil(t, req.Status)
		assert.Equal(t, cloudapitype.PolicyIssueStatusInProgress, *req.Status)
		require.NotNil(t, req.Priority)
		assert.Equal(t, cloudapitype.PolicyIssuePriorityP1, *req.Priority)
		assert.Nil(t, req.AssignedTo)
	})

	t.Run("assignee", func(t *testing.T) {
		t.Parallel()
		req := buildUpdatePolicyIssueRequest(policyIssueEditArgs{
			assignee: "alice", changed: map[string]bool{"assignee": true},
		})
		require.NotNil(t, req.AssignedTo)
		assert.Equal(t, "alice", *req.AssignedTo)
	})

	t.Run("unassign", func(t *testing.T) {
		t.Parallel()
		req := buildUpdatePolicyIssueRequest(policyIssueEditArgs{unassign: true})
		require.NotNil(t, req.AssignedTo)
		assert.Empty(t, *req.AssignedTo)
	})
}

func TestPolicyIssueEdit_InvalidArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no change", []string{"i1"}, "--status, --priority, --assignee or --unassign"},
		{"fixed status", []string{"i1", "--status", "fixed"}, "open, in_progress, ignored"},
		{"by_design status", []string{"i1", "--status", "by_design"}, "open, in_progress, ignored"},
		{"bad priority", []string{"i1", "--priority", "p9"}, "p0, p1, p2, p3, p4"},
		{"empty status", []string{"i1", "--status", ""}, `invalid status ""`},
		{"empty priority", []string{"i1", "--priority", ""}, `invalid priority ""`},
		{
			"empty assignee",
			[]string{"i1", "--status", "open", "--assignee", ""},
			"--assignee must not be empty; use --unassign to clear the assignee",
		},
		{
			"assignee with unassign",
			[]string{"i1", "--assignee", "bob", "--unassign"},
			"--assignee and --unassign cannot be used together",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &mockPolicyIssueEditClient{}
			_, err := runPolicyIssueEditCmd(t, c, tt.args...)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, cmdCmd.ExitConfigurationError, cmdCmd.ExitCodeFor(err))
			assert.False(t, c.factoryCalled)
			assert.Empty(t, c.calls)
		})
	}
}

func TestPolicyIssueEdit_RejectsMultipleIDs(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueEditClient{}
	_, err := runPolicyIssueEditCmd(t, c, "i1", "i2", "--status", "open")
	require.Error(t, err)
	assert.Empty(t, c.calls)
}

func TestPolicyIssueEdit_UpdateAlias(t *testing.T) {
	t.Parallel()

	parent := &cobra.Command{Use: "issue"}
	parent.AddCommand(newPolicyIssueEditCmd())
	cmd, _, err := parent.Find([]string{"update", "i1"})
	require.NoError(t, err)
	assert.Equal(t, "edit", cmd.Name())
}

func TestPolicyIssueEdit_Text(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueEditClient{}
	out, err := runPolicyIssueEditCmd(t, c, "i1", "--status", "ignored", "--priority", "p2")
	require.NoError(t, err)
	assert.Equal(t, []string{"i1"}, c.calls)

	var want bytes.Buffer
	require.NoError(t, renderPolicyIssueGetText(&want, cloudapitype.GetPolicyIssueResponse{
		PolicyIssue: cloudapitype.PolicyIssue{
			ID:       "i1",
			Kind:     cloudapitype.PolicyIssueKindAudit,
			Status:   cloudapitype.PolicyIssueStatusIgnored,
			Priority: cloudapitype.PolicyIssuePriorityP2,
		},
	}))
	assert.Equal(t, want.String(), out)
}

func TestPolicyIssueEdit_JSON(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueEditClient{}
	out, err := runPolicyIssueEditCmd(t, c, "i1", "--status", "in_progress", "--output", "json")
	require.NoError(t, err)

	var got cloudapitype.GetPolicyIssueResponse
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "i1", got.PolicyIssue.ID)
	assert.Equal(t, cloudapitype.PolicyIssueStatusInProgress, got.PolicyIssue.Status)
}

func TestPolicyIssueEdit_APIError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyIssueEditClient{err: errors.New("updating policy issue: boom")}
	out, err := runPolicyIssueEditCmd(t, c, "i1", "--status", "open")
	require.EqualError(t, err, "updating policy issue: boom")
	assert.Equal(t, cmdCmd.ExitCodeError, cmdCmd.ExitCodeFor(err))
	assert.Empty(t, out)
}
