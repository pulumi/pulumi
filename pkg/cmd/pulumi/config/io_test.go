// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPrettyKeyForProject(t *testing.T) {
	t.Parallel()

	projectName := tokens.PackageName("test-package")

	assert.Equal(t, "foo", PrettyKeyForProject(config.MustMakeKey("test-package", "foo"), projectName))
	assert.Equal(t, "other-package:bar", PrettyKeyForProject(config.MustMakeKey("other-package", "bar"), projectName))
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
		t.Context(),
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
			DefaultSecretManagerF: func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
				return nil, nil
			},
			BackendF: func() backend.Backend {
				return &backend.MockBackend{
					GetLatestConfigurationF: func(context.Context, backend.Stack) (backend.LatestConfiguration, error) {
						t.Fatalf("GetLatestConfiguration should not be called in typical getStackConfiguration calls.")
						return backend.LatestConfiguration{}, nil
					},
				}
			},
		},
		nil,
		"",
		nil,
	)
}

func TestGetStackConfigurationOrLatest(t *testing.T) {
	t.Parallel()
	// Don't check return values. Just check that GetLatestConfiguration() is called.
	called := false
	_, _, _ = GetStackConfigurationOrLatest(
		t.Context(),
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
			DefaultSecretManagerF: func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
				return nil, nil
			},
			BackendF: func() backend.Backend {
				return &backend.MockBackend{
					GetLatestConfigurationF: func(context.Context, backend.Stack) (backend.LatestConfiguration, error) {
						called = true
						return backend.LatestConfiguration{}, nil
					},
				}
			},
		},
		nil,
		"",
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

	_, _, err = openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
}

func TestOpenStackEnvUnsupportedBackend(t *testing.T) {
	t.Parallel()

	be := &backend.MockBackend{NameF: func() string { return "test" }}
	stack := &backend.MockStack{BackendF: func() backend.Backend { return be }}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	_, _, err = openStackEnv(t.Context(), stack, &projectStack, nil)
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
			_ map[string]string,
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

	openEnv, diags, err := openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
	require.Len(t, diags, 0)
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

	openEnv, diags, err := openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
	require.Len(t, diags, 0)
	assert.Equal(t, env, openEnv.Properties)
}

func TestOpenStackEnvVersionPinned(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"test:string": esc.NewValue("esc"),
		}),
	}

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yamlBody []byte,
			duration time.Duration,
			_ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "test-org", org)
			assert.Contains(t, string(yamlBody), "project/env@3")
			assert.Contains(t, string(yamlBody), "project/other@stable")
			return &esc.Environment{Properties: env}, nil, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - project/env@3\n  - project/other@stable"), &projectStack)
	require.NoError(t, err)

	openEnv, diags, err := openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
	require.Len(t, diags, 0)
	assert.Equal(t, env, openEnv.Properties)
}

func TestOpenStackEnvOverrides(t *testing.T) {
	t.Parallel()

	var gotOverrides map[string]string
	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yamlBody []byte,
			duration time.Duration,
			environmentOverrides map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			gotOverrides = environmentOverrides
			return &esc.Environment{Properties: map[string]esc.Value{}}, nil, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - proj/env"), &projectStack)
	require.NoError(t, err)

	_, diags, err := openStackEnv(t.Context(), stack, &projectStack,
		[]string{"proj/env=proj/other@tag", "proj/env2=proj/other2"})
	require.NoError(t, err)
	require.Len(t, diags, 0)
	assert.Equal(t, map[string]string{
		"proj/env":  "proj/other@tag",
		"proj/env2": "proj/other2",
	}, gotOverrides)
}

