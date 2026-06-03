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

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPolicyGroupRemoveClient stubs policyGroupRemoveClient. It records the
// arguments it was called with and optionally returns an error.
type mockPolicyGroupRemoveClient struct {
	err      error
	gotOrg   string
	gotGroup string
}

func (m *mockPolicyGroupRemoveClient) DeletePolicyGroup(_ context.Context, org, group string) error {
	m.gotOrg = org
	m.gotGroup = group
	return m.err
}

func stubPolicyGroupRemoveFactory(c policyGroupRemoveClient, org string) policyGroupRemoveClientFactory {
	return func(_ context.Context, _ string) (policyGroupRemoveClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyGroupRemoveFactory(err error) policyGroupRemoveClientFactory {
	return func(_ context.Context, _ string) (policyGroupRemoveClient, string, error) {
		return nil, "", err
	}
}

func TestPolicyGroupRemove_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupRemoveClient{}
	err := runPolicyGroupRemove(t.Context(), &buf,
		stubPolicyGroupRemoveFactory(c, "acme"), "prod-policies",
		policyGroupRemoveArgs{yes: true, outputFormat: defaultPolicyGroupRemoveOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, "Removed policy group prod-policies from organization acme.\n", buf.String())
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "prod-policies", c.gotGroup)
}

func TestPolicyGroupRemove_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupRemoveClient{}
	args := policyGroupRemoveArgs{yes: true, outputFormat: defaultPolicyGroupRemoveOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupRemove(t.Context(), &buf,
		stubPolicyGroupRemoveFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organizationName": "acme",
		"name": "prod-policies"
	}`, buf.String())
}

func TestPolicyGroupRemove_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupRemoveClient{err: errors.New("cannot delete default group")}
	err := runPolicyGroupRemove(t.Context(), &buf,
		stubPolicyGroupRemoveFactory(c, "acme"), "default-policy-group",
		policyGroupRemoveArgs{yes: true, outputFormat: defaultPolicyGroupRemoveOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "removing policy group: cannot delete default group", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestPolicyGroupRemove_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runPolicyGroupRemove(t.Context(), &buf,
		failingPolicyGroupRemoveFactory(errors.New("not logged in")),
		"prod-policies", policyGroupRemoveArgs{yes: true, outputFormat: defaultPolicyGroupRemoveOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
