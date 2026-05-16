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
	"encoding/json"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// capturedMemberCall records the inputs to a single ListOrganizationMembers
// call.
type capturedMemberCall struct {
	org               string
	mode              string
	continuationToken *string
}

// mockOrgMemberListClient stubs orgMemberListClient. Pages are pulled from
// `responses` in order; further calls beyond the slice return the final
// response with a nil continuation token.
type mockOrgMemberListClient struct {
	responses []apitype.ListOrganizationMembersResponse
	err       error
	captured  *[]capturedMemberCall
	calls     int
}

func (m *mockOrgMemberListClient) ListOrganizationMembers(
	_ context.Context, orgName, mode string, continuationToken *string,
) (apitype.ListOrganizationMembersResponse, error) {
	if m.captured != nil {
		*m.captured = append(*m.captured, capturedMemberCall{
			org:               orgName,
			mode:              mode,
			continuationToken: continuationToken,
		})
	}
	if m.err != nil {
		return apitype.ListOrganizationMembersResponse{}, m.err
	}
	idx := m.calls
	m.calls++
	if idx >= len(m.responses) {
		return apitype.ListOrganizationMembersResponse{}, nil
	}
	return m.responses[idx], nil
}

func stubMemberFactory(c orgMemberListClient, orgName string) orgMemberListClientFactory {
	return func(_ context.Context, _ string) (orgMemberListClient, string, error) {
		return c, orgName, nil
	}
}

func failingMemberFactory(err error) orgMemberListClientFactory {
	return func(_ context.Context, _ string) (orgMemberListClient, string, error) {
		return nil, "", err
	}
}

// defaultMemberListArgs returns an args struct primed with the default
// (terminal table) renderer. Tests that want JSON should use
// jsonMemberListArgs instead.
func defaultMemberListArgs() orgMemberListArgs {
	return orgMemberListArgs{renderOutput: renderOrgMemberListTable}
}

// jsonMemberListArgs is the args equivalent of `--output json`.
func jsonMemberListArgs(t *testing.T) orgMemberListArgs {
	t.Helper()
	a := defaultMemberListArgs()
	a.renderOutput = renderOrgMemberListJSON
	return a
}

func sampleMember(login, name, role string) apitype.OrganizationMember {
	return apitype.OrganizationMember{
		Role:          role,
		User:          apitype.UserInfo{Name: name, GitHubLogin: login},
		Created:       "2026-05-01T12:00:00Z",
		KnownToPulumi: true,
		FGARole:       apitype.FGARole{ID: "role-" + role, Name: role, ModifiedAt: "2026-04-01T00:00:00Z"},
	}
}

func sampleMemberResponse() apitype.ListOrganizationMembersResponse {
	return apitype.ListOrganizationMembersResponse{
		Members: []apitype.OrganizationMember{
			sampleMember("alice", "Alice", "admin"),
			sampleMember("bob", "Bob", "member"),
		},
	}
}

func TestOrgMemberList_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{sampleMemberResponse()}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.NoError(t, err)

	out := buf.String()
	// Table headers.
	assert.Contains(t, out, "USER")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "ROLE")
	assert.Contains(t, out, "JOINED")

	// Rows.
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "admin")
	assert.Contains(t, out, "bob")
	assert.Contains(t, out, "Bob")
	assert.Contains(t, out, "member")
	assert.Contains(t, out, "2026-05-01T12:00:00Z")

	assert.Contains(t, out, "2 member(s)")
}

func TestOrgMemberList_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{{}}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.NoError(t, err)
	assert.Equal(t, "No members found for this organization.\n", buf.String())
}

func TestOrgMemberList_DefaultOutput_FallsBackToNameWhenGithubLoginMissing(t *testing.T) {
	t.Parallel()

	resp := apitype.ListOrganizationMembersResponse{
		Members: []apitype.OrganizationMember{{
			Role:    "admin",
			User:    apitype.UserInfo{Name: "Display Name"},
			Created: "2026-05-01T12:00:00Z",
			FGARole: apitype.FGARole{Name: "admin"},
		}},
	}

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{resp}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Display Name")
}

func TestOrgMemberList_DefaultOutput_FallsBackToBuiltinRoleWhenFgaRoleEmpty(t *testing.T) {
	t.Parallel()

	resp := apitype.ListOrganizationMembersResponse{
		Members: []apitype.OrganizationMember{{
			Role:    "billingManager",
			User:    apitype.UserInfo{GitHubLogin: "carol", Name: "Carol"},
			Created: "2026-05-01T12:00:00Z",
		}},
	}

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{resp}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "billingManager")
}

func TestOrgMemberList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{sampleMemberResponse()}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), jsonMemberListArgs(t))
	require.NoError(t, err)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))

	assert.Equal(t, 2, env.Count)
	require.Len(t, env.Members, 2)
	assert.Equal(t, "alice", env.Members[0].User.GitHubLogin)
	assert.Equal(t, "admin", env.Members[0].FGARole.Name)
}

