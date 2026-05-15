// Copyright 2026, Pulumi Corporation.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func mustNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &n))
	require.Equal(t, yaml.DocumentNode, n.Kind)
	require.Len(t, n.Content, 1)
	return n.Content[0]
}

func TestMergeProviderIntoEnv_EmptyDoc(t *testing.T) {
	provider := mustNode(t, `fn::open::aws-login:
  static:
    accessKeyId: a
    secretAccessKey:
      fn::secret: s
`)

	out, err := mergeProviderIntoEnv(nil, []any{"aws", "login"}, provider)
	require.NoError(t, err)
	assert.YAMLEq(t, `values:
  aws:
    login:
      fn::open::aws-login:
        static:
          accessKeyId: a
          secretAccessKey:
            fn::secret: s
`, string(out))
}

func TestMergeProviderIntoEnv_ReplacesExisting(t *testing.T) {
	current := []byte(`values:
  aws:
    login:
      fn::open::aws-login:
        static:
          accessKeyId: old
          secretAccessKey:
            fn::secret: old
  other: keep-me
`)
	provider := mustNode(t, `fn::open::aws-login:
  static:
    accessKeyId: new
    secretAccessKey:
      fn::secret: new
`)

	out, err := mergeProviderIntoEnv(current, []any{"aws", "login"}, provider)
	require.NoError(t, err)
	assert.YAMLEq(t, `values:
  aws:
    login:
      fn::open::aws-login:
        static:
          accessKeyId: new
          secretAccessKey:
            fn::secret: new
  other: keep-me
`, string(out))
}

func TestMergeProviderIntoEnv_PreservesSiblings(t *testing.T) {
	current := []byte(`values:
  unrelated:
    foo: bar
imports:
  - default/base
`)
	provider := mustNode(t, `fn::open::gcp-login:
  project: 1
  accessToken:
    accessToken:
      fn::secret: t
`)

	out, err := mergeProviderIntoEnv(current, []any{"gcp", "login"}, provider)
	require.NoError(t, err)
	assert.YAMLEq(t, `values:
  unrelated:
    foo: bar
  gcp:
    login:
      fn::open::gcp-login:
        project: 1
        accessToken:
          accessToken:
            fn::secret: t
imports:
  - default/base
`, string(out))
}

func TestSecretNode_WrapsScalar(t *testing.T) {
	n := secretNode("hunter2")
	require.Equal(t, yaml.MappingNode, n.Kind)
	require.Len(t, n.Content, 2)
	assert.Equal(t, "fn::secret", n.Content[0].Value)
	assert.Equal(t, "hunter2", n.Content[1].Value)
	assert.Equal(t, "!!str", n.Content[1].Tag)
}
