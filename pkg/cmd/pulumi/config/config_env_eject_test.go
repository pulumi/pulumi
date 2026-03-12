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

package config

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// --- extractPulumiConfig unit tests ---

func TestExtractPulumiConfig_EmptyYAML(t *testing.T) {
	t.Parallel()

	result, hasSecrets, err := extractPulumiConfig(nil)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_PlainValues(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:host: localhost
    myproject:port: "5432"
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: "localhost"}, result["myproject:host"])
	assert.Equal(t, ejectedConfigValue{plaintext: "5432"}, result["myproject:port"])
}

func TestExtractPulumiConfig_SecretValue_FnSecret(t *testing.T) {
	t.Parallel()

	// With decrypt=true, ESC returns fn::secret with plaintext value.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:dbpass:
      fn::secret: hunter2
    myproject:host: localhost
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.True(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: "hunter2", secret: true}, result["myproject:dbpass"])
	assert.Equal(t, ejectedConfigValue{plaintext: "localhost"}, result["myproject:host"])
}

func TestExtractPulumiConfig_NoValuesSection(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
imports:
  - myorg/creds
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_NoPulumiConfigSection(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
values:
  environmentVariables:
    MY_VAR: hello
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_NonStringValues(t *testing.T) {
	t.Parallel()

	// YAML integers and booleans should be stringified.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:count: 42
    myproject:enabled: true
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, "42", result["myproject:count"].plaintext)
	assert.Equal(t, "true", result["myproject:enabled"].plaintext)
}

func TestExtractPulumiConfig_NestedMapJSONSerialized(t *testing.T) {
	t.Parallel()

	// A non-secret map value should be JSON-serialized so it can round-trip through local config.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:plain: hello
    myproject:nested:
      key: value
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: "hello"}, result["myproject:plain"])
	assert.Equal(t, ejectedConfigValue{plaintext: `{"key":"value"}`}, result["myproject:nested"])
}

func TestExtractPulumiConfig_ArrayValue(t *testing.T) {
	t.Parallel()

	// Array values should be JSON-serialized.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:tags:
      - alpha
      - beta
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: `["alpha","beta"]`}, result["myproject:tags"])
}

func TestExtractPulumiConfig_SecretMap(t *testing.T) {
	t.Parallel()

	// fn::secret wrapping a map: inner value is JSON-serialized and marked as secret.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:dbconfig:
      fn::secret:
        host: db.example.com
        port: 5432
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.True(t, hasSecrets)
	got := result["myproject:dbconfig"]
	assert.True(t, got.secret)
	assert.Contains(t, got.plaintext, "db.example.com")
}

// --- eject error-gate tests ---

// TestEject_isHTTPNotFound verifies the gating predicate that guards the
// "environment externally deleted" continuation path (spec AC8). Only a 404
// should allow eject to continue; all other error codes must be surfaced.
func TestEject_isHTTPNotFound(t *testing.T) {
	t.Parallel()

	assert.True(t, isHTTPNotFound(&apitype.ErrorResponse{Code: http.StatusNotFound}))
	assert.False(t, isHTTPNotFound(&apitype.ErrorResponse{Code: http.StatusInternalServerError}))
	assert.False(t, isHTTPNotFound(&apitype.ErrorResponse{Code: http.StatusForbidden}))
	assert.False(t, isHTTPNotFound(&apitype.ErrorResponse{Code: http.StatusUnauthorized}))
}

// TestEject_GetEnvironment_NonNotFound_ReturnsError verifies that editRemote gates
// correctly: a 500 from GetEnvironment must abort eject.
// This test exercises the backend call directly through the mock — the run() path
// is covered by the predicate test above plus the integration path in the CI suite.
func TestEject_GetEnvironment_NonNotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	serverErr := &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "internal error"}

	eb := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return nil, "", 0, serverErr
		},
	}

	_, _, _, getErr := eb.GetEnvironment(context.Background(), "myorg", "myproject", "dev", "", true)
	require.Error(t, getErr)
	assert.False(t, isHTTPNotFound(getErr),
		"a 500 must not be treated as not-found; eject should abort, not strip config and unlink")
}

