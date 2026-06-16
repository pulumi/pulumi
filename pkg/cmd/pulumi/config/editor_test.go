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
	"fmt"
	"testing"

	escEncoding "github.com/pulumi/esc/syntax/encoding"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

	t.Run("remote stack returns ESC editor", func(t *testing.T) {
		t.Parallel()
		s := remoteStackForEditor(t, []byte("values:\n  pulumiConfig: {}\n"), "etag", nil)
		editor, err := newConfigEditor(ctx, s, ps, config.NopEncrypter, "")
		require.NoError(t, err)
		require.IsType(t, &escConfigEditor{}, editor)
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

// remoteStackForEditor builds a remote (ESC-backed) MockStack whose EnvironmentsBackend returns the
// given definition YAML and etag from GetEnvironment, and whose UpdateEnvironmentWithProject calls
// onUpdate (if non-nil) with the uploaded YAML, returning that callback's result.
func remoteStackForEditor(
	t *testing.T,
	getYAML []byte,
	getEtag string,
	onUpdate func(yaml []byte, etag string) (apitype.EnvironmentDiagnostics, error),
) *backend.MockStack {
	t.Helper()
	env := "testProject/testStack"
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(
			_ context.Context, _, _, _, _ string, _ bool,
		) ([]byte, string, int, error) {
			return getYAML, getEtag, 0, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, yaml []byte, etag string,
		) (apitype.EnvironmentDiagnostics, error) {
			if onUpdate != nil {
				return onUpdate(yaml, etag)
			}
			return nil, nil
		},
	}
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}
}

// editForRemote runs a single Set or Remove against an escConfigEditor backed by initialYAML and
// returns the uploaded YAML.
func editForRemote(
	t *testing.T,
	initialYAML string,
	edit func(t *testing.T, editor *escConfigEditor),
) []byte {
	t.Helper()
	ctx := t.Context()
	var uploaded []byte
	s := remoteStackForEditor(t, []byte(initialYAML), "etag",
		func(yaml []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			uploaded = yaml
			return nil, nil
		})
	editor, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	esc, ok := editor.(*escConfigEditor)
	require.True(t, ok)
	edit(t, esc)
	require.NoError(t, esc.Save(ctx))
	return uploaded
}

func TestSaveRemoteConfigValuesOverwrites(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// The env already has testProject:a; it is overwritten and the new key testProject:b is added.
	existingYAML := "values:\n  pulumiConfig:\n    testProject:a: original\n"
	var uploaded []byte
	s := remoteStackForEditor(t, []byte(existingYAML), "etag",
		func(yaml []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			uploaded = yaml
			return nil, nil
		})

	c := config.Map{
		config.MustMakeKey("testProject", "a"): config.NewValue("updated"),
		config.MustMakeKey("testProject", "b"): config.NewValue("new"),
	}
	require.NoError(t, SaveRemoteConfigValues(ctx, s, c))

	got := string(uploaded)
	require.Contains(t, got, "testProject:a: updated", "an existing key is overwritten")
	require.Contains(t, got, "testProject:b: new", "a new key is written")
}

func TestESCConfigEditorSet(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("plaintext scalar is a plain string", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n", func(t *testing.T, e *escConfigEditor) {
			key := config.MustMakeKey("testProject", "key")
			require.NoError(t, e.Set(ctx, key, config.NewValue("value"), false))
		})
		require.Equal(t, "string", scalarTag(t, uploaded, "testProject:key"))
		require.Equal(t, "value", scalarValue(t, uploaded, "testProject:key"))
	})

	t.Run("typed values are native, not quoted strings", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name      string
			value     config.Value
			wantTag   string
			wantValue string
		}{
			{"bool", config.NewTypedValue("true", config.TypeBool), "bool", "true"},
			{"int", config.NewTypedValue("42", config.TypeInt), "int", "42"},
			{"float", config.NewTypedValue("3.14", config.TypeFloat), "float", "3.14"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n",
					func(t *testing.T, e *escConfigEditor) {
						key := config.MustMakeKey("testProject", c.name)
						require.NoError(t, e.Set(ctx, key, c.value, false))
					})
				require.Equal(t, c.wantTag, scalarTag(t, uploaded, "testProject:"+c.name))
				require.Equal(t, c.wantValue, scalarValue(t, uploaded, "testProject:"+c.name))
			})
		}
	})

	t.Run("object value is a native mapping with array preserved", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n", func(t *testing.T, e *escConfigEditor) {
			key := config.MustMakeKey("testProject", "obj")
			require.NoError(t, e.Set(ctx, key, config.NewObjectValue(`{"a":1,"b":[2,3]}`), false))
		})
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(uploaded, &doc))
		values := doc["values"].(map[string]any)
		pc := values["pulumiConfig"].(map[string]any)
		obj := pc["testProject:obj"].(map[string]any)
		require.Equal(t, 1, obj["a"])
		require.Equal(t, []any{2, 3}, obj["b"])
	})

	t.Run("secret scalar is wrapped as fn::secret in plaintext", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n", func(t *testing.T, e *escConfigEditor) {
			key := config.MustMakeKey("testProject", "secret")
			require.NoError(t, e.Set(ctx, key, config.NewSecureValue("plaintext"), false))
		})
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(uploaded, &doc))
		values := doc["values"].(map[string]any)
		pc := values["pulumiConfig"].(map[string]any)
		secret := pc["testProject:secret"].(map[string]any)
		require.Equal(t, "plaintext", secret["fn::secret"])
	})
}

