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

package cli

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLooksLikeSecret(t *testing.T) {
	t.Parallel()

	highEntropyValue := "aB3$xZ9!mK7@qW2"

	tests := []struct {
		name     string
		path     resource.PropertyPath
		node     yaml.Node
		expected bool
	}{
		{
			name: "non-scalar node returns false",
			path: resource.PropertyPath{"password"},
			node: yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
		},
		{
			name: "non-string tag returns false",
			path: resource.PropertyPath{"password"},
			node: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "12345"},
		},
		{
			name: "key does not match pattern",
			path: resource.PropertyPath{"username"},
			node: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
		},
		{
			name: "key matches pattern but low entropy value",
			path: resource.PropertyPath{"password"},
			node: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "hello"},
		},
		{
			name:     "key matches password with high entropy value",
			path:     resource.PropertyPath{"password"},
			node:     yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
			expected: true,
		},
		{
			name:     "key matches token with high entropy value",
			path:     resource.PropertyPath{"token"},
			node:     yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
			expected: true,
		},
		{
			name:     "key matches secret with high entropy value",
			path:     resource.PropertyPath{"secret"},
			node:     yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
			expected: true,
		},
		{
			name:     "case insensitive key match",
			path:     resource.PropertyPath{"Password"},
			node:     yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
			expected: true,
		},
		{
			name: "integer path element returns false",
			path: resource.PropertyPath{0},
			node: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
		},
		{
			name:     "nested path uses last element",
			path:     resource.PropertyPath{"config", "dbPassword"},
			node:     yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: highEntropyValue},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := looksLikeSecret(tt.path, tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}
