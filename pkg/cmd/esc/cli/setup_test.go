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

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
)

// The presets are synthetic so that these cases test the resolver itself rather than any one
// provider's names; each provider's own test covers what its presets resolve to.
var testPolicyChoices = []policyChoice{
	{name: "AdminPolicy", id: "admin-id", alias: policyAliasAdmin, desc: policyAdminAccess},
	{name: "ReadonlyPolicy", id: "readonly-id", alias: policyAliasReadonly, desc: policyReadonlyAccess},
}

// resolvePolicy only reaches for the command's colors when it prompts, so a zero-value
// setupCommand is enough to cover every case that resolves without prompting.
func TestResolvePolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		policy string
		want   string
	}{
		{"generic admin alias", "admin", "admin-id"},
		{"generic readonly alias", "readonly", "readonly-id"},
		{"official admin name", "AdminPolicy", "admin-id"},
		{"official readonly name", "ReadonlyPolicy", "readonly-id"},
		{"aliases are case-insensitive", "AdMiN", "admin-id"},
		{"official names are case-insensitive", "adminpolicy", "admin-id"},
		// The point of accepting custom policies: anything that is not a preset is a native
		// identifier and is handed to the provider untouched, casing included.
		{"custom value", "SomeCustomPolicy", "SomeCustomPolicy"},
		{
			"custom value that looks like a preset",
			"AdminPolicyPlus",
			"AdminPolicyPlus",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &setupCommand{}
			// --yes must not change how an explicit policy resolves; it only removes the prompt.
			for _, yes := range []bool{false, true} {
				got, err := s.resolvePolicy(c.policy, testPolicyChoices, yes)
				require.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}

// Each preset carries its own description, so the prompt cannot mislabel a choice by listing the
// presets in an unexpected order.
func TestPolicyChoiceLabel(t *testing.T) {
	t.Parallel()

	for _, c := range testPolicyChoices {
		assert.Equal(t, c.name+" - "+c.desc, c.label())
	}
}

// An unknown value is a custom policy even when no presets are offered at all.
func TestResolvePolicy_NoChoices(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}
	got, err := s.resolvePolicy("roles/custom", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "roles/custom", got)

	// Omitted with nothing to prompt with is a programming error, not a panic.
	_, err = s.resolvePolicy("", nil, false)
	assert.Error(t, err)
}

// With --yes there is nobody to prompt, so an omitted policy is an error rather than a
// silent default: admin is needed for Deployments and readonly for Insights.
func TestResolvePolicy_OmittedWithYes(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}
	_, err := s.resolvePolicy("", testPolicyChoices, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy must be set when using --yes")
	// The message names the presets so the user has something to copy.
	assert.Contains(t, err.Error(), "AdminPolicy")
	assert.Contains(t, err.Error(), "ReadonlyPolicy")
}

func TestESCEnvName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "aws-login/my-account-env",
		escEnvName("aws-login", cloudsetup.CloudAccount{Name: "My Account", ID: "123456789012"}))
	// Accounts without a name fall back to the ID.
	assert.Equal(t, "aws-login/123456789012-env",
		escEnvName("aws-login", cloudsetup.CloudAccount{ID: "123456789012"}))
}

// The plan the user confirms has to distinguish a create from a replacement, because
// setup writes the login block either way.
func TestPlanEnvLine(t *testing.T) {
	t.Parallel()

	setup := &setupCommand{env: &envCommand{esc: &escCommand{
		client: &testPulumiClient{
			environments: map[string]*testEnvironment{
				"org/aws-login/existing-env": {},
			},
		},
	}}}
	ref := func(name string) environmentRef {
		return environmentRef{orgName: "org", projectName: "aws-login", envName: name}
	}

	assert.Equal(t,
		"create ESC environment org/aws-login/new-env",
		setup.planEnvLine(t.Context(), ref("new-env"), awsLoginPath))

	assert.Equal(t,
		"update ESC environment org/aws-login/existing-env (exists; its `aws.login` block will be replaced)",
		setup.planEnvLine(t.Context(), ref("existing-env"), awsLoginPath))
}

// --project feeds the cloud trust policy's subject and the environment ref, and neither
// rejects anything itself, so these have to fail before any cloud resource is created.
func TestValidateESCProject(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateESCProject("gcp-login"))
	require.NoError(t, validateESCProject("My_Project.1-x"))

	// A slash is silently mangled by parseRef: `acme/a/b/sandbox-env` becomes environment
	// `bsandbox-env` in project `a`, after the trust policy was written with `a/b/...`.
	assert.Error(t, validateESCProject("a/b"))
	// An apostrophe reaches Azure's Graph $filter unescaped.
	assert.Error(t, validateESCProject("o'brien"))
	assert.Error(t, validateESCProject(""))
	assert.Error(t, validateESCProject("has space"))
}
