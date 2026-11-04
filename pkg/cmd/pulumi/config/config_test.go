// Copyright 2025, Pulumi Corporation.
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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConfig(t *testing.T) {
	ctx := t.Context()

	openEnv := &esc.Environment{
		Properties: map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{
				"env:value":  esc.NewValue("envVal1"),
				"env:secret": esc.NewSecret("envSecret1"),
				"common:obj": esc.NewValue(map[string]esc.Value{
					"envValue":    esc.NewValue("envVal2"),
					"commonValue": esc.NewValue("envVal3"),
					"commonArray": esc.NewValue([]esc.Value{esc.NewValue("envVal4"), esc.NewValue("envVal5")}),
				}),
				"env:obj": esc.NewValue(map[string]esc.Value{
					"secret": esc.NewValue([]esc.Value{esc.NewSecret("envSecret2")}),
				}),
			}),
		},
	}

	checkEnv := &esc.Environment{
		Properties: map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{
				"env:value":  esc.NewValue("envVal1"),
				"env:secret": {Secret: true, Unknown: true},
				"common:obj": esc.NewValue(map[string]esc.Value{
					"envValue":    esc.NewValue("envVal2"),
					"commonValue": esc.NewValue("envVal3"),
					"commonArray": esc.NewValue([]esc.Value{esc.NewValue("envVal4"), esc.NewValue("envVal5")}),
				}),
				"env:obj": esc.NewValue(map[string]esc.Value{
					"secret": esc.NewValue([]esc.Value{{Secret: true, Unknown: true}}),
				}),
			}),
		},
	}

	plainEnv := &esc.Environment{
		Properties: map[string]esc.Value{
			"pulumiConfig": esc.NewValue(map[string]esc.Value{
				"env:value": esc.NewValue("envVal1"),
				"common:obj": esc.NewValue(map[string]esc.Value{
					"envValue":    esc.NewValue("envVal2"),
					"commonValue": esc.NewValue("envVal3"),
					"commonArray": esc.NewValue([]esc.Value{esc.NewValue("envVal4"), esc.NewValue("envVal5")}),
				}),
				"env:obj": esc.NewValue(map[string]esc.Value{
					"value": esc.NewValue([]esc.Value{esc.NewValue("envVal6")}),
				}),
			}),
		},
	}

	commonObjValue := `{"commonValue":"cfgVal2","commonArray":["cfgVal3","cfgVal4"]}`

	cfg := map[config.Key]config.Value{
		config.MustMakeKey("cfg", "value"):  config.NewValue("cfgVal1"),
		config.MustMakeKey("cfg", "secret"): config.NewSecureValue("Y2ZnU2VjcmV0MQ==" /*base64 of cfgSecret1*/),
		config.MustMakeKey("common", "obj"): config.NewObjectValue(commonObjValue),
	}

	plainCfg := map[config.Key]config.Value{
		config.MustMakeKey("cfg", "value"):  config.NewValue("cfgVal1"),
		config.MustMakeKey("common", "obj"): config.NewObjectValue(commonObjValue),
	}

	t.Run("with no env and with cfg and showSecrets=true openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, cfg, nil)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 0, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:secret  cfgSecret1
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2"}
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and no cfg and showSecrets=true openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, config.Map{}, openEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
common:obj  {"commonArray":["envVal4","envVal5"],"commonValue":"envVal3","envValue":"envVal2"}
env:obj     {"secret":["envSecret2"]}
env:secret  envSecret1
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and cfg and showSecrets=true openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, cfg, openEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:secret  cfgSecret1
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["envSecret2"]}
env:secret  envSecret1
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and cfg and showSecrets=false openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, cfg, openEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, false, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 0, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:secret  [secret]
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["[secret]"]}
env:secret  [secret]
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and cfg and showSecrets=true openEnv=false", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, cfg, checkEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, false, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:secret  cfgSecret1
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["[unknown]"]}
env:secret  [unknown]
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and cfg and showSecrets=false openEnv=false", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, cfg, checkEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, false, false, false, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 0, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:secret  [secret]
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["[secret]"]}
env:secret  [secret]
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with plain env and plain cfg and showSecrets=true openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, plainCfg, plainEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 0, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 0, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"value":["envVal6"]}
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and plain cfg and showSecrets=true openEnv=true", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, false)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, plainCfg, openEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["envSecret2"]}
env:secret  envSecret1
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})

	t.Run("with env and plain cfg and showSecrets=true openEnv=true and cached crypter", func(t *testing.T) {
		t.Parallel()

		secretsManager, calledEncryptValue, calledBatchEncrypt, calledDecryptValue,
			calledBatchDecrypt := getCountingBase64SecretsManager(ctx, t, true)
		preparedStack, project, projectStack, secretsManagerLoader := prepareConfig(t, secretsManager, plainCfg, openEnv)

		var stdout bytes.Buffer
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true, "")
		require.NoError(t, err)

		require.Equal(t, 0, *calledEncryptValue)
		require.Equal(t, 1, *calledBatchEncrypt)
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 0, *calledBatchDecrypt)

		expectedStdOut := strings.TrimSpace(`
KEY         VALUE
cfg:value   cfgVal1
common:obj  {"commonArray":["cfgVal3","cfgVal4"],"commonValue":"cfgVal2","envValue":"envVal2"}
env:obj     {"secret":["envSecret2"]}
env:secret  envSecret1
env:value   envVal1
`)
		require.Equal(t, expectedStdOut, strings.TrimSpace(stdout.String()))
	})
}

