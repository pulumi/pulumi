// Copyright 2016-2026, Pulumi Corporation.
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
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakeEncrypter wraps a simple prefix so tests can verify ciphertext without real crypto.
type fakeEncrypter struct{ prefix string }

func (f *fakeEncrypter) EncryptValue(_ context.Context, plaintext string) (string, error) {
	return f.prefix + plaintext, nil
}

func (f *fakeEncrypter) BatchEncrypt(_ context.Context, secrets []string) ([]string, error) {
	out := make([]string, len(secrets))
	for i, s := range secrets {
		out[i] = f.prefix + s
	}
	return out, nil
}

// newTestEditor creates a LocalConfigEditor backed by an in-memory ProjectStack.
// The stack is configured so that Save() writes to a temp file via cmdStack.ConfigFile.
func newTestEditor(t *testing.T, ps *workspace.ProjectStack, encrypter config.Encrypter) *LocalConfigEditor {
	t.Helper()
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV: tokens.MustParseStackName("testStack"),
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{} // local
		},
	}
	return &LocalConfigEditor{stack: s, ps: ps, encrypter: encrypter}
}

// newEmptyProjectStack returns an empty ProjectStack.
func newEmptyProjectStack() *workspace.ProjectStack {
	return &workspace.ProjectStack{Config: config.Map{}}
}

func TestLocalConfigEditor_SetPlainValue(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	key := config.MustMakeKey("myproject", "host")
	v := config.NewValue("localhost")
	require.NoError(t, editor.Set(context.Background(), key, v, false))

	got, ok, err := ps.Config.Get(key, false)
	require.NoError(t, err)
	require.True(t, ok)
	raw, err := got.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "localhost", raw)
	assert.False(t, got.Secure())
}

func TestLocalConfigEditor_SetSecretValue_EncryptsEagerly(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	enc := &fakeEncrypter{prefix: "enc:"}
	editor := newTestEditor(t, ps, enc)

	key := config.MustMakeKey("myproject", "password")
	// Handler passes plaintext with secure=true; editor must encrypt in Set().
	v := config.NewSecureValue("hunter2")
	require.NoError(t, editor.Set(context.Background(), key, v, false))

	got, ok, err := ps.Config.Get(key, false)
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, got.Secure(), "value should be marked as secret")

	// The stored value should be the ciphertext produced by fakeEncrypter.
	raw, err := got.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "enc:hunter2", raw, "ciphertext should be enc:<plaintext>")
}

func TestLocalConfigEditor_SetPath(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	// --path: key name contains the property path
	key := config.MustMakeKey("myproject", "db.host")
	v := config.NewValue("localhost")
	require.NoError(t, editor.Set(context.Background(), key, v, true /*path*/))

	// The parent key "db" should exist as an object.
	parentKey := config.MustMakeKey("myproject", "db")
	got, ok, err := ps.Config.Get(parentKey, false)
	require.NoError(t, err)
	require.True(t, ok, "parent key should exist")
	assert.True(t, got.Object(), "parent value should be an object")
}

func TestLocalConfigEditor_RemoveExistingKey(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, editor.Set(context.Background(), key, config.NewValue("localhost"), false))

	require.NoError(t, editor.Remove(context.Background(), key, false))

	_, ok, err := ps.Config.Get(key, false)
	require.NoError(t, err)
	assert.False(t, ok, "key should have been removed")
}

func TestLocalConfigEditor_RemoveNonexistentKey_IsNoOp(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	key := config.MustMakeKey("myproject", "missing")
	err := editor.Remove(context.Background(), key, false)
	assert.NoError(t, err, "removing a non-existent key should be a no-op")
}

func TestLocalConfigEditor_SetAll_Batch(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	enc := &fakeEncrypter{prefix: "enc:"}
	editor := newTestEditor(t, ps, enc)

	ctx := context.Background()
	pairs := []struct {
		key    config.Key
		val    config.Value
		secret bool
	}{
		{config.MustMakeKey("myproject", "host"), config.NewValue("localhost"), false},
		{config.MustMakeKey("myproject", "port"), config.NewValue("5432"), false},
		{config.MustMakeKey("myproject", "password"), config.NewSecureValue("s3cr3t"), true},
	}

	for _, p := range pairs {
		require.NoError(t, editor.Set(ctx, p.key, p.val, false),
			"Set(%s) failed", p.key)
	}

	// Verify plaintext values.
	for _, p := range pairs {
		if p.secret {
			continue
		}
		got, ok, err := ps.Config.Get(p.key, false)
		require.NoError(t, err)
		require.True(t, ok)
		raw, err := got.Value(config.NopDecrypter)
		require.NoError(t, err)
		expected, _ := p.val.Value(config.NopDecrypter)
		assert.Equal(t, expected, raw, "value mismatch for %s", p.key)
	}

	// Verify the secret value was encrypted.
	passwordKey := config.MustMakeKey("myproject", "password")
	got, ok, err := ps.Config.Get(passwordKey, false)
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, got.Secure())
	raw, err := got.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "enc:s3cr3t", raw)
}

