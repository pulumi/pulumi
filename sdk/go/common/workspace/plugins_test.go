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
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyPluginSelection_Prerelease(t *testing.T) {
	t.Parallel()

	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.0")
	v3 := semver.MustParse("0.3.0-alpha")
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

	result := LegacySelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", nil)
	assert.NotNil(t, result)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.2.0", result.Version.String())
}

func TestLegacyPluginSelection_PrereleaseRequested(t *testing.T) {
	t.Parallel()

	v1 := semver.MustParse("0.1.0")
	v2 := semver.MustParse("0.2.0-alpha")
	v3 := semver.MustParse("0.3.0-alpha")
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

	v := semver.MustParse("0.2.0")
	result := LegacySelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", &v)
	assert.NotNil(t, result)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.3.0-alpha", result.Version.String())
}

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
	result := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NotNil(t, result)
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
	result := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.Nil(t, result)
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
	result := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NotNil(t, result)
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
	result := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NotNil(t, result)
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
	result := SelectCompatiblePlugin(candidatePlugins, ResourcePlugin, "myplugin", requested)
	assert.NotNil(t, result)
	assert.Equal(t, "myplugin", result.Name)
	assert.Equal(t, "0.2.0", result.Version.String())
}

func newMockReadCloser(data []byte) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

func newMockReadCloserString(data string) (io.ReadCloser, int64, error) {
	return newMockReadCloser([]byte(data))
}

