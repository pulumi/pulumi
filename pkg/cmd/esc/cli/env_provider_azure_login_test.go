// Copyright 2026, Pulumi Corporation.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBuildAzureLoginStaticNode(t *testing.T) {
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
