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

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// orgMemberGetCall records a single ListOrganizationMembers invocation made
// by the command under test.
type orgMemberGetCall struct {
	org               string
	mode              string
	continuationToken *string
}

// orgMemberGetPage is the canned response for a single call.
type orgMemberGetPage struct {
	resp apitype.ListOrganizationMembersResponse
	err  error
}

// mockOrgMemberGetClient stubs orgMemberGetClient. It returns pages in the
// order given, and records every call it received.
type mockOrgMemberGetClient struct {
	pages []orgMemberGetPage
	calls []orgMemberGetCall
}

func (m *mockOrgMemberGetClient) ListOrganizationMembers(
	_ context.Context, org, mode string, continuationToken *string,
) (apitype.ListOrganizationMembersResponse, error) {
	m.calls = append(m.calls, orgMemberGetCall{
		org:               org,
		mode:              mode,
		continuationToken: continuationToken,
	})
	if len(m.calls) > len(m.pages) {
		return apitype.ListOrganizationMembersResponse{}, errors.New("mock: unexpected extra call")
	}
	page := m.pages[len(m.calls)-1]
	return page.resp, page.err
}

func stubOrgMemberGetFactory(c orgMemberGetClient, org string) orgMemberGetClientFactory {
	return func(_ context.Context, _ string) (orgMemberGetClient, string, error) {
		return c, org, nil
	}
}

func failingOrgMemberGetFactory(err error) orgMemberGetClientFactory {
	return func(_ context.Context, _ string) (orgMemberGetClient, string, error) {
		return nil, "", err
	}
}

func aliceMember() apitype.OrganizationMember {
	return apitype.OrganizationMember{
		Role: "admin",
		User: apitype.UserInfo{
			Name:        "Alice Example",
			GitHubLogin: "alice",
			AvatarURL:   "https://example.com/alice.png",
		},
		Created:       "2025-01-02T03:04:05Z",
		KnownToPulumi: true,
		FGARole: apitype.FGARole{
			ID:         "admin",
			Name:       "Admin",
			ModifiedAt: "2025-01-02T03:04:05Z",
		},
	}
}

func TestOrgMemberGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "alice", orgMemberGetArgs{})
	require.NoError(t, err)

	expected := "User name:       Alice Example\n" +
		"GitHub login:    alice\n" +
		"Role:            admin\n" +
		"FGA role:        Admin\n" +
		"FGA role ID:     admin\n" +
		"Joined at:       2025-01-02T03:04:05Z\n"
	assert.Equal(t, expected, buf.String())
	assert.Equal(t, []orgMemberGetCall{
		{org: "acme", mode: "frontend", continuationToken: nil},
	}, c.calls)
}

func TestOrgMemberGet_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "alice",
		orgMemberGetArgs{output: "json"})
	require.NoError(t, err)

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

func TestOrgMemberGet_MatchesCaseInsensitively(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "ALICE",
		orgMemberGetArgs{})
	require.NoError(t, err)
	// Single call confirms the case-insensitive match returned on the first
	// page; we don't need the full rendered output, only that the lookup
	// succeeded against the lower-cased login.
	require.Len(t, c.calls, 1)
}

func TestOrgMemberGet_PaginationAcrossPages(t *testing.T) {
	t.Parallel()

	bob := apitype.OrganizationMember{
		User:    apitype.UserInfo{Name: "Bob", GitHubLogin: "bob"},
		Role:    "member",
		FGARole: apitype.FGARole{ID: "member", Name: "Member"},
	}

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{
				Members:           []apitype.OrganizationMember{aliceMember()},
				ContinuationToken: "page-2",
			}},
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{bob},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "bob",
		orgMemberGetArgs{output: "json"})
	require.NoError(t, err)

	tok := "page-2"
	assert.Equal(t, []orgMemberGetCall{
		{org: "acme", mode: "frontend", continuationToken: nil},
		{org: "acme", mode: "frontend", continuationToken: &tok},
	}, c.calls)
	assert.JSONEq(t, `{
		"role": "member",
		"user": {
			"name": "Bob",
			"githubLogin": "bob",
			"avatarUrl": ""
		},
		"created": "",
		"knownToPulumi": false,
		"virtualAdmin": false,
		"fgaRole": {
			"id": "member",
			"name": "Member",
			"modifiedAt": ""
		}
	}`, buf.String())
}

func TestOrgMemberGet_FallsBackToBackendMode(t *testing.T) {
	t.Parallel()

	// Member is only present in the "backend" list. Confirms we exhaust the
	// frontend list (including pagination) before falling through.
	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{Members: nil}},
			{resp: apitype.ListOrganizationMembersResponse{
				Members: []apitype.OrganizationMember{aliceMember()},
			}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "alice",
		orgMemberGetArgs{output: "json"})
	require.NoError(t, err)
	assert.Equal(t, []orgMemberGetCall{
		{org: "acme", mode: "frontend", continuationToken: nil},
		{org: "acme", mode: "backend", continuationToken: nil},
	}, c.calls)
}

func TestOrgMemberGet_NotFound(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{resp: apitype.ListOrganizationMembersResponse{Members: nil}},
			{resp: apitype.ListOrganizationMembersResponse{Members: nil}},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "ghost", orgMemberGetArgs{})
	require.Error(t, err)
	assert.Equal(t, `organization member "ghost" not found in acme`, err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgMemberGet_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{}
	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "alice",
		orgMemberGetArgs{output: "yaml"})
	require.Error(t, err)
	assert.Equal(t,
		`invalid --output value "yaml" (must be 'default' or 'json')`,
		err.Error())
	// Validation must run before any API call.
	assert.Empty(t, c.calls)
}

func TestOrgMemberGet_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockOrgMemberGetClient{
		pages: []orgMemberGetPage{
			{err: errors.New("boom")},
		},
	}

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		stubOrgMemberGetFactory(c, "acme"), "alice", orgMemberGetArgs{})
	require.Error(t, err)
	assert.Equal(t, "getting organization member: boom", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgMemberGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgMemberGet(t.Context(), &buf,
		failingOrgMemberGetFactory(errors.New("not logged in")),
		"alice", orgMemberGetArgs{})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
