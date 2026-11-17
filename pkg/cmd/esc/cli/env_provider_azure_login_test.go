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

func TestBuildAzureLoginStaticNode(t *testing.T) {
	t.Parallel()
	node := buildAzureLoginStaticNode("client-id", "tenant-id", "/subscriptions/sub", "shhh")
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::azure-login:
  clientId: client-id
  tenantId: tenant-id
  subscriptionId: /subscriptions/sub
  clientSecret:
    fn::secret: shhh
`, string(out))
}

func TestBuildAzureLoginOIDCNode_Required(t *testing.T) {
	t.Parallel()
	node := buildAzureLoginOIDCNode("client-id", "tenant-id", "/subscriptions/sub", nil)
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::azure-login:
  clientId: client-id
  tenantId: tenant-id
  subscriptionId: /subscriptions/sub
  oidc: true
`, string(out))
}

func TestBuildAzureLoginOIDCNode_WithSubjectAttributes(t *testing.T) {
	t.Parallel()
	node := buildAzureLoginOIDCNode("client-id", "tenant-id", "/subscriptions/sub",
		[]string{"env", "team"})
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::azure-login:
  clientId: client-id
  tenantId: tenant-id
  subscriptionId: /subscriptions/sub
  oidc: true
  subjectAttributes:
    - env
    - team
`, string(out))
}
