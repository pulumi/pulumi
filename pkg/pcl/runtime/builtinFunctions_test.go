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

package runtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// evaluateLocals binds a program of local variables and evaluates each one with no
// resources, invokes, or environment.
func evaluateLocals(t *testing.T, source string) map[string]property.Value {
	parser := syntax.NewParser()
	require.NoError(t, parser.ParseFile(strings.NewReader(source), "main.pp"))
	require.False(t, parser.Diagnostics.HasErrors(), parser.Diagnostics.Error())
	program, diags, err := pcl.BindProgram(parser.Files, schema.NewNullLoader())
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), diags.Error())

	ectx := NewEvalContext("", "", "", "", "", nil, nil, nil, nil, nil)
	values := map[string]property.Value{}
	for _, node := range program.Nodes {
		local := node.(*pcl.LocalVariable)
		value, poison, diags := ectx.Evaluate(local.Definition.Value)
		require.Nil(t, poison)
		require.False(t, diags.HasErrors(), "%s: %s", local.Name(), diags.Error())
		values[local.Name()] = value
	}
	return values
}

func TestLength(t *testing.T) {
	t.Parallel()

	values := evaluateLocals(t, `
string = length("abc")
tuple = length([1, 2, 3])
object = length({"a" = 1, "b" = 2})
emptyObject = length({})
map = length({for k, v in {"a" = 1, "b" = 2, "c" = 3} : k => v})
`)
	assert.Equal(t, map[string]property.Value{
		"string":      property.New(3.0),
		"tuple":       property.New(3.0),
		"object":      property.New(2.0),
		"emptyObject": property.New(0.0),
		"map":         property.New(3.0),
	}, values)
}

func TestRange(t *testing.T) {
	t.Parallel()

	list := func(ns ...float64) property.Value {
		values := make([]property.Value, len(ns))
		for i, n := range ns {
			values[i] = property.New(n)
		}
		return property.New(values)
	}
	values := evaluateLocals(t, `
to = range(3)
fromTo = range(2, 5)
empty = range(0)
empty2 = range(3, 3)
reversed = range(5, 2)
`)
	assert.Equal(t, map[string]property.Value{
		"to":       list(0, 1, 2),
		"fromTo":   list(2, 3, 4),
		"empty":    list(),
		"empty2":   list(),
		"reversed": list(5, 4, 3),
	}, values)
}
