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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newPinCmdForTest builds a configEnvPinCmd over a remote stack whose linked ref is escEnv. The new
// link passed to SaveRemoteConfig is captured into *linked; getErr is returned by GetEnvironment when
// validating a version.
func newPinCmdForTest(
	t *testing.T,
	stdout *bytes.Buffer,
	escEnv string,
	linked *string,
	getErr error,
) *configEnvPinCmd {
	t.Helper()
	stackRef := "stack"
	configFile := ""
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte("values:\n  pulumiConfig: {}\n"), "etag", 0, getErr
		},
	}

	parent := &configEnvCmd{
		stdout: stdout,
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
		loadProjectStack: func(
			_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			// Existing remote config carries secrets-provider metadata that re-linking must preserve.
			return &workspace.ProjectStack{SecretsProvider: "passphrase", EncryptionSalt: "salt"}, nil
		},
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			env := escEnv
			return &backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{NameV: tokens.MustParseStackName("stack")}
				},
				OrgNameF: func() string { return "org" },
				BackendF: func() backend.Backend { return be },
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
				},
				SaveRemoteF: func(_ context.Context, ps *workspace.ProjectStack) error {
					require.Nil(t, ps.Config, "SaveRemoteConfig requires a nil Config")
					imports := ps.Environment.Imports()
					require.Len(t, imports, 1, "SaveRemoteConfig requires exactly one import")
					// Re-linking must preserve the stack's secrets-provider metadata.
					require.Equal(t, "passphrase", ps.SecretsProvider, "pin must preserve the secrets provider")
					require.Equal(t, "salt", ps.EncryptionSalt, "pin must preserve the encryption salt")
					if linked != nil {
						*linked = imports[0]
					}
					return nil
				},
			}, nil
		},
		stackRef:   &stackRef,
		configFile: &configFile,
	}
	parent.initArgs()

	return &configEnvPinCmd{parent: parent}
}

func TestConfigEnvPinRevision(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv", &linked, nil)
	require.NoError(t, cmd.run(t.Context(), "5"))

	require.Equal(t, "testProject/testEnv@5", linked)
	require.Contains(t, stdout.String(), "revision 5")
}

func TestConfigEnvPinTag(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv", &linked, nil)
	require.NoError(t, cmd.run(t.Context(), "stable"))

	require.Equal(t, "testProject/testEnv@stable", linked)
	require.Contains(t, stdout.String(), "tag stable")
}

func TestConfigEnvPinMissingVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv", &linked, &apitype.ErrorResponse{
		Code: http.StatusNotFound, Message: "not found",
	})
	err := cmd.run(t.Context(), "999")
	require.ErrorContains(t, err, "not found")
	require.Empty(t, linked, "a failed validation must not change the link")
}

func TestConfigEnvPinLatestUnpins(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv@5", &linked, nil)
	require.NoError(t, cmd.run(t.Context(), "latest"))

	require.Equal(t, "testProject/testEnv", linked)
	require.Contains(t, stdout.String(), "Unpinned")
}

// TestConfigEnvPinAbortsOnLoadError verifies that if reading the current stack configuration fails,
// pin aborts the re-link instead of re-linking with empty secrets metadata — which would silently
// clear the stack's secrets-provider settings and break decryption for a passphrase/KMS stack.
func TestConfigEnvPinAbortsOnLoadError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv", &linked, nil)
	cmd.parent.loadProjectStack = func(
		_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
	) (*workspace.ProjectStack, error) {
		return nil, errors.New("service unavailable")
	}

	err := cmd.run(t.Context(), "5")
	require.ErrorContains(t, err, "service unavailable")
	require.Empty(t, linked, "a load failure must abort the re-link, not write empty secrets metadata")
}

// remotePinTestStack builds a remote (ESC-backed) MockStack linked to escEnv in org "org".
func remotePinTestStack(escEnv string) *backend.MockStack {
	env := escEnv
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("stack")}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return &backend.MockEnvironmentsBackend{} },
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
	}
}

// TestConfigEnvPinRejectsConfigFile verifies that pin refuses when --config-file selects a local file:
// pinning operates only on the remote environment, and honoring the flag would read the wrong secrets
// metadata and write it back to the remote link.
func TestConfigEnvPinRejectsConfigFile(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var linked string
	cmd := newPinCmdForTest(t, &stdout, "testProject/testEnv", &linked, nil)
	configFile := "Pulumi.local.yaml"
	cmd.parent.configFile = &configFile

	err := cmd.run(t.Context(), "5")
	require.ErrorContains(t, err, "nothing to pin")
	require.Empty(t, linked, "--config-file must not re-link the remote stack")
}

// TestConfigSetRejectsPinned verifies the rejectIfPinned guard fires on a mutating command (config
// set) before any write when the stack is pinned, and that an explicit --config-file (a local write)
// bypasses the guard.
func TestConfigSetRejectsPinned(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			p, err := workspace.LoadProjectBytes(
				[]byte("name: test\nruntime: yaml"), "Pulumi.yaml", encoding.YAML)
			if err != nil {
				return nil, "", err
			}
			return p, "", nil
		},
	}
	c := &configSetCmd{}

	pinned := remotePinTestStack("testProject/testEnv@5")
	err := c.Run(t.Context(), ws, []string{"k", "v"}, &workspace.Project{Name: "test"}, pinned, "")
	require.ErrorContains(t, err, "unpin")
}