func TestParseEnvironmentOverrides(t *testing.T) {
	t.Parallel()

	overrides, err := parseEnvironmentOverrides(nil)
	require.NoError(t, err)
	assert.Nil(t, overrides)

	overrides, err = parseEnvironmentOverrides([]string{"proj/env=proj/other@tag", "a=b"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"proj/env": "proj/other@tag", "a": "b"}, overrides)

	for _, bad := range []string{"noequals", "=replacement", "match="} {
		_, err := parseEnvironmentOverrides([]string{bad})
		assert.ErrorContains(t, err, "invalid --override-env value")
	}
}

func TestOpenStackEnvVersionPinnedLiteral(t *testing.T) {
	t.Parallel()

	env := map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"test:string": esc.NewValue("esc"),
		}),
	}

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		OpenYAMLEnvironmentF: func(
			ctx context.Context,
			org string,
			yamlBody []byte,
			duration time.Duration,
			_ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "test-org", org)
			assert.Contains(t, string(yamlBody), "project/env@3")
			return &esc.Environment{Properties: env}, nil, nil
		},
	}
	stack := &backend.MockStack{
		OrgNameF: func() string { return "test-org" },
		BackendF: func() backend.Backend { return be },
	}

	var projectStack workspace.ProjectStack
	stackYAML := "environment:\n  imports:\n    - project/env@3\n" +
		"  values:\n    pulumiConfig:\n      test:string: esc"
	err := yaml.Unmarshal([]byte(stackYAML), &projectStack)
	require.NoError(t, err)

	openEnv, diags, err := openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
	require.Len(t, diags, 0)
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
			encrypter := &secrets.MockEncrypter{EncryptValueF: func(_ string) string { return "ciphertext" }}
			return encrypter
		},
		DecrypterF: func() config.Decrypter {
			decrypter := &secrets.MockDecrypter{
				DecryptValueF: func(_ string) string {
					return "plaintext"
				},
				BatchDecryptF: func(_ []string) []string {
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
		stack.DefaultSecretManagerF = func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
			return mockSecretsManager, nil
		}

		return stack
	}

	stack := getMockStack("mystack")

	var projectStack workspace.ProjectStack
	err := yaml.Unmarshal([]byte("environment:\n  - test"), &projectStack)
	require.NoError(t, err)

	project := workspace.Project{Name: tokens.PackageName("project")}

	ctx := t.Context()
	cfg, err := getStackConfigurationFromProjectStack(
		ctx,
		stack,
		&project,
		mockSecretsManager,
		&projectStack,
		nil,
		true,
	)
	require.NoError(t, err)

	assert.Nil(t, cfg.Config)
	cfg.Config = config.Map{}

	err = pkgWorkspace.ApplyProjectConfig(
		ctx, "mystack", &project, cfg.Environment, cfg.Config, config.NopEncrypter, config.NopDecrypter,
	)
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
			encrypter := &secrets.MockEncrypter{EncryptValueF: func(_ string) string { return "ciphertext" }}
			return encrypter
		},
		DecrypterF: func() config.Decrypter {
			decrypter := &secrets.MockDecrypter{
				DecryptValueF: func(_ string) string {
					return "plaintext"
				},
				BatchDecryptF: func(_ []string) []string {
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
		stack.DefaultSecretManagerF = func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
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
			t.Context(),
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
			_ map[string]string,
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

	_, diags, err := openStackEnv(t.Context(), stack, &projectStack, nil)
	require.NoError(t, err)
	require.Len(t, diags, 1)
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
			_ map[string]string,
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

	_, _, err = openStackEnv(t.Context(), stack, &projectStack, nil)
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
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "test-project",
			}, "/test/path", nil
		},
	}

	for _, tt := range tests {
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

// newManagedStack returns stack acme/otherproj/new managing environment p/old, so a test fails if the stack's project
// or name leaks into an ESC call. Every ESC call and GetLatestConfiguration fails the test unless the test passes
// evaluate or replaces the backend's function.
func newManagedStack(
	t *testing.T,
	evaluate func(
		org string,
		project string,
		name string,
		definition []byte,
	) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error),
) (*backend.MockStack, *backend.MockManagedEnvironmentsBackend) {
	t.Helper()

	if evaluate == nil {
		evaluate = func(
			string, string, string, []byte,
		) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
			t.Error("unexpected EvaluateManagedEnvironment call")
			return nil, nil, errors.New("unexpected call")
		}
	}
	be := &backend.MockManagedEnvironmentsBackend{
		MockEnvironmentsBackend: backend.MockEnvironmentsBackend{
			MockBackend: backend.MockBackend{
				NameF: func() string { return "test" },
				GetLatestConfigurationF: func(context.Context, backend.Stack) (backend.LatestConfiguration, error) {
					t.Error("unexpected GetLatestConfiguration call")
					return backend.LatestConfiguration{}, nil
				},
			},
			CheckYAMLEnvironmentF: func(
				context.Context, string, []byte,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				t.Error("unexpected CheckYAMLEnvironment call")
				return nil, nil, errors.New("unexpected call")
			},
			OpenYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration, map[string]string,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				t.Error("unexpected OpenYAMLEnvironment call")
				return nil, nil, errors.New("unexpected call")
			},
		},
		EvaluateManagedEnvironmentF: func(
			_ context.Context, org string, project string, name string, definition []byte,
		) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
			return evaluate(org, project, name, definition)
		},
		EncryptEnvironmentSecretsF: func(context.Context, string, string, string, [][]byte) ([]string, error) {
			t.Error("unexpected EncryptEnvironmentSecrets call")
			return nil, errors.New("unexpected call")
		},
	}
	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "acme/otherproj/new",
				NameV:               tokens.MustParseStackName("new"),
				ProjectV:            "otherproj",
				FullyQualifiedNameV: tokens.QName("acme/otherproj/new"),
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{ManagedEnvironment: "p/old"}
		},
		OrgNameF: func() string { return "acme" },
		BackendF: func() backend.Backend { return be },
	}
	return stack, be
}