func TestESCConfigEditorPreservesUntouchedContent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const initial = `# top comment
imports:
  - shared-env
values:
  environmentVariables:
    FOO: bar # keep me
  pulumiConfig:
    testProject:existing: keep
`
	t.Run("set", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
			key := config.MustMakeKey("testProject", "added")
			require.NoError(t, e.Set(ctx, key, config.NewValue("new"), false))
		})
		out := string(uploaded)
		require.Contains(t, out, "# top comment")
		require.Contains(t, out, "imports:")
		require.Contains(t, out, "- shared-env")
		require.Contains(t, out, "FOO: bar # keep me")
		require.Contains(t, out, "testProject:existing: keep")
		require.Contains(t, out, "testProject:added: new")
	})

	t.Run("remove", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
			key := config.MustMakeKey("testProject", "existing")
			require.NoError(t, e.Remove(ctx, key, false))
		})
		out := string(uploaded)
		require.Contains(t, out, "# top comment")
		require.Contains(t, out, "imports:")
		require.Contains(t, out, "- shared-env")
		require.Contains(t, out, "FOO: bar # keep me")
		require.NotContains(t, out, "testProject:existing")
	})
}

func TestESCConfigEditorRemove(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const initial = `values:
  pulumiConfig:
    testProject:a: 1
    testProject:b: 2
`
	uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
		require.NoError(t, e.Remove(ctx, config.MustMakeKey("testProject", "a"), false))
	})
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	_, hasA := pc["testProject:a"]
	require.False(t, hasA)
	require.Equal(t, 2, pc["testProject:b"])
}

// TestESCConfigEditorArrayIndexPath verifies that path keys with array-index segments round-trip:
// Set materializes a native YAML sequence under the namespaced config key, and Remove deletes an
// element by index. This matches local `pulumi config set --path 'foo[0]'` and `esc env set`.
func TestESCConfigEditorArrayIndexPath(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	pulumiConfigArray := func(t *testing.T, uploaded []byte) []any {
		t.Helper()
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(uploaded, &doc))
		pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
		arr, ok := pc["testProject:foo"].([]any)
		require.True(t, ok, "expected testProject:foo to be a sequence, got %#v", pc["testProject:foo"])
		return arr
	}

	t.Run("set creates and extends a sequence", func(t *testing.T) {
		t.Parallel()
		uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n", func(t *testing.T, e *escConfigEditor) {
			require.NoError(t, e.Set(ctx, config.MustMakeKey("testProject", "foo[0]"), config.NewValue("a"), true))
			require.NoError(t, e.Set(ctx, config.MustMakeKey("testProject", "foo[1]"), config.NewValue("b"), true))
		})
		require.Equal(t, []any{"a", "b"}, pulumiConfigArray(t, uploaded))
	})

	t.Run("remove deletes an element by index", func(t *testing.T) {
		t.Parallel()
		const initial = `values:
  pulumiConfig:
    testProject:foo:
      - a
      - b
`
		uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
			require.NoError(t, e.Remove(ctx, config.MustMakeKey("testProject", "foo[0]"), true))
		})
		require.Equal(t, []any{"b"}, pulumiConfigArray(t, uploaded))
	})
}

func TestESCConfigEditorEtagConflict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	s := remoteStackForEditor(t, []byte("values:\n  pulumiConfig: {}\n"), "etag",
		func(_ []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			// The cloud backend translates esc's 409 into ErrConfigConflict; the editor contracts
			// on that sentinel, not on the esc client's error type.
			return nil, fmt.Errorf("updating environment: %w", backend.ErrConfigConflict)
		})
	editor, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	require.NoError(t, editor.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
	err = editor.Save(ctx)
	require.ErrorContains(t, err, "modified concurrently")
}

