// Copyright 2023, Pulumi Corporation.  All rights reserved.
package ast

import (
	"testing"

	"github.com/pulumi/environments/syntax"
	"github.com/stretchr/testify/assert"
)

func TestEscapeInterpolationWorks(t *testing.T) {
	t.Parallel()
	node := syntax.String("Hello $${world}!")
	parts, diags := parseInterpolate(node, node.Value())
	assert.Empty(t, diags)
	assert.Len(t, parts, 1, "Expected one interpolation part")
	assert.Equal(t, "Hello ${world}!", parts[0].Text)
}