func loadTestProjectStack(t *testing.T, stackYAML string) *workspace.ProjectStack {
	t.Helper()

	var ps workspace.ProjectStack
	require.NoError(t, yaml.Unmarshal([]byte(stackYAML), &ps))
	return &ps
}

const managedStackYAML = `environment:
  values:
    pulumiConfig:
      otherproj:key: env
`

func TestOpenStackEnvManaged(t *testing.T) {
	t.Parallel()

	var evaluated []string
	stack, _ := newManagedStack(t, func(
		org string,
		project string,
		name string,
		definition []byte,
	) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
		evaluated = append(evaluated, org+"/"+project+"/"+name)
		assert.YAMLEq(t, "values: {pulumiConfig: {otherproj:key: env}}", string(definition))
		return &backend.ManagedEnvironmentEvaluation{
			Environment: &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{"otherproj:key": esc.NewValue("env")}),
			}},
			BaseRevision:    7,
			ChangeRequestID: "cr-1",
		}, nil, nil
	})

	project := workspace.Project{Name: "otherproj"}
	cfg, err := getStackConfigurationFromProjectStack(
		t.Context(), stack, &project, nil, loadTestProjectStack(t, managedStackYAML), nil, true)
	require.NoError(t, err)

	assert.Equal(t, []string{"acme/p/old"}, evaluated)
	assert.Equal(t, &apitype.UpdateProgramEnvironment{
		BaseRevision:    7,
		ChangeRequestID: "cr-1",
		DraftRevision:   0,
	}, cfg.ManagedEnvironment)
	assert.Equal(t, esc.NewValue(map[string]esc.Value{"otherproj:key": esc.NewValue("env")}), cfg.Environment)
}

