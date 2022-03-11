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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func TestPluginSelection_ExactMatch(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

type mockReadCloser struct {
	bytes    []byte
	position int64
}

func (mock *mockReadCloser) Read(p []byte) (int, error) {
	if mock.position == int64(len(mock.bytes)) {
		return 0, io.EOF
	}
	slice := mock.bytes[mock.position:]
	copied := copy(p, slice)
	mock.position = mock.position + int64(copied)
	return copied, nil
}

func (mock *mockReadCloser) Close() error {
	return nil
}

func newMockReadCloser(data []byte) (io.ReadCloser, int64, error) {
	return &mockReadCloser{bytes: data}, int64(len(data)), nil
}

func newMockReadCloserString(data string) (io.ReadCloser, int64, error) {
	bytes := []byte(data)
	return &mockReadCloser{bytes: bytes}, int64(len(bytes)), nil
}

//nolint:paralleltest // mutates environment variables
func TestPluginDownload(t *testing.T) {
	t.Run("Test Downloading From Pulumi GitHub Releases", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://github.com/pulumi/pulumi-mockdl/releases/download/v4.32.0/"+
					"pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser([]byte{})
		}
		_, _, err := source.Download(*info.Version, "darwin", "amd64", getHTTPResponse)
		assert.Nil(t, err)
	})
	t.Run("Test Downloading From get.pulumi.com", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://github.com/pulumi/pulumi-mockdl/releases/"+
				"download/v4.32.0/pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz" {
				return nil, -1, errors.New("404 not found")
			}
			assert.Equal(t,
				"https://get.pulumi.com/releases/plugins/pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser([]byte{})
		}
		_, _, err := source.Download(*info.Version, "darwin", "amd64", getHTTPResponse)
		assert.Nil(t, err)
	})
	t.Run("Test Downloading From Custom Server URL", func(t *testing.T) {
		t.Skip()

		version := semver.MustParse("4.32.0")
		info := PluginInfo{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://customurl.jfrog.io/artifactory/pulumi-packages/"+
					"package-name/pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser([]byte{})
		}
		_, _, err := source.Download(*info.Version, "darwin", "amd64", getHTTPResponse)
		assert.Nil(t, err)
	})
	t.Run("Test Downloading From GitHub Private Releases", func(t *testing.T) {
		t.Skip()

		token := "RaNd0m70K3n_"
		os.Setenv("PULUMI_EXPERIMENTAL", "true")
		os.Setenv("GITHUB_REPOSITORY_OWNER", "private")
		os.Setenv("GITHUB_TOKEN", token)
		version := semver.MustParse("1.22.0")
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "private",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://github.com/pulumi/pulumi-private/releases/download/"+
				"v1.22.0/pulumi-resource-private-v1.22.0-darwin-amd64.tar.gz" {
				return nil, -1, errors.New("404 not found")
			}

			if req.URL.String() == "https://api.github.com/repos/private/pulumi-private/releases/tags/v1.22.0" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authentication"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.github.com/repos/private/pulumi-private/releases/assets/654321",
						"name": "pulumi-private_1.22.0_checksums.txt"
					  },
					  {
						"url": "https://api.github.com/repos/private/pulumi-private/releases/assets/123456",
						"name": "pulumi-resource-private-v1.22.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.github.com/repos/private/pulumi-private/releases/assets/123456", req.URL.String())
			assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authentication"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser([]byte{})
		}
		_, _, err := source.Download(*info.Version, "darwin", "amd64", getHTTPResponse)
		assert.Nil(t, err)
	})
}

