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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	result := LegacySelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", nil)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	v := semver.MustParse("0.2.0")
	result := LegacySelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", &v)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result := SelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", requested)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result := SelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", requested)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v21,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange(">=0.2.0 <0.3.0")
	result := SelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", requested)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result := SelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", requested)
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
			Kind:    apitype.ResourcePlugin,
			Version: &v1,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v2,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: nil,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "notmyplugin",
			Kind:    apitype.ResourcePlugin,
			Version: &v3,
		},
		{
			Name:    "myplugin",
			Kind:    apitype.AnalyzerPlugin,
			Version: &v3,
		},
	}

	requested := semver.MustParseRange("0.2.0")
	result := SelectCompatiblePlugin(candidatePlugins, apitype.ResourcePlugin, "myplugin", requested)
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
			Kind:              apitype.PluginKind("resource"),
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
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:              apitype.PluginKind("resource"),
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
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:              apitype.PluginKind("resource"),
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
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:    apitype.PluginKind("resource"),
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
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.32.0" {
				assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
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
			assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			// Test that the asset isn't on github
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.32.0" {
				return nil, -1, errors.New("404 not found")
			}

			if req.URL.String() == "https://api.git.org/repos/ourorg/mock/releases/tags/v4.32.0" {
				assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
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
			assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
				Kind:              apitype.PluginKind("resource"),
				Checksums: map[string][]byte{
					"darwin-amd64": {0},
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
				Kind:              apitype.PluginKind("resource"),
				Checksums: map[string][]byte{
					"darwin-amd64": checksum,
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
				Kind:              apitype.PluginKind("resource"),
				Checksums: map[string][]byte{
					"windows-amd64": {0},
				},
			}
			source, err := spec.GetSource()
			require.NoError(t, err)
			r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
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
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			assert.Equal(t,
				"https://gitlab.com/api/v4/projects/278964/releases/v1.23.4/downloads/"+
					"pulumi-resource-mock-gitlab-v1.23.4-windows-arm64.tar.gz", req.URL.String())
			assert.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(context.Background(), *spec.Version, "windows", "arm64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("GitHub Releases with invalid token", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		version := semver.MustParse("4.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		attempts := 0
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			attempts++

			if req.Header.Get("Authorization") == "token "+token {
				// Fail with a 401 Unauthorized
				return nil, -1, &downloadError{code: 401}
			}

			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v4.32.0" {
				assert.Equal(t, "", req.Header.Get("Authorization"))
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
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, 3, attempts) // Failed attempt, then two successful attempts, first for the tag then the asset.
		assert.Equal(t, int(l), len(readBytes))
		assert.Equal(t, expectedBytes, readBytes)
	})
	t.Run("GitHub Releases with disallowed", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		version := semver.MustParse("5.32.0")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mockdl",
			Version:           &version,
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		attempts := 0
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			attempts++

			if req.Header.Get("Authorization") == "token "+token {
				// Fail with a 403 Forbidden
				return nil, -1, &downloadError{
					code:   403,
					header: http.Header{"x-ratelimit-remaining": []string{"42"}},
				}
			}

			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/tags/v5.32.0" {
				assert.Equal(t, "", req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"assets": [
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/654321",
						"name": "pulumi-mockdl_5.32.0_checksums.txt"
					  },
					  {
						"url": "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456",
						"name": "pulumi-resource-mockdl-v5.32.0-darwin-amd64.tar.gz"
					  }
					]
				  }
				`)
			}

			assert.Equal(t, "https://api.github.com/repos/pulumi/pulumi-mockdl/releases/assets/123456", req.URL.String())
			assert.Equal(t, "application/octet-stream", req.Header.Get("Accept"))
			return newMockReadCloser(expectedBytes)
		}
		r, l, err := source.Download(context.Background(), *spec.Version, "darwin", "amd64", getHTTPResponse)
		require.NoError(t, err)
		readBytes, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, 3, attempts) // Failed attempt, then two successful attempts, first for the tag then the asset.
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
			Kind:              apitype.PluginKind("resource"),
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
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Custom http URL", func(t *testing.T) {
		spec := PluginSpec{
			PluginDownloadURL: "http://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mock-latest",
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		assert.Nil(t, version)
		assert.EqualError(t, err, "GetLatestVersion is not supported for plugins from http sources")
	})
	t.Run("Custom https URL", func(t *testing.T) {
		spec := PluginSpec{
			PluginDownloadURL: "https://customurl.jfrog.io/artifactory/pulumi-packages/package-name",
			Name:              "mock-latest",
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		require.NoError(t, err)
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		assert.Nil(t, version)
		assert.EqualError(t, err, "GetLatestVersion is not supported for plugins from http sources")
	})
	t.Run("Private Pulumi GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mock-private",
			Kind:              apitype.PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.github.com/repos/pulumi/pulumi-mock-private/releases/latest" {
				assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v4.37.5"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Internal GitHub Releases", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "github://api.git.org/ourorg/mock",
			Name:              "mock-private",
			Kind:              apitype.PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("4.37.5")
		source, err := spec.GetSource()
		assert.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://api.git.org/repos/ourorg/mock/releases/latest" {
				assert.Equal(t, "token "+token, req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))
				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v4.37.5"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("GitLab Releases", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", token)
		spec := PluginSpec{
			PluginDownloadURL: "gitlab://gitlab.com/278964",
			Name:              "mock-gitlab",
			Kind:              apitype.PluginKind("resource"),
		}
		expectedVersion := semver.MustParse("1.23.0")
		source, err := spec.GetSource()
		require.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			if req.URL.String() == "https://gitlab.com/api/v4/projects/278964/releases/permalink/latest" {
				assert.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
				assert.Equal(t, "application/json", req.Header.Get("Accept"))

				// Minimal JSON from the releases API to get the test to pass
				return newMockReadCloserString(`{
					"tag_name": "v1.23"
				}`)
			}

			panic("Unexpected call to getHTTPResponse")
		}
		version, err := source.GetLatestVersion(context.Background(), getHTTPResponse)
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, *version)
	})
	t.Run("Hit GitHub ratelimit", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		spec := PluginSpec{
			PluginDownloadURL: "",
			Name:              "mock-latest",
			Kind:              apitype.PluginKind("resource"),
		}
		source, err := spec.GetSource()
		assert.NoError(t, err)
		getHTTPResponse := func(req *http.Request) (io.ReadCloser, int64, error) {
			return nil, 0, newDownloadError(403, req.URL, http.Header{"X-Ratelimit-Remaining": []string{"0"}})
		}
		_, err = source.GetLatestVersion(context.Background(), getHTTPResponse)
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

func TestPluginDownloadOverrideArray_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		overrides     pluginDownloadOverrideArray
		input         string
		expectedURL   string
		expectedMatch bool
	}{
		{
			name: "No match",
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`^test-plugin$`), url: "https://example.com/test-plugin"},
			},
			input:         "another-plugin",
			expectedURL:   "",
			expectedMatch: false,
		},
		{
			name: "Simple match",
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`^test-plugin$`), url: "https://example.com/test-plugin"},
			},
			input:         "test-plugin",
			expectedURL:   "https://example.com/test-plugin",
			expectedMatch: true,
		},
		{
			name: "Match with name placeholders",
			overrides: pluginDownloadOverrideArray{
				{
					reg: regexp.MustCompile(`^(?P<org>[\w-]+)-v(?P<repo>\d+\.\d+\.\d+)$`),
					url: "https://example.com/${org}/${repo}/plugin.zip",
				},
			},
			input:         "my-plugin-v1.2.3",
			expectedURL:   "https://example.com/my-plugin/1.2.3/plugin.zip",
			expectedMatch: true,
		},
		{
			name: "Match with index placeholders",
			overrides: pluginDownloadOverrideArray{
				{
					reg: regexp.MustCompile(`^(?P<org>[\w-]+)-v(?P<repo>\d+\.\d+\.\d+)$`),
					url: "https://example.com/$1/$2/plugin.zip",
				},
			},
			input:         "my-plugin-v1.2.3",
			expectedURL:   "https://example.com/my-plugin/1.2.3/plugin.zip",
			expectedMatch: true,
		},
		{
			name: "Match with $0 placeholder",
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`^.+$`), url: "https://example.com/downloads?source=$0"},
			},
			input:         "test-plugin",
			expectedURL:   "https://example.com/downloads?source=test-plugin",
			expectedMatch: true,
		},
		{
			name: "Multiple overrides, second matches",
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`^test-plugin$`), url: "https://example.com/test-plugin"},
				{reg: regexp.MustCompile(`^another-plugin$`), url: "https://example.com/another-plugin"},
			},
			input:         "another-plugin",
			expectedURL:   "https://example.com/another-plugin",
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actualURL, actualMatch := tt.overrides.get(tt.input)
			if actualURL != tt.expectedURL {
				assert.Equal(t, tt.expectedURL, actualURL)
			}
			if actualMatch != tt.expectedMatch {
				assert.Equal(t, tt.expectedMatch, actualMatch)
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
		Kind:              apitype.LanguagePlugin,
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
	}).DownloadToFile(context.Background(), spec)
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

//nolint:paralleltest // mutates pluginDownloadURLOverridesParsed
func TestPluginSpec_GetSource(t *testing.T) {
	tests := []struct {
		name               string
		spec               PluginSpec
		overrides          pluginDownloadOverrideArray
		expectedSourceType string
		expectedURL        string
		expectedErrMsg     string
	}{
		{
			name: "Use PluginDownloadURL (HTTP)",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "https://example.com/test-plugin",
			},
			expectedSourceType: "*workspace.httpSource",
			expectedURL:        "https://example.com/test-plugin",
		},
		{
			name: "Use PluginDownloadURL (GitHub)",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "github://api.github.com/owner/repo",
			},
			expectedSourceType: "*workspace.githubSource",
			expectedURL:        "github://api.github.com/owner/repo",
		},
		{
			name: "Use PluginDownloadURL (GitLab)",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "gitlab://mygitlab.example.com/proj1",
			},
			expectedSourceType: "*workspace.gitlabSource",
			expectedURL:        "gitlab://mygitlab.example.com/proj1",
		},
		{
			name: "Use PluginDownloadURL (Git)",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "git://github.com/test/test",
			},
			expectedSourceType: "*workspace.gitSource",
			expectedURL:        "https://github.com/test/test.git",
		},
		{
			name: "Use fallback source",
			spec: PluginSpec{
				Name: "test-plugin",
				Kind: apitype.PluginKind("resource"),
			},
			expectedSourceType: "*workspace.fallbackSource",
			expectedURL:        "github://api.github.com/pulumi/pulumi-test-plugin",
		},
		{
			name: "Apply override (HTTP)",
			spec: PluginSpec{
				Name: "test-plugin",
				Kind: apitype.PluginKind("resource"),
			},
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`test-plugin`), url: "https://example.com/test-plugin"},
			},
			expectedSourceType: "*workspace.httpSource",
			expectedURL:        "https://example.com/test-plugin",
		},
		{
			name: "Apply override (GitHub)",
			spec: PluginSpec{
				Name: "test-plugin",
				Kind: apitype.PluginKind("resource"),
			},
			overrides: pluginDownloadOverrideArray{
				{reg: regexp.MustCompile(`test-plugin`), url: "github://api.github.com/test-org/test-plugin"},
			},
			expectedSourceType: "*workspace.githubSource",
			expectedURL:        "github://api.github.com/test-org/test-plugin",
		},
		{
			name: "Apply checksums",
			spec: PluginSpec{
				Name:      "test-plugin",
				Kind:      apitype.PluginKind("resource"),
				Checksums: map[string][]byte{"checksum1": []byte("checksum2")},
			},
			expectedSourceType: "*workspace.checksumSource",
			expectedURL:        "github://api.github.com/pulumi/pulumi-test-plugin",
		},
		{
			name: "Invalid URL",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "://invalid-url",
			},
			expectedErrMsg: "parse \"://invalid-url\": missing protocol scheme",
		},
		{
			name: "Unknown scheme",
			spec: PluginSpec{
				Name:              "test-plugin",
				Kind:              apitype.PluginKind("resource"),
				PluginDownloadURL: "unknown://example.com/plugin",
			},
			expectedErrMsg: "unknown plugin source scheme: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginDownloadURLOverridesParsed = tt.overrides

			source, err := tt.spec.GetSource()
			assert.Equal(t, tt.expectedErrMsg != "", err != nil)
			if err != nil {
				assert.Equal(t, tt.expectedErrMsg, err.Error())
				return
			}
			actualSourceType := reflect.TypeOf(source).String()
			assert.Equal(t, tt.expectedSourceType, actualSourceType)
			assert.Equal(t, tt.expectedURL, source.URL())
		})
	}
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
				Kind:    apitype.ResourcePlugin,
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
				Kind:    apitype.ResourcePlugin,
				Version: &v1,
			},
			IncludeAmbient: false,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace at version v0.1.0",
		},
		{
			Name: "ResourceWithoutVersion",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    apitype.ResourcePlugin,
				Version: nil,
			},
			IncludeAmbient: true,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace or on your $PATH",
		},
		{
			Name: "ResourceWithoutVersion_ExcludeAmbient",
			Plugin: PluginInfo{
				Name:    "myplugin",
				Kind:    apitype.ResourcePlugin,
				Version: nil,
			},
			IncludeAmbient: false,
			ExpectedError:  "no resource plugin 'pulumi-resource-myplugin' found in the workspace",
		},
		{
			Name: "LanguageWithoutVersion",
			Plugin: PluginInfo{
				Name:    "dotnet",
				Kind:    apitype.LanguagePlugin,
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
			assert.EqualError(t, err, tt.ExpectedError)
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
	path, err := GetPluginPath(d, apitype.LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ambientPath, path)

	// Lookup the plugin with ambient search turned off
	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "true")
	path, err = GetPluginPath(d, apitype.LanguagePlugin, "nodejs", nil, nil)
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
	path, err := GetPluginPath(d, apitype.ResourcePlugin, "mock", nil, nil)
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
	path, err := GetPluginPath(d, apitype.LanguagePlugin, "nodejs", nil, nil)
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
	path, err := GetPluginPath(d, apitype.LanguagePlugin, "nodejs", nil, nil)
	require.NoError(t, err)
	// We expect the ambient path to be returned, but not to warn because it resolves to the same file as the
	// bundled path.
	assert.Equal(t, ambientPath, path)
	assert.Empty(t, stderr.String())
}

// Test that GetPluginInfo works against shimless plugins (i.e. those without a direct executable file).
//
//nolint:paralleltest // modifies environment variables
func TestPluginInfoShimless(t *testing.T) {
	// Create a fake plugin in temp
	pathDir := t.TempDir()

	pluginPath := filepath.Join(pathDir, "pulumi-resource-mock")
	err := os.MkdirAll(pluginPath, 0o700) //nolint: gosec
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pluginPath, "PulumiPlugin.yaml"), []byte(`runtime: nodejs`), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pluginPath, "test.ts"), []byte(`testcode`), 0o600)
	require.NoError(t, err)

	stat, err := os.Stat(pluginPath)
	require.NoError(t, err)

	var stderr bytes.Buffer
	d := diag.DefaultSink(
		iotest.LogWriter(t), // stdout
		&stderr,
		diag.FormatOptions{Color: "never"},
	)

	info, err := GetPluginInfo(d, apitype.ResourcePlugin, "mock", nil, []ProjectPlugin{
		{
			Name: "mock",
			Kind: apitype.ResourcePlugin,
			Path: pluginPath,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, pluginPath, info.Path)
	assert.Equal(t, uint64(23), info.Size)
	assert.Equal(t, stat.ModTime(), info.InstallTime)
	assert.Equal(t, stat.ModTime(), info.SchemaTime)
	// schemaPaths are odd, they're one directory up from the plugin directory
	assert.Equal(t, filepath.Join(filepath.Dir(pluginPath), "schema-mock.json"), info.SchemaPath)
}

//nolint:paralleltest // modifies environment variables
func TestProjectPluginsWithUncleanPath(t *testing.T) {
	tempdir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempdir, "pulumi-resource-aws"), []byte{}, 0o600)
	require.NoError(t, err)

	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(diagtest.LogSink(t), apitype.ResourcePlugin, "aws", nil, []ProjectPlugin{
		{
			Name: "aws",
			Kind: apitype.ResourcePlugin,
			Path: tempdir + "/", // path with a trailing slash
		},
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempdir, "pulumi-resource-aws"), path)
}

//nolint:paralleltest // modifies environment variables
func TestProjectPluginsWithSymlink(t *testing.T) {
	tempdir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempdir, "subdir"), 0o700)
	require.NoError(t, err)
	err = os.Symlink(filepath.Join(tempdir, "subdir"), filepath.Join(tempdir, "symlink"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempdir, "subdir", "pulumi-resource-aws"), []byte{}, 0o600)
	require.NoError(t, err)

	t.Setenv("PULUMI_IGNORE_AMBIENT_PLUGINS", "false")
	path, err := GetPluginPath(diagtest.LogSink(t), apitype.ResourcePlugin, "aws", nil, []ProjectPlugin{
		{
			Name: "aws",
			Kind: apitype.ResourcePlugin,
			Path: filepath.Join(tempdir, "symlink"),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempdir, "symlink", "pulumi-resource-aws"), path)
}

func TestNewPluginSpec(t *testing.T) {
	t.Parallel()

	v1 := semver.MustParse("1.0.0")
	v0deadbeef := semver.MustParse("0.0.0-xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	cases := []struct {
		name               string
		source             string
		version            *semver.Version
		kind               apitype.PluginKind
		pluginDownloadURL  string
		ExpectedPluginSpec PluginSpec
		Error              error
	}{
		{
			name:   "regular plugin",
			source: "pulumi-example",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "pulumi-example",
				Kind:              apitype.ResourcePlugin,
				Version:           nil,
				PluginDownloadURL: "",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "plugin with version",
			source: "pulumi-example@v1.0.0",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "pulumi-example",
				Kind:              apitype.ResourcePlugin,
				Version:           &v1,
				PluginDownloadURL: "",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "plugin with invalid semver",
			source: "pulumi-example@v1.0.0.0",
			kind:   apitype.ResourcePlugin,
			Error:  errors.New("VERSION must be valid semver: Invalid character(s) found in patch number \"0.0\""),
		},
		{
			name:   "git plugin",
			source: "github.com/pulumi/pulumi-example",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           nil,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "git plugin with version",
			source: "github.com/pulumi/pulumi-example@v1.0.0",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           &v1,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "git plugin with commit hash version",
			source: "github.com/pulumi/pulumi-example@deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           &v0deadbeef,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "git plugin with invalid version hash",
			source: "github.com/pulumi/pulumi-example@abcdxyz",
			kind:   apitype.ResourcePlugin,
			Error:  errors.New("VERSION must be valid semver or git commit hash: abcdxyz"),
		},
		{
			name:   "https prefixed git plugin",
			source: "https://github.com/pulumi/pulumi-example",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           nil,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "https prefixed git plugin with version",
			source: "https://github.com/pulumi/pulumi-example@v1.0.0",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           &v1,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "https prefixed git plugin with commit hash version",
			source: "https://github.com/pulumi/pulumi-example@deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "github.com_pulumi_pulumi-example.git",
				Kind:              apitype.ResourcePlugin,
				Version:           &v0deadbeef,
				PluginDownloadURL: "git://github.com/pulumi/pulumi-example",
				PluginDir:         "",
				Checksums:         nil,
			},
		},
		{
			name:   "https prefixed git plugin with invalid version hash",
			source: "https://github.com/pulumi/pulumi-example@abcdxyz",
			kind:   apitype.ResourcePlugin,
			Error:  errors.New("VERSION must be valid semver or git commit hash: abcdxyz"),
		},
		{
			name:   "local plugin",
			source: "./test/plugin",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "./test/plugin",
				Kind:              apitype.ResourcePlugin,
				Version:           nil,
				PluginDownloadURL: "",
				Checksums:         nil,
			},
		},
		{
			name:   "local plugin absolute path",
			source: "/test/plugin",
			kind:   apitype.ResourcePlugin,
			ExpectedPluginSpec: PluginSpec{
				Name:              "/test/plugin",
				Kind:              apitype.ResourcePlugin,
				Version:           nil,
				PluginDownloadURL: "",
				Checksums:         nil,
			},
		},
		{
			name:    "conflicting versions error",
			source:  "plugin@v1.0.0",
			version: &v1,
			kind:    apitype.ResourcePlugin,
			Error:   errors.New("cannot specify a version when the version is part of the name"),
		},
		{
			name:    "passed in version is used",
			source:  "plugin",
			kind:    apitype.ResourcePlugin,
			version: &v1,
			ExpectedPluginSpec: PluginSpec{
				Name:              "plugin",
				Kind:              apitype.ResourcePlugin,
				Version:           &v1,
				PluginDownloadURL: "",
				Checksums:         nil,
			},
		},
		{
			name:              "plugin download url and git url",
			source:            "github.com/pulumi/pulumi-example@v1.0.0",
			kind:              apitype.ResourcePlugin,
			pluginDownloadURL: "https://example.com/pulumi-example",
			Error:             errors.New("cannot specify a plugin download URL when the plugin name is a URL"),
		},
		{
			name:   "invalid version with git plugin",
			source: "github.com/pulumi/pulumi-example@v1.0.0.0",
			kind:   apitype.ResourcePlugin,
			Error:  errors.New("VERSION must be valid semver or git commit hash: v1.0.0.0"),
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			spec, err := NewPluginSpec(c.source, c.kind, c.version, c.pluginDownloadURL, nil)
			if c.Error != nil {
				require.EqualError(t, err, c.Error.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.ExpectedPluginSpec, spec)
		})
	}
}

func TestGitSourceDownloadSemver(t *testing.T) {
	t.Parallel()

	// Create a fake plugin.
	version := semver.MustParse("1.0.0")

	gitSource := &gitSource{
		url:  "https://example.com/repo/test",
		path: "path",
		cloneOrPull: func(ctx context.Context, url string, ref plumbing.ReferenceName, tmpdir string, shallow bool) error {
			require.Equal(t, "https://example.com/repo/test", url)
			require.Equal(t, plumbing.ReferenceName("refs/tags/v1.0.0"), ref)
			err := os.MkdirAll(filepath.Join(tmpdir, "path"), 0o700)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(tmpdir, filepath.Join("path", "test")), []byte("a string"), 0o600)
			require.NoError(t, err)

			return nil
		},
	}
	readCloser, l, err := gitSource.Download(context.Background(), version, "unused", "unused",
		func(*http.Request) (io.ReadCloser, int64, error) { panic("unused") })
	require.NoError(t, err)
	require.NotNil(t, readCloser)
	require.Greater(t, l, int64(0))

	zip, err := gzip.NewReader(readCloser)
	require.NoError(t, err)

	tarReader := tar.NewReader(zip)
	header, err := tarReader.Next()
	require.NoError(t, err)
	require.Equal(t, "path/test", header.Name)

	buf, err := io.ReadAll(tarReader)
	require.NoError(t, err)
	require.Equal(t, "a string", string(buf))
}

func TestGitSourceDownloadHEAD(t *testing.T) {
	t.Parallel()

	// Create a fake plugin.
	version := semver.Version{}

	gitSource := &gitSource{
		url:  "https://example.com/repo/test",
		path: "path",
		cloneOrPull: func(ctx context.Context, url string, ref plumbing.ReferenceName, tmpdir string, shallow bool) error {
			require.Equal(t, "https://example.com/repo/test", url)
			require.Equal(t, plumbing.HEAD, ref)
			err := os.MkdirAll(filepath.Join(tmpdir, "path"), 0o700)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(tmpdir, filepath.Join("path", "test")), []byte("a string"), 0o600)
			require.NoError(t, err)

			return nil
		},
	}
	readCloser, l, err := gitSource.Download(context.Background(), version, "unused", "unused",
		func(*http.Request) (io.ReadCloser, int64, error) { panic("unused") })
	require.NoError(t, err)
	require.NotNil(t, readCloser)
	require.Greater(t, l, int64(0))

	zip, err := gzip.NewReader(readCloser)
	require.NoError(t, err)

	tarReader := tar.NewReader(zip)
	header, err := tarReader.Next()
	require.NoError(t, err)
	require.Equal(t, "path/test", header.Name)

	buf, err := io.ReadAll(tarReader)
	require.NoError(t, err)
	require.Equal(t, "a string", string(buf))
}

func TestGitSourceDownloadHash(t *testing.T) {
	t.Parallel()

	// Create a fake plugin.
	version := semver.MustParse("0.0.0-xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	gitSource := &gitSource{
		url:  "https://example.com/repo/test",
		path: "path",
		cloneAndCheckoutRevision: func(ctx context.Context, url string, revision plumbing.Revision, tmpdir string) error {
			require.Equal(t, "https://example.com/repo/test", url)
			require.Equal(t, plumbing.Revision("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"), revision)
			err := os.MkdirAll(filepath.Join(tmpdir, "path"), 0o700)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(tmpdir, filepath.Join("path", "test")), []byte("a string"), 0o600)
			require.NoError(t, err)

			return nil
		},
	}
	readCloser, l, err := gitSource.Download(context.Background(), version, "unused", "unused",
		func(*http.Request) (io.ReadCloser, int64, error) { panic("unused") })
	require.NoError(t, err)
	require.NotNil(t, readCloser)
	require.Greater(t, l, int64(0))

	zip, err := gzip.NewReader(readCloser)
	require.NoError(t, err)

	tarReader := tar.NewReader(zip)
	header, err := tarReader.Next()
	require.NoError(t, err)
	require.Equal(t, "path/test", header.Name)

	buf, err := io.ReadAll(tarReader)
	require.NoError(t, err)
	require.Equal(t, "a string", string(buf))
}

func TestGitSourceGetLatestVersion(t *testing.T) {
	t.Parallel()

	gitSource := &gitSource{
		url: "testdata/latest-version.git",
	}
	version, err := gitSource.GetLatestVersion(context.Background(), func(*http.Request) (io.ReadCloser, int64, error) {
		panic("should not be called")
	})
	require.NoError(t, err)
	require.Equal(t, semver.MustParse("0.1.1"), *version)
}

func TestLocalName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		pluginName        string
		pluginDownloadURL string
		expected          string
		expectedPath      string
	}{
		{
			name:       "simple",
			pluginName: "pulumi-example",
			expected:   "pulumi-example",
		},
		{
			name:              "git plugin download url",
			pluginName:        "pulumi-example",
			pluginDownloadURL: "git://github.com/pulumi/pulumi-example",
			expected:          "github.com_pulumi_pulumi-example.git",
		},
		{
			name:              "git plugin download url with path",
			pluginName:        "pulumi-example",
			pluginDownloadURL: "git://github.com/pulumi/pulumi-example/path",
			expected:          "github.com_pulumi_pulumi-example.git",
			expectedPath:      "path",
		},
		{
			name:              "invalid git plugin download url",
			pluginName:        "pulumi-example",
			pluginDownloadURL: "git://github",
			expected:          "github",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			spec := PluginSpec{
				Name:              c.pluginName,
				PluginDownloadURL: c.pluginDownloadURL,
			}
			name, path := spec.LocalName()
			require.Equal(t, c.expected, name)
			require.Equal(t, c.expectedPath, path)
		})
	}
}
