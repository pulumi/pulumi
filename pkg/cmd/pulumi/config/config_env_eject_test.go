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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ejectCapture records what the eject produced.
type ejectCapture struct {
	removeCalled  bool
	deleteCalled  bool
	deletedOrg    string
	deletedEnv    string
	removeBlocked bool
	deleteErr     error
}

// newEjectCmdForTest builds a configEnvEjectCmd over a remote (remote-config) stack backed by an
// environment whose decrypted definition is envYAML. envNotFound makes GetEnvironment return a 404.
// encrypter, if non-nil, overrides the secrets manager's encrypter (used to inject a failing encrypter).
func newEjectCmdForTest(
	t *testing.T,
	stdin io.Reader,
	stdout io.Writer,
	envYAML string,
	envNotFound bool,
	deleteNotFound bool,
	cap *ejectCapture,
	secretsManager secrets.Manager,
) *configEnvEjectCmd {
	t.Helper()
	stackRef := "stack"
	configFile := ""
	env := "test/stack"

	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, decrypt bool) ([]byte, string, int, error) {
			if envNotFound {
				return nil, "", 0, &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
			}
			require.True(t, decrypt, "eject must request a decrypted environment definition")
			return []byte(envYAML), "etag", 0, nil
		},
		DeleteEnvironmentWithProjectF: func(_ context.Context, org, proj, name string) error {
			cap.deleteCalled = true
			cap.deletedOrg = org
			cap.deletedEnv = proj + "/" + name
			if deleteNotFound {
				return &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
			}
			return cap.deleteErr
		},
	}

	if secretsManager == nil {
		secretsManager = b64.NewBase64SecretsManager()
	}

	parent := &configEnvCmd{
		stdin:       stdin,
		stdout:      stdout,
		interactive: false,
		ssml:        cmdStack.NewStackSecretsManagerLoaderFromEnv(),
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				p, err := workspace.LoadProjectBytes(
					[]byte("name: test\nruntime: yaml"), "Pulumi.yaml", encoding.YAML)
				if err != nil {
					return nil, "", err
				}
				return p, "", nil
			},
		},
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return &backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						StringV:             "org/stack",
						NameV:               tokens.MustParseStackName("stack"),
						ProjectV:            "test",
						FullyQualifiedNameV: "org/test/stack",
					}
				},
				OrgNameF: func() string { return "org" },
				BackendF: func() backend.Backend { return be },
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
				},
				DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
					return secretsManager, nil
				},
				RemoveRemoteF: func(_ context.Context) error {
					if cap.removeBlocked {
						return errors.New("link removal failed")
					}
					cap.removeCalled = true
					return nil
				},
			}, nil
		},
		stackRef:   &stackRef,
		configFile: &configFile,
	}

	return &configEnvEjectCmd{parent: parent, yes: true, secretsProvider: "default"}
}

// setupEjectProject chdirs into a temp dir holding Pulumi.yaml so DetectProjectStackPath resolves the
// destination Pulumi.stack.yaml. It returns the destination path (the file does not exist yet).
func setupEjectProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0o600))
	t.Chdir(dir)
	return filepath.Join(dir, "Pulumi.stack.yaml")
}

// loadEjectedConfig loads the written local stack file and returns its parsed ProjectStack.
func loadEjectedConfig(t *testing.T, path string) *workspace.ProjectStack {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	p, err := workspace.LoadProjectBytes([]byte("name: test\nruntime: yaml"), "Pulumi.yaml", encoding.YAML)
	require.NoError(t, err)
	ps, err := workspace.LoadProjectStackBytes(
		diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
		p, b, "Pulumi.stack.yaml", encoding.YAML)
	require.NoError(t, err)
	return ps
}