//nolint:paralleltest // mutates environment variables
func TestPluginDownload(t *testing.T) {
	expectedBytes := []byte{1, 2, 3}
	token := "RaNd0m70K3n_"

	t.Run("Pulumi GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		version := semver.MustParse("4.30.0")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.30.0" {
				assert.Equal(t, "", req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/654321",
						"name": "pulumi-mockdl_4.30.0_checksums.txt"
					  },
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456",
						"name": "pulumi-resource-mockdl-v4.30.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456", req.URL.String())
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("get.pulumi.com", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "otherdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-otherdl/releases/tags/v4.32.0" {
				return nil, -1, errors.New("404 not found")
			}
			assert.Equal(t,
				"https://get.pulumi.com/releases/plugins/pulumi-resource-otherdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("Custom http URL", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "http://customurl.jfrog.io/artifactory/pulumi-packages/package-name/v${VERSION}/${OS}/${ARCH}",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"http://customurl.jfrog.io/artifactory/pulumi-packages/"+
					"package-name/v4.32.0/darwin/amd64/pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("Custom https URL", func(t *testing.T) {
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/" +
				"package-name/${NAME}/v${VERSION}/${OS}/${ARCH}/",
			Name:    "mockdl",
			Version: &version,
			Kind:    PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://customurl.jfrog.io/artifactory/pulumi-packages/"+
					"package-name/mockdl/v4.32.0/darwin/amd64/pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz",
				req.URL.String())
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("Private Pulumi GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.32.0" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/654321",
						"name": "pulumi-mockdl_4.32.0_checksums.txt"
					  },
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456",
						"name": "pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456", req.URL.String())
			assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("Internal GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "github://api.git.org/ourorg/mock",
			Name:              "mockdl",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.32.0" {
				return nil, -1, errors.New("404 not found")
			}

			if req.URL.String() == "https://api.git.org/repos/ourorg/mock/releases/tags/v4.32.0" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.git.org/repos/ourorg/mock/releases/assets/654321",
						"name": "pulumi-mockdl_4.32.0_checksums.txt"
					  },
					  {
						"url": "https://api.git.org/repos/ourorg/mock/releases/assets/123456",
						"name": "pulumi-resource-mockdl-v4.32.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.git.org/repos/ourorg/mock/releases/assets/123456", req.URL.String())
			assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("Pulumi GitHub Releases With Checksum", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		version := semver.MustParse("4.30.0")
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.30.0" {
				assert.Equal(t, "", req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/654321",
						"name": "pulumi-mockdl_4.30.0_checksums.txt"
					  },
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456",
						"name": "pulumi-resource-mockdl-v4.30.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456", req.URL.String())
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}

		chksum := "039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81" //nolint:gosec

		t.Run("Invalid Checksum", func(t *testing.T) {
			spec := PluginSpec{
				PluginDownloadURL: "",
				Name:              "mockdl",
				Version:           &version,
				Kind:              PluginKind("resource"),
				Checksums: map[string][]byte{
					"darwin-amd64": {0},
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
			require.NoError(t, err)
			readBytes, err := io.ReadAll(r)
			assert.Error(t, err, "invalid checksum, expected 00, actual "+chksum)
			assert.Equal(t, int(l), len(readBytes))
			assert.Equal(t, expectedBytes, readBytes)
		})

		t.Run("Valid Checksum", func(t *testing.T) {
			checksum, err := hex.DecodeString(chksum)
			assert.NoError(t, err)

			spec := PluginSpec{
				PluginDownloadURL: "",
				Name:              "mockdl",
				Version:           &version,
				Kind:              PluginKind("resource"),
				Checksums: map[string][]byte{
					"darwin-amd64": checksum,
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
			require.NoError(t, err)
			readBytes, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, int(l), len(readBytes))
			assert.Equal(t, expectedBytes, readBytes)
		})

		t.Run("Missing Checksum", func(t *testing.T) {
			// In this test the specification has checksums, but is missing the checksum for the current platform.
			// There are two sensible ways to handle this:
			// 1. Behave as if no checksums were specified at all, and simply fall back to not checking anything.
			// 2. Error that the checksum for the current platform is missing.
			// We choose to do the former, for now as that's more lenient.
			spec := PluginSpec{
				PluginDownloadURL: "",
				Name:              "mockdl",
				Version:           &version,
				Kind:              PluginKind("resource"),
				Checksums: map[string][]byte{
					"windows-amd64": {0},
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(*spec.Version, "darwin", "amd64", getHTTPResponse)
			require.NoError(t, err)
			readBytes, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, int(l), len(readBytes))
			assert.Equal(t, expectedBytes, readBytes)
		})
	})
	t.Run("GitLab Releases", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", token)
		version := semver.MustParse("1.23.4")
		spec := PluginSpec{
			PluginDownloadURL: "gitlab://gitlab.com/278964",
			Name:              "mock-gitlab",
			Version:           &version,
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://gitlab.com/api/v4/projects/278964/releases/v1.23.4/downloads/"+
					"pulumi-resource-mock-gitlab-v1.23.4-windows-arm64.tar.gz", req.URL.String())
			assert.Equal(t, fmt.Sprintf("Bearer %s", token), req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(*spec.Version, "windows", "arm64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
}

//nolint:paralleltest // mutates environment variables
func TestPluginGetLatestVersion(t *testing.T) {
	token := "RaNd0m70K3n_"

	t.Run("Pulumi GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source, err := spec.GetSource()
		assert.NoError(t, err)
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
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Custom http URL", func(t *testing.T) {
		spec := PluginSpec{
			PluginDownloadURL: "http://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		version, err := source.GetLatestVersion(getHTTPResponse)
		assert.Nil(t, version)
		assert.Equal(t, "GetLatestVersion is not supported for plugins from http sources", err.Error())
	})
	t.Run("Custom https URL", func(t *testing.T) {
		spec := PluginSpec{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		version, err := source.GetLatestVersion(getHTTPResponse)
		assert.Nil(t, version)
		assert.Equal(t, "GetLatestVersion is not supported for plugins from http sources", err.Error())
	})
	t.Run("Private Pulumi GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mock-private",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mock-private/releases/latest" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v4.37.5"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Internal GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "github://api.git.org/ourorg/mock",
			Name:              "mock-private",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source, err := spec.GetSource()
		assert.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.git.org/repos/ourorg/mock/releases/latest" {
				assert.Equal(t, fmt.Sprintf("token %s", token), req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v4.37.5"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("GitLab Releases", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "gitlab://gitlab.com/278964",
			Name:              "mock-gitlab",
			Kind:              PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("1.23.0")
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://gitlab.com/api/v4/projects/278964/releases/permalink/latest" {
				assert.Equal(t, fmt.Sprintf("Bearer %s", token), req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))

				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v1.23"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Hit GitHub ratelimit", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mock-latest",
			Kind:              PluginKind("resource"),
		}
		source, err := spec.GetSource()
		assert.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			return nil, 0, newDownloadError(403, req.URL, http.Header{"X-Ratelimit-Remaining": []string{"0"}})
		}
		_, err = source.GetLatestVersion(getHTTPResponse)
		assert.ErrorContains(t, err, "rate limit exceeded")
		assert.ErrorContains(t, err, "https://api.github.com/repos/pulumi/pulumi-mock-latest/releases/latest")
	})
}

func TestParsePluginDownloadURLOverride(t *testing.T) {
	t.Parallel()

	type match struct {
		name string
		url  string
		ok   bool
	}

	tests := []struct {
		input         string
		expected      pluginDownloadOverrideArray
		matches       []match
		expectedError string
	}{
		{
			input:    "",
			expected: pluginDownloadOverrideArray{},
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
			input:         "=", // missing regex and url
			expectedError: "expected format to be \"regexp1=URL1,regexp2=URL2\"; got \"=\"",
		},
		{
			input:         "^foo.*=", // missing url
			expectedError: "expected format to be \"regexp1=URL1,regexp2=URL2\"; got \"^foo.*=\"",
		},
		{
			input:         "=https://foo", // missing regex
			expectedError: "expected format to be \"regexp1=URL1,regexp2=URL2\"; got \"=https://foo\"",
		},
		{
			input:         "^foo.*=https://foo,", // trailing comma
			expectedError: "expected format to be \"regexp1=URL1,regexp2=URL2\"; got \"^foo.*=https://foo,\"",
		},
		{
			input:         "[=https://foo", // invalid regex
			expectedError: "error parsing regexp: missing closing ]: `[`",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			actual, err := parsePluginDownloadURLOverrides(tt.input)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
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

func TestDownloadToFile_retries(t *testing.T) {
	t.Parallel()

	// Verifies that DownloadToFile retries on transient errors
	// when trying to download plugins,
	// and that it calls the wrapper and retry functions as expected.
	//
	// Regression test for https://github.com/pulumi/pulumi/issues/12456.

	var numRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "expected GET request")
		assert.Regexp(t, `/pulumi-language-myplugin-v1.0.0-\S+\.tar\.gz`, r.URL.Path,
			"unexpected URL path")

		// Fails all requests with a 500 error.
		// This will cause every download attempt to fail
		// and be retried.
		w.WriteHeader(http.StatusInternalServerError)

		numRequests++
	}))
	t.Cleanup(server.Close)
	defer func() {
		assert.Equal(t, 5, numRequests,
			"server received more requests than expected")
	}()

	// Create a fake plugin.
	version := semver.MustParse("1.0.0")
	spec := PluginSpec{
		Name:              "myplugin",
		Kind:              LanguagePlugin,
		Version:           &version,
		PluginDownloadURL: server.URL,
		PluginDir:         t.TempDir(),
	}

	// numRetries is tracked separately from numRequests.
	// numRequests is the number of requests received by the server,
	// while numRetries is the number of times the retry function is called.
	// These should match--the function is called on all failures.
	var numRetries int
	currentTime := time.Now()
	_, err := (&pluginDownloader{
		OnRetry: func(err error, attempt, limit int, delay time.Duration) {
			assert.Equal(t, 5, limit, "unexpected retry limit")
			numRetries++
			assert.Equal(t, numRetries, attempt, "unexpected attempt number")
		},
		After: func(d time.Duration) <-chan time.Time {
			currentTime = currentTime.Add(d)
			ch := make(chan time.Time, 1)
			ch <- currentTime
			return ch
		},
	}).DownloadToFile(spec)
	assert.ErrorContains(t, err, "failed to download plugin: myplugin-1.0.0")
	assert.Equal(t, numRequests, numRetries)
}

//nolint:paralleltest // changes directory for process
func TestUnmarshalProjectWithProviderList(t *testing.T) {
	t.Parallel()
	tempdir := t.TempDir()
	pyaml := filepath.Join(tempdir, "Pulumi.yaml")

	// write to pyaml
	err := os.WriteFile(pyaml, []byte(`name: test-yaml
runtime: yaml
description: "Test Pulumi YAML"
plugins:
  providers:
  - name: aws
    version: 1.0.0
    path: ../bin/aws`), 0o600)
	assert.NoError(t, err)

	proj, err := LoadProject(pyaml)
	assert.NoError(t, err)
	assert.NotNil(t, proj.Plugins)
	assert.Equal(t, 1, len(proj.Plugins.Providers))
	assert.Equal(t, "aws", proj.Plugins.Providers[0].Name)
	assert.Equal(t, "1.0.0", proj.Plugins.Providers[0].Version)
	assert.Equal(t, "../bin/aws", proj.Plugins.Providers[0].Path)
}

func TestPluginBadSource(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("4.30.0")
	spec := PluginSpec{
		PluginDownloadURL: "strange-scheme://what.is.this?oh-no",
		Name:              "mockdl",
		Version:           &version,
		Kind:              PluginKind("resource"),
	}
	source, err := spec.GetSource()
	assert.ErrorContains(t, err, "unknown plugin source scheme: strange-scheme")
	assert.Nil(t, source)
}

func TestMissingErrorText(t *testing.T) {
	t.Parallel()

	v1 := semver.MustParse("0.1.0")
	tests := []struct {
		Name           string
		Plugin         PluginInfo
		IncludeAmbient bool
		ExpectedError  string
	}{
		{
			Name: "ResourceWithVersion",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    ResourcePlugin,
				Version: &v1,
			},
			IncludeAmbient: true,
			ExpectedError: "no resource plugin 'pulumi-resource-myplugin' found in the workspace at version v0.1.0 " +
				"or on your $PATH",
		},
		{
			Name: "ResourceWithVersion_ExcludeAmbient",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    ResourcePlugin,
				Version: &v1,
			},
			IncludeAmbient: false,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace at version v0.1.0",
		},
		{
			Name: "ResourceWithoutVersion",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    ResourcePlugin,
				Version: nil,
			},
			IncludeAmbient: true,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace or on your $PATH",
		},
		{
			Name: "ResourceWithoutVersion_ExcludeAmbient",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    ResourcePlugin,
				Version: nil,
			},
			IncludeAmbient: false,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace",
		},
		{
			Name: "LanguageWithoutVersion",
			Plugin: PluginInfo{
				Name:    "dotnet",
				Kind:    LanguagePlugin,
				Version: nil,
			},
			IncludeAmbient: true,
			ExpectedError:  "no language plugin 'pulumi-language-dotnet' found in the workspace or on your $PATH",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			err := NewMissingError(tt.Plugin.Kind, tt.Plugin.Name, tt.Plugin.Version, tt.IncludeAmbient)
			assert.Equal(t, tt.ExpectedError, err.Error())
		})
	}
}

