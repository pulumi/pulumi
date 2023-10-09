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
	"testing"

	"github.com/pulumi/esc/syntax"
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
