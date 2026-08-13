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

package pcl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

// A condition block should bind cleanly when trueValue/falseValue reference
// resources declared in their respective branches, and the outer condition
// attribute can reference resources declared at the top level.
func TestConditionBinding(t *testing.T) {
	t.Parallel()

	source := `
resource "outer" "infra:index:Vpc" {
	cidrBlock = "10.0.0.0/16"
}

condition "cond" {
	condition = outer.cidrBlock == "10.0.0.0/16"

	true {
		resource "resB" "infra:index:Vpc" {
			cidrBlock = "10.0.1.0/24"
		}
	}
	trueValue = resB.cidrBlock

	false {
		resource "resC" "infra:index:Vpc" {
			cidrBlock = "10.0.2.0/24"
		}
	}
	falseValue = resC.cidrBlock
}
`

	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.NoError(t, err)
	require.Empty(t, diags)
	require.NotNil(t, program)

	var cond *pcl.Condition
	for _, n := range program.Nodes {
		if c, ok := n.(*pcl.Condition); ok {
			cond = c
			break
		}
	}
	require.NotNil(t, cond, "expected a Condition node")

	assert.NotNil(t, cond.Condition, "condition attribute should be bound")
	assert.NotNil(t, cond.TrueExpression, "trueValue should be bound")
	assert.NotNil(t, cond.FalseExpression, "falseValue should be bound")

	require.NotNil(t, cond.TrueBlock)
	require.Len(t, cond.TrueBlock.Nodes, 1)
	assert.Equal(t, "resB", cond.TrueBlock.Nodes[0].Name())

	require.NotNil(t, cond.FalseBlock)
	require.Len(t, cond.FalseBlock.Nodes, 1)
	assert.Equal(t, "resC", cond.FalseBlock.Nodes[0].Name())

	// Nested resources must not leak into the outer program.
	for _, n := range program.Nodes {
		assert.NotEqual(t, "resB", n.Name(), "resB should not be a top-level node")
		assert.NotEqual(t, "resC", n.Name(), "resC should not be a top-level node")
	}
}

// falseValue must not see resources declared in the true branch (and vice versa).
func TestConditionBranchScopeIsolation(t *testing.T) {
	t.Parallel()

	source := `
condition "cond" {
	condition = true

	true {
		resource "resB" "infra:index:Vpc" {
			cidrBlock = "10.0.1.0/24"
		}
	}
	trueValue = resB.cidrBlock

	false {
		resource "resC" "infra:index:Vpc" {
			cidrBlock = "10.0.2.0/24"
		}
	}
	falseValue = resB.cidrBlock
}
`

	_, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.Error(t, err)
	require.NotEmpty(t, diags)

	var found bool
	for _, d := range diags {
		if d.Summary == "undefined variable resB" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'undefined variable resB' diagnostic, got: %v", diags)
}
