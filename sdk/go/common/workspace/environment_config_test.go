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

package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
)

func scalar(t *testing.T, v any) *yaml.Node {
	t.Helper()
	var n yaml.Node
	require.NoError(t, n.Encode(v))
	return &n
}

func TestEnvironmentSetPulumiConfigYAML(t *testing.T) {
	t.Parallel()

	var ps ProjectStack
	require.NoError(t, encoding.YAML.Unmarshal([]byte(`environment:
  imports:
    - shared # keep me
  values:
    pulumiConfig: {}
`), &ps))

	require.NoError(t, ps.Environment.SetPulumiConfig("aws:region", nil, scalar(t, "us-west-2")))
	require.NoError(t, ps.Environment.SetPulumiConfig("proj:count", nil, scalar(t, 3)))
	require.NoError(t, ps.Environment.SetPulumiConfig("proj:nested", []any{"inner", 0}, scalar(t, "first")))
	require.NoError(t, ps.Environment.SetPulumiConfig("proj:nested", []any{"inner", 1}, scalar(t, true)))
	require.NoError(t, ps.Environment.SetPulumiConfig("aws:region", nil, scalar(t, "us-east-1")))

	out, err := encoding.YAML.Marshal(ps)
	require.NoError(t, err)
	assert.Equal(t, `environment:
  imports:
    - shared # keep me
  values:
    pulumiConfig:
      aws:region: us-east-1
      proj:count: 3
      proj:nested:
        inner:
          - first
          - true
`, string(out))

	removed, err := ps.Environment.RemovePulumiConfig("proj:nested", []any{"inner", 0})
	require.NoError(t, err)
	assert.True(t, removed)
	removed, err = ps.Environment.RemovePulumiConfig("proj:count", nil)
	require.NoError(t, err)
	assert.True(t, removed)
	removed, err = ps.Environment.RemovePulumiConfig("proj:missing", nil)
	require.NoError(t, err)
	assert.False(t, removed)

	out, err = encoding.YAML.Marshal(ps)
	require.NoError(t, err)
	assert.Equal(t, `environment:
  imports:
    - shared # keep me
  values:
    pulumiConfig:
      aws:region: us-east-1
      proj:nested:
        inner:
          - true
`, string(out))

	assert.ErrorContains(t, ps.Environment.SetPulumiConfig("proj:nested", []any{"inner", 5}, scalar(t, 1)),
		"out of range")
	assert.ErrorContains(t, ps.Environment.SetPulumiConfig("aws:region", []any{"x"}, scalar(t, 1)),
		"expected a mapping")
}

func TestEnvironmentSetPulumiConfigCreatesValues(t *testing.T) {
	t.Parallel()

	env, err := NewEnvironmentDefinition([]byte("imports:\n  - shared\n"))
	require.NoError(t, err)
	require.NoError(t, env.SetPulumiConfig("a:b", nil, scalar(t, "c")))
	// Definition() renders with yaml.v3's default indentation; the stack file itself is written
	// through encoding.YAML.
	assert.Equal(t, "imports:\n    - shared\nvalues:\n    pulumiConfig:\n        a:b: c\n", string(env.Definition()))
}

func TestEnvironmentSetPulumiConfigJSON(t *testing.T) {
	t.Parallel()

	var ps ProjectStack
	require.NoError(t, encoding.JSON.Unmarshal([]byte(`{"environment": {"values": {"pulumiConfig": {}}}}`), &ps))
	require.NoError(t, ps.Environment.SetPulumiConfig("aws:region", nil, scalar(t, "us-west-2")))
	assert.JSONEq(t, `{"values":{"pulumiConfig":{"aws:region":"us-west-2"}}}`, string(ps.Environment.Definition()))

	removed, err := ps.Environment.RemovePulumiConfig("aws:region", nil)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.JSONEq(t, `{"values":{"pulumiConfig":{}}}`, string(ps.Environment.Definition()))
}

func TestEnvironmentSetPulumiConfigRejectsImportList(t *testing.T) {
	t.Parallel()

	env := NewEnvironment([]string{"shared"})
	assert.ErrorIs(t, env.SetPulumiConfig("a:b", nil, scalar(t, "c")), ErrEnvironmentNotDefinition)
	var none *Environment
	assert.ErrorIs(t, none.SetPulumiConfig("a:b", nil, scalar(t, "c")), ErrEnvironmentNotDefinition)
}
