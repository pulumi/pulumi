// Copyright 2016-2025, Pulumi Corporation.
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

package config

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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	_, _, _ = GetStackConfiguration(
		context.Background(),
		nil, /*sink*/
		stack.SecretsManagerLoader{},
		&backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{
					StringV:             "org/project/name",
					NameV:               tokens.MustParseStackName("name"),
					ProjectV:            "project",
					FullyQualifiedNameV: tokens.QName("org/project/name"),
				}
			},
			LoadRemoteF: func(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error) {
				return workspace.LoadProjectStack(cmdutil.Diag(), project, "Pulumi.name.yaml")
			},
			DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
				return nil, nil
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
	)
}

func TestGetStackConfigurationOrLatest(t *testing.T) {
	t.Parallel()
	// Don't check return values. Just check that GetLatestConfiguration() is called.
	called := false
	_, _, _ = GetStackConfigurationOrLatest(
		context.Background(),
		nil, /*sink*/
		stack.SecretsManagerLoader{},
		&backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{
					StringV:             "org/project/name",
					NameV:               tokens.MustParseStackName("name"),
					ProjectV:            "project",
					FullyQualifiedNameV: tokens.QName("org/project/name"),
				}
			},
			LoadRemoteF: func(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error) {
				return nil, workspace.ErrProjectNotFound
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

	_, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.NoError(t, err)
}

func TestOpenStackEnvUnsupportedBackend(t *testing.T) {
	t.Parallel()

	be := &backend.MockBackend{NameF: func() string { return "test" }}
	stack := &backend.MockStack{BackendF: func() backend.Backend { return be }}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	_, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.Error(t, err)
}

func getMockStackWithEnv(t *testing.T, env map[string]esc.Value) *backend.MockStack {
	t.Helper()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yaml []byte,
			duration time.Duration,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "test-org", org)
			assert.NotEmpty(t, yaml)
			assert.Equal(t, 2*time.Hour, duration)
			return &esc.Environment{Properties: env}, nil, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	return stack
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
		"files": esc.NewValue(map[string]esc.Value{
			"TEST_FILE": esc.NewSecret("sensitive"),
		}),
	}

	stack := getMockStackWithEnv(t, env)

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	openEnv, diags, err := openStackEnv(context.Background(), stack, &projectStack)
	require.NoError(t, err)
	assert.Len(t, diags, 0)
	assert.Equal(t, env, openEnv.Properties)
}

func TestOpenStackEnvLiteral(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"test:string": esc.NewValue("esc"),
		}),
		"environmentVariables": esc.NewValue(map[string]esc.Value{
			"TEST_VAR": esc.NewSecret("hunter2"),
		}),
		"files": esc.NewValue(map[string]esc.Value{
			"TEST_FILE": esc.NewSecret("sensitive"),
		}),
	}

	stack := getMockStackWithEnv(t, env)

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  imports:\n    - test"), &projectStack)
	require.NoError(t, err)

	openEnv, diags, err := openStackEnv(context.Background(), stack, &projectStack)
	require.NoError(t, err)
	assert.Len(t, diags, 0)
	assert.Equal(t, env, openEnv.Properties)
}

