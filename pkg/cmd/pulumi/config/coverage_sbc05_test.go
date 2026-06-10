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
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func migrateCmd(stdout io.Writer) *configEnvInitCmd {
	return &configEnvInitCmd{parent: &configEnvCmd{stdout: stdout}}
}

func mustYAMLNode(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(s), &n))
	return &n
}

func marshalYAMLNode(t *testing.T, n *yaml.Node) string {
	t.Helper()
	b, err := yaml.Marshal(n)
	require.NoError(t, err)
	return string(b)
}

func TestBuildMigratedDefinition(t *testing.T) {
	t.Parallel()

	t.Run("seeds values.pulumiConfig from an empty definition", func(t *testing.T) {
		t.Parallel()
		doc, err := buildMigratedDefinition(nil, map[string]any{"proj:key": "v", "proj:flag": true}, "")
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(marshalYAMLNode(t, doc)), &got))
		pc := got["values"].(map[string]any)["pulumiConfig"].(map[string]any)
		require.Equal(t, "v", pc["proj:key"])
		require.Equal(t, true, pc["proj:flag"])
	})

	t.Run("rejects an unparseable definition", func(t *testing.T) {
		t.Parallel()
		_, err := buildMigratedDefinition([]byte("\tnot: [valid"), nil, "")
		require.ErrorContains(t, err, "unmarshaling stack environment definition")
	})

	t.Run("strips the stack's own import", func(t *testing.T) {
		t.Parallel()
		inline := []byte(`{"imports":["org/proj/stack","other"],"values":{"pulumiConfig":{}}}`)
		doc, err := buildMigratedDefinition(inline, nil, "org/proj/stack")
		require.NoError(t, err)
		out := marshalYAMLNode(t, doc)
		require.NotContains(t, out, "org/proj/stack")
		require.Contains(t, out, "other")
	})
}

func TestRemoveSelfImport(t *testing.T) {
	t.Parallel()

	t.Run("removing the sole self import deletes the imports key", func(t *testing.T) {
		t.Parallel()
		doc := mustYAMLNode(t, "imports:\n  - self\nvalues: {}\n")
		require.NoError(t, removeSelfImport(doc, "self"))
		require.NotContains(t, marshalYAMLNode(t, doc), "imports")
	})

	t.Run("non-sequence imports is a no-op", func(t *testing.T) {
		t.Parallel()
		doc := mustYAMLNode(t, "imports: scalar\n")
		require.NoError(t, removeSelfImport(doc, "self"))
		require.Contains(t, marshalYAMLNode(t, doc), "scalar")
	})
}

func TestMergeImportNodes(t *testing.T) {
	t.Parallel()

	t.Run("merges new entries and de-duplicates", func(t *testing.T) {
		t.Parallel()
		target := mustYAMLNode(t, "imports:\n  - a\nvalues: {}\n")
		source := mustYAMLNode(t, "imports:\n  - a\n  - b\n")
		require.NoError(t, mergeImportNodes(target, source))
		var got map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(marshalYAMLNode(t, target)), &got))
		require.Equal(t, []any{"a", "b"}, got["imports"])
	})

	t.Run("creates imports when the target lacks it", func(t *testing.T) {
		t.Parallel()
		target := mustYAMLNode(t, "values: {}\n")
		source := mustYAMLNode(t, "imports:\n  - x\n")
		require.NoError(t, mergeImportNodes(target, source))
		require.Contains(t, marshalYAMLNode(t, target), "- x")
	})

	t.Run("empty source is a no-op", func(t *testing.T) {
		t.Parallel()
		target := mustYAMLNode(t, "values: {}\n")
		require.NoError(t, mergeImportNodes(target, mustYAMLNode(t, "values: {}\n")))
	})

	t.Run("malformed non-sequence target imports errors", func(t *testing.T) {
		t.Parallel()
		target := mustYAMLNode(t, "imports: scalar\n")
		source := mustYAMLNode(t, "imports:\n  - x\n")
		require.ErrorContains(t, mergeImportNodes(target, source), "not a sequence")
	})
}

func TestCloneYAMLNode(t *testing.T) {
	t.Parallel()

	require.Nil(t, cloneYAMLNode(nil))

	orig := mustYAMLNode(t, "a: 1\n")
	clone := cloneYAMLNode(orig)
	// Mutating the clone's tree must not affect the original.
	clone.Content[0].Content[1].Value = "2"
	require.Equal(t, "1", orig.Content[0].Content[1].Value)
}

func TestValueToYAMLNode(t *testing.T) {
	t.Parallel()

	scalar, err := valueToYAMLNode("hi")
	require.NoError(t, err)
	require.Equal(t, yaml.ScalarNode, scalar.Kind)
	require.Equal(t, "hi", scalar.Value)

	mapping, err := valueToYAMLNode(map[string]any{"k": "v"})
	require.NoError(t, err)
	require.Equal(t, yaml.MappingNode, mapping.Kind)
}

func TestYAMLNodesDiffer(t *testing.T) {
	t.Parallel()

	a := mustYAMLNode(t, "x: 1\n")
	same := mustYAMLNode(t, "x: 1\n")
	diff := mustYAMLNode(t, "x: 2\n")

	d, err := yamlNodesDiffer(a, same)
	require.NoError(t, err)
	require.False(t, d)

	d, err = yamlNodesDiffer(a, diff)
	require.NoError(t, err)
	require.True(t, d)
}