//nolint:paralleltest // modifies environment variables
func TestBundledPluginSearch(t *testing.T) {
	// Get the path of this executable
	exe, err := os.Executable()
	require.NoError(t, err)

	// Create a fake side-by-side plugin next to this executable, it must match one of our bundled names
	bundledPath := filepath.Join(filepath.Dir(exe), "pulumi-language-nodejs")
	err = os.WriteFile(bundledPath, []byte{}, 0o700) //nolint: gosec // we intended to write an executable file here
	require.NoError(t, err)
	bundledPath, _ = filepath.EvalSymlinks(bundledPath)
	t.Cleanup(func() {
		err := os.Remove(bundledPath)
		require.NoError(t, err)
	})

	// Create another copy of the fake plugin in $PATH
	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)
	ambientPath := filepath.Join(pathDir, "pulumi-language-nodejs")
	err = os.WriteFile(ambientPath, []byte{}, 0o700) //nolint: gosec
	require.NoError(t, err)

	d := diagtest.LogSink(t)

	// Lookup the plugin with ambient search turned on
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(d, LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ambientPath, path)

	// Lookup the plugin with ambient search turned off
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "true")
	path, err = GetPluginPath(d, LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, bundledPath, path)
}

//nolint:paralleltest // modifies environment variables
func TestAmbientPluginsWarn(t *testing.T) {
	// Create a fake plugin in the path
	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)
	ambientPath := filepath.Join(pathDir, "pulumi-resource-mock")
	err := os.WriteFile(ambientPath, []byte{}, 0o700) //nolint: gosec
	require.NoError(t, err)

	var stderr bytes.Buffer
	d := diag.DefaultSink(
		iotest.LogWriter(t), // stdout
		&stderr,
		diag.FormatOptions{Color: "never"},
	)

	// Lookup the plugin with ambient search turned on
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(d, ResourcePlugin, "mock", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ambientPath, path)

	// Check we get a warning about loading this plugin
	expectedMessage := fmt.Sprintf("warning: using pulumi-resource-mock from $PATH at %s\n", ambientPath)
	assert.Equal(t, expectedMessage, stderr.String())
}