// TestConfigEnvEjectRoundTripsValues verifies plaintext, scalar-secret, object, and array values
// round-trip into the local file: plaintext stays plaintext and a top-level secret becomes a local
// secure value that decrypts back to the original plaintext.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectRoundTripsValues(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  pulumiConfig:
    test:plain: hello
    test:flag: true
    test:obj:
      env: prod
    test:list:
      - a
      - b
    test:password:
      fn::secret: hunter2
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "hunter2", "plaintext secret must not be written to disk")

	ps := loadEjectedConfig(t, path)
	m, err := ps.Config.AsDecryptedPropertyMap(t.Context(), config.Base64Crypter)
	require.NoError(t, err)

	assert.Equal(t, "hello", m.Get("test:plain").AsString())
	assert.True(t, m.Get("test:flag").AsBool())
	assert.Equal(t, "prod", m.Get("test:obj").AsMap().Get("env").AsString())
	list := m.Get("test:list").AsArray().AsSlice()
	require.Len(t, list, 2)
	assert.Equal(t, "a", list[0].AsString())

	pw := m.Get("test:password")
	assert.True(t, pw.Secret(), "top-level secret must round-trip as a secret")
	assert.Equal(t, "hunter2", pw.AsString())
}

// TestConfigEnvEjectNestedSecrets verifies a secret nested inside an object and a secret nested inside
// an array each become an encrypted local secure value: not plaintext, and decrypting once yields the
// original plaintext (not double-encrypted).
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectNestedSecrets(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  pulumiConfig:
    test:obj:
      inner:
        fn::secret: nestedObjSecret
    test:list:
      - plainItem
      - fn::secret: nestedArrSecret
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "nestedObjSecret", "nested object secret must not be plaintext on disk")
	assert.NotContains(t, string(raw), "nestedArrSecret", "nested array secret must not be plaintext on disk")

	ps := loadEjectedConfig(t, path)
	m, err := ps.Config.AsDecryptedPropertyMap(t.Context(), config.Base64Crypter)
	require.NoError(t, err)

	inner := m.Get("test:obj").AsMap().Get("inner")
	require.True(t, inner.Secret(), "nested object value must be a secret")
	assert.Equal(t, "nestedObjSecret", inner.AsString())

	arr := m.Get("test:list").AsArray().AsSlice()
	require.Len(t, arr, 2)
	assert.Equal(t, "plainItem", arr[0].AsString())
	require.True(t, arr[1].Secret(), "nested array value must be a secret")
	assert.Equal(t, "nestedArrSecret", arr[1].AsString())
}

// failingEncrypter errors on any encryption attempt.
type failingEncrypter struct{}

func (failingEncrypter) EncryptValue(_ context.Context, _ string) (string, error) {
	return "", errors.New("encryption boom")
}

func (failingEncrypter) BatchEncrypt(_ context.Context, _ []string) ([]string, error) {
	return nil, errors.New("encryption boom")
}

// failingSecretsManager hands out a failingEncrypter.
type failingSecretsManager struct{}

func (failingSecretsManager) Type() string                { return "failing" }
func (failingSecretsManager) State() json.RawMessage      { return nil }
func (failingSecretsManager) Encrypter() config.Encrypter { return failingEncrypter{} }
func (failingSecretsManager) Decrypter() config.Decrypter { return config.NopDecrypter }

// TestConfigEnvEjectWriteBeforeUnlinkAndNoPlaintextOnFailure verifies the file is written before
// unlink and that a re-encryption failure leaves no file on disk and does not unlink the stack.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectWriteBeforeUnlinkAndNoPlaintextOnFailure(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  pulumiConfig:
    test:password:
      fn::secret: hunter2
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, failingSecretsManager{})
	cmd.secretsProvider = "default"

	err := cmd.run(t.Context())
	require.Error(t, err)
	assert.NoFileExists(t, path, "no file (and no plaintext) must remain after a re-encryption failure")
	assert.False(t, cap.removeCalled, "stack must not be unlinked when the local write fails")
	assert.False(t, cap.deleteCalled, "environment must not be deleted when the local write fails")
}

// TestConfigEnvEjectPreservesImports verifies the env's imports become the local stack's environment
// imports.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectPreservesImports(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `imports:
  - shared/base
  - shared/db
