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

package analysis

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressionAt(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
	syntax, diags, err := eval.LoadYAMLBytes("def", []byte(def))
	require.NoError(t, err)
	require.Empty(t, diags)

	execContext, err := esc.NewExecContext(make(map[string]esc.Value))
	require.NoError(t, err)

	env, diags := eval.CheckEnvironment(
		t.Context(),
		"def",
		syntax,
		nil,
		testProviders{},
		testEnvironments{},
		execContext,
		false,
		eval.EvalOptions{},
	)
	require.Empty(t, diags)

	analysis := New(*env, map[string]*schema.Schema{"test": testProviderSchema})

	visitExprs(env, func(path string, x esc.Expr) {
		t.Run(path, func(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
			pos := x.Range.Begin
			pos.Byte = 0

			actual, ok := analysis.ExpressionAtPos(pos)
			require.True(t, ok)
			assert.Equal(t, x, *actual)
		})
	})

	t.Run("none", func(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
		actual, ok := analysis.ExpressionAtPos(esc.Pos{})
		require.False(t, ok)
		assert.Nil(t, actual)
	})
}