func TestOrgMemberList_JSONOutput_EmptyArray(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{{}}}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), jsonMemberListArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{"members":[],"count":0}`, buf.String())
}

func TestOrgMemberList_FirstPageOnlyByDefault(t *testing.T) {
	t.Parallel()

	// Server has more pages, but with no --count and no --all the command
	// should stop after the first page.
	pages := []apitype.ListOrganizationMembersResponse{
		{
			Members:           []apitype.OrganizationMember{sampleMember("alice", "Alice", "admin")},
			ContinuationToken: "page-2",
		},
		{
			Members: []apitype.OrganizationMember{sampleMember("bob", "Bob", "member")},
		},
	}

	var captured []capturedMemberCall
	c := &mockOrgMemberListClient{responses: pages, captured: &captured}
	var buf bytes.Buffer
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), jsonMemberListArgs(t))
	require.NoError(t, err)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, 1, env.Count)
	require.Len(t, captured, 1)
	assert.Equal(t, "", deref(captured[0].continuationToken))
}

func TestOrgMemberList_All_FetchesEveryPage(t *testing.T) {
	t.Parallel()

	pages := []apitype.ListOrganizationMembersResponse{
		{
			Members:           []apitype.OrganizationMember{sampleMember("alice", "Alice", "admin")},
			ContinuationToken: "page-2",
		},
		{
			Members:           []apitype.OrganizationMember{sampleMember("bob", "Bob", "member")},
			ContinuationToken: "page-3",
		},
		{
			Members: []apitype.OrganizationMember{sampleMember("carol", "Carol", "admin")},
		},
	}

	var captured []capturedMemberCall
	c := &mockOrgMemberListClient{responses: pages, captured: &captured}
	var buf bytes.Buffer
	args := jsonMemberListArgs(t)
	args.all = true
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), args)
	require.NoError(t, err)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, 3, env.Count)
	require.Len(t, captured, 3)
	assert.Equal(t, "", deref(captured[0].continuationToken))
	assert.Equal(t, "page-2", deref(captured[1].continuationToken))
	assert.Equal(t, "page-3", deref(captured[2].continuationToken))
}

func TestOrgMemberList_Count_TruncatesAcrossPages(t *testing.T) {
	t.Parallel()

	pages := []apitype.ListOrganizationMembersResponse{
		{
			Members: []apitype.OrganizationMember{
				sampleMember("alice", "Alice", "admin"),
				sampleMember("bob", "Bob", "member"),
			},
			ContinuationToken: "page-2",
		},
		{
			Members: []apitype.OrganizationMember{
				sampleMember("carol", "Carol", "admin"),
				sampleMember("dan", "Dan", "member"),
			},
			ContinuationToken: "page-3",
		},
	}

	var captured []capturedMemberCall
	c := &mockOrgMemberListClient{responses: pages, captured: &captured}
	var buf bytes.Buffer
	args := jsonMemberListArgs(t)
	args.count = 3
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), args)
	require.NoError(t, err)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, 3, env.Count)
	// Two pages fetched: first satisfied 2 of 3, second pushed us to 4 and we
	// truncated. No third page should have been requested.
	require.Len(t, captured, 2)
}

func TestOrgMemberList_Count_StopsAtServerEndEvenIfShort(t *testing.T) {
	t.Parallel()

	pages := []apitype.ListOrganizationMembersResponse{
		{
			Members: []apitype.OrganizationMember{
				sampleMember("alice", "Alice", "admin"),
			},
		},
	}

	c := &mockOrgMemberListClient{responses: pages}
	var buf bytes.Buffer
	args := jsonMemberListArgs(t)
	args.count = 100
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), args)
	require.NoError(t, err)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, 1, env.Count)
}

func TestOrgMemberList_AlwaysQueriesFrontendMembers(t *testing.T) {
	t.Parallel()

	var captured []capturedMemberCall
	c := &mockOrgMemberListClient{
		responses: []apitype.ListOrganizationMembersResponse{{}},
		captured:  &captured,
	}
	var buf bytes.Buffer
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.NoError(t, err)
	require.Len(t, captured, 1)
	assert.Equal(t, "frontend", captured[0].mode)
}

func TestOrgMemberList_NegativeCount(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{{}}}
	args := defaultMemberListArgs()
	args.count = -1
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--count must be non-negative")
}

func TestOrgMemberList_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberListClient{err: errors.New("server boom")}
	err := runOrgMemberList(t.Context(), &buf, stubMemberFactory(c, "acme"), defaultMemberListArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing organization members")
	assert.Contains(t, err.Error(), "server boom")
}

func TestOrgMemberList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgMemberList(t.Context(), &buf, failingMemberFactory(errors.New("not logged in")),
		defaultMemberListArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestOrgMemberList_OrgFlagPropagatesToFactory(t *testing.T) {
	t.Parallel()

	var capturedOrg string
	factory := func(_ context.Context, orgFlag string) (orgMemberListClient, string, error) {
		capturedOrg = orgFlag
		return &mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{{}}}, "acme", nil
	}

	var buf bytes.Buffer
	args := defaultMemberListArgs()
	args.org = "acme"
	err := runOrgMemberList(t.Context(), &buf, factory, args)
	require.NoError(t, err)
	assert.Equal(t, "acme", capturedOrg)
}

func TestNewOrgMemberListCmd_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	var captured []capturedMemberCall
	c := &mockOrgMemberListClient{
		responses: []apitype.ListOrganizationMembersResponse{sampleMemberResponse()},
		captured:  &captured,
	}
	cmd := newOrgMemberListCmdWith(stubMemberFactory(c, "acme"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--org", "acme",
		"--all",
		"--output", "json",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Len(t, captured, 1)
	assert.Equal(t, "frontend", captured[0].mode)

	var env orgMemberListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, 2, env.Count)
}

func TestNewOrgMemberListCmd_CountAndAllAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	cmd := newOrgMemberListCmdWith(stubMemberFactory(
		&mockOrgMemberListClient{responses: []apitype.ListOrganizationMembersResponse{{}}}, "acme"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--count", "5", "--all"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
}
