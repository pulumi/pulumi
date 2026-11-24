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

package pcl

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseSingleBlock(t *testing.T, source string) *hclsyntax.Block {
	t.Helper()

	file, diags := hclsyntax.ParseConfig([]byte(source), "program.pp", hcl.InitialPos)
	require.False(t, diags.HasErrors(), diags.Error())
	body, ok := file.Body.(*hclsyntax.Body)
	require.True(t, ok)
	require.Len(t, body.Blocks, 1)
	return body.Blocks[0]
}

func TestLabelsErrorfWithLabels(t *testing.T) {
	t.Parallel()

	block := parseSingleBlock(t, "resource a b c {\n}\n")

	diag := labelsErrorf(block, "resource variables must have exactly two labels")
	require.NotNil(t, diag.Subject)
	assert.Equal(t, "resource variables must have exactly two labels", diag.Summary)
	assert.Equal(t, block.LabelRanges[0].Start, diag.Subject.Start)
	assert.Equal(t, block.LabelRanges[2].End, diag.Subject.End)
}

// Regression test for https://github.com/pulumi/pulumi/issues/25424: a block with no labels used to make
// labelsErrorf panic with "index out of range [0] with length 0".
func TestLabelsErrorfWithoutLabels(t *testing.T) {
	t.Parallel()

	block := parseSingleBlock(t, "config {\n}\n")
	require.Empty(t, block.LabelRanges)

	var diag *hcl.Diagnostic
	require.NotPanics(t, func() {
		diag = labelsErrorf(block, "config variables must have exactly one or two labels")
	})
	require.NotNil(t, diag.Subject)
	assert.Equal(t, hcl.DiagError, diag.Severity)
	assert.Equal(t, "config variables must have exactly one or two labels", diag.Summary)
	assert.Equal(t, block.DefRange(), *diag.Subject)
}
