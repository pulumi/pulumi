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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func sampleRoles() []apitype.Role {
	return []apitype.Role{
		{
			ID:          "role-1",
			OrgID:       "my-org",
			Name:        "Stack Reader",
			Description: "Read-only access to stacks",
			UXPurpose:   "organization",
			Version:     1,
			Created:     "2026-01-01T00:00:00Z",
			Modified:    "2026-02-01T00:00:00Z",
		},
		{
			ID:           "role-2",
			OrgID:        "my-org",
			Name:         "CI Bot",
			Description:  "Limited token role for CI",
			UXPurpose:    "token",
			Version:      3,
			IsOrgDefault: true,
			Created:      "2026-01-15T00:00:00Z",
			Modified:     "2026-03-01T00:00:00Z",
		},
	}
}

func TestOrgRoleList_TableOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: sampleRoles()}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListTable)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "DESCRIPTION")
	assert.Contains(t, out, "PURPOSE")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "DEFAULT")
	assert.Contains(t, out, "role-1")
	assert.Contains(t, out, "Stack Reader")
	assert.Contains(t, out, "Read-only access to stacks")
	assert.Contains(t, out, "organization")
	assert.Contains(t, out, "role-2")
	assert.Contains(t, out, "CI Bot")
	assert.Contains(t, out, "yes")
	assert.Contains(t, out, "2 role(s)")
}

func TestOrgRoleList_TableOutput_DropsEmptyColumns(t *testing.T) {
	t.Parallel()

	roles := []apitype.Role{
		{ID: "role-1", OrgID: "my-org", Name: "Minimal", Version: 1},
	}

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: roles}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListTable)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "VERSION")
	assert.NotContains(t, out, "DESCRIPTION")
	assert.NotContains(t, out, "PURPOSE")
	assert.NotContains(t, out, "DEFAULT")
}

func TestOrgRoleList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: []apitype.Role{}}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListTable)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `No custom roles found for organization "my-org".`)
}

func TestOrgRoleList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: sampleRoles()}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organization": "my-org",
		"roles": [
			{
				"id": "role-1",
				"name": "Stack Reader",
				"description": "Read-only access to stacks",
				"purpose": "organization",
				"version": 1,
				"isOrgDefault": false,
				"created": "2026-01-01T00:00:00Z",
				"modified": "2026-02-01T00:00:00Z"
			},
			{
				"id": "role-2",
				"name": "CI Bot",
				"description": "Limited token role for CI",
				"purpose": "token",
				"version": 3,
				"isOrgDefault": true,
				"created": "2026-01-15T00:00:00Z",
				"modified": "2026-03-01T00:00:00Z"
			}
		],
		"count": 2
	}`, buf.String())
}

func TestOrgRoleList_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: []apitype.Role{}}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{"organization": "my-org", "roles": [], "count": 0}`, buf.String())
}

func TestOrgRoleList_PurposeFilter_PropagatesToClient(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{roles: []apitype.Role{}}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "team", renderRoleListJSON)
	require.NoError(t, err)
	assert.Equal(t, "team", c.capturedPurpose)
	assert.Equal(t, "my-org", c.capturedOrg)
}

func TestOrgRoleList_InvalidOutput(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleListCmdWith(stubRoleFactory(&mockOrgRoleClient{roles: sampleRoles()}, "my-org"))
	cmd.SetArgs([]string{"--output", "xml"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `output "xml" not supported`)
}

func TestOrgRoleList_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgRoleClient{listErr: errors.New("server error")}
	err := runOrgRoleList(t.Context(), &buf, stubRoleFactory(c, "my-org"), "", "", renderRoleListTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing organization roles")
	assert.Contains(t, err.Error(), "server error")
}

func TestOrgRoleList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgRoleList(t.Context(), &buf,
		failingRoleFactory(errors.New("not logged in")), "", "", renderRoleListTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestOrgRoleList_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{roles: sampleRoles()}
	cmd := newOrgRoleListCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--output", "json", "--purpose", "organization"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"count": 2`)
	assert.Equal(t, "organization", c.capturedPurpose)
}
