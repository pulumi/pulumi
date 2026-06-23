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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// orgMemberEditUpdateCall records a single UpdateOrganizationMember invocation.
type orgMemberEditUpdateCall struct {
	org       string
	userLogin string
	req       apitype.UpdateOrganizationMemberRequest
}

// mockOrgMemberEditClient stubs orgMemberEditClient. updateErr is returned by
// UpdateOrganizationMember; listPages are returned in order by
// ListOrganizationMembers. Calls are recorded on updateCalls/listCalls.
type mockOrgMemberEditClient struct {
	updateErr   error
	updateCalls []orgMemberEditUpdateCall

	listPages []orgMemberGetPage
	listCalls []orgMemberGetCall
}

func (m *mockOrgMemberEditClient) UpdateOrganizationMember(
	_ context.Context, org, userLogin string, req apitype.UpdateOrganizationMemberRequest,
) error {
	m.updateCalls = append(m.updateCalls, orgMemberEditUpdateCall{
		org:       org,
		userLogin: userLogin,
		req:       req,
	})
	return m.updateErr
}

func (m *mockOrgMemberEditClient) ListOrganizationMembers(
	_ context.Context, org, mode string, continuationToken *string,
) (apitype.ListOrganizationMembersResponse, error) {
	m.listCalls = append(m.listCalls, orgMemberGetCall{
		org:               org,
		mode:              mode,
		continuationToken: continuationToken,
	})
	if len(m.listCalls) > len(m.listPages) {
		return apitype.ListOrganizationMembersResponse{}, errors.New("mock: unexpected extra call")
	}
	page := m.listPages[len(m.listCalls)-1]
	return page.resp, page.err
}

func (m *mockOrgMemberEditClient) ListOrgRoles(
	_ context.Context, _, _ string,
) ([]apitype.Role, error) {
	return nil, nil
}

func stubOrgMemberEditFactory(c orgMemberEditClient, org string) orgMemberEditClientFactory {
	return func(_ context.Context, _ string) (orgMemberEditClient, string, error) {
		return c, org, nil
	}
}

func failingOrgMemberEditFactory(err error) orgMemberEditClientFactory {
	return func(_ context.Context, _ string) (orgMemberEditClient, string, error) {
		return nil, "", err
	}
}

func TestOrgMemberEdit_DefaultOutput_RoleOnly(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberEditClient{
		listPages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		stubOrgMemberEditFactory(c, "acme"), "alice",
		orgMemberEditArgs{
			outputFormat: defaultOrgMemberGetOutputFormat(),
			role:         "admin",
			changedFlags: map[string]bool{"role": true},
		})
	require.NoError(t, err)

	assert.Equal(t, []orgMemberEditUpdateCall{
		{
			org:       "acme",
			userLogin: "alice",
			req:       apitype.UpdateOrganizationMemberRequest{Role: ptr("admin")},
		},
	}, c.updateCalls)

	expected := "User name:       Alice Example\n" +
		"GitHub login:    alice\n" +
		"Role:            admin\n" +
		"FGA role:        Admin\n" +
		"FGA role ID:     admin\n" +
		"Joined at:       2025-01-02T03:04:05Z\n"
	assert.Equal(t, expected, buf.String())
}

func TestOrgMemberEdit_JSONOutput_BothFlags(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberEditClient{
		listPages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	args := orgMemberEditArgs{
		outputFormat: defaultOrgMemberGetOutputFormat(),
		role:         "admin",
		fgaRoleID:    "role-abc",
		changedFlags: map[string]bool{"role": true, "fga-role-id": true},
	}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		stubOrgMemberEditFactory(c, "acme"), "alice", args)
	require.NoError(t, err)

	assert.Equal(t, []orgMemberEditUpdateCall{
		{
			org:       "acme",
			userLogin: "alice",
			req: apitype.UpdateOrganizationMemberRequest{
				Role:      ptr("admin"),
				FgaRoleId: ptr("role-abc"),
			},
		},
	}, c.updateCalls)

	assert.JSONEq(t, `{
		"role": "admin",
		"user": {
			"name": "Alice Example",
			"githubLogin": "alice",
			"avatarUrl": "https://example.com/alice.png"
		},
		"created": "2025-01-02T03:04:05Z",
		"knownToPulumi": true,
		"virtualAdmin": false,
		"fgaRole": {
			"id": "admin",
			"name": "Admin",
			"modifiedAt": "2025-01-02T03:04:05Z"
		}
	}`, buf.String())
}

func TestOrgMemberEdit_NoFlagsChanged(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberEditClient{}
	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		stubOrgMemberEditFactory(c, "acme"), "alice",
		orgMemberEditArgs{
			outputFormat: defaultOrgMemberGetOutputFormat(),
			role:         "admin",
			fgaRoleID:    "role-abc",
			changedFlags: map[string]bool{"role": false, "fga-role-id": false},
		})
	require.Error(t, err)
	assert.Equal(t,
		"no changes specified; pass --role, --fga-role-id, or --fga-role-name",
		err.Error())
	assert.Empty(t, c.updateCalls)
	assert.Empty(t, c.listCalls)
}

func TestOrgMemberEdit_UpdateError(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberEditClient{
		updateErr: errors.New("boom"),
	}

	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		stubOrgMemberEditFactory(c, "acme"), "alice",
		orgMemberEditArgs{
			outputFormat: defaultOrgMemberGetOutputFormat(),
			role:         "admin",
			changedFlags: map[string]bool{"role": true},
		})
	require.Error(t, err)
	assert.Equal(t, "updating organization member: boom", err.Error())
	assert.Empty(t, c.listCalls)
	assert.Equal(t, "", buf.String())
}

func TestOrgMemberEdit_GetAfterPatchError(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberEditClient{
		listPages: []orgMemberGetPage{
			{err: errors.New("boom")},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		stubOrgMemberEditFactory(c, "acme"), "alice",
		orgMemberEditArgs{
			outputFormat: defaultOrgMemberGetOutputFormat(),
			role:         "admin",
			changedFlags: map[string]bool{"role": true},
		})
	require.Error(t, err)
	assert.Equal(t,
		"reading organization member after edit: getting organization member: boom",
		err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgMemberEdit_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgMemberEdit(t.Context(), &buf,
		failingOrgMemberEditFactory(errors.New("not logged in")),
		"alice",
		orgMemberEditArgs{
			outputFormat: defaultOrgMemberGetOutputFormat(),
			role:         "admin",
			changedFlags: map[string]bool{"role": true},
		})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
