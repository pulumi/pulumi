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

package eval

import (
	"context"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainDepth counts a value plus its Trace.Base merge-history chain.
func chainDepth(v *esc.Value) int {
	n := 0
	for cur := v; cur != nil; cur = cur.Trace.Base {
		n++
	}
	return n
}

// inlineEnvironments is an in-memory EnvironmentLoader so tests can build import
// topologies without testdata files.
type inlineEnvironments map[string][]byte

func (e inlineEnvironments) LoadEnvironment(_ context.Context, name string) ([]byte, Decrypter, error) {
	src, ok := e[name]
	if !ok {
		return nil, nil, os.ErrNotExist
	}
	return src, rot128{}, nil
}

// deepImportChain is a 5-environment stack all merging the same key, so the
// Trace.Base chain is deep enough (>2) for Full and None to diverge observably.
var deepImportChain = inlineEnvironments{
	"a":    []byte("values:\n  shared: from-a\n"),
	"b":    []byte("imports:\n  - a\nvalues:\n  shared: from-b\n"),
	"c":    []byte("imports:\n  - b\nvalues:\n  shared: from-c\n"),
	"d":    []byte("imports:\n  - c\nvalues:\n  shared: from-d\n"),
	"root": []byte("imports:\n  - d\nvalues:\n  shared: from-root\n"),
}

func evalShared(t *testing.T, envs inlineEnvironments, opts EvalOptions) esc.Value {
	t.Helper()
	env, _, err := LoadYAMLBytes("root", envs["root"])
	require.NoError(t, err)
	execContext, err := esc.NewExecContext(map[string]esc.Value{})
	require.NoError(t, err)
	result, diags := EvalEnvironment(t.Context(), "root", env, rot128{}, testProviders{},
		envs, execContext, opts)
	require.False(t, diags.HasErrors(), "%v", diags)
	require.NotNil(t, result)
	return result.Properties["shared"]
}

func TestEvalEnvironment_TraceModeFull(t *testing.T) {
	t.Parallel()
	shared := evalShared(t, deepImportChain, EvalOptions{TraceMode: TraceModeFull})
	assert.Greater(t, chainDepth(&shared), 2,
		"Full must preserve a chain deeper than 2 for this 5-environment fixture")
}

// None is the fix for the import-depth payload blowup: the whole Trace is dropped.
func TestEvalEnvironment_TraceModeNone(t *testing.T) {
	t.Parallel()
	shared := evalShared(t, deepImportChain, EvalOptions{TraceMode: TraceModeNone})
	assert.Equal(t, esc.Trace{}, shared.Trace, "None must omit the entire Trace")
	assert.Equal(t, 1, chainDepth(&shared), "None must reduce the chain to the value itself")
}

// Guards the public-API promise: EvalOptions{} stays Full, so no Trace consumer
// regresses without opting in.
func TestEvalEnvironment_ZeroOptionsDefaultsToFull(t *testing.T) {
	t.Parallel()
	zero := evalShared(t, deepImportChain, EvalOptions{})
	full := evalShared(t, deepImportChain, EvalOptions{TraceMode: TraceModeFull})
	assert.Equal(t, chainDepth(&full), chainDepth(&zero),
		"EvalOptions{} must behave exactly like TraceModeFull")
	assert.Greater(t, chainDepth(&zero), 2)
}

// None must apply through nested data, not just the top-level value.
func TestEvalEnvironment_TraceModeNone_NestedData(t *testing.T) {
	t.Parallel()
	envs := inlineEnvironments{
		"a":    []byte("values:\n  obj:\n    k: from-a\n"),
		"root": []byte("imports:\n  - a\nvalues:\n  obj:\n    k: from-root\n"),
	}
	env, _, err := LoadYAMLBytes("root", envs["root"])
	require.NoError(t, err)
	execContext, err := esc.NewExecContext(map[string]esc.Value{})
	require.NoError(t, err)

	result, diags := EvalEnvironment(t.Context(), "root", env, rot128{}, testProviders{},
		envs, execContext, EvalOptions{TraceMode: TraceModeNone})
	require.False(t, diags.HasErrors(), "%v", diags)
	require.NotNil(t, result)

	obj := result.Properties["obj"]
	assert.Nil(t, obj.Trace.Base, "None must drop Base on the object")
	nested := obj.Value.(map[string]esc.Value)["k"]
	assert.Nil(t, nested.Trace.Base, "None must drop Base on nested data too")
}