func getCountingBase64SecretsManager(
	ctx context.Context,
	t *testing.T,
	withCachedCrypter bool,
) (*secrets.MockSecretsManager, *int, *int, *int, *int) {
	calledEncryptValue := 0
	calledBatchEncrypt := 0
	calledDecryptValue := 0
	calledBatchDecrypt := 0
	encrypter := &secrets.MockEncrypter{
		EncryptValueF: func(input string) string {
			calledEncryptValue++
			ct, err := config.Base64Crypter.EncryptValue(ctx, input)
			require.NoError(t, err)
			return ct
		},
		BatchEncryptF: func(input []string) []string {
			calledBatchEncrypt++
			ct, err := config.Base64Crypter.BatchEncrypt(ctx, input)
			require.NoError(t, err)
			return ct
		},
	}
	decrypter := &secrets.MockDecrypter{
		DecryptValueF: func(input string) string {
			calledDecryptValue++
			pt, err := config.Base64Crypter.DecryptValue(ctx, input)
			require.NoError(t, err)
			return pt
		},
		BatchDecryptF: func(input []string) []string {
			calledBatchDecrypt++
			pt, err := config.Base64Crypter.BatchDecrypt(ctx, input)
			require.NoError(t, err)
			return pt
		},
	}
	cachedCrypter := config.NewCiphertextToPlaintextCachedCrypter(encrypter, decrypter)
	secretsManager := &secrets.MockSecretsManager{
		TypeF: func() string { return "mock" },
		EncrypterF: func() config.Encrypter {
			if withCachedCrypter {
				return cachedCrypter
			}
			return encrypter
		},
		DecrypterF: func() config.Decrypter {
			if withCachedCrypter {
				return cachedCrypter
			}
			return decrypter
		},
	}
	return secretsManager, &calledEncryptValue, &calledBatchEncrypt, &calledDecryptValue, &calledBatchDecrypt
}

func prepareConfig(
	t *testing.T,
	secretsManager secrets.Manager,
	cfg config.Map,
	env *esc.Environment,
) (backend.MockStack, workspace.Project, *workspace.ProjectStack, cmdStack.SecretsManagerLoader) {
	snapshot := &deploy.Snapshot{SecretsManager: stack.NewBatchingCachingSecretsManager(secretsManager)}

	mockStack := backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "testOrg/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{}
		},
		OrgNameF: func() string {
			return "testOrg"
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
		BackendF: func() backend.Backend {
			return &backend.MockEnvironmentsBackend{
				CheckYAMLEnvironmentF: func(
					ctx context.Context,
					org string,
					yaml []byte,
				) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
					return env, apitype.EnvironmentDiagnostics{}, nil
				},
				OpenYAMLEnvironmentF: func(
					ctx context.Context,
					org string,
					yaml []byte,
					duration time.Duration,
					_ map[string]string,
				) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
					return env, apitype.EnvironmentDiagnostics{}, nil
				},
			}
		},
	}

	project := workspace.Project{Name: "testProject"}
	stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{
		Environment: workspace.NewEnvironment([]string{"env"}),
		Config:      cfg,
	})
	require.NoError(t, err)

	projectStack, err := workspace.LoadProjectStackBytes(
		cmdutil.Diag(), &project, stackYAML, "Pulumi.stack.yaml", encoding.YAML,
	)
	require.NoError(t, err)

	ssml := cmdStack.SecretsManagerLoader{FallbackToState: true}

	return mockStack, project, projectStack, ssml
}

func TestConfigSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		expected string
		path     bool
	}{
		{
			name:     "toplevel bool",
			args:     []string{"testProject:test", "true"},
			expected: "config:\n  testProject:test: \"true\"\n",
		},
		{
			name:     "toplevel int",
			args:     []string{"testProject:test", "123"},
			expected: "config:\n  testProject:test: \"123\"\n",
		},
		{
			name:     "toplevel float",
			args:     []string{"testProject:test", "123.456"},
			expected: "config:\n  testProject:test: \"123.456\"\n",
		},
		{
			name:     "path'd bool",
			args:     []string{"testProject:test[0]", "true"},
			expected: "config:\n  testProject:test:\n    - true\n",
			path:     true,
		},
		{
			name:     "path'd int",
			args:     []string{"testProject:test[0]", "123"},
			expected: "config:\n  testProject:test:\n    - 123\n",
			path:     true,
		},
		{
			name:     "path'd float",
			args:     []string{"testProject:test[0]", "123.456"},
			expected: "config:\n  testProject:test:\n    - \"123.456\"\n",
			path:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV:               tokens.MustParseStackName("testStack"),
						FullyQualifiedNameV: "org/testProject/testStack",
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
				LoadProjectStack: func(
					_ context.Context,
					diags diag.Sink,
					project *workspace.Project,
					_ backend.Stack,
					_ string,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(diags, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			configFile := filepath.Join(tmpdir, "Pulumi.stack.yaml")

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(t.Context(), ws, c.args, &project, &s, configFile)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

func TestConfigSetTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		expected string
		typ      string
		path     bool
	}{
		{
			name:     "toplevel bool",
			args:     []string{"testProject:test", "true"},
			typ:      "bool",
			expected: "config:\n  testProject:test: true\n",
		},
		{
			name:     "toplevel int",
			args:     []string{"testProject:test", "123"},
			typ:      "int",
			expected: "config:\n  testProject:test: 123\n",
		},
		{
			name:     "toplevel float",
			args:     []string{"testProject:test", "123.456"},
			typ:      "float",
			expected: "config:\n  testProject:test: 123.456\n",
		},
		{
			name:     "toplevel string",
			args:     []string{"testProject:test", "123"},
			typ:      "string",
			expected: "config:\n  testProject:test: \"123\"\n",
		},
		{
			name:     "path'd bool",
			args:     []string{"testProject:test[0]", "true"},
			typ:      "bool",
			expected: "config:\n  testProject:test:\n    - true\n",
			path:     true,
		},
		{
			name:     "path'd int",
			args:     []string{"testProject:test[0]", "123"},
			typ:      "int",
			expected: "config:\n  testProject:test:\n    - 123\n",
			path:     true,
		},
		{
			name:     "path'd float",
			args:     []string{"testProject:test[0]", "123.456"},
			typ:      "float",
			expected: "config:\n  testProject:test:\n    - 123.456\n",
			path:     true,
		},
		{
			name:     "path'd string",
			args:     []string{"testProject:test[0]", "123"},
			typ:      "string",
			expected: "config:\n  testProject:test:\n    - \"123\"\n",
			path:     true,
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV:               tokens.MustParseStackName("testStack"),
						FullyQualifiedNameV: "org/testProject/testStack",
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
				Type: c.typ,
				LoadProjectStack: func(_ context.Context, d diag.Sink, project *workspace.Project, _ backend.Stack,
					_ string,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(d, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			configFile := filepath.Join(tmpdir, "Pulumi.stack.yaml")

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(t.Context(), ws, c.args, &project, &s, configFile)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

func TestConfigSetAll(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		plaintextArgs []string
		secretArgs    []string
		jsonArg       string
		path          bool
		expected      string
		expectError   string
	}{
		{
			name:          "plaintext values",
			plaintextArgs: []string{"testProject:key1=value1", "testProject:key2=value2"},
			expected:      "config:\n  testProject:key1: value1\n  testProject:key2: value2\n",
		},
		{
			name:          "plaintext with path",
			plaintextArgs: []string{"testProject:nested.key1=value1", "testProject:nested.key2=value2"},
			path:          true,
			expected:      "config:\n  testProject:nested:\n    key1: value1\n    key2: value2\n",
		},
		{
			name:       "secret values",
			secretArgs: []string{"testProject:secretKey1=secret1", "testProject:secretKey2=secret2"},
			expected: "config:\n  testProject:secretKey1:\n    secure: c2VjcmV0MQ==\n" +
				"  testProject:secretKey2:\n    secure: c2VjcmV0Mg==\n",
		},
		{
			name:       "secret with path",
			secretArgs: []string{"testProject:nested.secret1=secret1"},
			path:       true,
			expected:   "config:\n  testProject:nested:\n    secret1:\n      secure: c2VjcmV0MQ==\n",
		},
		{
			name:          "mixed plaintext and secret",
			plaintextArgs: []string{"testProject:plainKey=plainValue"},
			secretArgs:    []string{"testProject:secretKey=secretValue"},
			expected: "config:\n  testProject:plainKey: plainValue\n" +
				"  testProject:secretKey:\n    secure: c2VjcmV0VmFsdWU=\n",
		},
		{
			name:     "json plaintext values",
			jsonArg:  `{"testProject:key1": {"value": "value1"}, "testProject:key2": {"value": "value2"}}`,
			expected: "config:\n  testProject:key1: value1\n  testProject:key2: value2\n",
		},
		{
			name: "json secret values",
			jsonArg: `{"testProject:secretKey1": {"value": "secret1", "secret": true}, ` +
				`"testProject:secretKey2": {"value": "secret2", "secret": true}}`,
			expected: "config:\n  testProject:secretKey1:\n    secure: c2VjcmV0MQ==\n" +
				"  testProject:secretKey2:\n    secure: c2VjcmV0Mg==\n",
		},
		{
			name: "json mixed plaintext and secret",
			jsonArg: `{"testProject:plainKey": {"value": "plainValue"}, ` +
				`"testProject:secretKey": {"value": "secretValue", "secret": true}}`,
			expected: "config:\n  testProject:plainKey: plainValue\n" +
				"  testProject:secretKey:\n    secure: c2VjcmV0VmFsdWU=\n",
		},
		{
			name:          "json with plaintext flag should error",
			jsonArg:       `{"testProject:key": {"value": "val"}}`,
			plaintextArgs: []string{"testProject:otherkey=value"},
			expectError:   "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:        "json with secret flag should error",
			jsonArg:     `{"testProject:key": {"value": "val"}}`,
			secretArgs:  []string{"testProject:secretkey=secretvalue"},
			expectError: "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:        "json with path flag should error",
			jsonArg:     `{"testProject:key": {"value": "val"}}`,
			path:        true,
			expectError: "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:    "json with invalid key",
			jsonArg: `{"testProject:key1:invalid": {"value": "value"}}`,
			expectError: "invalid --json object key \"testProject:key1:invalid\": " +
				"could not parse testProject:key1:invalid as a configuration key " +
				"(configuration keys should be of the form `<namespace>:<name>`)",
		},
		{
			name:        "json with nil value",
			jsonArg:     `{"testProject:key1": {"value": null}}`,
			expected:    "config:\n  testProject:key1: null\n",
			expectError: `value for --json object key "testProject:key1" is nil`,
		},
		{
			name:        "json with malformed input",
			jsonArg:     `{`, // missing closing braces
			expectError: "could not parse --json argument: unexpected end of JSON input",
		},
		{
			name:     "json with object value",
			jsonArg:  `{"testProject:key1": {"value": "{\"inner\":\"value2\"}", "objectValue": {"inner": "value2"}}}`,
			expected: "config:\n  testProject:key1:\n    inner: value2\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV:               tokens.MustParseStackName("testStack"),
						FullyQualifiedNameV: "org/testProject/testStack",
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			tmpdir := t.TempDir()
			configFile := filepath.Join(tmpdir, "Pulumi.stack.yaml")

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &workspace.Project{
						Name: "testProject",
					}, "", nil
				},
			}

			// Create the command
			stackName := "testStack"
			lm := &cmdBackend.MockLoginManager{
				CurrentF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
				) (backend.Backend, error) {
					return &backend.MockBackend{
						GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
							return &s, nil
						},
					}, nil
				},
				LoginF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					color colors.Colorization, _ ...cmdBackend.LoginOption,
				) (backend.Backend, error) {
					return &backend.MockBackend{
						GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
							return &s, nil
						},
					}, nil
				},
			}

			// Create mock encrypter factory
			mockEncrypterFactory := &mockEncrypterFactory{
				encrypter: &secrets.MockEncrypter{
					EncryptValueF: func(plaintext string) string {
						return base64.StdEncoding.EncodeToString([]byte(plaintext))
					},
				},
			}

			cmd := newConfigSetAllCmd(ws, &stackName, lm, mockEncrypterFactory, &configFile)
			cmd.SetContext(t.Context())

			// Set flags based on test case
			if c.jsonArg != "" {
				err := cmd.PersistentFlags().Set("json", c.jsonArg)
				require.NoError(t, err)
			}
			for _, pt := range c.plaintextArgs {
				err := cmd.PersistentFlags().Set("plaintext", pt)
				require.NoError(t, err)
			}

			for _, sec := range c.secretArgs {
				err := cmd.PersistentFlags().Set("secret", sec)
				require.NoError(t, err)
			}
			if c.path {
				err := cmd.PersistentFlags().Set("path", "true")
				require.NoError(t, err)
			}

			// Execute the command
			err := cmd.RunE(cmd, []string{})

			// Check for expected error
			if c.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expectError)
				return
			}

			require.NoError(t, err)

			// Verify the config was set correctly
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

type mockEncrypterFactory struct {
	encrypter config.Encrypter
}

func (m *mockEncrypterFactory) GetEncrypter(
	_ context.Context,
	_ backend.Stack,
	_ *workspace.ProjectStack,
) (config.Encrypter, cmdStack.SecretsManagerState, error) {
	return m.encrypter, cmdStack.SecretsManagerUnchanged, nil
}

func TestConfigRefresh(t *testing.T) {
	t.Parallel()
	minimalDeployment := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"0001-01-01T00:00:00Z","magic":"","version":""}}`),
	}

	setupBackend := func(latestConfig backend.LatestConfiguration) (cmdBackend.LoginManager, pkgWorkspace.Context) {
		var mockBackend *backend.MockBackend
		mockBackend = &backend.MockBackend{
			GetLatestConfigurationF: func(_ context.Context, _ backend.Stack) (backend.LatestConfiguration, error) {
				return latestConfig, nil
			},
			ExportDeploymentF: func(_ context.Context, _ backend.Stack) (*apitype.UntypedDeployment, error) {
				return minimalDeployment, nil
			},
			GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{
					RefF: func() backend.StackReference {
						return &backend.MockStackReference{
							NameV:               tokens.MustParseStackName("testStack"),
							FullyQualifiedNameV: "org/testProject/testStack",
						}
					},
					ConfigLocationF: func() backend.StackConfigLocation {
						return backend.StackConfigLocation{}
					},
					BackendF: func() backend.Backend {
						return mockBackend
					},
				}, nil
			},
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				return &backend.MockStackReference{
					NameV:               tokens.MustParseStackName(s),
					FullyQualifiedNameV: tokens.QName("org/testProject/" + s),
				}, nil
			},
		}
		// Wire up the top-level mock stack's BackendF to return this backend.
		// GetStackF returns a mock stack whose BackendF also returns this backend.

		lm := &cmdBackend.MockLoginManager{
			LoginF: func(
				_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
				_ string, _ *workspace.Project, _ bool, _ bool, _ colors.Colorization, _ ...cmdBackend.LoginOption,
			) (backend.Backend, error) {
				return mockBackend, nil
			},
		}

		ws := &pkgWorkspace.MockContext{
			ReadProjectF: func(string) (*workspace.Project, string, error) {
				return &workspace.Project{Name: "testProject"}, "", nil
			},
			GetStoredCredentialsF: func() (workspace.Credentials, error) {
				return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
			},
		}

		return lm, ws
	}

	t.Run("environments from backend are written to config file", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		configPath := filepath.Join(tmpdir, "Pulumi.testStack.yaml")

		lm, ws := setupBackend(backend.LatestConfiguration{
			Config: config.Map{
				config.MustMakeKey("testProject", "key1"): config.NewValue("value1"),
			},
			Environments: []string{"env1", "env2"},
		})

		stackName := "testStack"
		cmd := newConfigRefreshCmd(ws, &stackName, lm, &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("force", "true"))

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		data, err := os.ReadFile(configPath)
		require.NoError(t, err)

		require.EqualValues(t,
			`config:
  testProject:key1: value1
