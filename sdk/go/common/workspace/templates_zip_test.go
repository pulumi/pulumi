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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
				assert.NoError(t, err)
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

//nolint:paralleltest // uses shared server URL
func TestRetrieveZIPTemplates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		fileName := "foo"
		fileDirName := "bar/baz"
		buf := new(bytes.Buffer)
		writer := zip.NewWriter(buf)
		data := []byte("foo")
		fileDirPathParts := strings.Split(fileDirName, "/")
		_, err := writer.Create(strings.Join(fileDirPathParts[:len(fileDirPathParts)-1], "/") + "/")
		if err != nil {
			t.Errorf("Failed to create directory in zip archive: %s", err)
		}
		fileHandle, err := writer.Create(fileName)
		if err != nil {
			t.Errorf("Failed to create file in zip archive: %s", err)
		}
		_, err = fileHandle.Write(data)
		if err != nil {
			t.Errorf("Failed to write to zip file: %s", err)
		}
		nestedFileHandle, err := writer.Create(fileDirName)
		if err != nil {
			t.Errorf("Failed to create nested file in zip archive: %s", err)
		}
		_, err = nestedFileHandle.Write(data)
		if err != nil {
			t.Errorf("Failed to write nested file to zip archive: %s", err)
		}
		writer.Close()
		rw.Header().Set("Content-Type", "application/zip")
		rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", fileName))
		_, err = rw.Write(buf.Bytes())
		if err != nil {
			t.Errorf("Failed to write to response: %s", err)
		}
	}))
	defer server.Close()
	tests := []struct {
		testName      string
		templateURL   string
		expectedError string
	}{
		{
			testName:    "valid_zip_url",
			templateURL: server.URL + "/foo.zip",
		},
		{
			testName:    "invalid_zip_url",
			templateURL: "not a url",
			expectedError: "failed to retrieve zip archive: " +
				"invalid template URL: not%20a%20url",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			_, err := retrieveZIPTemplates(tt.templateURL)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
