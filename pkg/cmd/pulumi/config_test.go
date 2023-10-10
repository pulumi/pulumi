// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPrettyKeyForProject(t *testing.T) {
	t.Parallel()

	proj := &workspace.Project{
		Name:    tokens.PackageName("test-package"),
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}

	assert.Equal(t, "foo", prettyKeyForProject(config.MustMakeKey("test-package", "foo"), proj))
	assert.Equal(t, "other-package:bar", prettyKeyForProject(config.MustMakeKey("other-package", "bar"), proj))
	assert.Panics(t, func() { config.MustMakeKey("other:package", "bar") })
}

func TestSecretDetection(t *testing.T) {
	t.Parallel()

	assert.True(t, looksLikeSecret(config.MustMakeKey("test", "token"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))
	assert.True(t, looksLikeSecret(config.MustMakeKey("test", "apiToken"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))

	// The key name does not match the pattern, so even though this "looks like" a secret, we say it is not.
	assert.False(t, looksLikeSecret(config.MustMakeKey("test", "okay"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))
}

func TestGetStackConfigurationDoesNotGetLatestConfiguration(t *testing.T) {
	t.Parallel()
	// Don't check return values. Just check that GetLatestConfiguration() is not called.
	_, _, _ = getStackConfiguration(
		context.Background(),
		&backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{
					StringV:             "org/project/name",
					NameV:               "name",
					ProjectV:            "project",
					FullyQualifiedNameV: tokens.QName("org/project/name"),
				}
			},
			BackendF: func() backend.Backend {
				return &backend.MockBackend{
					GetLatestConfigurationF: func(context.Context, backend.Stack) (config.Map, error) {
						t.Fatalf("GetLatestConfiguration should not be called in typical getStackConfiguration calls.")
						return config.Map{}, nil
					},
				}
			},
		},
		nil,
		nil,
	)
}

func TestGetStackConfigurationOrLatest(t *testing.T) {
	t.Parallel()
	// Don't check return values. Just check that GetLatestConfiguration() is called.
	called := false
	_, _, _ = getStackConfigurationOrLatest(
		context.Background(),
		&backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{
					StringV:             "org/project/name",
					NameV:               "name",
					ProjectV:            "project",
					FullyQualifiedNameV: tokens.QName("org/project/name"),
				}
			},
			DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
				return nil, nil
			},
			BackendF: func() backend.Backend {
				return &backend.MockBackend{
					GetLatestConfigurationF: func(context.Context, backend.Stack) (config.Map, error) {
						called = true
						return config.Map{}, nil
					},
				}
			},
		},
		nil,
		nil,
	)
	if !called {
		t.Fatalf("GetLatestConfiguration should be called in getStackConfigurationOrLatest.")
	}
}

func TestNeedsCrypter(t *testing.T) {
	t.Parallel()

	t.Run("no secrets, no env", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewValue("bar")}
		assert.False(t, needsCrypter(m, esc.Value{}))
	})

	t.Run("secrets, no env", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewSecureValue("bar")}
		assert.True(t, needsCrypter(m, esc.Value{}))
	})

	t.Run("no secrets, no secrets in env", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewValue("bar")}
		env := esc.NewValue(map[string]esc.Value{"password": esc.NewValue("hunter2")})
		assert.False(t, needsCrypter(m, env))
	})

	t.Run("no secrets, secrets in env", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewValue("bar")}
		env := esc.NewValue(map[string]esc.Value{"password": esc.NewSecret("hunter2")})
		assert.True(t, needsCrypter(m, env))
	})

	t.Run("no secrets, secrets in env array", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewValue("bar")}
		env := esc.NewValue(map[string]esc.Value{"password": esc.NewValue([]esc.Value{esc.NewSecret("hunter2")})})
		assert.True(t, needsCrypter(m, env))
	})

	t.Run("secrets, secrets in env", func(t *testing.T) {
		t.Parallel()
		m := config.Map{config.MustMakeKey("test", "foo"): config.NewSecureValue("bar")}
		env := esc.NewValue(map[string]esc.Value{"password": esc.NewSecret("hunter2")})
		assert.True(t, needsCrypter(m, env))
	})
}

func TestOpenStackEnvNoEnv(t *testing.T) {
	t.Parallel()

	be := &backend.MockBackend{NameF: func() string { return "test" }}
	stack := &backend.MockStack{BackendF: func() backend.Backend { return be }}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte(""), &projectStack)
	require.NoError(t, err)

	_, _, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.NoError(t, err)
}

func TestOpenStackEnvUnsupportedBackend(t *testing.T) {
	t.Parallel()

	be := &backend.MockBackend{NameF: func() string { return "test" }}
	stack := &backend.MockStack{BackendF: func() backend.Backend { return be }}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	_, _, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.Error(t, err)
}

func TestOpenStackEnv(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"test:string": esc.NewValue("esc"),
		}),
		"environmentVariables": esc.NewValue(map[string]esc.Value{
			"TEST_VAR": esc.NewSecret("hunter2"),
		}),
	}

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yaml []byte,
			duration time.Duration,
		) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
			assert.Equal(t, "test-org", org)
			assert.Equal(t, []byte(`{"imports":["test"]}`), yaml)
			assert.Equal(t, 2*time.Hour, duration)
			return &esc.Environment{Properties: env}, nil, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	pulumiEnv, envVars, diags, err := openStackEnv(context.Background(), stack, &projectStack)
	require.NoError(t, err)
	assert.Len(t, diags, 0)
	assert.Equal(t, env["pulumiConfig"], pulumiEnv)
	assert.Equal(t, env["environmentVariables"].Value.(map[string]esc.Value), envVars)
}

func TestOpenStackEnvDiags(t *testing.T) {
	t.Parallel()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yaml []byte,
			duration time.Duration,
		) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
			return nil, []apitype.EnvironmentDiagnostic{{Summary: "diag"}}, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	_, _, diags, err := openStackEnv(context.Background(), stack, &projectStack)
	require.NoError(t, err)
	assert.Len(t, diags, 1)
}

func TestOpenStackEnvError(t *testing.T) {
	t.Parallel()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yaml []byte,
			duration time.Duration,
		) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
			return nil, nil, errors.New("error")
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	_, _, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.Error(t, err)
}