//nolint:paralleltest // modifies environment variables
func TestBundledPluginsDoNotWarn(t *testing.T) {
	// Get the path of this executable
	exe, err := os.Executable()
	require.NoError(t, err)

	// Create a fake side-by-side plugin next to this executable, it must match one of our bundled names
	bundledPath := filepath.Join(filepath.Dir(exe), "pulumi-language-nodejs")
	err = os.WriteFile(bundledPath, []byte{}, 0o700) //nolint: gosec // we intended to write an executable file here
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Remove(bundledPath)
		require.NoError(t, err)
	})

	// Add the executable directory to PATH
	t.Setenv("PATH", filepath.Dir(exe))

	var stderr bytes.Buffer
	d := diag.DefaultSink(
		iotest.LogWriter(t), // stdout
		&stderr,
		diag.FormatOptions{Color: "never"},
	)

	// Lookup the plugin with ambient search turned on
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(d, LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, bundledPath, path)

	// Check we don't get a warning about loading this plugin, because it's the bundled one _even_ though it's also on PATH
	assert.Empty(t, stderr.String())
}

// Regression test for https://github.com/pulumi/pulumi/issues/13656
//
//nolint:paralleltest // modifies environment variables
func TestSymlinkPathPluginsDoNotWarn(t *testing.T) {
	// Get the path of this executable
	exe, err := os.Executable()
	require.NoError(t, err)

	// Create a fake side-by-side plugin next to this executable, it must match one of our bundled names
	bundledPath := filepath.Join(filepath.Dir(exe), "pulumi-language-nodejs")
	err = os.WriteFile(bundledPath, []byte{}, 0o700) //nolint: gosec
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Remove(bundledPath)
		require.NoError(t, err)
	})

	// Create a fake plugin in the path that is a symlink to the bundled plugin
	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)
	ambientPath := filepath.Join(pathDir, "pulumi-language-nodejs")
	err = os.Symlink(bundledPath, ambientPath)
	require.NoError(t, err)

	var stderr bytes.Buffer
	d := diag.DefaultSink(
		iotest.LogWriter(t), // stdout
		&stderr,
		diag.FormatOptions{Color: "never"},
	)

	// Lookup the plugin with ambient search turned on
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(d, LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	// We expect the ambient path to be returned, but not to warn because it resolves to the same file as the
	// bundled path.
	assert.Equal(t, ambientPath, path)
	assert.Empty(t, stderr.String())
}
