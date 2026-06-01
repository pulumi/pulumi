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
	"encoding/base64"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func base64Crypters() (config.Encrypter, config.Decrypter) {
	enc := &secrets.MockEncrypter{
		EncryptValueF: func(plaintext string) string {
			return base64.StdEncoding.EncodeToString([]byte(plaintext))
		},
	}
	dec := &secrets.MockDecrypter{
		DecryptValueF: func(ciphertext string) string {
			decoded, _ := base64.StdEncoding.DecodeString(ciphertext)
			return string(decoded)
		},
	}
	return enc, dec
}

func localStackForEditor(remote bool) backend.MockStack {
	return backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			if remote {
				env := "testProject/testStack"
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			}
			return backend.StackConfigLocation{}
		},
	}
}

func TestNewConfigEditor(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ps := &workspace.ProjectStack{Config: config.Map{}}

	t.Run("local stack", func(t *testing.T) {
		t.Parallel()
		s := localStackForEditor(false)
		editor, err := newConfigEditor(ctx, &s, ps, config.NopEncrypter, "")
		require.NoError(t, err)
		require.IsType(t, &localConfigEditor{}, editor)
	})

	t.Run("remote stack is rejected", func(t *testing.T) {
		t.Parallel()
		s := localStackForEditor(true)
		_, err := newConfigEditor(ctx, &s, ps, config.NopEncrypter, "")
		require.Error(t, err)
	})
}

func TestLocalConfigEditor(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	enc, dec := base64Crypters()

	newEditor := func() (*localConfigEditor, *workspace.ProjectStack) {
		s := localStackForEditor(false)
		ps := &workspace.ProjectStack{Config: config.Map{}}
		return &localConfigEditor{stack: &s, ps: ps, encrypter: enc}, ps
	}

	t.Run("plaintext value", func(t *testing.T) {
		t.Parallel()
		editor, ps := newEditor()
		key := config.MustMakeKey("testProject", "key")
		require.NoError(t, editor.Set(ctx, key, config.NewValue("value"), false))

		got, ok, err := ps.Config.Get(key, false)
		require.NoError(t, err)
		require.True(t, ok)
		require.False(t, got.Secure())
		v, err := got.Value(config.NopDecrypter)
		require.NoError(t, err)
		require.Equal(t, "value", v)
	})

	t.Run("secret value is encrypted by the editor", func(t *testing.T) {
		t.Parallel()
		editor, ps := newEditor()
		key := config.MustMakeKey("testProject", "secret")
		require.NoError(t, editor.Set(ctx, key, config.NewSecureValue("plaintext"), false))

		got, ok, err := ps.Config.Get(key, false)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, got.Secure())
		require.False(t, got.Object())
		// The editor encrypted the plaintext: the stored ciphertext is base64, and decrypting
		// recovers the plaintext.
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("plaintext")), mustRawValue(t, got))
		v, err := got.Value(dec)
		require.NoError(t, err)
		require.Equal(t, "plaintext", v)
	})

	t.Run("secure object value passes through unencrypted", func(t *testing.T) {
		t.Parallel()
		// Secure object values carry per-leaf ciphertext the caller already produced. Even with a
		// real (non-Nop) encrypter, the editor must not re-encrypt the serialized object as one blob;
		// it must store it unchanged with its object and secure flags intact.
		editor, ps := newEditor()
		key := config.MustMakeKey("testProject", "obj")
		objJSON := `{"inner":{"secure":"` + base64.StdEncoding.EncodeToString([]byte("leaf")) + `"}}`
		require.NoError(t, editor.Set(ctx, key, config.NewSecureObjectValue(objJSON), false))

		got, ok, err := ps.Config.Get(key, false)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, got.Secure())
		require.True(t, got.Object())
		require.Equal(t, objJSON, mustRawValue(t, got))
	})

	t.Run("remove", func(t *testing.T) {
		t.Parallel()
		editor, ps := newEditor()
		key := config.MustMakeKey("testProject", "key")
		require.NoError(t, editor.Set(ctx, key, config.NewValue("value"), false))
		require.NoError(t, editor.Remove(ctx, key, false))

		_, ok, err := ps.Config.Get(key, false)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("remove absent key is a no-op", func(t *testing.T) {
		t.Parallel()
		editor, _ := newEditor()
		require.NoError(t, editor.Remove(ctx, config.MustMakeKey("testProject", "missing"), false))
	})
}

func mustRawValue(t *testing.T, v config.Value) string {
	t.Helper()
	raw, err := v.Value(config.NopDecrypter)
	require.NoError(t, err)
	return raw
}

// TestConfigSetAllRemoteRejected guards the IsRemote check added to set-all, which previously had
// none and failed only downstream in SaveRemoteConfig.
func TestConfigSetAllRemoteRejected(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	env := "testProject/testStack"
	s := backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
	}

	// No --config-file: the stack's remote location governs, so set-all must reject.
	configFile := ""

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, "", nil
		},
	}

	stackName := "testStack"
	mockBackend := &backend.MockBackend{
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return &s, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			_ string, _ *workspace.Project, _ bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			_ string, _ *workspace.Project, _ bool, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}

	mockEncrypterFactory := &mockEncrypterFactory{encrypter: config.NopEncrypter}
	cmd := newConfigSetAllCmd(ws, &stackName, lm, mockEncrypterFactory, &configFile)
	cmd.SetContext(ctx)
	require.NoError(t, cmd.PersistentFlags().Set("plaintext", "testProject:key=value"))

	err := cmd.RunE(cmd, []string{})
	require.ErrorContains(t, err, "config set-all not supported for remote stack config")
	require.ErrorContains(t, err, "pulumi env set testProject/testStack pulumiConfig.<key> <value>")
}
