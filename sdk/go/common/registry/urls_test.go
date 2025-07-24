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

package registry

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseVersion(v string) *semver.Version {
	version, err := semver.Parse(v)
	if err != nil {
		panic(err)
	}
	return &version
}

func TestParseRegistryURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expected    *URLInfo
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid registry URL with version",
			url:  "registry://templates/test-source/test-publisher/test-template@1.0.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "test-source",
				publisher:    "test-publisher",
				name:         "test-template",
				version:      mustParseVersion("1.0.0"),
			},
			expectError: false,
		},
		{
			name: "valid registry URL without version",
			url:  "registry://templates/test-source/test-publisher/test-template",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "test-source",
				publisher:    "test-publisher",
				name:         "test-template",
				version:      nil,
			},
			expectError: false,
		},
		{
			name: "valid registry URL with latest version",
			url:  "registry://templates/test-source/test-publisher/test-template@latest",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "test-source",
				publisher:    "test-publisher",
				name:         "test-template",
				version:      nil, // latest is treated as nil version
			},
			expectError: false,
		},
		{
			name: "valid registry URL with complex version",
			url:  "registry://templates/my-source/my-publisher/my-template@2.1.0-beta.1",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "my-source",
				publisher:    "my-publisher",
				name:         "my-template",
				version:      mustParseVersion("2.1.0-beta.1"),
			},
			expectError: false,
		},
		{
			name: "valid registry URL with double-encoded name",
			url:  "registry://templates/test-source/test-publisher/test%252Ftemplate@1.0.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "test-source",
				publisher:    "test-publisher",
				name:         "test/template",
				version:      mustParseVersion("1.0.0"),
			},
			expectError: false,
		},
		{
			name: "valid registry URL with packages resource type",
			url:  "registry://packages/test-source/test-publisher/test-package@1.0.0",
			expected: &URLInfo{
				resourceType: "packages",
				source:       "test-source",
				publisher:    "test-publisher",
				name:         "test-package",
				version:      mustParseVersion("1.0.0"),
			},
			expectError: false,
		},
		{
			name:        "invalid URL format",
			url:         "not-a-valid-url",
			expectError: true,
			errorMsg:    "invalid registry URL",
		},
		{
			name:        "wrong scheme",
			url:         "https://test-host/templates/test-source/test-publisher/test-template@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL scheme",
		},
		{
			name:        "missing resource type",
			url:         "registry:////test-source/test-publisher/test-template@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL: missing resource type",
		},
		{
			name:        "missing source",
			url:         "registry://templates//test-publisher/test-template@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL: missing source",
		},
		{
			name:        "missing publisher",
			url:         "registry://templates/test-source//test-template@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL: missing publisher",
		},
		{
			name:        "missing name",
			url:         "registry://templates/test-source/test-publisher/@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL: missing name",
		},
		{
			name:        "missing version",
			url:         "registry://templates/test-source/test-publisher/test-template@",
			expectError: true,
			errorMsg:    "invalid registry URL: missing version",
		},
		{
			name:        "too many path segments",
			url:         "registry://templates/test-source/test-publisher/test-template/extra@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL format",
		},
		{
			name:        "too few path segments",
			url:         "registry://templates/test-template@1.0.0",
			expectError: true,
			errorMsg:    "invalid registry URL format",
		},
		{
			name:        "multiple @ symbols",
			url:         "registry://templates/test-source/test-publisher/test-template@1.0.0@extra",
			expectError: true,
			errorMsg:    "invalid registry URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRegistryURL(tt.url)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ResourceType(), result.ResourceType())
				assert.Equal(t, tt.expected.Source(), result.Source())
				assert.Equal(t, tt.expected.Publisher(), result.Publisher())
				assert.Equal(t, tt.expected.Name(), result.Name())
				if tt.expected.Version() == nil {
					assert.Nil(t, result.Version())
				} else {
					require.NotNil(t, result.Version())
					assert.True(t, tt.expected.Version().Equals(*result.Version()))
				}
			}
		})
	}
}

func TestURLInfoString(t *testing.T) {
	t.Run("with version", func(t *testing.T) {
		info := &URLInfo{
			resourceType: "templates",
			source:       "test-source",
			publisher:    "test-publisher",
			name:         "test-template",
			version:      mustParseVersion("1.0.0"),
		}

		expected := "registry://templates/test-source/test-publisher/test-template@1.0.0"
		assert.Equal(t, expected, info.String())
	})

	t.Run("without version", func(t *testing.T) {
		info := &URLInfo{
			resourceType: "templates",
			source:       "test-source",
			publisher:    "test-publisher",
			name:         "test-template",
			version:      nil,
		}

		expected := "registry://templates/test-source/test-publisher/test-template"
		assert.Equal(t, expected, info.String())
	})
}