values:
  pulumiConfig:
    test:k: v
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	ps := loadEjectedConfig(t, path)
	assert.Equal(t, []string{"shared/base", "shared/db"}, ps.Environment.Imports())
}

// TestConfigEnvEjectKeepEnv verifies --keep-env skips the delete but still unlinks.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectKeepEnv(t *testing.T) {
	setupEjectProject(t)
	envYAML := "values:\n  pulumiConfig:\n    test:k: v\n"
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	cmd.keepEnv = true
	require.NoError(t, cmd.run(t.Context()))

	assert.True(t, cap.removeCalled)
	assert.False(t, cap.deleteCalled, "--keep-env must not delete the environment")
}

// TestConfigEnvEjectDeletesByDefault verifies the default path (with --yes) deletes the environment.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectDeletesByDefault(t *testing.T) {
	setupEjectProject(t)
	envYAML := "values:\n  pulumiConfig:\n    test:k: v\n"
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	assert.True(t, cap.removeCalled)
	assert.True(t, cap.deleteCalled)
	assert.Equal(t, "test/stack", cap.deletedEnv)
}

// TestConfigEnvEjectAlreadyDeletedEnv verifies a missing environment (GetEnvironment 404) still
// completes the unlink and reports the env deleted.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectAlreadyDeletedEnv(t *testing.T) {
	setupEjectProject(t)
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, "", true /*envNotFound*/, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	assert.True(t, cap.removeCalled, "unlink must still happen when the env is already gone")
	assert.False(t, cap.deleteCalled, "no delete call is needed for an already-missing env")
}

// TestConfigEnvEjectDeleteNotFound verifies a delete that returns 404 is treated as success.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectDeleteNotFound(t *testing.T) {
	setupEjectProject(t)
	envYAML := "values:\n  pulumiConfig:\n    test:k: v\n"
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, true /*deleteNotFound*/, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	assert.True(t, cap.removeCalled)
	assert.Contains(t, stdout.String(), "Deleted environment")
}

// TestConfigEnvEjectSecretsDefaultProvider verifies non-interactive eject with secrets present and no
// --secrets-provider defaults to the "default" provider (matching stack init/new) rather than failing:
// the secret is re-encrypted locally and the stack is unlinked.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectSecretsDefaultProvider(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  pulumiConfig:
    test:password:
      fn::secret: hunter2
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	cmd.secretsProvider = "" // no explicit provider in non-interactive mode

	require.NoError(t, cmd.run(t.Context()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "hunter2", "secret must be re-encrypted, not plaintext on disk")

	ps := loadEjectedConfig(t, path)
	m, err := ps.Config.AsDecryptedPropertyMap(t.Context(), config.Base64Crypter)
	require.NoError(t, err)
	pw := m.Get("test:password")
	require.True(t, pw.Secret(), "the password must remain a secret")
	assert.Equal(t, "hunter2", pw.AsString())

	assert.True(t, cap.removeCalled, "the stack should be unlinked after a successful eject")
}

// TestConfigEnvEjectNullValue verifies a null config value round-trips as null rather than the literal
// string "<nil>" the default formatting would otherwise produce.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectNullValue(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := "values:\n  pulumiConfig:\n    test:empty: null\n"
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)

	require.NoError(t, cmd.run(t.Context()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "<nil>", "a null value must not serialize as the string <nil>")
}

// TestConfigEnvEjectRejectsNonRemoteStack verifies a non-remote (local-config) stack is rejected.
func TestConfigEnvEjectRejectsNonRemoteStack(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, "", false, false, &cap, nil)
	cmd.parent.requireStack = func(
		_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
		_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
	) (backend.Stack, error) {
		return &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("stack")}
			},
			OrgNameF:        func() string { return "org" },
			BackendF:        func() backend.Backend { return &backend.MockEnvironmentsBackend{} },
			ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		}, nil
	}

	err := cmd.run(t.Context())
	require.ErrorContains(t, err, "does not use remote configuration")
	assert.False(t, cap.removeCalled)
	assert.False(t, cap.deleteCalled)
}

