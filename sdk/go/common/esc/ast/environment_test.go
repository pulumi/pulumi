// Copyright 2023, Pulumi Corporation.  All rights reserved.

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