func TestCreateMigratedEnvironment(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	src := mustYAMLNode(t, "values:\n  pulumiConfig:\n    proj:key: v\n")

	t.Run("uploads the definition", func(t *testing.T) {
		t.Parallel()
		var created []byte
		be := &backend.MockEnvironmentsBackend{
			CreateEnvironmentF: func(_ context.Context, _, _, _ string, y []byte) (apitype.EnvironmentDiagnostics, error) {
				created = y
				return nil, nil
			},
		}
		require.NoError(t, migrateCmd(&bytes.Buffer{}).createMigratedEnvironment(ctx, be, "org", "proj", "env", src))
		require.Contains(t, string(created), "proj:key: v")
	})

	t.Run("backend failure is wrapped", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			CreateEnvironmentF: func(_ context.Context, _, _, _ string, _ []byte) (apitype.EnvironmentDiagnostics, error) {
				return nil, errors.New("boom")
			},
		}
		err := migrateCmd(&bytes.Buffer{}).createMigratedEnvironment(ctx, be, "org", "proj", "env", src)
		require.ErrorContains(t, err, "creating environment")
	})

	t.Run("diagnostics are an error", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			CreateEnvironmentF: func(_ context.Context, _, _, _ string, _ []byte) (apitype.EnvironmentDiagnostics, error) {
				return apitype.EnvironmentDiagnostics{{Summary: "bad"}}, nil
			},
		}
		err := migrateCmd(&bytes.Buffer{}).createMigratedEnvironment(ctx, be, "org", "proj", "env", src)
		require.ErrorContains(t, err, "creating environment")
	})
}

func TestMergeMigratedEnvironment(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("merges into the existing definition", func(t *testing.T) {
		t.Parallel()
		var uploaded []byte
		be := &backend.MockEnvironmentsBackend{
			UpdateEnvironmentWithProjectF: func(
				_ context.Context, _, _, _ string, y []byte, _ string,
			) (apitype.EnvironmentDiagnostics, error) {
				uploaded = y
				return nil, nil
			},
		}
		def := []byte("values:\n  pulumiConfig:\n    proj:existing: keep\n")
		src := mustYAMLNode(t, "values:\n  pulumiConfig:\n    proj:added: new\n")
		require.NoError(t, migrateCmd(&bytes.Buffer{}).mergeMigratedEnvironment(
			ctx, be, "org", "proj", "env", "etag", def, src))
		require.Contains(t, string(uploaded), "proj:existing: keep")
		require.Contains(t, string(uploaded), "proj:added: new")
	})

	t.Run("warns when overwriting a changed key", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		be := &backend.MockEnvironmentsBackend{
			UpdateEnvironmentWithProjectF: func(
				_ context.Context, _, _, _ string, _ []byte, _ string,
			) (apitype.EnvironmentDiagnostics, error) {
				return nil, nil
			},
		}
		def := []byte("values:\n  pulumiConfig:\n    proj:k: old\n")
		src := mustYAMLNode(t, "values:\n  pulumiConfig:\n    proj:k: new\n")
		require.NoError(t, migrateCmd(&buf).mergeMigratedEnvironment(ctx, be, "org", "proj", "env", "etag", def, src))
		require.Contains(t, buf.String(), "overwriting existing key")
	})

	t.Run("rejects an unparseable existing definition", func(t *testing.T) {
		t.Parallel()
		err := migrateCmd(&bytes.Buffer{}).mergeMigratedEnvironment(
			ctx, &backend.MockEnvironmentsBackend{}, "org", "proj", "env",
			"etag", []byte("\tnot: [valid"), mustYAMLNode(t, "values: {}\n"))
		require.ErrorContains(t, err, "unmarshaling environment definition")
	})

	t.Run("surfaces an etag conflict as retryable", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			UpdateEnvironmentWithProjectF: func(
				_ context.Context, _, _, _ string, _ []byte, _ string,
			) (apitype.EnvironmentDiagnostics, error) {
				return nil, fmt.Errorf("update: %w", backend.ErrConfigConflict)
			},
		}
		err := migrateCmd(&bytes.Buffer{}).mergeMigratedEnvironment(
			ctx, be, "org", "proj", "env", "etag", []byte("values: {}\n"),
			mustYAMLNode(t, "values:\n  pulumiConfig:\n    proj:k: v\n"))
		require.ErrorContains(t, err, "modified concurrently")
	})
}

func TestGetMigrationTarget(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("reports not exists on 404", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
				return nil, "", 0, &apitype.ErrorResponse{Code: http.StatusNotFound}
			},
		}
		_, _, exists, err := migrateCmd(&bytes.Buffer{}).getMigrationTarget(ctx, be, "org", "proj", "env")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("reports exists with def and etag", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
				return []byte("values: {}\n"), "etag", 0, nil
			},
		}
		def, etag, exists, err := migrateCmd(&bytes.Buffer{}).getMigrationTarget(ctx, be, "org", "proj", "env")
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, "etag", etag)
		require.Equal(t, "values: {}\n", string(def))
	})

	t.Run("other GetEnvironment failures are wrapped", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
				return nil, "", 0, errors.New("boom")
			},
		}
		_, _, _, err := migrateCmd(&bytes.Buffer{}).getMigrationTarget(ctx, be, "org", "proj", "env")
		require.ErrorContains(t, err, "getting environment")
	})
}
