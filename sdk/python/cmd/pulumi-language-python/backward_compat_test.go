// Copyright 2016-2025, Pulumi Corporation.
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

package main

import (
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestBackwardCompatibility_NoTypechecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options map[string]interface{}
	}{
		{
			name:    "Empty options",
			options: map[string]interface{}{},
		},
		{
			name: "Only toolchain pip",
			options: map[string]interface{}{
				"toolchain": "pip",
			},
		},
		{
			name: "Only toolchain poetry",
			options: map[string]interface{}{
				"toolchain": "poetry",
			},
		},
		{
			name: "Toolchain with virtualenv",
			options: map[string]interface{}{
				"toolchain":  "pip",
				"virtualenv": "myenv",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test that parseOptions works without typechecker
			pythonOpts, err := parseOptions("/root", "/root/program", tt.options)
			require.NoError(t, err)
			// Default typechecker should be empty (no typechecker)
			assert.Equal(t, toolchain.TypeCheckerNone, pythonOpts.Typechecker)

			// Test that RuntimeOptionsPrompts continues to work
			host := &pythonLanguageHost{}
			opts, err := structpb.NewStruct(tt.options)
			require.NoError(t, err)

			req := &pulumirpc.RuntimeOptionsRequest{
				Info: &pulumirpc.ProgramInfo{
					Options: opts,
				},
			}

			resp, err := host.RuntimeOptionsPrompts(t.Context(), req)
			require.NoError(t, err)
			assert.NotNil(t, resp)

			foundTypechecker := false
			for _, prompt := range resp.Prompts {
				if prompt.Key == "typechecker" {
					foundTypechecker = true
					break
				}
			}
			assert.False(t, foundTypechecker, "typechecker prompt should not be automatically present")
		})
	}
}

func TestParseOptions_ValidTypechecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options map[string]interface{}
	}{
		{
			name: "Typechecker mypy",
			options: map[string]interface{}{
				"toolchain":   "pip",
				"typechecker": "mypy",
			},
		},
		{
			name: "Typechecker pyright",
			options: map[string]interface{}{
				"toolchain":   "pip",
				"typechecker": "pyright",
			},
		},
		{
			name: "Typechecker none",
			options: map[string]interface{}{
				"toolchain":   "pip",
				"typechecker": "none",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test that parseOptions doesn't return an error for valid typechecker values
			_, err := parseOptions("/root", "/root/program", tt.options)
			require.NoError(t, err, "parseOptions should accept %s as a valid typechecker option", tt.options["typechecker"])
		})
	}
}

func TestParseOptions_InvalidTypechecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		options     map[string]interface{}
		expectedErr string
	}{
		{
			name: "Invalid typechecker value",
			options: map[string]interface{}{
				"toolchain":   "pip",
				"typechecker": "invalid",
			},
			expectedErr: "unsupported typechecker option: invalid",
		},
		{
			name: "Non-string typechecker",
			options: map[string]interface{}{
				"toolchain":   "pip",
				"typechecker": 123,
			},
			expectedErr: "typechecker option must be a string",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseOptions("/root", "/root/program", tt.options)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
