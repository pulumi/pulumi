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
)

// mockOrgMemberRemoveClient stubs orgMemberRemoveClient. It records the
// arguments it was called with and optionally returns an error.
type mockOrgMemberRemoveClient struct {
	err          error
	gotOrg       string
	gotUserLogin string
}

func (m *mockOrgMemberRemoveClient) RemoveOrganizationMember(_ context.Context, org, userLogin string) error {
	m.gotOrg = org
	m.gotUserLogin = userLogin
	return m.err
}

func stubOrgMemberRemoveFactory(c orgMemberRemoveClient, org string) orgMemberRemoveClientFactory {
	return func(_ context.Context, _ string) (orgMemberRemoveClient, string, error) {
		return c, org, nil
	}
}

func failingOrgMemberRemoveFactory(err error) orgMemberRemoveClientFactory {
	return func(_ context.Context, _ string) (orgMemberRemoveClient, string, error) {
		return nil, "", err
	}
}

func TestOrgMemberRemove_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberRemoveClient{}
	err := runOrgMemberRemove(t.Context(), &buf,
		stubOrgMemberRemoveFactory(c, "acme"), "alice", orgMemberRemoveArgs{})
	require.NoError(t, err)

	assert.Equal(t, "Removed member alice from organization acme.\n", buf.String())
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "alice", c.gotUserLogin)
}

func TestOrgMemberRemove_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberRemoveClient{}
	err := runOrgMemberRemove(t.Context(), &buf,
		stubOrgMemberRemoveFactory(c, "acme"), "alice",
		orgMemberRemoveArgs{output: "json"})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organizationName": "acme",
		"userLogin": "alice"
	}`, buf.String())
}

func TestOrgMemberRemove_InvalidOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberRemoveClient{}
	err := runOrgMemberRemove(t.Context(), &buf,
		stubOrgMemberRemoveFactory(c, "acme"), "alice",
		orgMemberRemoveArgs{output: "yaml"})
	require.Error(t, err)
	assert.Equal(t,
		`invalid --output value "yaml" (must be 'default' or 'json')`,
		err.Error())
	// Validation must run before the API call.
	assert.Equal(t, "", c.gotOrg)
	assert.Equal(t, "", c.gotUserLogin)
}

func TestOrgMemberRemove_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockOrgMemberRemoveClient{err: errors.New("cannot remove self")}
	err := runOrgMemberRemove(t.Context(), &buf,
		stubOrgMemberRemoveFactory(c, "acme"), "alice",
		orgMemberRemoveArgs{})
	require.Error(t, err)
	assert.Equal(t, "removing organization member: cannot remove self", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgMemberRemove_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgMemberRemove(t.Context(), &buf,
		failingOrgMemberRemoveFactory(errors.New("not logged in")),
		"alice", orgMemberRemoveArgs{})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