func TestStackEnvConfig(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"string":     esc.NewValue("esc"),
			"aws:region": esc.NewValue("us-west-2"),
			"api:domain": esc.NewValue("test"),
			"ui:domain":  esc.NewValue("test"),
		}),
		"environmentVariables": esc.NewValue(map[string]esc.Value{
			"TEST_VAR": esc.NewSecret("hunter2"),
		}),
		"files": esc.NewValue(map[string]esc.Value{
			"TEST_FILE": esc.NewSecret("sensitive"),
		}),
	}

	mockSecretsManager := &secrets.MockSecretsManager{
		EncrypterF: func() config.Encrypter {
			encrypter := &secrets.MockEncrypter{EncryptValueF: func() string { return "ciphertext" }}
			return encrypter
		},
		DecrypterF: func() config.Decrypter {
			decrypter := &secrets.MockDecrypter{
				DecryptValueF: func() string {
					return "plaintext"
				},
				BatchDecryptF: func() []string {
					return []string{
						"whatiamdoing",
					}
				},
			}

			return decrypter
		},
	}

	getMockStack := func(name string) *backend.MockStack {
		stack := getMockStackWithEnv(t, env)
		stack.RefF = func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "org/project/" + name,
				NameV:               tokens.MustParseStackName(name),
				ProjectV:            "project",
				FullyQualifiedNameV: tokens.QName("org/project/" + name),
			}
		}
		stack.DefaultSecretManagerF = func(info *workspace.ProjectStack) (secrets.Manager, error) {
			return mockSecretsManager, nil
		}

		return stack
	}

	stack := getMockStack("mystack")

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	project := workspace.Project{Name: tokens.PackageName("project")}

	ctx := context.Background()
	cfg, err := getStackConfigurationFromProjectStack(
		ctx,
		stack,
		&project,
		mockSecretsManager,
		&projectStack,
	)
	require.NoError(t, err)

	assert.Nil(t, cfg.Config)
	cfg.Config = config.Map{}

	err = workspace.ApplyProjectConfig(ctx, "mystack", &project, cfg.Environment, cfg.Config, config.NopEncrypter)
	require.NoError(t, err)

	assert.Equal(t, config.Map{
		config.MustMakeKey("project", "string"): config.NewValue("esc"),
		config.MustMakeKey("aws", "region"):     config.NewValue("us-west-2"),
		config.MustMakeKey("api", "domain"):     config.NewValue("test"),
		config.MustMakeKey("ui", "domain"):      config.NewValue("test"),
	}, cfg.Config)
}

func TestCopyConfig(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"test:string": esc.NewValue("esc"),
		}),
		"environmentVariables": esc.NewValue(map[string]esc.Value{
			"TEST_VAR": esc.NewSecret("hunter2"),
		}),
		"files": esc.NewValue(map[string]esc.Value{
			"TEST_FILE": esc.NewSecret("sensitive"),
		}),
	}

	mockSecretsManager := &secrets.MockSecretsManager{
		EncrypterF: func() config.Encrypter {
			encrypter := &secrets.MockEncrypter{EncryptValueF: func() string { return "ciphertext" }}
			return encrypter
		},
		DecrypterF: func() config.Decrypter {
			decrypter := &secrets.MockDecrypter{
				DecryptValueF: func() string {
					return "plaintext"
				},
				BatchDecryptF: func() []string {
					return []string{
						"whatiamdoing",
					}
				},
			}

			return decrypter
		},
	}

	getMockStack := func(name string) *backend.MockStack {
		stack := getMockStackWithEnv(t, env)
		stack.RefF = func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "org/project/" + name,
				NameV:               tokens.MustParseStackName(name),
				ProjectV:            "project",
				FullyQualifiedNameV: tokens.QName("org/project/" + name),
			}
		}
		stack.DefaultSecretManagerF = func(info *workspace.ProjectStack) (secrets.Manager, error) {
			return mockSecretsManager, nil
		}

		return stack
	}

	sourceStack := getMockStack("mystack")

	var sourceProjectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &sourceProjectStack)
	require.NoError(t, err)

	t.Run("TestCopyConfigIncludesEnvironments", func(t *testing.T) {
		destinationStack := getMockStack("mystack2")

		var destinationProjectStack workspace.ProjectStack
		err := yaml.Unmarshal([]byte("environment:\n  - test2"), &destinationProjectStack)
		require.NoError(t, err)

		requiresSaving, err := stack.CopyEntireConfigMap(
			context.Background(),
			stack.SecretsManagerLoader{},
			sourceStack,
			&sourceProjectStack,
			destinationStack,
			&destinationProjectStack,
		)
		require.NoError(t, err)
		assert.True(t, requiresSaving, "expected config file changes requiring saving")

		// Assert that only the source stack's environment
		// remains in the destination stack.
		envImports := destinationProjectStack.Environment.Imports()
		assert.Contains(t, envImports, "test")
		assert.NotContains(t, envImports, "test2")
	})
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
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
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

	_, diags, err := openStackEnv(context.Background(), stack, &projectStack)
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
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
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

	_, _, err = openStackEnv(context.Background(), stack, &projectStack)
	assert.Error(t, err)
}

func TestParseConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		path    bool
		wantKey config.Key
		wantErr string
	}{
		{
			name:    "namespaced key",
			input:   "mynamespace:mykey",
			path:    false,
			wantKey: config.MustMakeKey("mynamespace", "mykey"),
		},
		{
			name:    "namespaced key old-style",
			input:   "aws:config:region",
			path:    false,
			wantKey: config.MustMakeKey("aws", "region"),
		},
		{
			name:    "path-like key",
			input:   "mynamespace:mykey.segment1.segment2",
			path:    false,
			wantKey: config.MustMakeKey("mynamespace", "mykey.segment1.segment2"),
		},
		{
			name:    "path key",
			input:   "mynamespace:mykey.segment1.segment2",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "mykey.segment1.segment2"),
		},
		{
			name:    "path segments with colons",
			input:   "mynamespace:mykey.segment1:suffix",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "mykey.segment1:suffix"),
		},
		{
			name:    "non-path segments with colons fail to parse",
			input:   "mynamespace:mykey.segment1:suffix",
			path:    false,
			wantErr: "configuration keys should be of the form `<namespace>:<name>`",
		},
		{
			name:    "path with invalid top-level segment",
			input:   "mynamespace:subnamespace:mykey.segment1.segment2",
			path:    true,
			wantErr: "configuration keys should be of the form `<namespace>:<name>`",
		},
		{
			name:    "invalid path with colon before bracket",
			input:   "mynamespace:my:key[\"segment\"]",
			path:    true,
			wantErr: "configuration keys should be of the form `<namespace>:<name>`",
		},
		{
			name:    "key paths",
			input:   "mynamespace:mykey[\"segment1:suffix\"]",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "mykey[\"segment1:suffix\"]"),
		},
		{
			name:    "bracket as top-level path segment",
			input:   "mynamespace:[\"foo\"]",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "[\"foo\"]"),
		},
		{
			name:    "invalid path",
			input:   "mynamespace:.segment1",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", ".segment1"),
		},
		{
			name:    "path with multiple brackets",
			input:   "mynamespace:mykey[\"segment1\"][\"segment2\"]",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "mykey[\"segment1\"][\"segment2\"]"),
		},
		{
			name:    "empty segment in path",
			input:   "mynamespace:mykey..segment",
			path:    true,
			wantKey: config.MustMakeKey("mynamespace", "mykey..segment"),
		},
		{
			name:    "old-style key as non-path",
			input:   "aws:config:region.value",
			path:    false,
			wantKey: config.MustMakeKey("aws", "region.value"),
		},
		{
			name:    "no namespace uses project name",
			input:   "mykey",
			path:    false,
			wantKey: config.MustMakeKey("test-project", "mykey"),
		},
		{
			name:    "no namespace with path segments",
			input:   "mykey.segment1.segment2",
			path:    true,
			wantKey: config.MustMakeKey("test-project", "mykey.segment1.segment2"),
		},
		{
			name:    "no namespace with brackets",
			input:   "mykey[\"segment1\"]",
			path:    true,
			wantKey: config.MustMakeKey("test-project", "mykey[\"segment1\"]"),
		},
		{
			name:    "no namespace with colon in path segment",
			input:   "mykey.segment:with:colons",
			path:    true,
			wantKey: config.MustMakeKey("test-project", "mykey.segment:with:colons"),
		},
		{
			name:    "no namespace with colon in non-path fails",
			input:   "mykey:with:colons",
			path:    false,
			wantErr: "configuration keys should be of the form `<namespace>:<name>`",
		},
	}

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "test-project",
			}, "/test/path", nil
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseConfigKey(ws, tt.input, tt.path)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, got)
		})
	}
}
