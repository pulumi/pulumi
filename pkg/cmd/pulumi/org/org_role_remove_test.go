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

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
)

func TestOrgRoleRemove_Yes_NoConfirm(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-1", "--yes"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "role-1", c.deleteID)
	assert.False(t, c.deleteForce)
	assert.Contains(t, buf.String(), `Removed role "role-1"`)
}

func TestOrgRoleRemove_ForceFlagPassesThrough(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1", "--yes", "--force"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.True(t, c.deleteForce)
}

func TestOrgRoleRemove_NonInteractiveWithoutYes(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.Empty(t, c.deleteID, "client must not be called without confirmation")
}

func TestOrgRoleRemove_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-1", "--yes", "--force", "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"organization":"my-org","action":"Removed","roleId":"role-1","forced":true}`,
		buf.String())
}

func TestOrgRoleRemove_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1", "--yes", "--output", "xml"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `output "xml" not supported`)
}

func TestOrgRoleRemove_DeleteError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{deleteErr: errors.New("conflict")}
	cmd := newOrgRoleRemoveCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1", "--yes"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestOrgRoleRemove_FactoryError(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleRemoveCmdWith(failingRoleFactory(errors.New("not logged in")))
	cmd.SetArgs([]string{"role-1", "--yes"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestOrgRoleRemove_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleRemoveCmd()
	assert.Equal(t, "remove <role-id>", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("force"))
	yes := cmd.Flags().Lookup("yes")
	require.NotNil(t, yes)
	assert.Equal(t, "y", yes.Shorthand)
}
