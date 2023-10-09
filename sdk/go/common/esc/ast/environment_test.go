// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc/syntax/encoding"
)

func TestExample(t *testing.T) {
	t.Parallel()

	const example = `
imports:
  - green-channel
  - us-west-2
config:
  aws:
    fn::open:
      provider: aws-oidc
      inputs:
        sessionName: site-prod-session
        roleArn: some-role-arn
  pulumi:
    aws:defaultTags:
      tags:
        environment: prod
`

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}

func TestExample2(t *testing.T) {
	t.Parallel()

	const example = `
imports:
  - green-channel
  - us-west-2
config:
  aws:
    fn::open::aws-oidc:
      sessionName: site-prod-session
      roleArn: some-role-arn
  pulumi:
    aws:defaultTags:
      tags:
        environment: prod
`

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}

func TestEmptyDocument(t *testing.T) {
	t.Parallel()

	const example = ``

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}
