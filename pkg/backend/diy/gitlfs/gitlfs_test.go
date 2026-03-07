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

package gitlfs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPointerParsing tests LFS pointer file parsing
func TestPointerParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected *Pointer
		wantErr  bool
	}{
		{
			name: "valid pointer",
			input: `version https://git-lfs.github.com/spec/v1
oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393
size 12345
`,
			expected: &Pointer{
				Version: "https://git-lfs.github.com/spec/v1",
				OID:     "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
				Size:    12345,
			},
			wantErr: false,
		},
		{
			name:     "missing version",
			input:    "oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 12345\n",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "missing oid",
			input:    "version https://git-lfs.github.com/spec/v1\nsize 12345\n",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid oid format",
			input:    "version https://git-lfs.github.com/spec/v1\noid md5:abc123\nsize 12345\n",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := Parse([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Version, result.Version)
			assert.Equal(t, tt.expected.OID, result.OID)
			assert.Equal(t, tt.expected.Size, result.Size)
		})
	}
}

// TestPointerBytes tests LFS pointer file serialization
func TestPointerBytes(t *testing.T) {
	t.Parallel()

	pointer := &Pointer{
		Version: LFSVersion,
		OID:     "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
		Size:    12345,
	}

	bytes := pointer.Bytes()
	expected := `version https://git-lfs.github.com/spec/v1
oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393
size 12345
`
	assert.Equal(t, expected, string(bytes))

	// Test round-trip
	parsed, err := Parse(bytes)
	require.NoError(t, err)
	assert.Equal(t, pointer.Version, parsed.Version)
	assert.Equal(t, pointer.OID, parsed.OID)
	assert.Equal(t, pointer.Size, parsed.Size)
}

// TestIsPointer tests LFS pointer detection
func TestIsPointer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name: "valid pointer",
			input: []byte(`version https://git-lfs.github.com/spec/v1
oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393
size 12345
`),
			expected: true,
		},
		{
			name:     "json content",
			input:    []byte(`{"foo": "bar"}`),
			expected: false,
		},
		{
			name:     "binary content",
			input:    []byte{0x00, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "empty",
			input:    []byte{},
			expected: false,
		},
		{
			name:     "almost pointer but invalid",
			input:    []byte("version https://git-lfs.github.com/spec/v1\n"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsPointer(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNewPointer tests creating a pointer from content
func TestNewPointer(t *testing.T) {
	t.Parallel()

	data := []byte("hello world")
	pointer := NewPointer(data)

	assert.Equal(t, LFSVersion, pointer.Version)
	assert.Equal(t, int64(11), pointer.Size)
	assert.Contains(t, pointer.OID, "sha256:")
	require.Len(t, pointer.SHA256(), 64)
}

// TestComputeOID tests OID computation
func TestComputeOID(t *testing.T) {
	t.Parallel()

	// Known SHA256 hash for "hello world"
	data := []byte("hello world")
	oid := ComputeOID(data)
	assert.Equal(t, "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", oid)
}

// TestBatchAPIClientDownload tests the LFS Batch API client download operation
func TestBatchAPIClientDownload(t *testing.T) {
	t.Parallel()

	server := newMockLFSServer()
	defer server.Close()

	client := NewClient(server.URL, nil)
	resp, err := client.Batch(context.Background(), &BatchRequest{
		Operation: "download",
		Objects: []ObjectSpec{
			{OID: "sha256:abc123", Size: 100},
		},
	})

	require.NoError(t, err)
	require.Len(t, resp.Objects, 1)
	assert.Equal(t, "sha256:abc123", resp.Objects[0].OID)
	assert.Contains(t, resp.Objects[0].Actions, "download")
}

// TestBatchAPIClientUpload tests the LFS Batch API client upload operation
func TestBatchAPIClientUpload(t *testing.T) {
	t.Parallel()

	server := newMockLFSServer()
	defer server.Close()

	client := NewClient(server.URL, nil)
	resp, err := client.Batch(context.Background(), &BatchRequest{
		Operation: "upload",
		Objects: []ObjectSpec{
			{OID: "sha256:def456", Size: 200},
		},
	})

	require.NoError(t, err)
	require.Len(t, resp.Objects, 1)
	assert.Equal(t, "sha256:def456", resp.Objects[0].OID)
	assert.Contains(t, resp.Objects[0].Actions, "upload")
}

// newMockLFSServer creates a mock LFS server for testing
func newMockLFSServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/objects/batch" {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check headers
		if r.Header.Get("Accept") != LFSMediaType {
			http.Error(w, "invalid accept header", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Content-Type") != LFSMediaType {
			http.Error(w, "invalid content-type header", http.StatusBadRequest)
			return
		}

		// Parse request
		var req BatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Return mock response
		resp := BatchResponse{
			Transfer: "basic",
			Objects: []ObjectResult{
				{
					OID:  req.Objects[0].OID,
					Size: req.Objects[0].Size,
					Actions: map[string]Action{
						"download": {
							Href: "http://localhost/download/" + req.Objects[0].OID,
						},
						"upload": {
							Href: "http://localhost/upload/" + req.Objects[0].OID,
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", LFSMediaType)
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
}

// TestBuildLFSURL tests LFS URL construction
func TestBuildLFSURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host     string
		owner    string
		repo     string
		expected string
	}{
		{
			host:     "github.com",
			owner:    "myorg",
			repo:     "myrepo",
			expected: "https://github.com/myorg/myrepo.git/info/lfs",
		},
		{
			host:     "gitlab.com",
			owner:    "company",
			repo:     "project",
			expected: "https://gitlab.com/company/project.git/info/lfs",
		},
		{
			host:     "gitea.example.com",
			owner:    "team",
			repo:     "state",
			expected: "https://gitea.example.com/team/state.git/info/lfs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			result := BuildLFSURL(tt.host, tt.owner, tt.repo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTokenAuth tests token authentication
func TestTokenAuth(t *testing.T) {
	t.Parallel()

	auth := NewTokenAuth("test-token")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	err := auth.Authenticate(req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
}

// TestBasicAuth tests basic authentication
func TestBasicAuth(t *testing.T) {
	t.Parallel()

	auth := NewBasicAuth("user", "pass")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	err := auth.Authenticate(req)
	require.NoError(t, err)

	authHeader := req.Header.Get("Authorization")
	assert.True(t, len(authHeader) > 0)
	assert.Contains(t, authHeader, "Basic ")
}

// TestPointerValidation tests pointer validation
func TestPointerValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pointer *Pointer
		wantErr bool
	}{
		{
			name: "valid pointer",
			pointer: &Pointer{
				Version: LFSVersion,
				OID:     "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
				Size:    100,
			},
			wantErr: false,
		},
		{
			name: "missing version",
			pointer: &Pointer{
				OID:  "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
				Size: 100,
			},
			wantErr: true,
		},
		{
			name: "missing oid",
			pointer: &Pointer{
				Version: LFSVersion,
				Size:    100,
			},
			wantErr: true,
		},
		{
			name: "invalid oid format",
			pointer: &Pointer{
				Version: LFSVersion,
				OID:     "md5:invalid",
				Size:    100,
			},
			wantErr: true,
		},
		{
			name: "negative size",
			pointer: &Pointer{
				Version: LFSVersion,
				OID:     "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
				Size:    -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.pointer.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPointerSHA256 tests extracting SHA256 from OID
func TestPointerSHA256(t *testing.T) {
	t.Parallel()

	pointer := &Pointer{
		OID: "sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393",
	}

	sha := pointer.SHA256()
	assert.Equal(t, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", sha)
}