environment:
  - env1
  - env2
`,
			data)
	})

	t.Run("nil environments do not overwrite existing environments", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		configPath := filepath.Join(tmpdir, "Pulumi.testStack.yaml")

		require.NoError(t, os.WriteFile(configPath,
			[]byte("environment:\n  - existing-env\nconfig:\n  testProject:old: old-value\n"), 0o600))

		lm, ws := setupBackend(backend.LatestConfiguration{
			Config: config.Map{
				config.MustMakeKey("testProject", "key1"): config.NewValue("value1"),
			},
		})

		stackName := "testStack"
		cmd := newConfigRefreshCmd(ws, &stackName, lm, &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("force", "true"))

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		data, err := os.ReadFile(configPath)
		require.NoError(t, err)

		require.EqualValues(t, `environment:
  - existing-env
config:
  testProject:key1: value1
`, data)
	})
}

func TestConfigPathOperations(t *testing.T) {
	t.Parallel()
	type testArgs struct {
		Key                   string
		Value                 string
		Secret                bool
		Path                  bool
		TopLevelKey           string
		TopLevelExpectedValue string
		ExpectFailure         bool
	}

	tests := [][]testArgs{
		{
			{
				Key:                   "testProject:aConfigValue",
				Value:                 "this value is a value",
				TopLevelKey:           "testProject:aConfigValue",
				TopLevelExpectedValue: "this value is a value",
			},
			{
				Key:                   "testProject:anotherConfigValue",
				Value:                 "this value is another value",
				TopLevelKey:           "testProject:anotherConfigValue",
				TopLevelExpectedValue: "this value is another value",
			},
			{
				Key:                   "testProject:bEncryptedSecret",
				Value:                 "this super secret is encrypted",
				Secret:                true,
				TopLevelKey:           "testProject:bEncryptedSecret",
				TopLevelExpectedValue: "this super secret is encrypted",
			},
			{
				Key:                   "testProject:anotherEncryptedSecret",
				Value:                 "another encrypted secret",
				Secret:                true,
				TopLevelKey:           "testProject:anotherEncryptedSecret",
				TopLevelExpectedValue: "another encrypted secret",
			},
			// Overwriting a top-level string value is allowed.
			{
				Key:                   "testProject:aConfigValue.inner",
				Value:                 "new value",
				Path:                  true,
				TopLevelKey:           "testProject:aConfigValue",
				TopLevelExpectedValue: `{"inner":"new value"}`,
			},
			{
				Key:                   "testProject:anotherConfigValue[0]",
				Value:                 "new value",
				Path:                  true,
				TopLevelKey:           "testProject:anotherConfigValue",
				TopLevelExpectedValue: `["new value"]`,
			},
			{
				Key:                   "testProject:bEncryptedSecret.inner",
				Value:                 "new value",
				Path:                  true,
				TopLevelKey:           "testProject:bEncryptedSecret",
				TopLevelExpectedValue: `{"inner":"new value"}`,
			},
			{
				Key:                   "testProject:anotherEncryptedSecret[0]",
				Value:                 "new value",
				Path:                  true,
				TopLevelKey:           "testProject:anotherEncryptedSecret",
				TopLevelExpectedValue: `["new value"]`,
			},
		},
		{
			{
				Key:                   "testProject:[]",
				Value:                 "square brackets value",
				TopLevelKey:           "testProject:[]",
				TopLevelExpectedValue: "square brackets value",
			},
			{
				Key:                   "testProject:x.y",
				Value:                 "x.y value",
				TopLevelKey:           "testProject:x.y",
				TopLevelExpectedValue: "x.y value",
			},
			{
				Key:                   "testProject:0",
				Value:                 "0 value",
				Path:                  true,
				TopLevelKey:           "testProject:0",
				TopLevelExpectedValue: "0 value",
			},
			{
				Key:                   "testProject:true",
				Value:                 "value",
				Path:                  true,
				TopLevelKey:           "testProject:true",
				TopLevelExpectedValue: "value",
			},
		},
		{
			{
				Key:                   `testProject:["test.Key"]`,
				Value:                 "test key value",
				Path:                  true,
				TopLevelKey:           "testProject:test.Key",
				TopLevelExpectedValue: "test key value",
			},
			{
				Key:                   `testProject:nested["test.Key"]`,
				Value:                 "nested test key value",
				Path:                  true,
				TopLevelKey:           "testProject:nested",
				TopLevelExpectedValue: `{"test.Key":"nested test key value"}`,
			},
			{
				Key:                   "testProject:outer.inner",
				Value:                 "value",
				Path:                  true,
				TopLevelKey:           "testProject:outer",
				TopLevelExpectedValue: `{"inner":"value"}`,
			},

			// Wrong type
			{Key: "testProject:outer[0]", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:outer.inner.nested", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:outer.inner[0]", Value: "v", Path: true, ExpectFailure: true},
		},
		{
			{
				Key:                   "testProject:names[0]",
				Value:                 "a",
				Path:                  true,
				TopLevelKey:           "testProject:names",
				TopLevelExpectedValue: `["a"]`,
			},
			{
				Key:                   "testProject:names[1]",
				Value:                 "b",
				Path:                  true,
				TopLevelKey:           "testProject:names",
				TopLevelExpectedValue: `["a","b"]`,
			},
			{
				Key:                   "testProject:names[2]",
				Value:                 "c",
				Path:                  true,
				TopLevelKey:           "testProject:names",
				TopLevelExpectedValue: `["a","b","c"]`,
			},
			{
				Key:                   "testProject:names[3]",
				Value:                 "super secret name",
				Path:                  true,
				Secret:                true,
				TopLevelKey:           "testProject:names",
				TopLevelExpectedValue: `["a","b","c","super secret name"]`,
			},

			// Index out of range.
			{Key: "testProject:names[-1]", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:names[5]", Value: "v", Path: true, ExpectFailure: true},

			// Wrong type
			{Key: "testProject:names.nested", Value: "v", Path: true, ExpectFailure: true},
		},
		{
			{
				Key:                   "testProject:servers[0].port",
				Value:                 "80",
				Path:                  true,
				TopLevelKey:           "testProject:servers",
				TopLevelExpectedValue: `[{"port":80}]`,
			},
			{
				Key:                   "testProject:servers[0].host",
				Value:                 "example",
				Path:                  true,
				TopLevelKey:           "testProject:servers",
				TopLevelExpectedValue: `[{"host":"example","port":80}]`,
			},
		},
		{
			{
				Key:                   "testProject:a.b[0].c",
				Value:                 "true",
				Path:                  true,
				TopLevelKey:           "testProject:a",
				TopLevelExpectedValue: `{"b":[{"c":true}]}`,
			},
			{
				Key:                   "testProject:a.b[1].c",
				Value:                 "false",
				Path:                  true,
				TopLevelKey:           "testProject:a",
				TopLevelExpectedValue: `{"b":[{"c":true},{"c":false}]}`,
			},
		},
		{
			{
				Key:                   "testProject:tokens[0]",
				Value:                 "shh",
				Path:                  true,
				Secret:                true,
				TopLevelKey:           "testProject:tokens",
				TopLevelExpectedValue: `["shh"]`,
			},
			{
				Key:                   "testProject:foo.bar",
				Value:                 "don't tell",
				Path:                  true,
				Secret:                true,
				TopLevelKey:           "testProject:foo",
				TopLevelExpectedValue: `{"bar":"don't tell"}`,
			},
		},
		{
			{
				Key:                   "testProject:semiInner.a.b.c.d",
				Value:                 "1",
				Path:                  true,
				TopLevelKey:           "testProject:semiInner",
				TopLevelExpectedValue: `{"a":{"b":{"c":{"d":1}}}}`,
			},
			{
				Key:                   "testProject:wayInner.a.b.c.d.e.f.g.h.i.j.k",
				Value:                 "false",
				Path:                  true,
				TopLevelKey:           "testProject:wayInner",
				TopLevelExpectedValue: `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":false}}}}}}}}}}}`,
			},
		},
		{
			{
				Key:                   "testProject:foo1[0]",
				Value:                 "false",
				Path:                  true,
				TopLevelKey:           "testProject:foo1",
				TopLevelExpectedValue: `[false]`,
			},
			{
				Key:                   "testProject:foo2[0]",
				Value:                 "true",
				Path:                  true,
				TopLevelKey:           "testProject:foo2",
				TopLevelExpectedValue: `[true]`,
			},
			{
				Key:                   "testProject:foo3[0]",
				Value:                 "10",
				Path:                  true,
				TopLevelKey:           "testProject:foo3",
				TopLevelExpectedValue: `[10]`,
			},
			{
				Key:                   "testProject:foo4[0]",
				Value:                 "0",
				Path:                  true,
				TopLevelKey:           "testProject:foo4",
				TopLevelExpectedValue: `[0]`,
			},
			{
				Key:                   "testProject:foo5[0]",
				Value:                 "00",
				Path:                  true,
				TopLevelKey:           "testProject:foo5",
				TopLevelExpectedValue: `["00"]`,
			},
			{
				Key:                   "testProject:foo6[0]",
				Value:                 "01",
				Path:                  true,
				TopLevelKey:           "testProject:foo6",
				TopLevelExpectedValue: `["01"]`,
			},
			{
				Key:                   "testProject:foo7[0]",
				Value:                 "0123456",
				Path:                  true,
				TopLevelKey:           "testProject:foo7",
				TopLevelExpectedValue: `["0123456"]`,
			},
		},
		{
			{
				Key:                   "testProject:bar1.inner",
				Value:                 "false",
				Path:                  true,
				TopLevelKey:           "testProject:bar1",
				TopLevelExpectedValue: `{"inner":false}`,
			},
			{
				Key:                   "testProject:bar2.inner",
				Value:                 "true",
				Path:                  true,
				TopLevelKey:           "testProject:bar2",
				TopLevelExpectedValue: `{"inner":true}`,
			},
			{
				Key:                   "testProject:bar3.inner",
				Value:                 "10",
				Path:                  true,
				TopLevelKey:           "testProject:bar3",
				TopLevelExpectedValue: `{"inner":10}`,
			},
			{
				Key:                   "testProject:bar4.inner",
				Value:                 "0",
				Path:                  true,
				TopLevelKey:           "testProject:bar4",
				TopLevelExpectedValue: `{"inner":0}`,
			},
			{
				Key:                   "testProject:bar5.inner",
				Value:                 "00",
				Path:                  true,
				TopLevelKey:           "testProject:bar5",
				TopLevelExpectedValue: `{"inner":"00"}`,
			},
			{
				Key:                   "testProject:bar6.inner",
				Value:                 "01",
				Path:                  true,
				TopLevelKey:           "testProject:bar6",
				TopLevelExpectedValue: `{"inner":"01"}`,
			},
			{
				Key:                   "testProject:bar7.inner",
				Value:                 "0123456",
				Path:                  true,
				TopLevelKey:           "testProject:bar7",
				TopLevelExpectedValue: `{"inner":"0123456"}`,
			},
		},
		{
			// Syntax errors.
			{Key: "testProject:root[", Value: "v", Path: true, ExpectFailure: true},
			{Key: `testProject:root["nested]`, Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:root.array[abc]", Value: "v", Path: true, ExpectFailure: true},
			// First path segment must be a non-empty string.
			{Key: `testProject:[""]`, Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:[0]", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:.foo", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:.[0]", Value: "v", Path: true, ExpectFailure: true},
			// A "secure" key that is a map with a single string value is reserved by the system.
			{Key: "testProject:key.secure", Value: "v", Path: true, ExpectFailure: true},
			{Key: "testProject:super.nested.map.secure", Value: "v", Path: true, ExpectFailure: true},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV:               tokens.MustParseStackName("testStack"),
						FullyQualifiedNameV: "org/testProject/testStack",
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
				DefaultSecretManagerF: func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
					return &secrets.MockSecretsManager{
						TypeF: func() string { return "mock" },
						EncrypterF: func() config.Encrypter {
							return &secrets.MockEncrypter{
								EncryptValueF: func(plaintext string) string {
									return base64.StdEncoding.EncodeToString([]byte(plaintext))
								},
							}
						},
						DecrypterF: func() config.Decrypter {
							return &secrets.MockDecrypter{
								DecryptValueF: func(ciphertext string) string {
									decoded, _ := base64.StdEncoding.DecodeString(ciphertext)
									return string(decoded)
								},
							}
						},
					}, nil
				},
			}

			tmpdir := t.TempDir()
			configFile := filepath.Join(tmpdir, "Pulumi.stack.yaml")

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &project, tmpdir, nil
				},
			}

			for _, operation := range test {
				configSetCmd := &configSetCmd{
					Path:   operation.Path,
					Secret: operation.Secret,
					LoadProjectStack: func(
						_ context.Context,
						diags diag.Sink,
						project *workspace.Project,
						_ backend.Stack,
						configFile string,
					) (*workspace.ProjectStack, error) {
						data, err := os.ReadFile(configFile)
						if os.IsNotExist(err) {
							return &workspace.ProjectStack{
								Config: config.Map{},
							}, nil
						}
						require.NoError(t, err)
						return workspace.LoadProjectStackBytes(diags, project, data, "Pulumi.stack.yaml", encoding.YAML)
					},
				}

				err := configSetCmd.Run(t.Context(), ws, []string{operation.Key, operation.Value}, &project, &s, configFile)
				if operation.ExpectFailure {
					require.Error(t, err)
					continue
				}
				require.NoError(t, err)

				// Validate that the config was correct using getConfig
				key, err := config.ParseKey(operation.TopLevelKey)
				require.NoError(t, err)

				ps, err := cmdStack.LoadProjectStack(t.Context(), cmdutil.Diag(), &project, &s, configFile)
				require.NoError(t, err)

				cfg, err := ps.Config.Copy(config.NopDecrypter, config.NopEncrypter)
				require.NoError(t, err)

				v, ok, err := cfg.Get(key, false)
				require.NoError(t, err)
				require.True(t, ok, "key %s not found", operation.TopLevelKey)

				// Decrypt if needed
				var actualValue string
				if v.Secure() {
					secretsManager, err := s.DefaultSecretManager(t.Context(), ps)
					require.NoError(t, err)
					decrypter := secretsManager.Decrypter()
					actualValue, err = v.Value(decrypter)
					require.NoError(t, err)
				} else {
					var err error
					actualValue, err = v.Value(config.NopDecrypter)
					require.NoError(t, err)
				}

				require.Equal(t, operation.TopLevelExpectedValue, actualValue)
			}
		})
	}
}