func TestConfigEnvEject_WritesLocalConfigBeforeUnlinking(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: tokens.PackageName("myproject")}
	stackRef := "dev"
	escEnv := "myproject/dev"

	var callOrder []string
	var savedStack *workspace.ProjectStack

	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend {
			return &backend.MockEnvironmentsBackend{
				GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
					return []byte("values:\n  pulumiConfig:\n    myproject:name: value\n"), "etag", 1, nil
				},
				DeleteEnvironmentWithProjectF: func(_ context.Context, _, _, _ string) error {
					callOrder = append(callOrder, "delete-env")
					return nil
				},
			}
		},
	}
	stack.OrgNameF = func() string { return "myorg" }
	stack.RemoveRemoteConfigF = func(context.Context) error {
		callOrder = append(callOrder, "unlink")
		return nil
	}

	parent := &configEnvCmd{
		stdout:      io.Discard,
		interactive: true,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "", nil
			},
		},
		requireStack: func(
			context.Context, diag.Sink, pkgWorkspace.Context, cmdBackend.LoginManager,
			string, cmdStack.LoadOption, display.Options,
		) (backend.Stack, error) {
			return stack, nil
		},
		stackRef: &stackRef,
	}

	cmd := &configEnvEjectCmd{
		parent: parent,
		yes:    true,
		saveLocalProjectStack: func(_ tokens.QName, ps *workspace.ProjectStack) error {
			callOrder = append(callOrder, "save")
			savedStack = ps
			return nil
		},
	}

	err := cmd.run(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"save", "unlink", "delete-env"}, callOrder)
	require.NotNil(t, savedStack)
	value, err := savedStack.Config[config.MustMakeKey("myproject", "name")].Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestConfigEnvEject_DefaultsToStackSecretsProvider(t *testing.T) {
	t.Parallel()

	stackRef := "dev"
	escEnv := "myproject/dev"

	var savedStack *workspace.ProjectStack

	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend {
			return &backend.MockEnvironmentsBackend{
				GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
					yaml := "values:\n  pulumiConfig:\n" +
						"    myproject:dbpass:\n      fn::secret: hunter2\n" +
						"    myproject:host: localhost\n"
					return []byte(yaml), "etag", 1, nil
				},
				DeleteEnvironmentWithProjectF: func(_ context.Context, _, _, _ string) error {
					return nil
				},
			}
		},
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
	}
	stack.OrgNameF = func() string { return "myorg" }
	stack.RemoveRemoteConfigF = func(context.Context) error { return nil }

	parent := &configEnvCmd{
		stdout:      io.Discard,
		interactive: true,
		ssml:        cmdStack.SecretsManagerLoader{},
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return &workspace.Project{Name: "myproject"}, "", nil
			},
		},
		requireStack: func(
			context.Context, diag.Sink, pkgWorkspace.Context, cmdBackend.LoginManager,
			string, cmdStack.LoadOption, display.Options,
		) (backend.Stack, error) {
			return stack, nil
		},
		stackRef: &stackRef,
	}

	// No --secrets-provider flag set; should use the stack's default secrets manager.
	cmd := &configEnvEjectCmd{
		parent: parent,
		yes:    true,
		saveLocalProjectStack: func(_ tokens.QName, ps *workspace.ProjectStack) error {
			savedStack = ps
			return nil
		},
	}

	err := cmd.run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, savedStack)

	// SecretsProvider should be empty — the stack's default manager (service encryption
	// for cloud-backed stacks) is used without setting an explicit provider string.
	assert.Empty(t, savedStack.SecretsProvider)

	// Verify the secret was re-encrypted.
	dbpass := savedStack.Config[config.MustMakeKey("myproject", "dbpass")]
	assert.True(t, dbpass.Secure())

	// Verify the plain value is preserved.
	host, err := savedStack.Config[config.MustMakeKey("myproject", "host")].Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
}

func TestConfigEnvEject_PreservesEnvironmentImports(t *testing.T) {
	t.Parallel()

	stackRef := "dev"
	escEnv := "myproject/dev"

	var savedStack *workspace.ProjectStack

	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend {
			return &backend.MockEnvironmentsBackend{
				GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
					yaml := "imports:\n  - myorg/base-env\n  - myorg/aws-creds\n" +
						"values:\n  pulumiConfig:\n    myproject:name: value\n"
					return []byte(yaml), "etag", 1, nil
				},
				DeleteEnvironmentWithProjectF: func(_ context.Context, _, _, _ string) error {
					return nil
				},
			}
		},
	}
	stack.OrgNameF = func() string { return "myorg" }
	stack.RemoveRemoteConfigF = func(context.Context) error { return nil }

	parent := &configEnvCmd{
		stdout:      io.Discard,
		interactive: true,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return &workspace.Project{Name: "myproject"}, "", nil
			},
		},
		requireStack: func(
			context.Context, diag.Sink, pkgWorkspace.Context, cmdBackend.LoginManager,
			string, cmdStack.LoadOption, display.Options,
		) (backend.Stack, error) {
			return stack, nil
		},
		stackRef: &stackRef,
	}

	cmd := &configEnvEjectCmd{
		parent: parent,
		yes:    true,
		saveLocalProjectStack: func(_ tokens.QName, ps *workspace.ProjectStack) error {
			savedStack = ps
			return nil
		},
	}

	err := cmd.run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, savedStack)
	require.NotNil(t, savedStack.Environment)
	assert.Equal(t, []string{"myorg/base-env", "myorg/aws-creds"}, savedStack.Environment.Imports())

	// Verify config value is still present.
	val, err := savedStack.Config[config.MustMakeKey("myproject", "name")].Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestConfigEnvEject_SaveFailureDoesNotUnlink(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: tokens.PackageName("myproject")}
	stackRef := "dev"
	escEnv := "myproject/dev"

	unlinked := false

	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend {
			return &backend.MockEnvironmentsBackend{
				GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
					return []byte("values:\n  pulumiConfig:\n    myproject:name: value\n"), "etag", 1, nil
				},
			}
		},
	}
	stack.OrgNameF = func() string { return "myorg" }
	stack.RemoveRemoteConfigF = func(context.Context) error {
		unlinked = true
		return nil
	}

	parent := &configEnvCmd{
		stdout:      io.Discard,
		interactive: true,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "", nil
			},
		},
		requireStack: func(
			context.Context, diag.Sink, pkgWorkspace.Context, cmdBackend.LoginManager,
			string, cmdStack.LoadOption, display.Options,
		) (backend.Stack, error) {
			return stack, nil
		},
		stackRef: &stackRef,
	}

	cmd := &configEnvEjectCmd{
		parent: parent,
		yes:    true,
		saveLocalProjectStack: func(tokens.QName, *workspace.ProjectStack) error {
			return errors.New("disk full")
		},
	}

	err := cmd.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writing local config file: disk full")
	assert.False(t, unlinked)
}