func TestESCConfigEditorPassesEtag(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var gotEtag string
	s := remoteStackForEditor(t, []byte("values:\n  pulumiConfig: {}\n"), "the-etag",
		func(_ []byte, etag string) (apitype.EnvironmentDiagnostics, error) {
			gotEtag = etag
			return nil, nil
		})
	editor, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	require.NoError(t, editor.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
	require.NoError(t, editor.Save(ctx))
	require.Equal(t, "the-etag", gotEtag)
}

func TestCheckRemoteProjectStackNilGuard(t *testing.T) {
	t.Parallel()
	env := "testProject/testStack"
	remote := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
	}
	require.Error(t, checkRemoteProjectStack(remote, nil))
	require.NoError(t, checkRemoteProjectStack(remote, &workspace.ProjectStack{}))

	local := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
	}
	require.NoError(t, checkRemoteProjectStack(local, nil))
}

// scalarTag returns the YAML scalar kind ("string"/"int"/"float"/"bool") of values.pulumiConfig.<key>.
func scalarTag(t *testing.T, doc []byte, key string) string {
	t.Helper()
	n := pulumiConfigScalar(t, doc, key)
	switch n.Tag {
	case "!!str":
		return "string"
	case "!!int":
		return "int"
	case "!!float":
		return "float"
	case "!!bool":
		return "bool"
	default:
		return n.Tag
	}
}

func scalarValue(t *testing.T, doc []byte, key string) string {
	t.Helper()
	return pulumiConfigScalar(t, doc, key).Value
}

func pulumiConfigScalar(t *testing.T, doc []byte, key string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(doc, &root))
	n, ok := escEncoding.YAMLSyntax{Node: &root}.Get(resource.PropertyPath{"values", "pulumiConfig", key})
	require.True(t, ok, "key %q not found", key)
	require.Equal(t, yaml.ScalarNode, n.Kind)
	return n
}

func TestESCConfigEditorConfigKeys(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const initial = `values:
  pulumiConfig:
    testProject:a: 1
    testProject:b: 2
`
	s := remoteStackForEditor(t, []byte(initial), "etag", nil)
	editor, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	esc, ok := editor.(*escConfigEditor)
	require.True(t, ok)

	keys, err := esc.ConfigKeys()
	require.NoError(t, err)
	got := map[string]bool{}
	for _, k := range keys {
		got[k.String()] = true
	}
	require.Equal(t, map[string]bool{"testProject:a": true, "testProject:b": true}, got)
}

// TestESCConfigEditorReplaceConfig verifies exact replacement: keys absent from the new set are removed
// (not merely left in place as SaveRemoteConfigValues does), surviving keys are overwritten, and new
// keys are added.
func TestESCConfigEditorReplaceConfig(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const initial = `values:
  pulumiConfig:
    testProject:a: 1
    testProject:b: 2
    testProject:c: 3
`
	uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
		c := config.Map{
			config.MustMakeKey("testProject", "a"): config.NewValue("updated"),
			config.MustMakeKey("testProject", "d"): config.NewValue("new"),
		}
		require.NoError(t, e.ReplaceConfig(ctx, c))
	})

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "updated", pc["testProject:a"], "surviving key overwritten")
	require.Equal(t, "new", pc["testProject:d"], "new key added")
	_, hasB := pc["testProject:b"]
	_, hasC := pc["testProject:c"]
	require.False(t, hasB, "key absent from the new set is removed")
	require.False(t, hasC, "key absent from the new set is removed")
}

// TestNewESCConfigEditorFromDefUsesSeededEtag verifies the seeded constructor writes with the supplied
// etag and never re-reads the environment, giving commit a no-gap force-with-lease write.
func TestNewESCConfigEditorFromDefUsesSeededEtag(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var gotEtag string
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(
			_ context.Context, _, _, _, _ string, _ bool,
		) ([]byte, string, int, error) {
			t.Fatal("a seeded editor must not read the environment")
			return nil, "", 0, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, _ []byte, etag string,
		) (apitype.EnvironmentDiagnostics, error) {
			gotEtag = etag
			return nil, nil
		},
	}

	editor, err := newESCConfigEditorFromDef(
		be, "org", "testProject", "testStack", "", []byte("values:\n  pulumiConfig: {}\n"), "lease-etag")
	require.NoError(t, err)
	require.NoError(t, editor.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
	require.NoError(t, editor.Save(ctx))
	require.Equal(t, "lease-etag", gotEtag)
}

