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

func TestBuildGCPLoginStaticNode_Required(t *testing.T) {
	t.Parallel()
	node := buildGCPLoginStaticNode(123456789, "ya29.token", "", "")
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::gcp-login:
  project: 123456789
  accessToken:
    accessToken:
      fn::secret: ya29.token
`, string(out))
}

func TestBuildGCPLoginStaticNode_WithImpersonation(t *testing.T) {
	t.Parallel()
	node := buildGCPLoginStaticNode(1, "ya29.token", "sa@proj.iam.gserviceaccount.com", "1h")
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::gcp-login:
  project: 1
  accessToken:
    accessToken:
      fn::secret: ya29.token
    serviceAccount: sa@proj.iam.gserviceaccount.com
    tokenLifetime: 1h
`, string(out))
}

func TestBuildGCPLoginOIDCNode_Required(t *testing.T) {
	t.Parallel()
	node := buildGCPLoginOIDCNode(123456789, "pool", "provider", "sa@proj.iam.gserviceaccount.com", "", "", nil)
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::gcp-login:
  project: 123456789
  oidc:
    workloadPoolId: pool
    providerId: provider
    serviceAccount: sa@proj.iam.gserviceaccount.com
`, string(out))
}

func TestBuildGCPLoginOIDCNode_WithOptionals(t *testing.T) {
	t.Parallel()
	node := buildGCPLoginOIDCNode(1, "pool", "provider", "sa@proj.iam.gserviceaccount.com",
		"us-central1", "1h", []string{"env", "team"})
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::gcp-login:
  project: 1
  oidc:
    workloadPoolId: pool
    providerId: provider
    serviceAccount: sa@proj.iam.gserviceaccount.com
    region: us-central1
    tokenLifetime: 1h
    subjectAttributes:
      - env
      - team
`, string(out))
}
