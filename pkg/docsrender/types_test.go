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

package docsrender

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Getting Started", "getting-started"},
		{"foo--bar", "foo-bar"},
		{"  leading spaces  ", "leading-spaces"},
		{"Special!@#Characters$%^Here", "specialcharactershere"},
		{"already-slug", "already-slug"},
		{"MixedCase123", "mixedcase123"},
		{"", ""},
		{"with_underscores_here", "with-underscores-here"},
		{"multiple   spaces", "multiple-spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, slugify(tt.input))
		})
	}
}

func TestIsRegistryPath(t *testing.T) {
	t.Parallel()

	assert.True(t, IsRegistryPath("registry/packages/aws"))
	assert.True(t, IsRegistryPath("/registry/packages/aws"))
	assert.True(t, IsRegistryPath("registry"))
	assert.False(t, IsRegistryPath("docs/iac/concepts"))
	assert.False(t, IsRegistryPath(""))
	assert.False(t, IsRegistryPath("registryfoo"))
}

func TestIsAPIDocsPath(t *testing.T) {
	t.Parallel()

	assert.True(t, IsAPIDocsPath("registry/packages/aws/api-docs/s3"))
	assert.True(t, IsAPIDocsPath("/registry/packages/aws/api-docs/s3/bucket"))
	assert.False(t, IsAPIDocsPath("registry/packages/aws"))
	assert.False(t, IsAPIDocsPath("registry/packages/aws/install"))
	assert.False(t, IsAPIDocsPath("docs/iac/concepts"))
	assert.False(t, IsAPIDocsPath(""))
}
