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
	"strings"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRuntimeOptionsPrompts_Typechecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		options          map[string]interface{}
		expectedKeys     []string
		expectedDefaults map[string]string
		expectedChoices  map[string][]string
	}{
		{
			name:             "No toolchain selected",
			options:          map[string]interface{}{},
			expectedKeys:     []string{"toolchain"},
			expectedDefaults: map[string]string{"toolchain": "pip"},
			expectedChoices: map[string][]string{
				"toolchain": {"pip", "poetry", "uv"},
			},
		},
		{
			name:         "Pip toolchain selected",
			options:      map[string]interface{}{"toolchain": "pip"},
			expectedKeys: []string{"virtualenv"},
			expectedDefaults: map[string]string{
				"virtualenv": "venv",
			},
			expectedChoices: map[string][]string{
				"virtualenv": {"venv"},
			},
		},
		{
			name:             "Poetry toolchain selected",
			options:          map[string]interface{}{"toolchain": "poetry"},
			expectedKeys:     []string{},
			expectedDefaults: map[string]string{},
			expectedChoices:  map[string][]string{},
		},
		{
			name:             "Uv toolchain selected",
			options:          map[string]interface{}{"toolchain": "uv"},
			expectedKeys:     []string{},
			expectedDefaults: map[string]string{},
			expectedChoices:  map[string][]string{},
		},
		{
			name:             "Typechecker already set",
			options:          map[string]interface{}{"toolchain": "pip", "typechecker": "pyright"},
			expectedKeys:     []string{"virtualenv"},
			expectedDefaults: map[string]string{"virtualenv": "venv"},
			expectedChoices: map[string][]string{
				"virtualenv": {"venv"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host := &pythonLanguageHost{}

			// Convert options to structpb.Struct
			opts, err := structpb.NewStruct(tt.options)
			require.NoError(t, err)

			req := &pulumirpc.RuntimeOptionsRequest{
				Info: &pulumirpc.ProgramInfo{
					Options: opts,
				},
			}

			resp, err := host.RuntimeOptionsPrompts(t.Context(), req)
			require.NoError(t, err)

			// Check that we got the expected prompt keys
			actualKeys := make([]string, len(resp.Prompts))
			for i, prompt := range resp.Prompts {
				actualKeys[i] = prompt.Key
			}
			assert.ElementsMatch(t, tt.expectedKeys, actualKeys)

			// Check defaults and choices
			for _, prompt := range resp.Prompts {
				if expectedDefault, ok := tt.expectedDefaults[prompt.Key]; ok {
					assert.Equal(t, expectedDefault, prompt.Default.StringValue,
						"Default for %s should be %s", prompt.Key, expectedDefault)
				}

				if expectedChoices, ok := tt.expectedChoices[prompt.Key]; ok {
					actualChoices := make([]string, len(prompt.Choices))
					for i, choice := range prompt.Choices {
						// Skip choices marked as "[not found]"
						if !strings.Contains(choice.DisplayName, "[not found]") {
							actualChoices[i] = choice.StringValue
						}
					}
					// Filter out empty strings from actualChoices
					var filteredChoices []string
					for _, choice := range actualChoices {
						if choice != "" {
							filteredChoices = append(filteredChoices, choice)
						}
					}
					assert.Subset(t, expectedChoices, filteredChoices,
						"Choices for %s should contain %v", prompt.Key, expectedChoices)
				}
			}
		})
	}
}

func TestRuntimeOptionsPrompts_NoAutomaticTypechecker(t *testing.T) {
	t.Parallel()

	host := &pythonLanguageHost{}

	// Test with pip toolchain - should not get automatic typechecker prompt
	opts, err := structpb.NewStruct(map[string]interface{}{"toolchain": "pip"})
	require.NoError(t, err)

	req := &pulumirpc.RuntimeOptionsRequest{
		Info: &pulumirpc.ProgramInfo{
			Options: opts,
		},
	}

	resp, err := host.RuntimeOptionsPrompts(t.Context(), req)
	require.NoError(t, err)

	// Verify there is no typechecker prompt
	var typecheckerPrompt *pulumirpc.RuntimeOptionPrompt
	for _, prompt := range resp.Prompts {
		if prompt.Key == "typechecker" {
			typecheckerPrompt = prompt
			break
		}
	}

	assert.Nil(t, typecheckerPrompt, "Should not have an automatic typechecker prompt")
}