// TestConfigSetSecretRemoteCommand exercises the full `config set --secret` command path against a
// remote stack, verifying the plaintext is passed through to the editor (not encrypted locally) and
// uploaded as fn::secret.
//
//nolint:paralleltest // exercises the command with shared mock state
func TestConfigSetSecretRemoteCommand(t *testing.T) {
	ctx := t.Context()

	var uploaded []byte
	s := remoteStackForEditor(t, []byte("values:\n  pulumiConfig: {}\n"), "etag",
		func(y []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			uploaded = y
			return nil, nil
		})

	cmd := &configSetCmd{
		Secret: true,
		LoadProjectStack: func(
			_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{Config: config.Map{}}, nil
		},
	}

	project := &workspace.Project{Name: "testProject"}
	ws := &pkgWorkspace.MockContext{}
	err := cmd.Run(ctx, ws, []string{"testProject:tok", "shh"}, project, s, "")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "shh", pc["testProject:tok"].(map[string]any)["fn::secret"])
}

// TestESCConfigEditorStripsEnvVersion verifies an "@version" suffix on the linked env ref is stripped
// before addressing the environment.
func TestESCConfigEditorStripsEnvVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var gotProject, gotName string
	env := "myproj/myenv@7"
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(
			_ context.Context, _, project, name, _ string, _ bool,
		) ([]byte, string, int, error) {
			gotProject, gotName = project, name
			return []byte("values:\n  pulumiConfig: {}\n"), "etag", 0, nil
		},
	}
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}

	_, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	require.Equal(t, "myproj", gotProject)
	require.Equal(t, "myenv", gotName)
}

// TestESCConfigEditorPinnedVersion verifies the editor reads the pinned revision when the stack's
// linked ref carries an @version, and that Save refuses to write to a pinned environment.
func TestESCConfigEditorPinnedVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var gotVersion string
	env := "testProject/testStack@5"
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, version string, _ bool) ([]byte, string, int, error) {
			gotVersion = version
			return []byte("values:\n  pulumiConfig: {}\n"), "etag", 0, nil
		},
	}
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}

	editor, err := newConfigEditor(ctx, s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	require.Equal(t, "5", gotVersion, "the editor must read the pinned revision, not the latest")

	require.NoError(t, editor.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
	require.ErrorContains(t, editor.Save(ctx), "pinned", "Save must refuse to write to a pinned environment")
}

// TestESCConfigEditorSecureObject covers the set-all --json objectValue+secret path: a secure object
// value set with path=true must upload as fn::secret wrapping the native object (not a stringified
// blob, not plaintext).
func TestESCConfigEditorSecureObject(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	uploaded := editForRemote(t, "values:\n  pulumiConfig: {}\n", func(t *testing.T, e *escConfigEditor) {
		key := config.MustMakeKey("testProject", "obj")
		require.NoError(t, e.Set(ctx, key, config.NewSecureObjectValue(`{"inner":"v","n":1}`), true))
	})
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	secret := pc["testProject:obj"].(map[string]any)
	inner, ok := secret["fn::secret"].(map[string]any)
	require.True(t, ok, "expected fn::secret to wrap a native object, got %#v", secret["fn::secret"])
	require.Equal(t, "v", inner["inner"])
	require.Equal(t, 1, inner["n"])
}

// TestESCConfigEditorEmptyDefinition covers a freshly linked env whose definition has no values node
// yet: the first Set must create values.pulumiConfig rather than fail on the missing path.
func TestESCConfigEditorEmptyDefinition(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	uploaded := editForRemote(t, "{}\n", func(t *testing.T, e *escConfigEditor) {
		require.NoError(t, e.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
	})
	require.Equal(t, "v", scalarValue(t, uploaded, "testProject:k"))
}

// TestESCConfigEditorPreservesExistingSecretCiphertext locks in the design's core claim: the editor
// works at the node level with decrypt=false, so an existing fn::secret ciphertext on an untouched key
// round-trips byte-for-byte when a different key is edited (it is never decrypted or re-wrapped).
func TestESCConfigEditorPreservesExistingSecretCiphertext(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const initial = `values:
  pulumiConfig:
    testProject:existing:
      fn::secret:
        ciphertext: ZXhpc3Rpbmctc2VjcmV0
    testProject:plain: old
`
	uploaded := editForRemote(t, initial, func(t *testing.T, e *escConfigEditor) {
		require.NoError(t, e.Set(ctx, config.MustMakeKey("testProject", "plain"), config.NewValue("new"), false))
	})
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "new", pc["testProject:plain"])
	secret := pc["testProject:existing"].(map[string]any)["fn::secret"].(map[string]any)
	require.Equal(t, "ZXhpc3Rpbmctc2VjcmV0", secret["ciphertext"], "existing ciphertext must round-trip untouched")
}