func TestRegistryURL_RoundTrip(t *testing.T) {
	t.Run("with version", func(t *testing.T) {
		originalURL := "registry://templates/test-source/test-publisher/test-template@1.0.0"

		info, err := ParseRegistryURL(originalURL)
		require.NoError(t, err)

		roundTripURL := info.String()

		assert.Equal(t, originalURL, roundTripURL)
	})

	t.Run("without version", func(t *testing.T) {
		originalURL := "registry://templates/test-source/test-publisher/test-template"

		info, err := ParseRegistryURL(originalURL)
		require.NoError(t, err)

		roundTripURL := info.String()

		assert.Equal(t, originalURL, roundTripURL)
	})

	t.Run("with latest version", func(t *testing.T) {
		originalURL := "registry://templates/test-source/test-publisher/test-template@latest"

		info, err := ParseRegistryURL(originalURL)
		require.NoError(t, err)

		roundTripURL := info.String()

		// latest should be omitted in round-trip (becomes empty version)
		assert.Equal(t, "registry://templates/test-source/test-publisher/test-template", roundTripURL)
	})

	t.Run("with encoded name", func(t *testing.T) {
		info := &URLInfo{
			resourceType: "templates",
			source:       "test-source",
			publisher:    "test-publisher",
			name:         "test/template",
			version:      nil,
		}

		result := info.String()
		assert.Equal(t, "registry://templates/test-source/test-publisher/test%252Ftemplate", result)
	})

	t.Run("double-encoded name round-trip", func(t *testing.T) {
		info := &URLInfo{
			resourceType: "templates",
			source:       "test-source",
			publisher:    "test-publisher",
			name:         "test/template",
			version:      nil,
		}

		urlString := info.String()
		assert.Equal(t, "registry://templates/test-source/test-publisher/test%252Ftemplate", urlString)

		parsed, err := ParseRegistryURL(urlString)
		require.NoError(t, err)
		assert.Equal(t, info.ResourceType(), parsed.ResourceType())
		assert.Equal(t, info.Source(), parsed.Source())
		assert.Equal(t, info.Publisher(), parsed.Publisher())
		assert.Equal(t, info.Name(), parsed.Name())
		assert.Equal(t, info.Version(), parsed.Version())
	})
}

func TestParseRegistryURLOrPartial(t *testing.T) {
	tests := []struct {
		name        string
		registryURL string
		expected    *URLInfo
		expectError bool
		errorMsg    string
	}{
		{
			name:        "bare template name",
			registryURL: "csharp-documented",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "",
				publisher:    "",
				name:         "csharp-documented",
				version:      nil,
			},
		},
		{
			name:        "bare template name with version",
			registryURL: "csharp-documented@1.1.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "",
				publisher:    "",
				name:         "csharp-documented",
				version:      mustParseVersion("1.1.0"),
			},
		},
		{
			name:        "bare template name with latest version",
			registryURL: "csharp-documented@latest",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "",
				publisher:    "",
				name:         "csharp-documented",
				version:      nil, // latest becomes nil
			},
		},
		{
			name:        "publisher/name format",
			registryURL: "pulumi_local/csharp-documented",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      nil,
			},
		},
		{
			name:        "publisher/name with version",
			registryURL: "pulumi_local/csharp-documented@1.1.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      mustParseVersion("1.1.0"),
			},
		},
		{
			name:        "source/publisher/name format",
			registryURL: "private/pulumi_local/csharp-documented",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "private",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      nil,
			},
		},
		{
			name:        "source/publisher/name with version",
			registryURL: "private/pulumi_local/csharp-documented@1.1.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "private",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      mustParseVersion("1.1.0"),
			},
		},
		{
			name:        "full registry URL",
			registryURL: "registry://templates/private/pulumi_local/csharp-documented@1.1.0",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "private",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      mustParseVersion("1.1.0"),
			},
		},
		{
			name:        "full registry URL with encoding",
			registryURL: "registry://templates/private/pulumi_local/csharp%252Ddocumented",
			expected: &URLInfo{
				resourceType: "templates",
				source:       "private",
				publisher:    "pulumi_local",
				name:         "csharp-documented",
				version:      nil,
			},
		},
		{
			name:        "too many path segments",
			registryURL: "a/b/c/d/e",
			expectError: true,
			errorMsg:    "invalid registry URL format",
		},
		{
			name:        "empty",
			registryURL: "",
			expectError: true,
			errorMsg:    "missing name",
		},
		{
			name:        "empty version",
			registryURL: "template@",
			expectError: true,
			errorMsg:    "missing version after @",
		},
		{
			name:        "empty name with version",
			registryURL: "@1.0.0",
			expectError: true,
			errorMsg:    "missing name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRegistryURLOrPartial(tt.registryURL, "templates")

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ResourceType(), result.ResourceType())
				assert.Equal(t, tt.expected.Source(), result.Source())
				assert.Equal(t, tt.expected.Publisher(), result.Publisher())
				assert.Equal(t, tt.expected.Name(), result.Name())
				if tt.expected.Version() == nil {
					assert.Nil(t, result.Version())
				} else {
					require.NotNil(t, result.Version())
					assert.True(t, tt.expected.Version().Equals(*result.Version()))
				}
			}
		})
	}
}

func TestIsRegistryURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"registry://templates/private/pulumi_local/template@latest", true},
		{"registry://templates/private/pulumi_local/template@1.0.0", true},
		{"registry://templates/private/pulumi_local/template", true},
		{"registry://packages/github/pulumi/aws", true},
		{"https://github.com/pulumi/templates", false},
		{"template-name", false},
		{"private/pulumi_local/template", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsRegistryURL(tt.input)
			assert.Equal(t, tt.expected, result, "IsRegistryURL(%q) should return %v", tt.input, tt.expected)
		})
	}
}
