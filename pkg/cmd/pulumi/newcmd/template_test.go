// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"https://user:pass@example.com/path?param=value", "https://example.com/path"},
		{"https://user:pass@example.com", "https://example.com"},
		{"https://example.com/path?param=value", "https://example.com/path"},
		{"ssh://user@hostname/project/repo", "ssh://hostname/project/repo"},
		{"typescript", "typescript"},
		{"aws-typescript", "aws-typescript"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := sanitizeTemplate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
