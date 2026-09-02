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
	{name: "AdminPolicy", id: "admin-id", desc: "full access"},
	{name: "ReadonlyPolicy", id: "readonly-id", desc: "read-only access"},
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
		{"official admin name", "AdminPolicy", "admin-id"},
		{"official readonly name", "ReadonlyPolicy", "readonly-id"},
		{"official names are case-insensitive", "adminpolicy", "admin-id"},
		// The presets mean different things on each cloud, so there are no generic aliases;
		// "admin" is just another custom value now.
		{"a former alias is a custom value", "admin", "admin"},
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

	naming := envNaming{project: "aws-login"}
	assert.Equal(t, "aws-login/my-account-env",
		naming.escEnvName(cloudsetup.CloudAccount{Name: "My Account", ID: "123456789012"}))
	// Accounts without a name fall back to the ID.
	assert.Equal(t, "aws-login/123456789012-env",
		naming.escEnvName(cloudsetup.CloudAccount{ID: "123456789012"}))

	// An explicit name replaces the derived one, without the `-env` suffix the derivation adds.
	named := envNaming{project: "aws", envName: "prod-write"}
	assert.Equal(t, "aws/prod-write",
		named.escEnvName(cloudsetup.CloudAccount{Name: "My Account", ID: "123456789012"}))
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

	got, err := validateESCProject("gcp-login")
	require.NoError(t, err)
	assert.Equal(t, "gcp-login", got)

	// The service matches project names case-insensitively, so the name that reaches the OIDC
	// subject has to be lowercased here or the two disagree.
	got, err = validateESCProject("My_Project.1-X")
	require.NoError(t, err)
	assert.Equal(t, "my_project.1-x", got)

	// A slash is silently mangled by parseRef: `acme/a/b/sandbox-env` becomes environment
	// `bsandbox-env` in project `a`, after the trust policy was written with `a/b/...`.
	_, err = validateESCProject("a/b")
	assert.Error(t, err)
	// An apostrophe reaches Azure's Graph $filter unescaped.
	_, err = validateESCProject("o'brien")
	assert.Error(t, err)
	_, err = validateESCProject("")
	assert.Error(t, err)
	_, err = validateESCProject("has space")
	assert.Error(t, err)
}

// Two accounts that derive the same environment name have to be caught before setup runs:
// both would get cloud resources, but only the last login block written would survive.
func TestCheckDuplicateEnvNames(t *testing.T) {
	t.Parallel()

	naming := envNaming{project: "aws-login"}

	require.NoError(t, checkDuplicateEnvNames(naming, []cloudsetup.CloudAccount{
		{ID: "111111111111", Name: "Sandbox"},
		{ID: "222222222222", Name: "Production"},
	}))

	// Display names are not unique on any of the three providers, and the account ID is only
	// a fallback for an empty name, so it does not disambiguate these.
	err := checkDuplicateEnvNames(naming, []cloudsetup.CloudAccount{
		{ID: "111111111111", Name: "Sandbox"},
		{ID: "222222222222", Name: "Sandbox"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "111111111111")
	assert.Contains(t, err.Error(), "222222222222")
	assert.Contains(t, err.Error(), "aws-login/sandbox-env")

	// Sanitizing collapses different names onto the same environment too.
	require.Error(t, checkDuplicateEnvNames(naming, []cloudsetup.CloudAccount{
		{ID: "111111111111", Name: "My Team"},
		{ID: "222222222222", Name: "My/Team"},
	}))

	// An explicit --env-name would put every selected account in one environment, so it is
	// rejected before any cloud resource is created rather than reported as a name collision.
	named := envNaming{project: "aws", envName: "prod-write"}
	require.NoError(t, checkDuplicateEnvNames(named, []cloudsetup.CloudAccount{{ID: "111111111111"}}))
	err = checkDuplicateEnvNames(named, []cloudsetup.CloudAccount{
		{ID: "111111111111"},
		{ID: "222222222222"},
	})
	assert.ErrorContains(t, err, "--env-name names a single environment")
}

// --env-name lands in the OIDC subject and in the environment ref, so it has to be rejected
// and lowercased on the same terms as --project.
func TestValidateESCEnvName(t *testing.T) {
	t.Parallel()

	got, err := validateESCEnvName("")
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = validateESCEnvName("Prod-Write.1_x")
	require.NoError(t, err)
	assert.Equal(t, "prod-write.1_x", got)

	for _, bad := range []string{"a/b", "has space", "o'brien"} {
		_, err := validateESCEnvName(bad)
		assert.ErrorContains(t, err, "--env-name")
	}
}
