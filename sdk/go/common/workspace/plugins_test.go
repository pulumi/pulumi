// Copyright 2016-2019, Pulumi Corporation.
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

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func TestPluginSelection_ExactMatch(t *testing.T) {
	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.0")
	v3 := semver.MustParse("0.3.0")
	candidatePlugins := []PluginInfo{
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result, err := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NoError(t, err)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.2.0", result.Version.String())
}

func TestPluginSelection_ExactMatchNotFound(t *testing.T) {
	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.1")
	v3 := semver.MustParse("0.3.0")
	candidatePlugins := []PluginInfo{
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	_, err := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.Error(t, err)
}

func TestPluginSelection_PatchVersionSlide(t *testing.T) {
	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.0")
	v21 := semver.MustParse("0.2.1")
	v3 := semver.MustParse("0.3.0")
	candidatePlugins := []PluginInfo{
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v21,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange(">=0.2.0 <0.3.0")
	result, err := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NoError(t, err)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.2.1", result.Version.String())
}

func TestPluginSelection_EmptyVersionNoAlternatives(t *testing.T) {
	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.1")
	v3 := semver.MustParse("0.3.0")
	candidatePlugins := []PluginInfo{
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result, err := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NoError(t, err)
	assert.Equal(t, "myplugin", result.Name)
	assert.Nil(t, result.Version)
}

func TestPluginSelection_EmptyVersionWithAlternatives(t *testing.T) {
	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.0")
	v3 := semver.MustParse("0.3.0")
	candidatePlugins := []PluginInfo{
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result, err := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NoError(t, err)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.2.0", result.Version.String())
}

func TestPluginDownloadUrl(t *testing.T) {
	t.Run("Test Downloading From GitHub Releases", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "aws",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		serverURL := buildGitHubReleasesPluginURL(info.Kind, info.Name, info.Version, "darwin", "amd64")
		assert.Equal(t,
			"https://github.com/pulumi/pulumi-aws/releases/download/v4.32.0/pulumi-resource-aws-v4.32.0-darwin-amd64.tar.gz",
			serverURL)
	})
	t.Run("Test Downloading From get.pulumi.com", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "aws",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		serverURL := buildPulumiHostedPluginURL(info.Kind, info.Name, info.Version, "darwin", "amd64")
		assert.Equal(t,
			"https://get.pulumi.com/releases/plugins/pulumi-resource-aws-v4.32.0-darwin-amd64.tar.gz", serverURL)
	})
	t.Run("Test Downloading From Custom Server URL", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "aws",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		serverURL := buildUserSpecifiedPluginURL(info.PluginDownloadURL, info.Kind, info.Name, info.Version,
			"darwin", "amd64")
		assert.Equal(t,
			"https://customurl.jfrog.io/artifactory/pulumi-packages/package-name"+
				"/pulumi-resource-aws-v4.32.0-darwin-amd64.tar.gz",
			serverURL)
	})
}

func TestInterpolateURL(t *testing.T) {
	version := semver.MustParse("1.0.0")
	const os = "linux"
	const arch = "amd64"
	assert.Equal(t, "", interpolateURL("", version, os, arch))
	assert.Equal(t,
		"https://get.pulumi.com/releases/plugins",
		interpolateURL("https://get.pulumi.com/releases/plugins", version, os, arch))
	assert.Equal(t,
		"https://github.com/org/repo/releases/download/1.0.0",
		interpolateURL("https://github.com/org/repo/releases/download/${VERSION}", version, os, arch))
	assert.Equal(t,
		"https://github.com/org/repo/releases/download/1.0.0/linux/amd64",
		interpolateURL("https://github.com/org/repo/releases/download/${VERSION}/${OS}/${ARCH}", version, os, arch))
}
