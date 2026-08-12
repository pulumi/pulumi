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

package org

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgRoleAssign_TextOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleAssignCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-123", "platform"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "platform", c.assignTeam)
	assert.Equal(t, "role-123", c.assignID)
	assert.Equal(t, "my-org", c.capturedOrg)
	assert.Contains(t, buf.String(), `Assigned role "role-123" to team "platform"`)
}

func TestOrgRoleAssign_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleAssignCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-123", "platform", "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"organization":"my-org","action":"Assigned","team":"platform","roleId":"role-123"}`,
		buf.String())
}

func TestOrgRoleAssign_MissingTeamArg(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleAssignCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-123"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg(s), received 1")
}

func TestOrgRoleAssign_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleAssignCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-123", "platform", "--output", "xml"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `output "xml" not supported`)
}

func TestOrgRoleAssign_AssignError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{assignErr: errors.New("forbidden")}
	cmd := newOrgRoleAssignCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-123", "platform"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestOrgRoleAssign_FactoryError(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleAssignCmdWith(failingRoleFactory(errors.New("not logged in")))
	cmd.SetArgs([]string{"role-123", "platform"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestOrgRoleAssign_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleAssignCmd()
	assert.Equal(t, "assign <role-id> <team>", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("org"))
	require.NotNil(t, cmd.Flags().Lookup("output"))
}
