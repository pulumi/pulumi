// Copyright 2016-2024, Pulumi Corporation.
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

package diy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLockURLForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		lockPath string
		expected string
	}{
		{
			name:     "Local file URL",
			baseURL:  "file:///Users/user",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "file:///Users/user/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "Local file URL with query param",
			baseURL:  "file:///Users/user?no_tmp_dir=true",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "file:///Users/user/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json?no_tmp_dir=true",
		},
		{
			name:     "S3 URL",
			baseURL:  "s3://mybucket/testfile",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "s3://mybucket/testfile/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "S3 URL with query param",
			baseURL:  "s3://mybucket/testfile?region=eu-central-1",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "s3://mybucket/testfile/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json?region=eu-central-1",
		},
		{
			name:     "Local path",
			baseURL:  "/Users/user",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "/Users/user/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "Invalid URL format",
			baseURL:  ":bad:url",
			lockPath: "lock/file",
			expected: ":bad:url/lock/file",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			backend := &diyBackend{url: tt.baseURL}
			result := backend.lockURLForError(tt.lockPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
