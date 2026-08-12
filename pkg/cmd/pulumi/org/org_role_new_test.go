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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func writeRoleDetailsFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "details.json")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func sampleCreatedRole() apitype.Role {
	return apitype.Role{
		ID:          "role-new",
		OrgID:       "my-org",
		Name:        "Stack Reader",
		Description: "Read-only access",
		UXPurpose:   "organization",
		Version:     1,
		Created:     "2026-05-13T00:00:00Z",
		Modified:    "2026-05-13T00:00:00Z",
	}
}

func TestOrgRoleNew_TextOutput_SendsRequest(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{createResp: sampleCreatedRole()}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role","scopes":["stack:read"]}`)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"Stack Reader", detailsPath,
		"--description", "Read-only access",
		"--purpose", "organization",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, apitype.CreateRoleRequest{
		Name:        "Stack Reader",
		Description: "Read-only access",
		UXPurpose:   "organization",
		Details:     json.RawMessage(`{"__type":"role","scopes":["stack:read"]}`),
	}, c.createReq)
	assert.Equal(t, "my-org", c.capturedOrg)

	out := buf.String()
	assert.Contains(t, out, `Created role "Stack Reader"`)
	assert.Contains(t, out, "id: role-new")
	assert.Contains(t, out, "description: Read-only access")
	assert.Contains(t, out, "purpose: organization")
	assert.Contains(t, out, "version: 1")
}

func TestOrgRoleNew_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{createResp: sampleCreatedRole()}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role"}`)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Stack Reader", detailsPath, "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organization": "my-org",
		"action": "Created",
		"role": {
			"id": "role-new",
			"name": "Stack Reader",
			"description": "Read-only access",
			"purpose": "organization",
			"version": 1,
			"isOrgDefault": false,
			"created": "2026-05-13T00:00:00Z",
			"modified": "2026-05-13T00:00:00Z"
		}
	}`, buf.String())
}

func TestOrgRoleNew_ReadDetailsFromStdin(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{createResp: sampleCreatedRole()}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetIn(strings.NewReader(`{"__type":"role","scopes":["a"]}`))
	cmd.SetArgs([]string{"R", "-"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t,
		json.RawMessage(`{"__type":"role","scopes":["a"]}`),
		c.createReq.Details)
}

func TestOrgRoleNew_MissingDetailsArg(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	cmd.SetArgs([]string{"R"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg(s), received 1")
}

func TestOrgRoleNew_InvalidDetailsJSON(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{not json`)
	cmd.SetArgs([]string{"R", detailsPath})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")
}

func TestOrgRoleNew_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{createResp: sampleCreatedRole()}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role"}`)
	cmd.SetArgs([]string{"R", detailsPath, "--output", "xml"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `output "xml" not supported`)
}

func TestOrgRoleNew_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockOrgRoleClient{createErr: errors.New("server error")}
	cmd := newOrgRoleNewCmdWith(stubRoleFactory(c, "my-org"))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role"}`)
	cmd.SetArgs([]string{"R", detailsPath})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating organization role")
	assert.Contains(t, err.Error(), "server error")
}

func TestOrgRoleNew_FactoryError(t *testing.T) {
	t.Parallel()

	cmd := newOrgRoleNewCmdWith(failingRoleFactory(errors.New("not logged in")))
	detailsPath := writeRoleDetailsFile(t, `{"__type":"role"}`)
	cmd.SetArgs([]string{"R", detailsPath})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}
