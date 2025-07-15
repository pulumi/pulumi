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
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeArchivePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName   string
		dir        string
		fileName   string
		shouldFail bool
	}{
		{
			testName:   "valid_path",
			dir:        "foo",
			fileName:   "bar",
			shouldFail: false,
		},
		{
			testName:   "invalid_path",
			dir:        "foo",
			fileName:   "../../../../../../../../../../tmp/bar",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizeArchivePath(tt.dir, tt.fileName)
			if tt.shouldFail {
				assert.ErrorContains(t, err, "content filepath is tainted")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsZipArchiveURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName    string
		templateURL string
		expected    bool
	}{
		{
			testName:    "http_zip_archive_url",
			templateURL: "http://example.com/foo.zip",
			expected:    true,
		},
		{
			testName:    "https_zip_archive_url",
			templateURL: "https://localhost:3001/www-ai/api/project/foo.zip",
			expected:    true,
		},
		{
			testName:    "http_zip_archive_url_with_query",
			templateURL: "http://example.com/foo.zip?foo=bar",
			expected:    true,
		},
		{
			testName:    "http_zip_archive_url_with_fragment",
			templateURL: "http://example.com/foo.zip#foo",
			expected:    true,
		},
		{
			testName:    "http_zip_archive_url_with_query_and_fragment",
			templateURL: "http://example.com/foo.zip?foo=bar#foo",
			expected:    true,
		},
		{
			testName:    "git_ssh_url",
			templateURL: "ssh://github.com/pulumi/templates/archive/master.git",
			expected:    false,
		},
		{
			testName:    "git_https_url",
			templateURL: "https://github.com/pulumi/templates/archive/master",
			expected:    false,
		},
		{
			testName:    "git_ssh_url",
			templateURL: "git@gitlab.com:group/project.git",
			expected:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			result := isZIPTemplateURL(tt.templateURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetrieveZIPTemplates_FailsOnInvalidURLs(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []string{
		"path",
		"not a url",
		"ftp/example.com/foo.zip",
	}

	for _, templateURL := range cases {
		parsed, err := url.Parse(templateURL)
		require.NoError(t, err)

		// Act.
		_, err = RetrieveZIPTemplates(templateURL)

		// Assert.
		assert.ErrorContains(t, err, "invalid template URL: "+parsed.String())
	}
}

//nolint:paralleltest // uses shared server URL
func TestRetrieveZIPTemplates_FailsWhenPulumiYAMLIsMissing(t *testing.T) {
	// Arrange.
	cases := map[string][]string{
		"empty.zip":          {},
		"no-pulumi-yaml.zip": {"foo", "bar/baz"},
	}

	server := newTestServer(t, cases)

	for path := range cases {
		// Act.
		_, err := RetrieveZIPTemplates(server.URL + "/" + path)

		// Assert.
		assert.ErrorContains(t, err, "template does not contain a Pulumi.yaml file")
	}
}

//nolint:paralleltest // uses shared server URL
func TestRetrieveZIPTemplates_SucceedsWhenPulumiYAMLIsPresent(t *testing.T) {
	// Arrange.
	cases := map[string][]string{
		"just-pulumi-yaml.zip":                    {"Pulumi.yaml"},
		"pulumi-yaml-and-flat-files.zip":          {"Pulumi.yaml", "foo"},
		"pulumi-yaml-and-nested-files.zip":        {"Pulumi.yaml", "bar/baz"},
		"pulumi-yaml-and-mixture.zip":             {"Pulumi.yaml", "foo", "bar/baz"},
		"pulumi-yaml-at-top-level-and-nested.zip": {"Pulumi.yaml", "foo", "bar/Pulumi.yaml"},
	}

	server := newTestServer(t, cases)

	for path := range cases {
		// Act.
		_, err := RetrieveZIPTemplates(server.URL + "/" + path)

		// Assert.
		require.NoError(t, err)
	}
}

func TestRetrieveZIPTemplates_ReturnsMeaningfulErrorOn5xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
		_, err := rw.Write([]byte("Missing , or : between flow sequence items at line 30, column 20"))
		require.NoError(t, err)
	}))

	_, err := RetrieveZIPTemplates(server.URL)

	assert.ErrorContains(t, err, "failed to download template: 500 Internal Server Error\n"+
		"Missing , or : between flow sequence items at line 30, column 20")
}

func TestRetrieveZIPTemplates_RaisesDetectablePulumiCloud401Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// How we id a Pulumi Cloud from an arbitrary http response.
		rw.Header().Set("X-Pulumi-Request-ID", "123")
		rw.WriteHeader(http.StatusUnauthorized)
		_, err := rw.Write([]byte("Unauthorized"))
		require.NoError(t, err)
	}))

	_, err := RetrieveZIPTemplates(server.URL)

	// Make sure we can detect the error as a ErrPulumiCloudUnauthorized.
	// Allows us to handle it properly and retry w/ credentials etc if needed.
	assert.ErrorIs(t, err, ErrPulumiCloudUnauthorized)
}

func TestRetrieveZIPTemplates_RequestOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Assert that the Authorization header is set in the request.
		assert.Equal(t, "token 123", req.Header.Get("Authorization"))
	}))

	_, err := RetrieveZIPTemplates(server.URL, func(req *http.Request) {
		req.Header.Set("Authorization", "token 123")
	})
	assert.Error(t, err)
}

// Returns a new test HTTP server that responds to requests according to the supplied map. Keys in the map correspond to
// paths, while values are slices whose values correspond to filenames that should be present in the ZIP file served at
// that path.
func newTestServer(t *testing.T, zips map[string][]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		zipName := req.URL.Path[1:]
		files, ok := zips[zipName]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		buf := new(bytes.Buffer)
		writer := zip.NewWriter(buf)
		for _, file := range files {
			// For paths containing slashes, we need to create the directories first.
			dirs := strings.Split(file, "/")
			for i := 0; i < len(dirs)-1; i++ {
				path := strings.Join(dirs[:i+1], "/")
				_, err := writer.Create(path + "/")
				require.NoError(t, err)
			}

			fileHandle, err := writer.Create(file)
			require.NoError(t, err)

			// All files contain the same piece of test content.
			_, err = fileHandle.Write([]byte("test"))
			require.NoError(t, err)
		}
		writer.Close()

		rw.Header().Set("Content-Type", "application/zip")
		rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", zipName))

		_, err := rw.Write(buf.Bytes())
		require.NoError(t, err)
	}))
}
