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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func currentRole() apitype.Role {
	return apitype.Role{
		ID:          "role-1",
		OrgID:       "my-org",
		Name:        "Old Name",
		Description: "Old description",
		UXPurpose:   "organization",
		Version:     2,
		Details:     json.RawMessage(`{"__type":"role","scopes":["old"]}`),
		Created:     "2026-01-01T00:00:00Z",
		Modified:    "2026-02-01T00:00:00Z",
	}
}

func TestOrgRoleEdit_MergesUnsetFields(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{
		getResp:    currentRole(),
		updateResp: currentRole(),
	}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-1", "--name", "Auditor"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "role-1", c.updateID)
	assert.Equal(t, apitype.UpdateRoleRequest{
		Name:        "Auditor",
		Description: "Old description",
		Details:     json.RawMessage(`{"__type":"role","scopes":["old"]}`),
	}, c.updateReq)
}

func TestOrgRoleEdit_ExplicitEmptyDescriptionClears(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{
		getResp:    currentRole(),
		updateResp: currentRole(),
	}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))

	cmd.SetArgs([]string{"role-1", "--description", ""})
	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Empty(t, c.updateReq.Description)
}

func TestOrgRoleEdit_NoChangesError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{getResp: currentRole()}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to update")
}

func TestOrgRoleEdit_DetailsFromFile(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{
		getResp:    currentRole(),
		updateResp: currentRole(),
	}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role","scopes":["new"]}`)

	cmd.SetArgs([]string{"role-1", "--details-file", detailsPath})
	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{"__type":"role","scopes":["new"]}`), c.updateReq.Details)
}

func TestOrgRoleEdit_GetError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{getErr: errors.New("not found")}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1", "--name", "x"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `looking up role "role-1"`)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrgRoleEdit_UpdateError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{
		getResp:   currentRole(),
		updateErr: errors.New("server error"),
	}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"role-1", "--name", "x"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating organization role")
	assert.Contains(t, err.Error(), "server error")
}

func TestOrgRoleEdit_TextOutput(t *testing.T) {
	t.Parallel()

	updated := currentRole()
	updated.Name = "Auditor"
	updated.Version = 3
	c := &mockOrgRoleClient{getResp: currentRole(), updateResp: updated}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-1", "--name", "Auditor"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `Updated role "Auditor"`)
	assert.Contains(t, out, "version: 3")
}

func TestOrgRoleEdit_JSONOutput(t *testing.T) {
	t.Parallel()

	updated := currentRole()
	updated.Name = "Auditor"
	c := &mockOrgRoleClient{getResp: currentRole(), updateResp: updated}
	cmd := newOrgRoleEditCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"role-1", "--name", "Auditor", "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organization": "my-org",
		"action": "Updated",
		"role": {
			"id": "role-1",
			"name": "Auditor",
			"description": "Old description",
			"purpose": "organization",
			"version": 2,
			"isOrgDefault": false,
			"created": "2026-01-01T00:00:00Z",
			"modified": "2026-02-01T00:00:00Z"
		}
	}`, buf.String())
}

func TestOrgRoleEdit_FactoryError(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleEditCmdWith(failingRoleFactory(errors.New("not logged in")))
	cmd.SetArgs([]string{"role-1", "--name", "x"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestOrgRoleEdit_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleEditCmd()
	assert.Equal(t, "edit <role-id>", cmd.Use)

	require.NotNil(t, cmd.Flags().Lookup("name"))
	require.NotNil(t, cmd.Flags().Lookup("description"))
	require.NotNil(t, cmd.Flags().Lookup("details-file"))
	require.NotNil(t, cmd.Flags().Lookup("org"))
}
