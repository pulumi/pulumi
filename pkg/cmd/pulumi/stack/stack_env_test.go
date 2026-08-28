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

package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// fakeEnvUniverse is an in-memory stand-in for an organization's named ESC environments. It records
// every created definition so a test can assert both what was written and what was left alone.
type fakeEnvUniverse struct {
	t *testing.T

	// definitions maps "<project>/<name>" to the environment's YAML.
	definitions map[string]string
	// created lists "<project>/<name>" in creation order.
	created []string
	// createErrs fails the creation of the named environments.
	createErrs map[string]error
	// createDiags makes creation of the named environments succeed on the service but report
	// diagnostics, i.e. the environment exists with a definition the service rejected.
	createDiags map[string]apitype.EnvironmentDiagnostics
}

func newFakeEnvUniverse(t *testing.T, existing ...string) *fakeEnvUniverse {
	u := &fakeEnvUniverse{
		t:           t,
		definitions: map[string]string{},
		createErrs:  map[string]error{},
		createDiags: map[string]apitype.EnvironmentDiagnostics{},
	}
	for _, e := range existing {
		u.definitions[e] = "values: {}\n"
	}
	return u
}

// backend returns a mock backend that leaves UpdateEnvironmentDefinitionF unset: creating a stack's
// environments must never update one, and the mock panics if it does.
func (u *fakeEnvUniverse) backend() *backend.MockEnvironmentsBackend {
	return &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		GetEnvironmentDefinitionF: func(
			_ context.Context, org, envProject, envName, version string,
		) ([]byte, string, int, error) {
			assert.Equal(u.t, "acme", org)
			ref := envProject + "/" + envName
			def, ok := u.definitions[ref]
			if !ok {
				return nil, "", 0, fmt.Errorf("%w: %v", backend.ErrEnvironmentNotFound, ref)
			}
			return []byte(def), "etag-1", 1, nil
		},
		CreateEnvironmentF: func(
			_ context.Context, org, envProject, envName string, yaml []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			assert.Equal(u.t, "acme", org)
			ref := envProject + "/" + envName
			if err, ok := u.createErrs[ref]; ok {
				return nil, err
			}
			if diags, ok := u.createDiags[ref]; ok {
				// The service created the environment, then rejected its definition.
				u.definitions[ref] = string(yaml)
				return diags, nil
			}
			u.definitions[ref] = string(yaml)
			u.created = append(u.created, ref)
			return nil, nil
		},
	}
}

func (u *fakeEnvUniverse) stack() *backend.MockStack {
	be := u.backend()
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "acme/payments/dev",
			}
		},
		OrgNameF: func() string { return "acme" },
		BackendF: func() backend.Backend { return be },
	}
}

func TestCreateStackEnvironmentsCreatesBaseAndStackEnvironment(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)
	var out bytes.Buffer

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
		Stdout:     &out,
	})
	require.NoError(t, err)
	require.NotNil(t, env.MainEnvironment)
	assert.Equal(t, "payments/dev", env.MainEnvironment.String())
	assert.False(t, env.StackEnvironmentReused)

	assert.Equal(t, []string{"payments/base", "payments/dev"}, u.created)
	assert.Equal(t, "values: {}\n", u.definitions["payments/base"])
	assert.Equal(t, "imports:\n  - payments/base\nvalues: {}\n", u.definitions["payments/dev"])

	assert.Contains(t, out.String(), "Creating environment 'acme/payments/base'...\n")
	assert.Contains(t, out.String(), "Creating environment 'acme/payments/dev'... (imports payments/base)\n")
}

func TestCreateStackEnvironmentsReusesExistingBase(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t, "payments/base")
	u.definitions["payments/base"] = "values:\n  pulumiConfig:\n    payments:logLevel: info\n"
	var out bytes.Buffer

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
		Stdout:     &out,
	})
	require.NoError(t, err)
	assert.Equal(t, "payments/dev", env.MainEnvironment.String())

	// The pre-existing base is neither recreated nor edited.
	assert.Equal(t, []string{"payments/dev"}, u.created)
	assert.Equal(t, "values:\n  pulumiConfig:\n    payments:logLevel: info\n", u.definitions["payments/base"])
	assert.Contains(t, out.String(), "Environment 'acme/payments/base' already exists — reusing.\n")
	// Only the base was reused; the stack's own environment was created, so its values were written.
	assert.False(t, env.StackEnvironmentReused)
}

