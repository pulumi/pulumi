// Copyright 2024, Pulumi Corporation.
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

func TestBuildAWSLoginStaticNode_Required(t *testing.T) {
	node := buildAWSLoginStaticNode("AKIAEXAMPLE", "shhh", "")
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::aws-login:
  static:
    accessKeyId: AKIAEXAMPLE
    secretAccessKey:
      fn::secret: shhh
`, string(out))
}

func TestBuildAWSLoginStaticNode_WithSessionToken(t *testing.T) {
	node := buildAWSLoginStaticNode("AKIAEXAMPLE", "shhh", "tok")
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::aws-login:
  static:
    accessKeyId: AKIAEXAMPLE
    secretAccessKey:
      fn::secret: shhh
    sessionToken:
      fn::secret: tok
`, string(out))
}

func TestBuildAWSLoginOIDCNode_Required(t *testing.T) {
	node := buildAWSLoginOIDCNode("arn:aws:iam::123:role/r", "sess", "", nil, nil)
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::aws-login:
  oidc:
    roleArn: arn:aws:iam::123:role/r
    sessionName: sess
`, string(out))
}

func TestBuildAWSLoginOIDCNode_WithDuration(t *testing.T) {
	node := buildAWSLoginOIDCNode("arn:aws:iam::123:role/r", "sess", "1h", nil, nil)
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::aws-login:
  oidc:
    roleArn: arn:aws:iam::123:role/r
    sessionName: sess
    duration: 1h
`, string(out))
}

func TestBuildAWSLoginOIDCNode_WithOptionals(t *testing.T) {
	node := buildAWSLoginOIDCNode("arn:aws:iam::123:role/r", "sess", "1h",
		[]string{"arn1", "arn2"}, []string{"env", "team"})
	out, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.YAMLEq(t, `fn::open::aws-login:
  oidc:
    roleArn: arn:aws:iam::123:role/r
    sessionName: sess
    duration: 1h
    policyArns:
      - arn1
      - arn2
    subjectAttributes:
      - env
      - team
`, string(out))
}