// TestConfigEnvEjectCompositeSecret verifies that a composite fn::secret (one wrapping an object or
// array, as migration produces for a secret object) round-trips as a secure JSON value rather than
// Go's default formatting.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectCompositeSecret(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  pulumiConfig:
    test:obj:
      fn::secret:
        a: 1
        b: two
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "two", "composite secret must not be plaintext on disk")

	ps := loadEjectedConfig(t, path)
	m, err := ps.Config.AsDecryptedPropertyMap(t.Context(), config.Base64Crypter)
	require.NoError(t, err)
	v := m.Get("test:obj")
	require.True(t, v.Secret(), "composite secret must be a secret value")
	// The decrypted value is the JSON encoding of the object (valid JSON, not Go map formatting).
	assert.JSONEq(t, `{"a":1,"b":"two"}`, v.AsString())
}

// TestConfigEnvEjectStructuredImportWarns verifies that a structured import (with merge options) is
// preserved by name with a warning that its options are not carried into the local stack file.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectStructuredImportWarns(t *testing.T) {
	setupEjectProject(t)
	envYAML := `imports:
  - plain-env
  - structured-env:
      merge: false
values:
  pulumiConfig:
    test:k: v
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	require.NoError(t, cmd.run(t.Context()))
	assert.Contains(t, stdout.String(), "structured-env")
	assert.Contains(t, stdout.String(), "not preserved")
}

// TestConfigEnvEjectRefusesNonConfigValues verifies eject refuses, before any write/unlink/delete, when
// the backing environment carries values other than pulumiConfig (here environmentVariables) that a
// local stack file cannot hold — otherwise ejecting would drop them and the default delete would destroy
// the only copy.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectRefusesNonConfigValues(t *testing.T) {
	path := setupEjectProject(t)
	envYAML := `values:
  environmentVariables:
    AWS_REGION: us-west-2
  pulumiConfig:
    test:k: v
`
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)

	err := cmd.run(t.Context())
	require.ErrorContains(t, err, "environmentVariables")
	assert.NoFileExists(t, path, "eject must not write a local file when it refuses")
	assert.False(t, cap.removeCalled, "stack must not be unlinked when eject refuses")
	assert.False(t, cap.deleteCalled, "environment must not be deleted when eject refuses")
}

// TestConfigEnvEjectNonInteractiveRequiresYes verifies a non-interactive eject without --yes is rejected
// before any mutation rather than silently writing, unlinking, and deleting the backing environment.
func TestConfigEnvEjectNonInteractiveRequiresYes(t *testing.T) {
	t.Parallel()
	envYAML := "values:\n  pulumiConfig:\n    test:k: v\n"
	var stdout bytes.Buffer
	var cap ejectCapture
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)
	cmd.yes = false // parent.interactive is already false, so this is a non-TTY run with no --yes

	err := cmd.run(t.Context())
	require.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.False(t, cap.removeCalled)
	assert.False(t, cap.deleteCalled)
}

// TestConfigEnvEjectDeleteFailureWarns verifies that a failure to delete the environment after the
// stack is unlinked is reported as a warning, not a fatal error: the eject succeeded.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvEjectDeleteFailureWarns(t *testing.T) {
	setupEjectProject(t)
	envYAML := "values:\n  pulumiConfig:\n    test:k: v\n"
	var stdout bytes.Buffer
	cap := ejectCapture{deleteErr: errors.New("boom")}
	cmd := newEjectCmdForTest(t, nil, &stdout, envYAML, false, false, &cap, nil)

	require.NoError(t, cmd.run(t.Context()), "delete failure after unlink must not fail the eject")
	assert.True(t, cap.removeCalled, "stack must have been unlinked")
	assert.Contains(t, stdout.String(), "Warning: could not delete environment")
}
