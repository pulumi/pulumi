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
	"context"
	"fmt"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/esc/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribe(t *testing.T) {
	syntax, diags, err := eval.LoadYAMLBytes("def", []byte(def))
	require.NoError(t, err)
	require.Empty(t, diags)

	execContext, err := esc.NewExecContext(make(map[string]esc.Value))
	require.NoError(t, err)

	env, diags := eval.CheckEnvironment(context.Background(), "def", syntax, nil, testProviders{}, testEnvironments{}, execContext, false)
	require.Empty(t, diags)

	analysis := New(*env, map[string]*schema.Schema{"test": testProviderSchema})

	expected := map[esc.Pos]string{
		{Line: 5, Column: 5}:   "Decodes a string from its Base64 representation.",
		{Line: 5, Column: 21}:  "string",
		{Line: 7, Column: 5}:   "Decodes a value from its JSON representation.",
		{Line: 7, Column: 19}:  "string",
		{Line: 9, Column: 5}:   "Concatenates the elements of its second argument to create a single string. The first argument is placed between each element in the result.",
		{Line: 9, Column: 22}:  "array",
		{Line: 11, Column: 5}:  "Fetches values from an external source when the environment is opened.",
		{Line: 12, Column: 7}:  "The URL of the Vault server. Must contain a scheme and hostname, but no path.",
		{Line: 12, Column: 16}: "The URL of the Vault server. Must contain a scheme and hostname, but no path.",
		{Line: 13, Column: 7}:  "Options for JWT login. JWT login uses an OIDC token issued by the Pulumi Cloud to generate an ephemeral token.",
		{Line: 14, Column: 9}:  "The name of the authentication engine mount.",
		{Line: 14, Column: 16}: "The name of the authentication engine mount.",
		{Line: 15, Column: 9}:  "The name of the role to use for login.",
		{Line: 15, Column: 15}: "The name of the role to use for login.",
		{Line: 16, Column: 7}:  "Options for token login. Token login creates an ephemeral child token.",
		{Line: 17, Column: 9}:  "The display name of the ephemeral token. Defaults to 'pulumi'.",
		{Line: 17, Column: 22}: "The display name of the ephemeral token. Defaults to 'pulumi'.",
		{Line: 18, Column: 9}:  "The parent token.",
		{Line: 18, Column: 16}: "The parent token.",
		{Line: 19, Column: 9}:  "The maximum TTL of the ephemeral token.",
		{Line: 19, Column: 17}: "The maximum TTL of the ephemeral token.",
		{Line: 21, Column: 5}:  "Marks a value as secret.",
		{Line: 24, Column: 5}:  "Encodes a string into its Base64 representation.",
		{Line: 24, Column: 19}: "string",
		{Line: 26, Column: 5}:  "Encodes a value into its JSON representation.",
		{Line: 26, Column: 17}: "any",
		{Line: 28, Column: 5}:  "Encodes a value into its string representation.",
		{Line: 28, Column: 19}: "any",
		{Line: 30, Column: 5}:  "Fetches values from an external source when the environment is opened.",
		{Line: 30, Column: 21}: "any",
		{Line: 32, Column: 11}: "any",
	}

	check := func(pos esc.Pos) {
		pos.Byte = 0
		t.Run(fmt.Sprintf("%v:%v", pos.Line, pos.Column), func(t *testing.T) {
			expectedDescription, expectedOK := expected[pos]
			actualDescription, actualOK := analysis.Describe(pos)
			require.Equal(t, expectedOK, actualOK)
			assert.Equal(t, expectedDescription, actualDescription)
		})
	}

	visitExprs(env, func(path string, x esc.Expr) {
		check(x.Range.Begin)
		for _, rng := range x.KeyRanges {
			check(rng.Begin)
		}
		if x.Builtin != nil {
			check(x.Builtin.NameRange.Begin)
		}
	})

	t.Run("none", func(t *testing.T) {
		actual, ok := analysis.Describe(esc.Pos{})
		require.False(t, ok)
		assert.Equal(t, "", actual)
	})
}

func TestDescribeOpen(t *testing.T) {
	syntax, diags, err := eval.LoadYAMLBytes("def", []byte(`{"values": {"open": {"fn::open": {"provider": "test", "inputs": {"address": "foo"}}}}}`))
	require.NoError(t, err)
	require.Empty(t, diags)

	execContext, err := esc.NewExecContext(make(map[string]esc.Value))
	require.NoError(t, err)

	env, diags := eval.CheckEnvironment(context.Background(), "def", syntax, nil, testProviders{}, testEnvironments{}, execContext, false)
	require.Empty(t, diags)

	analysis := New(*env, map[string]*schema.Schema{"test": testProviderSchema})

	expected := "Fetches values from an external source when the environment is opened."
	actual, ok := analysis.Describe(esc.Pos{Line: 1, Column: 23})
	require.True(t, ok)
	assert.Equal(t, expected, actual)
}
