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

func TestEntries(t *testing.T) {
	t.Parallel()

	entry := func(key string, value property.Value) property.Value {
		return property.New(map[string]property.Value{"key": property.New(key), "value": value})
	}
	indexedEntry := func(key float64, value property.Value) property.Value {
		return property.New(map[string]property.Value{"key": property.New(key), "value": value})
	}
	list := func(entries ...property.Value) property.Value {
		return property.New(append([]property.Value{}, entries...))
	}
	values := evaluateLocals(t, `
uniform = entries({"a" = 1, "b" = 2})
mixed = entries({"a" = [true], "b" = [true, false]})
empty = entries({})
tuple = entries([1, 2, 3])
mixedTuple = entries([true, "hello"])
emptyTuple = entries([])
list = entries(range(3))
emptyList = entries(range(0))
`)
	assert.Equal(t, map[string]property.Value{
		"uniform": list(entry("a", property.New(1.0)), entry("b", property.New(2.0))),
		"mixed": list(
			entry("a", property.New([]property.Value{property.New(true)})),
			entry("b", property.New([]property.Value{property.New(true), property.New(false)})),
		),
		"empty": list(),
		"tuple": list(
			indexedEntry(0, property.New(1.0)),
			indexedEntry(1, property.New(2.0)),
			indexedEntry(2, property.New(3.0)),
		),
		"mixedTuple": list(indexedEntry(0, property.New(true)), indexedEntry(1, property.New("hello"))),
		"emptyTuple": list(),
		"list": list(
			indexedEntry(0, property.New(0.0)),
			indexedEntry(1, property.New(1.0)),
			indexedEntry(2, property.New(2.0)),
		),
		"emptyList": list(),
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