func TestGetStackConfigurationManagedEnvironmentImports(t *testing.T) {
	t.Parallel()

	stack, _ := newManagedStack(t, func(
		string, string, string, []byte,
	) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
		return &backend.ManagedEnvironmentEvaluation{Environment: &esc.Environment{}, BaseRevision: 7}, nil, nil
	})

	workspaceStack := loadTestProjectStack(t, `environment:
  imports:
    - proj/a
  values:
    pulumiConfig:
      otherproj:key: env
`)
	project := workspace.Project{Name: "otherproj"}
	cfg, err := getStackConfigurationFromProjectStack(t.Context(), stack, &project, nil, workspaceStack, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"proj/a"}, cfg.EnvironmentImports)
}

func TestOpenStackEnvManagedErrors(t *testing.T) {
	t.Parallel()

	fileHint := stack.ManagedEnvironmentMissingFromFileHint("acme", "p/old", "new", "")
	cases := []struct {
		name         string
		stackYAML    string
		envOverrides []string
		expected     string
	}{
		{"no environment", "", nil, fileHint},
		{"empty mapping", "environment: {}", nil, fileHint},
		{"list form", "environment:\n  - other", nil, fileHint},
		{
			"values under config",
			managedStackYAML + "config:\n  otherproj:key: stack\n",
			nil,
			"has values under `config:`",
		},
		{
			"override-env",
			managedStackYAML,
			[]string{"a=b"},
			"--override-env is not supported on stacks with an ESC-managed environment yet",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, _ := newManagedStack(t, nil)
			ps := loadTestProjectStack(t, c.stackYAML)

			_, _, _, managedEnv, err := openStackEnvWithManaged(t.Context(), s, ps, c.envOverrides, true)
			assert.ErrorContains(t, err, c.expected)
			assert.Nil(t, managedEnv)

			if c.envOverrides == nil {
				_, _, err = checkStackEnv(t.Context(), s, ps)
				assert.ErrorContains(t, err, c.expected)
			}
		})
	}
}

func TestOpenStackEnvManagedEvaluateError(t *testing.T) {
	t.Parallel()

	s, _ := newManagedStack(t, func(
		string, string, string, []byte,
	) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
		return nil, nil, errors.New("environment p/old not found")
	})

	_, _, _, managedEnv, err := openStackEnvWithManaged(
		t.Context(), s, loadTestProjectStack(t, managedStackYAML), nil, true)
	assert.ErrorContains(t, err, "environment p/old not found")
	assert.Nil(t, managedEnv)

	_, err = getStackConfigurationFromProjectStack(
		t.Context(), s, &workspace.Project{Name: "otherproj"}, nil, loadTestProjectStack(t, managedStackYAML), nil, true)
	assert.ErrorContains(t, err, "environment p/old not found")
}

func TestCheckStackEnvManaged(t *testing.T) {
	t.Parallel()

	s, be := newManagedStack(t, nil)
	checked := &esc.Environment{}
	be.CheckYAMLEnvironmentF = func(
		_ context.Context,
		org string,
		definition []byte,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		assert.Equal(t, "acme", org)
		assert.YAMLEq(t, "values: {pulumiConfig: {otherproj:key: env}}", string(definition))
		return checked, nil, nil
	}

	env, _, err := checkStackEnv(t.Context(), s, loadTestProjectStack(t, managedStackYAML))
	require.NoError(t, err)
	assert.Same(t, checked, env)
}

func TestPrintManagedEnvironmentNotices(t *testing.T) {
	t.Parallel()

	s, _ := newManagedStack(t, nil)

	var out bytes.Buffer
	printManagedEnvironmentNotices(&out, s, &apitype.UpdateProgramEnvironment{BaseRevision: 7, ChangeRequestID: "cr-1"})
	assert.Equal(t,
		"using local changes to environment p/old (based on revision 7); an update publishes them, a preview does not\n",
		out.String())

	out.Reset()
	printManagedEnvironmentNotices(&out, s, &apitype.UpdateProgramEnvironment{BaseRevision: 7})
	assert.Empty(t, out.String())
}