func TestLocalConfigEditor_RemoveAll_Batch(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	ctx := context.Background()
	keys := []config.Key{
		config.MustMakeKey("myproject", "a"),
		config.MustMakeKey("myproject", "b"),
		config.MustMakeKey("myproject", "c"),
	}
	for _, k := range keys {
		require.NoError(t, editor.Set(ctx, k, config.NewValue(k.Name()), false))
	}

	for _, k := range keys {
		require.NoError(t, editor.Remove(ctx, k, false))
	}

	for _, k := range keys {
		_, ok, err := ps.Config.Get(k, false)
		require.NoError(t, err)
		assert.False(t, ok, "key %s should have been removed", k)
	}
}

//nolint:paralleltest // changes global ConfigFile variable
func TestLocalConfigEditor_Save_WritesToFile(t *testing.T) {
	ctx := context.Background()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, editor.Set(ctx, key, config.NewValue("localhost"), false))

	tmpDir := t.TempDir()
	cmdStack.ConfigFile = filepath.Join(tmpDir, "Pulumi.testStack.yaml")
	defer func() { cmdStack.ConfigFile = "" }()

	require.NoError(t, editor.Save(ctx))

	// Read back and verify the key is present.
	loaded, err := workspace.LoadProjectStack(nil, &workspace.Project{Name: "myproject"}, cmdStack.ConfigFile)
	require.NoError(t, err)
	got, ok, err := loaded.Config.Get(key, false)
	require.NoError(t, err)
	require.True(t, ok)
	raw, err := got.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "localhost", raw)
}

func TestLocalConfigEditor_SetOverwritesPrevious(t *testing.T) {
	t.Parallel()

	ps := newEmptyProjectStack()
	editor := newTestEditor(t, ps, config.NopEncrypter)
	ctx := context.Background()

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, editor.Set(ctx, key, config.NewValue("first"), false))
	require.NoError(t, editor.Set(ctx, key, config.NewValue("second"), false))

	got, ok, err := ps.Config.Get(key, false)
	require.NoError(t, err)
	require.True(t, ok)
	raw, err := got.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "second", raw, "second Set should win")
}

func TestNewConfigEditor_ReturnsLocalEditorForLocalStack(t *testing.T) {
	t.Parallel()

	s := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{} // local
		},
	}
	ps := newEmptyProjectStack()

	editor, err := NewConfigEditor(context.Background(), s, ps, config.NopEncrypter)
	require.NoError(t, err)
	_, ok := editor.(*LocalConfigEditor)
	assert.True(t, ok, fmt.Sprintf("expected *LocalConfigEditor, got %T", editor))
}

// --- escConfigEditor helpers ---

// newESCEditor builds an escConfigEditor with a pre-loaded envDef map (already unmarshalled YAML).
// Tests that need to call Save can supply capturedYAML to inspect the serialised YAML.
func newESCEditor(
	t *testing.T,
	envDef map[string]any,
	etag string,
	capturedYAML *[]byte,
	saveErr error,
) *escConfigEditor {
	t.Helper()
	eb := &backend.MockEnvironmentsBackend{
		UpdateEnvironmentWithProjectF: func(
			_ context.Context,
			_, _, _ string,
			yamlBytes []byte,
			_ string,
		) (apitype.EnvironmentDiagnostics, error) {
			if capturedYAML != nil {
				*capturedYAML = yamlBytes
			}
			return nil, saveErr
		},
	}
	return &escConfigEditor{
		envBackend: eb,
		orgName:    "myorg",
		envProject: "myproject",
		envName:    "dev",
		envDef:     envDef,
		etag:       etag,
	}
}

// parsePulumiConfig unmarshals YAML and returns the values.pulumiConfig map.
func parsePulumiConfig(t *testing.T, yamlBytes []byte) map[string]any {
	t.Helper()
	var root map[string]any
	require.NoError(t, yaml.Unmarshal(yamlBytes, &root))
	values, ok := root["values"].(map[string]any)
	require.True(t, ok, "expected values key in YAML")
	pc, ok := values["pulumiConfig"].(map[string]any)
	require.True(t, ok, "expected pulumiConfig key in YAML")
	return pc
}

// --- escConfigEditor tests ---

func TestESCConfigEditor_SetPlainValue(t *testing.T) {
	t.Parallel()

	var captured []byte
	e := newESCEditor(t, nil, "etag1", &captured, nil)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, e.Set(context.Background(), key, config.NewValue("localhost"), false))
	require.NoError(t, e.Save(context.Background()))

	pc := parsePulumiConfig(t, captured)
	assert.Equal(t, "localhost", pc["myproject:host"])
}

func TestESCConfigEditor_SetSecretValue_WrapsFnSecret(t *testing.T) {
	t.Parallel()

	var captured []byte
	e := newESCEditor(t, nil, "etag1", &captured, nil)

	key := config.MustMakeKey("myproject", "password")
	require.NoError(t, e.Set(context.Background(), key, config.NewSecureValue("hunter2"), false))
	require.NoError(t, e.Save(context.Background()))

	pc := parsePulumiConfig(t, captured)
	secretNode, ok := pc["myproject:password"].(map[string]any)
	require.True(t, ok, "expected secret to be wrapped in a map")
	assert.Equal(t, "hunter2", secretNode["fn::secret"])
}

