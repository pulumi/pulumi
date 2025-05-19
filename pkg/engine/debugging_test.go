// Copyright 2025, Pulumi Corporation.
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

package engine

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func TestAttachDebugger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		attachDebugger []string
		spec           plugin.DebugSpec
		expected       bool
	}{
		{
			name:           "all",
			attachDebugger: []string{"all"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypeProgram,
			},
			expected: true,
		},
		{
			name:           "program",
			attachDebugger: []string{"program"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypeProgram,
			},
			expected: true,
		},
		{
			name:           "plugins",
			attachDebugger: []string{"plugins"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypePlugin,
				Name: "test-plugin",
			},
			expected: true,
		},
		{
			name:           "plugin:plugin-name",
			attachDebugger: []string{"plugin:plugin-name"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypePlugin,
				Name: "plugin-name",
			},
			expected: true,
		},
		{
			name:           "plugin:other-plugin-name",
			attachDebugger: []string{"plugin:plugin-name"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypePlugin,
				Name: "other-plugin-name",
			},
			expected: false,
		},
		{
			name:           "program with plugin spec",
			attachDebugger: []string{"program"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypePlugin,
				Name: "test-plugin",
			},
			expected: false,
		},
		{
			name:           "plugin with program spec",
			attachDebugger: []string{"plugin:plugin-name"},
			spec: plugin.DebugSpec{
				Type: plugin.DebugTypeProgram,
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			debugContext := newDebugContext(eventEmitter{}, test.attachDebugger)
			result := debugContext.AttachDebugger(test.spec)
			require.Equal(t, test.expected, result)
		})
	}
}