func TestCreateStackEnvironmentsReusesExistingStackEnvironment(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t, "payments/base", "payments/dev")
	u.definitions["payments/dev"] = "values:\n  pulumiConfig:\n    aws:region: eu-west-1\n"
	var out bytes.Buffer

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
		Values:     map[string]yaml.Node{"aws:region": mustScalar(t, "us-west-2")},
		Stdout:     &out,
	})
	require.NoError(t, err)
	assert.Equal(t, "payments/dev", env.MainEnvironment.String())

	assert.Empty(t, u.created)
	assert.Equal(t, "values:\n  pulumiConfig:\n    aws:region: eu-west-1\n", u.definitions["payments/dev"])
	assert.Contains(t, out.String(), "Environment 'acme/payments/dev' already exists — reusing.\n")
	// The caller has to know the values it passed were not written.
	assert.True(t, env.StackEnvironmentReused)
}

func TestCreateStackEnvironmentsWritesValues(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)
	secret := yaml.Node{}
	require.NoError(t, secret.Encode(map[string]string{"fn::secret": "hunter2"}))

	_, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
		Values: map[string]yaml.Node{
			"aws:region":          mustScalar(t, "us-west-2"),
			"payments:dbPassword": secret,
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "imports:\n  - payments/base\n"+
		"values:\n  pulumiConfig:\n    aws:region: us-west-2\n"+
		"    payments:dbPassword:\n      fn::secret: hunter2\n",
		u.definitions["payments/dev"])
}

func TestCreateStackEnvironmentsConflictOnCreateIsReuse(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)
	// The base appears between the probe and the create, exactly as a racing writer would produce.
	u.createErrs["payments/base"] = fmt.Errorf("%w: payments/base", backend.ErrEnvironmentConflict)
	var out bytes.Buffer

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
		Stdout:     &out,
	})
	require.NoError(t, err)
	assert.Equal(t, "payments/dev", env.MainEnvironment.String())
	assert.Equal(t, []string{"payments/dev"}, u.created)
	assert.Contains(t, out.String(), "Environment 'acme/payments/base' already exists — reusing.\n")
}

func TestCreateStackEnvironmentsStackNamedBase(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "base",
	})
	require.NoError(t, err)
	assert.Equal(t, "payments/base", env.MainEnvironment.String())
	// Exactly one environment, with no self-import.
	assert.Equal(t, []string{"payments/base"}, u.created)
	assert.Equal(t, "values: {}\n", u.definitions["payments/base"])
}

func TestCreateStackEnvironmentsFailureNamesWhatWasCreated(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)
	u.createErrs["payments/dev"] = errors.New("boom")

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
	})
	// No main environment is returned, so the caller never records one and the stack stays ordinary.
	assert.Nil(t, env.MainEnvironment)

	var birthErr *EnvironmentBirthError
	require.ErrorAs(t, err, &birthErr)
	assert.Equal(t, []string{"acme/payments/base"}, birthErr.Created)
	assert.Equal(t, "acme/payments/dev", birthErr.Failed)
	assert.Contains(t, err.Error(), "created environment(s) acme/payments/base")
	assert.Contains(t, err.Error(), "could not create environment acme/payments/dev: boom")
	assert.Contains(t, err.Error(), "the stack was created without 'mainEnvironment'")
}

// The service creates an environment before writing its definition, so a rejected definition leaves an
// environment behind. The error has to say that, or a rerun silently reuses the broken environment.
func TestCreateStackEnvironmentsInvalidDefinitionReportsEnvironmentExists(t *testing.T) {
	t.Parallel()

	u := newFakeEnvUniverse(t)
	u.createDiags["payments/dev"] = apitype.EnvironmentDiagnostics{{Summary: "bad value"}}

	env, err := CreateStackEnvironments(t.Context(), u.stack(), StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
	})
	assert.Nil(t, env.MainEnvironment)

	var birthErr *EnvironmentBirthError
	require.ErrorAs(t, err, &birthErr)
	assert.Equal(t, []string{"acme/payments/base"}, birthErr.Created)
	assert.Equal(t, "acme/payments/dev", birthErr.Failed)
	assert.True(t, birthErr.FailedExists)
	assert.Contains(t, err.Error(), "environment acme/payments/dev was created with an unusable definition")
	assert.Contains(t, err.Error(), "fix or delete it before re-running")
	assert.NotContains(t, err.Error(), "could not create environment acme/payments/dev")
}

func TestCreateStackEnvironmentsRefusesBackendWithoutEnvironments(t *testing.T) {
	t.Parallel()

	b := &backend.MockBackend{NameF: func() string { return "file://~" }}
	s := &backend.MockStack{
		RefF:     func() backend.StackReference { return &backend.MockStackReference{} },
		BackendF: func() backend.Backend { return b },
	}

	_, err := CreateStackEnvironments(t.Context(), s, StackEnvironmentOptions{
		EnvProject: "payments",
		EnvName:    "dev",
	})
	assert.ErrorContains(t, err, "backend file://~ does not support environments")
}

func mustScalar(t *testing.T, value string) yaml.Node {
	t.Helper()
	var n yaml.Node
	n.SetString(value)
	return n
}
