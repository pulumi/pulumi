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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/esc"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestListConfig(t *testing.T) {
	ctx := context.Background()

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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, false, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, false)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, false, false, false)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
		err := listConfig(ctx, secretsManagerLoader, &stdout, &project, &preparedStack, projectStack, true, false, true)
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
				NameV: tokens.MustParseStackName("testStack"),
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

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSet(t *testing.T) {
	ctx := context.Background()

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
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
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
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(diags, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			cmdStack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				cmdStack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(ctx, ws, c.args, &project, &s)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(cmdStack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSetTypes(t *testing.T) {
	ctx := context.Background()

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
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
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
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(d, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			cmdStack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				cmdStack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(ctx, ws, c.args, &project, &s)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(cmdStack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSetAll(t *testing.T) {
	ctx := context.Background()

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
			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func() (*workspace.Project, string, error) {
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
					color colors.Colorization,
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

			cmd := newConfigSetAllCmd(ws, &stackName, lm, mockEncrypterFactory)
			cmd.SetContext(ctx)

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
			data, err := os.ReadFile(stack.ConfigFile)
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
) (config.Encrypter, stack.SecretsManagerState, error) {
	return m.encrypter, stack.SecretsManagerUnchanged, nil
}
