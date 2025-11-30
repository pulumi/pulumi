// Copyright 2016-2023, Pulumi Corporation.
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

package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPulumiSourceURL(t *testing.T) {
	t.Parallel()

	source := newGetPulumiSource("aws", "resource")
	url := source.URL()
	
	assert.Equal(t, "https://get.pulumi.com/releases/plugins", url, 
		"getPulumiSource.URL() should return the base get.pulumi.com URL")
}

func TestPluginDownloadURLOverridesWithGetPulumi(t *testing.T) {
	t.Parallel()

	// Parse an override that matches get.pulumi.com
	overrides, err := parsePluginDownloadURLOverrides(
		"https://get.pulumi.com/(.*)=https://my-cdn.example.com/$1")
	
	assert.NoError(t, err, "Should parse override without error")
	assert.Len(t, overrides, 1, "Should have one override")
	
	// Test that it matches the get.pulumi.com URL
	url, ok := overrides.get("https://get.pulumi.com/releases/plugins")
	assert.True(t, ok, "Should match get.pulumi.com URL")
	assert.Equal(t, "https://my-cdn.example.com/releases/plugins", url,
		"Should apply the override correctly")
}

func TestPluginDownloadURLOverridesMultiple(t *testing.T) {
	t.Parallel()

	// Parse multiple overrides including get.pulumi.com
	overrides, err := parsePluginDownloadURLOverrides(
		"https://api.github.com/=https://github-mirror.com/,https://get.pulumi.com/(.*)=https://my-cdn.example.com/$1")
	
	assert.NoError(t, err, "Should parse multiple overrides without error")
	assert.Len(t, overrides, 2, "Should have two overrides")
	
	// Test GitHub override
	githubURL, ok := overrides.get("https://api.github.com/repos/pulumi/pulumi-aws")
	assert.True(t, ok, "Should match GitHub URL")
	assert.Equal(t, "https://github-mirror.com/", githubURL,
		"Should apply GitHub override")
	
	// Test get.pulumi.com override
	pulumiURL, ok := overrides.get("https://get.pulumi.com/releases/plugins")
	assert.True(t, ok, "Should match get.pulumi.com URL")
	assert.Equal(t, "https://my-cdn.example.com/releases/plugins", pulumiURL,
		"Should apply get.pulumi.com override")
}