func TestGetStackConfigurationOrLatestManaged(t *testing.T) {
	t.Parallel()

	s, _ := newManagedStack(t, nil)
	_, _, err := GetStackConfigurationOrLatest(t.Context(), nil, stack.SecretsManagerLoader{}, s, nil, "", nil)
	assert.ErrorIs(t, err, errDestroyManagedOutsideProject)
}

// deliverySlotEnv is what the service publishes: provenance, and project-qualified config.
func deliverySlotEnv(config map[string]esc.Value) *esc.Environment {
	properties := map[string]esc.Value{
		"delivery": esc.NewValue(map[string]esc.Value{"pipeline": esc.NewValue("acme/delivery/prod")}),
	}
	if config != nil {
		properties["pulumiConfig"] = esc.NewValue(config)
	}
	return &esc.Environment{Properties: properties}
}

// newDeliveryStack returns a stack in project "proj" with a Pulumi Delivery environment at revision 4. userEnv is what
// the stack's own environment opens to.
func newDeliveryStack(
	t *testing.T,
	userEnv *esc.Environment,
	open func() (*esc.Environment, apitype.EnvironmentDiagnostics, error),
) *backend.MockStack {
	t.Helper()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		OpenYAMLEnvironmentF: func(
			context.Context, string, []byte, time.Duration, map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return userEnv, nil, nil
		},
		OpenEnvironmentRevisionF: func(
			_ context.Context, org string, project string, name string, revision int, duration time.Duration,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "acme", org)
			assert.Equal(t, "pulumi-delivery", project)
			assert.Equal(t, "program-id", name)
			assert.Equal(t, 4, revision)
			assert.Equal(t, 2*time.Hour, duration)
			return open()
		},
	}
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "acme/proj/dev",
				NameV:               tokens.MustParseStackName("dev"),
				ProjectV:            "proj",
				FullyQualifiedNameV: tokens.QName("acme/proj/dev"),
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{DeliveryEnvironment: &apitype.StackDeliveryEnvironment{
				Environment: "pulumi-delivery/program-id",
				Revision:    4,
			}}
		},
		OrgNameF: func() string { return "acme" },
		BackendF: func() backend.Backend { return be },
	}
}

func TestOpenStackEnvDelivery(t *testing.T) {
	t.Parallel()

	slot := func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return deliverySlotEnv(map[string]esc.Value{"proj:image": esc.NewValue("v2")}), nil, nil
	}

	t.Run("a stack with no environment takes only the admitted config", func(t *testing.T) {
		t.Parallel()

		stack := newDeliveryStack(t, nil, slot)
		env, diags, err := openStackEnv(t.Context(), stack, &workspace.ProjectStack{}, nil)
		require.NoError(t, err)
		assert.Empty(t, diags)
		assert.Equal(t, map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{"proj:image": esc.NewValue("v2")}),
		}, env.Properties)
	})

	t.Run("the stack's environment keeps everything else", func(t *testing.T) {
		t.Parallel()

		stack := newDeliveryStack(t, &esc.Environment{Properties: map[string]esc.Value{
			"pulumiConfig":         esc.NewValue(map[string]esc.Value{"aws:region": esc.NewValue("us-west-2")}),
			"environmentVariables": esc.NewValue(map[string]esc.Value{"TOKEN": esc.NewValue("t")}),
		}}, slot)
		ps := loadTestProjectStack(t, "environment:\n  - shared\n")
		env, _, err := openStackEnv(t.Context(), stack, ps, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{
				"aws:region": esc.NewValue("us-west-2"),
				"proj:image": esc.NewValue("v2"),
			}),
			"environmentVariables": esc.NewValue(map[string]esc.Value{"TOKEN": esc.NewValue("t")}),
		}, env.Properties)
	})

	t.Run("a revision with no config adds nothing", func(t *testing.T) {
		t.Parallel()

		stack := newDeliveryStack(t, nil, func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return deliverySlotEnv(nil), nil, nil
		})
		env, diags, err := openStackEnv(t.Context(), stack, &workspace.ProjectStack{}, nil)
		require.NoError(t, err)
		assert.Empty(t, diags)
		assert.Nil(t, env)
	})

	t.Run("an open error names the stack", func(t *testing.T) {
		t.Parallel()

		stack := newDeliveryStack(t, nil, func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return nil, nil, errors.New("forbidden")
		})
		_, _, err := openStackEnv(t.Context(), stack, &workspace.ProjectStack{}, nil)
		assert.ErrorContains(t, err, "acme/proj/dev")
		assert.ErrorContains(t, err, "forbidden")
	})

	t.Run("diagnostics are returned", func(t *testing.T) {
		t.Parallel()

		diags := apitype.EnvironmentDiagnostics{{Summary: "bad"}}
		stack := newDeliveryStack(t, nil, func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return nil, diags, nil
		})
		env, got, err := openStackEnv(t.Context(), stack, &workspace.ProjectStack{}, nil)
		require.NoError(t, err)
		assert.Nil(t, env)
		assert.Equal(t, []apitype.EnvironmentDiagnostic(diags), got)
	})
}