func TestListConfigManagedDoesNotLoadSecretsManager(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		showSecrets bool
		jsonOut     bool
		expected    string
	}{
		{"plain", false, false, `
KEY         VALUE
env:nested  {"s":["[secret]"]}
env:secret  [secret]
env:value   envVal1
`},
		{"show secrets", true, false, `
KEY         VALUE
env:nested  {"s":["envSecret2"]}
env:secret  envSecret1
env:value   envVal1
`},
		{"json", false, true, `{
  "env:nested": {"secret": true},
  "env:secret": {"secret": true},
  "env:value": {"value": "envVal1", "secret": false}
}`},
		{"show secrets json", true, true, `{
  "env:nested": {"value": "{\"s\":[\"envSecret2\"]}", "objectValue": {"s": ["envSecret2"]}, "secret": true},
  "env:secret": {"value": "envSecret1", "secret": true},
  "env:value": {"value": "envVal1", "secret": false}
}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, be := newManagedStack(t, func(
				string, string, string, []byte,
			) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
				return &backend.ManagedEnvironmentEvaluation{Environment: &esc.Environment{Properties: map[string]esc.Value{
					"pulumiConfig": esc.NewValue(map[string]esc.Value{
						"env:value":  esc.NewValue("envVal1"),
						"env:secret": esc.NewSecret("envSecret1"),
						"env:nested": esc.NewValue(map[string]esc.Value{
							"s": esc.NewValue([]esc.Value{esc.NewSecret("envSecret2")}),
						}),
					}),
				}}}, nil, nil
			})
			be.CheckYAMLEnvironmentF = func(
				context.Context, string, []byte,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{Properties: map[string]esc.Value{
					"pulumiConfig": esc.NewValue(map[string]esc.Value{
						"env:value":  esc.NewValue("envVal1"),
						"env:secret": {Secret: true, Unknown: true},
						"env:nested": esc.NewValue(map[string]esc.Value{
							"s": esc.NewValue([]esc.Value{{Secret: true, Unknown: true}}),
						}),
					}),
				}}, nil, nil
			}

			project := workspace.Project{Name: "otherproj"}
			stackFile, ps := writeManagedListStack(t, &project)

			ssml := cmdStack.SecretsManagerLoader{}
			_, _, err := ssml.GetSecretsManager(t.Context(), s, ps)
			require.Error(t, err, "the stack secrets provider must fail if loaded")

			before, err := os.ReadFile(stackFile)
			require.NoError(t, err)

			var stdout bytes.Buffer
			err = listConfig(
				t.Context(), ssml, &stdout, &project, s, ps, c.showSecrets, c.jsonOut, c.showSecrets, stackFile)
			require.NoError(t, err)
			if c.jsonOut {
				assert.JSONEq(t, c.expected, stdout.String())
			} else {
				assert.Equal(t, strings.TrimSpace(c.expected), strings.TrimSpace(stdout.String()))
			}

			after, err := os.ReadFile(stackFile)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestGetConfigManagedDoesNotLoadSecretsManager(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		key      string
		path     bool
		jsonOut  bool
		open     bool
		expected string
	}{
		{"value", "env:value", false, false, true, "envVal1"},
		{"secret", "env:secret", false, false, true, "envSecret1"},
		{"path", "env:nested.s[0]", true, false, true, "envSecret2"},
		{
			"json", "env:nested", false, true, true,
			`{"value": "{\"s\":[\"envSecret2\"]}", "objectValue": {"s": ["envSecret2"]}, "secret": true}`,
		},
		{"not open", "env:value", false, false, false, "envVal1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, be := newManagedStack(t, func(
				string, string, string, []byte,
			) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
				return &backend.ManagedEnvironmentEvaluation{Environment: &esc.Environment{Properties: map[string]esc.Value{
					"pulumiConfig": esc.NewValue(map[string]esc.Value{
						"env:value":  esc.NewValue("envVal1"),
						"env:secret": esc.NewSecret("envSecret1"),
						"env:nested": esc.NewValue(map[string]esc.Value{
							"s": esc.NewValue([]esc.Value{esc.NewSecret("envSecret2")}),
						}),
					}),
				}}}, nil, nil
			})
			be.CheckYAMLEnvironmentF = func(
				context.Context, string, []byte,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{Properties: map[string]esc.Value{
					"pulumiConfig": esc.NewValue(map[string]esc.Value{"env:value": esc.NewValue("envVal1")}),
				}}, nil, nil
			}

			project := workspace.Project{Name: "otherproj"}
			stackFile, ps := writeManagedListStack(t, &project)

			ssml := cmdStack.SecretsManagerLoader{}
			_, _, err := ssml.GetSecretsManager(t.Context(), s, ps)
			require.Error(t, err, "the stack secrets provider must fail if loaded")

			before, err := os.ReadFile(stackFile)
			require.NoError(t, err)

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &project, filepath.Dir(stackFile), nil
				},
			}
			var stdout bytes.Buffer
			err = getConfig(t.Context(), &stdout, cmdutil.Diag(), ssml, ws, s,
				config.MustParseKey(c.key), c.path, c.jsonOut, c.open, stackFile)
			require.NoError(t, err)
			if c.jsonOut {
				assert.JSONEq(t, c.expected, stdout.String())
			} else {
				assert.Equal(t, c.expected+"\n", stdout.String())
			}

			after, err := os.ReadFile(stackFile)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestConfigManagedEnvironmentDiagnostics(t *testing.T) {
	t.Parallel()

	diagsFrom := func(summary string) apitype.EnvironmentDiagnostics {
		return apitype.EnvironmentDiagnostics{{
			Summary: summary,
			Range:   &esc.Range{Environment: "proj/dev", Begin: esc.Pos{Line: 5, Column: 20}},
		}}
	}

	cases := []struct {
		open     bool
		expected string
	}{
		{true, "opening environment:\nproj/dev:5:20: evaluate failed"},
		{false, "opening environment:\nproj/dev:5:20: check failed"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("open=%v", c.open), func(t *testing.T) {
			t.Parallel()

			s, be := newManagedStack(t, func(
				string, string, string, []byte,
			) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
				return nil, diagsFrom("evaluate failed"), nil
			})
			be.CheckYAMLEnvironmentF = func(
				context.Context, string, []byte,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return nil, diagsFrom("check failed"), nil
			}
			project := workspace.Project{Name: "otherproj"}
			stackFile, ps := writeManagedListStack(t, &project)
			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &project, filepath.Dir(stackFile), nil
				},
			}

			var listOut bytes.Buffer
			err := listConfig(t.Context(), cmdStack.SecretsManagerLoader{}, &listOut, &project, s, ps,
				c.open, false, c.open, stackFile)
			require.EqualError(t, err, c.expected)
			assert.Empty(t, listOut.String())

			var getOut bytes.Buffer
			err = getConfig(t.Context(), &getOut, cmdutil.Diag(), cmdStack.SecretsManagerLoader{}, ws, s,
				config.MustParseKey("env:value"), false, false, c.open, stackFile)
			require.EqualError(t, err, c.expected)
			assert.Empty(t, getOut.String())
		})
	}
}

func writeManagedListStack(
	t *testing.T,
	project *workspace.Project,
) (string, *workspace.ProjectStack) {
	t.Helper()

	stackYAML := `secretsprovider: bogus://unregistered
environment:
  values:
    pulumiConfig:
      env:value: envVal1
      env:secret:
        fn::secret:
          ciphertext: ZXNjeAAAAAEAAAEA
`
	stackFile := filepath.Join(t.TempDir(), "Pulumi.new.yaml")
	require.NoError(t, os.WriteFile(stackFile, []byte(stackYAML), 0o600))
	ps, err := workspace.LoadProjectStack(cmdutil.Diag(), project, stackFile)
	require.NoError(t, err)
	return stackFile, ps
}