//nolint:paralleltest // mutates environment variables
func TestPluginGetLatestVersion(t *testing.T) {
	t.Run("Test GetLatestVersion From Pulumi GitHub Releases", func(t *testing.T) {
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://api.github.com/repos/pulumi/pulumi-mock-latest/releases/latest",
				req.URL.String())
			// Minimal JSON from the releases API to get the test to pass
			return newMockReadCloserString(`{
				"tag_name": "v4.37.5"
			}`)
		}
		version, err := source.GetLatestVersion(getHTTPResponse)
		assert.Nil(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Test GetLatestVersion From Custom Server URL", func(t *testing.T) {
		info := PluginInfo{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		source := info.GetSource()
		version, err := source.GetLatestVersion(getHTTPResponse)
		assert.Nil(t, version)
		assert.Equal(t, "GetLatestVersion is not supported for plugins using PluginDownloadURL", err.Error())
	})
	t.Run("Test Downloading From GitHub Private Releases", func(t *testing.T) {
		token := "RaNd0m70K3n_"
		os.Setenv("PULUMI_EXPERIMENTAL", "true")
		os.Setenv("GITHUB_REPOSITORY_OWNER", "private")
		os.Setenv("GITHUB_TOKEN", token)
		info := PluginInfo{
			PluginDownloadURL: "",
			Name:              "private",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("1.0.2")
		source := info.GetSource()
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-private/releases/latest" {
				return nil, -1, errors.New("404 not found")
			}

			if req.URL.String() == "https://api.github.com/repos/private/pulumi-private/releases/latest" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authentication"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v1.0.2"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(getHTTPResponse)
		assert.Nil(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
}

func TestInterpolateURL(t *testing.T) {
	t.Parallel()

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

func TestParsePluginDownloadURLOverride(t *testing.T) {
	t.Parallel()

	type match struct {
		name string
		url  string
		ok   bool
	}

	tests := []struct {
		input       string
		expected    pluginDownloadOverrideArray
		matches     []match
		expectError bool
	}{
		{
			input:    "",
			expected: nil,
		},
		{
			input: "^foo.*=https://foo",
			expected: pluginDownloadOverrideArray{
				{
					reg: regexp.MustCompile("^foo.*"),
					url: "https://foo",
				},
			},
			matches: []match{
				{
					name: "foo",
					url:  "https://foo",
					ok:   true,
				},
				{
					name: "foo-bar",
					url:  "https://foo",
					ok:   true,
				},
				{
					name: "fo",
					url:  "",
					ok:   false,
				},
				{
					name: "",
					url:  "",
					ok:   false,
				},
				{
					name: "nope",
					url:  "",
					ok:   false,
				},
			},
		},
		{
			input: "^foo.*=https://foo,^bar.*=https://bar",
			expected: pluginDownloadOverrideArray{
				{
					reg: regexp.MustCompile("^foo.*"),
					url: "https://foo",
				},
				{
					reg: regexp.MustCompile("^bar.*"),
					url: "https://bar",
				},
			},
			matches: []match{
				{
					name: "foo",
					url:  "https://foo",
					ok:   true,
				},
				{
					name: "foo-bar",
					url:  "https://foo",
					ok:   true,
				},
				{
					name: "fo",
					url:  "",
					ok:   false,
				},
				{
					name: "",
					url:  "",
					ok:   false,
				},
				{
					name: "bar",
					url:  "https://bar",
					ok:   true,
				},
				{
					name: "barbaz",
					url:  "https://bar",
					ok:   true,
				},
				{
					name: "ba",
					url:  "",
					ok:   false,
				},
				{
					name: "nope",
					url:  "",
					ok:   false,
				},
			},
		},
		{
			input:       "=", // missing regex and url
			expectError: true,
		},
		{
			input:       "^foo.*=", // missing url
			expectError: true,
		},
		{
			input:       "=https://foo", // missing regex
			expectError: true,
		},
		{
			input:       "^foo.*=https://foo,", // trailing comma
			expectError: true,
		},
		{
			input:       "[=https://foo", // invalid regex
			expectError: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			actual, err := parsePluginDownloadURLOverrides(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, actual)

			if len(tt.matches) > 0 {
				for _, match := range tt.matches {
					actualURL, actualOK := actual.get(match.name)
					assert.Equal(t, match.url, actualURL)
					assert.Equal(t, match.ok, actualOK)
				}
			}
		})
	}
}