func TestDeliveryConfigPrecedence(t *testing.T) {
	t.Parallel()

	slotEnv := func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return deliverySlotEnv(map[string]esc.Value{
			"proj:image": esc.NewValue("delivery"),
			"proj:other": esc.NewValue("x"),
		}), nil, nil
	}
	userEnv := func(key string) *esc.Environment {
		return &esc.Environment{Properties: map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{key: esc.NewValue("local")}),
		}}
	}
	envStack := func(t *testing.T) *workspace.ProjectStack {
		return loadTestProjectStack(t, "environment:\n  - shared\n")
	}
	fileKey := config.MustMakeKey("proj", "image")
	fileStack := func(*testing.T) *workspace.ProjectStack {
		return &workspace.ProjectStack{Config: config.Map{fileKey: config.NewValue("local")}}
	}
	withDelivery := map[string]esc.Value{"proj:image": esc.NewValue("delivery"), "proj:other": esc.NewValue("x")}

	for _, tc := range []struct {
		name      string
		overrides bool
		env       *esc.Environment
		ps        func(*testing.T) *workspace.ProjectStack
		expected  map[string]esc.Value
		config    bool
	}{
		{
			name: "a local run keeps a qualified key of the stack's environment",
			env:  userEnv("proj:image"),
			ps:   envStack,
			expected: map[string]esc.Value{
				"proj:image": esc.NewValue("local"),
				"proj:other": esc.NewValue("x"),
			},
		},
		{
			name: "a local run keeps a bare key of the stack's environment",
			env:  userEnv("image"),
			ps:   envStack,
			expected: map[string]esc.Value{
				"image":      esc.NewValue("local"),
				"proj:other": esc.NewValue("x"),
			},
		},
		{
			name:     "a local run keeps a key of the stack's file",
			ps:       fileStack,
			expected: map[string]esc.Value{"proj:other": esc.NewValue("x")},
			config:   true,
		},
		{
			name:      "Delivery's work replaces a bare key of the stack's environment",
			overrides: true,
			env:       userEnv("image"),
			ps:        envStack,
			expected:  withDelivery,
		},
		{
			name:      "Delivery's work ignores a key of the stack's file",
			overrides: true,
			ps:        fileStack,
			expected:  withDelivery,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stack := newDeliveryStack(t, tc.env, slotEnv)
			location := stack.ConfigLocationF()
			location.DeliveryEnvironment.OverridesStackConfig = tc.overrides
			stack.ConfigLocationF = func() backend.StackConfigLocation { return location }

			ps := tc.ps(t)
			project := workspace.Project{Name: "proj"}
			cfg, err := getStackConfigurationFromProjectStack(t.Context(), stack, &project, nil, ps, nil, true)
			require.NoError(t, err)
			assert.Equal(t, esc.NewValue(tc.expected), cfg.Environment)

			_, inConfig := cfg.Config[fileKey]
			assert.Equal(t, tc.config, inConfig)
			assert.Equal(t, tc.ps(t).Config, ps.Config, "composing never changes the stack's file")
		})
	}

	t.Run("the same name in another namespace is not the same key", func(t *testing.T) {
		t.Parallel()

		stack := newDeliveryStack(t, nil, slotEnv)
		otherKey := config.MustMakeKey("aws", "image")
		ps := &workspace.ProjectStack{Config: config.Map{otherKey: config.NewValue("local")}}
		env, _, err := openStackEnv(t.Context(), stack, ps, nil)
		require.NoError(t, err)
		assert.Equal(t, esc.NewValue(withDelivery), env.Properties["pulumiConfig"])
	})
}