func TestESCConfigEditor_SetPath_NestedNavigation(t *testing.T) {
	t.Parallel()

	var captured []byte
	e := newESCEditor(t, nil, "etag1", &captured, nil)

	key := config.MustMakeKey("myproject", "db.host")
	require.NoError(t, e.Set(context.Background(), key, config.NewValue("localhost"), true /*path*/))
	require.NoError(t, e.Save(context.Background()))

	pc := parsePulumiConfig(t, captured)
	nested, ok := pc["myproject:db"].(map[string]any)
	require.True(t, ok, "expected myproject:db to be a map")
	assert.Equal(t, "localhost", nested["host"])
}

func TestESCConfigEditor_RemoveExistingKey(t *testing.T) {
	t.Parallel()

	initial := map[string]any{
		"values": map[string]any{
			"pulumiConfig": map[string]any{
				"myproject:host": "localhost",
			},
		},
	}
	var captured []byte
	e := newESCEditor(t, initial, "etag1", &captured, nil)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, e.Remove(context.Background(), key, false))
	require.NoError(t, e.Save(context.Background()))

	pc := parsePulumiConfig(t, captured)
	_, exists := pc["myproject:host"]
	assert.False(t, exists, "key should have been removed")
}

func TestESCConfigEditor_RemoveNonexistentKey_IsNoOp(t *testing.T) {
	t.Parallel()

	e := newESCEditor(t, nil, "etag1", nil, nil)
	key := config.MustMakeKey("myproject", "missing")
	assert.NoError(t, e.Remove(context.Background(), key, false))
}

func TestESCConfigEditor_Save_EtagConflict_ReturnsError(t *testing.T) {
	t.Parallel()

	conflictErr := &apitype.ErrorResponse{Code: http.StatusConflict, Message: "conflict"}
	e := newESCEditor(t, nil, "stale-etag", nil, conflictErr)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, e.Set(context.Background(), key, config.NewValue("localhost"), false))

	err := e.Save(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified concurrently")
}

func TestESCConfigEditor_PreservesExistingEnvSections(t *testing.T) {
	t.Parallel()

	// Existing env has imports and environmentVariables — we must not lose them.
	initial := map[string]any{
		"imports": []any{"myorg/shared/creds"},
		"values": map[string]any{
			"environmentVariables": map[string]any{
				"AWS_REGION": "us-east-1",
			},
			"pulumiConfig": map[string]any{},
		},
	}
	var captured []byte
	e := newESCEditor(t, initial, "etag1", &captured, nil)

	key := config.MustMakeKey("myproject", "host")
	require.NoError(t, e.Set(context.Background(), key, config.NewValue("localhost"), false))
	require.NoError(t, e.Save(context.Background()))

	var root map[string]any
	require.NoError(t, yaml.Unmarshal(captured, &root))

	// imports should still be present
	_, hasImports := root["imports"]
	assert.True(t, hasImports, "imports section should be preserved")

	// environmentVariables should still be present
	values := root["values"].(map[string]any)
	_, hasEnvVars := values["environmentVariables"]
	assert.True(t, hasEnvVars, "environmentVariables should be preserved")
}

func TestNewConfigEditor_ReturnsESCEditorForRemoteStack(t *testing.T) {
	t.Parallel()

	escEnv := "myproject/dev"
	initialYAML := []byte("values:\n  pulumiConfig: {}\n")

	eb := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(
			_ context.Context, _, _, _, _ string, _ bool,
		) ([]byte, string, int, error) {
			return initialYAML, "etag1", 1, nil
		},
	}

	s := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend { return eb },
		OrgNameF: func() string { return "myorg" },
	}
	ps := newEmptyProjectStack()

	editor, err := NewConfigEditor(context.Background(), s, ps, config.NopEncrypter)
	require.NoError(t, err)
	_, ok := editor.(*escConfigEditor)
	assert.True(t, ok, fmt.Sprintf("expected *escConfigEditor, got %T", editor))
}

func TestESCConfigEditor_SetPath_ArrayIndexReturnsError(t *testing.T) {
	t.Parallel()

	e := newESCEditor(t, nil, "etag1", nil, nil)

	// foo[0] produces path segments ["foo", 0] — the integer index should be rejected.
	key := config.MustMakeKey("myproject", "foo[0]")
	err := e.Set(context.Background(), key, config.NewValue("bar"), true /*path*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "array index paths are not supported")
}

func TestESCConfigEditor_RemovePath_ArrayIndexReturnsError(t *testing.T) {
	t.Parallel()

	initial := map[string]any{
		"values": map[string]any{
			"pulumiConfig": map[string]any{
				"myproject:foo": []any{"a", "b"},
			},
		},
	}
	e := newESCEditor(t, initial, "etag1", nil, nil)

	key := config.MustMakeKey("myproject", "foo[0]")
	err := e.Remove(context.Background(), key, true /*path*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "array index paths are not supported")
}
