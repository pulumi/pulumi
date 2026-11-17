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
	t.Parallel()
	provider := mustNode(t, `fn::open::aws-login:
  static:
    accessKeyId: a
    secretAccessKey:
      fn::secret: s
`)

	out, changed, err := mergeProviderIntoEnv(nil, []any{"aws", "login"}, provider, nil)
	require.NoError(t, err)
	assert.True(t, changed)
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
	t.Parallel()
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

	out, changed, err := mergeProviderIntoEnv(current, []any{"aws", "login"}, provider, nil)
	require.NoError(t, err)
	assert.True(t, changed)
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
	t.Parallel()
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

	out, changed, err := mergeProviderIntoEnv(current, []any{"gcp", "login"}, provider, nil)
	require.NoError(t, err)
	assert.True(t, changed)
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

func TestMergeProviderIntoEnv_WritesEnvVars(t *testing.T) {
	t.Parallel()
	provider := mustNode(t, `fn::open::aws-login:
  oidc:
    roleArn: arn
    sessionName: s
`)

	out, changed, err := mergeProviderIntoEnv(nil, []any{"aws", "login"}, provider,
		awsLoginEnvVars(propertyPathRef([]any{"aws", "login"})))
	require.NoError(t, err)
	assert.True(t, changed)
	assert.YAMLEq(t, `values:
  aws:
    login:
      fn::open::aws-login:
        oidc:
          roleArn: arn
          sessionName: s
  environmentVariables:
    AWS_ACCESS_KEY_ID: ${aws.login.accessKeyId}
    AWS_SECRET_ACCESS_KEY: ${aws.login.secretAccessKey}
    AWS_SESSION_TOKEN: ${aws.login.sessionToken}
`, string(out))
}

// Existing environmentVariables entries must survive; only our keys are added or updated.
func TestMergeProviderIntoEnv_MergesEnvVars(t *testing.T) {
	t.Parallel()
	current := []byte(`values:
  environmentVariables:
    KEEP_ME: value
    ARM_CLIENT_ID: stale
`)
	provider := mustNode(t, `fn::open::azure-login:
  clientId: c
  tenantId: t
  oidc: true
`)

	out, changed, err := mergeProviderIntoEnv(current, []any{"azure", "login"}, provider,
		azureLoginOIDCEnvVars(propertyPathRef([]any{"azure", "login"}), false))
	require.NoError(t, err)
	assert.True(t, changed)
	assert.YAMLEq(t, `values:
  azure:
    login:
      fn::open::azure-login:
        clientId: c
        tenantId: t
        oidc: true
  environmentVariables:
    KEEP_ME: value
    ARM_USE_OIDC: "true"
    ARM_CLIENT_ID: ${azure.login.clientId}
    ARM_TENANT_ID: ${azure.login.tenantId}
    ARM_OIDC_TOKEN: ${azure.login.oidc.token}
`, string(out))
}

// Re-merging an identical provider block and env vars reports no change, so the caller can
// skip the write and avoid bumping a revision.
func TestMergeProviderIntoEnv_NoChangeWhenIdentical(t *testing.T) {
	t.Parallel()
	provider := mustNode(t, `fn::open::aws-login:
  oidc:
    roleArn: arn
    sessionName: s
`)
	envVars := awsLoginEnvVars(propertyPathRef([]any{"aws", "login"}))

	first, changed, err := mergeProviderIntoEnv(nil, []any{"aws", "login"}, provider, envVars)
	require.NoError(t, err)
	require.True(t, changed)

	second, changed, err := mergeProviderIntoEnv(first, []any{"aws", "login"}, provider, envVars)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, string(first), string(second))
}

func TestSecretNode_WrapsScalar(t *testing.T) {
	t.Parallel()
	n := secretNode("hunter2")
	require.Equal(t, yaml.MappingNode, n.Kind)
	require.Len(t, n.Content, 2)
	assert.Equal(t, "fn::secret", n.Content[0].Value)
	assert.Equal(t, "hunter2", n.Content[1].Value)
	assert.Equal(t, "!!str", n.Content[1].Tag)
}