func TestPrintDeliveryNotices(t *testing.T) {
	t.Parallel()

	both := []config.Key{config.MustMakeKey("proj", "image")}
	notices := func(overrides bool, keys []config.Key) string {
		var out bytes.Buffer
		printDeliveryNotices(&out, deliverySlotEnv(nil), &apitype.StackDeliveryEnvironment{
			Environment:          "pulumi-delivery/program-id",
			Revision:             4,
			OverridesStackConfig: overrides,
		}, keys)
		return out.String()
	}

	assert.Equal(t, "using config from Pulumi Delivery (pipeline acme/delivery/prod), revision 4\n", notices(false, nil))

	local := notices(false, both)
	assert.Contains(t, local,
		"this stack sets config that Pulumi Delivery (pipeline acme/delivery/prod) also sets: proj:image")
	assert.Contains(t, local, "This run uses the stack's values")

	delivery := notices(true, both)
	assert.Contains(t, delivery,
		"Pulumi Delivery (pipeline acme/delivery/prod) overrides config that this stack also sets: proj:image")
}

func TestGetStackConfigurationDelivery(t *testing.T) {
	t.Parallel()

	slot := func() (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return deliverySlotEnv(map[string]esc.Value{"proj:image": esc.NewValue("v2")}), nil, nil
	}
	admitted := &apitype.StackDeliveryEnvironment{Environment: "pulumi-delivery/program-id", Revision: 4}

	t.Run("the configuration says which revision it composed", func(t *testing.T) {
		t.Parallel()

		project := workspace.Project{Name: "proj"}
		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), newDeliveryStack(t, nil, slot), &project, nil, &workspace.ProjectStack{}, nil, true)
		require.NoError(t, err)
		assert.Equal(t, admitted, cfg.DeliveryEnvironment)
		assert.Equal(t, esc.NewValue(map[string]esc.Value{"proj:image": esc.NewValue("v2")}), cfg.Environment)
		assert.Empty(t, cfg.EnvironmentImports)
	})

	t.Run("no Delivery environment sends nothing", func(t *testing.T) {
		t.Parallel()

		stack := getMockStackWithEnv(t, nil)
		stack.RefF = func() backend.StackReference { return &backend.MockStackReference{} }
		project := workspace.Project{Name: "proj"}
		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), stack, &project, nil, &workspace.ProjectStack{}, nil, true)
		require.NoError(t, err)
		assert.Nil(t, cfg.DeliveryEnvironment)
	})

	t.Run("config copied from the latest update loads", func(t *testing.T) {
		t.Parallel()

		ps := &workspace.ProjectStack{Config: config.Map{config.MustMakeKey("proj", "image"): config.NewValue("v1")}}
		project := workspace.Project{Name: "proj"}
		cfg, err := getStackConfigurationFromProjectStack(
			t.Context(), newDeliveryStack(t, nil, slot), &project, nil, ps, nil, false)
		require.NoError(t, err)
		assert.Equal(t, admitted, cfg.DeliveryEnvironment)
	})
}
