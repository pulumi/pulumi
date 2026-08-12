// Copyright 2026, Pulumi Corporation.
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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
)

// strictS3Server is a minimal fake of an S3-compatible object store that
// validates the CopyObject copy source the way strict implementations such
// as NetApp StorageGRID do: the bucket name and the object key must be
// separated by a literal, unencoded "/" in the x-amz-copy-source header.
// AWS itself tolerates a fully percent-encoded copy source (with "%2F" for
// the separators), but several S3-compatible servers do not, which is what
// https://github.com/pulumi/pulumi/issues/23478 is about.
type strictS3Server struct {
	mu          sync.Mutex
	copySources []string
}

func (s *strictS3Server) recordCopySource(src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.copySources = append(s.copySources, src)
}

func (s *strictS3Server) lastCopySource() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.copySources) == 0 {
		return ""
	}
	return s.copySources[len(s.copySources)-1]
}

func (s *strictS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	copySource := r.Header.Get("X-Amz-Copy-Source")
	if r.Method == http.MethodPut && copySource != "" {
		s.recordCopySource(copySource)

		// Strict copy source validation: "<bucket>/<key>", optionally with a
		// leading "/". The separator between the bucket and the key must be a
		// literal slash; if the whole value is percent-encoded there is no
		// key, which strict servers reject.
		trimmed := strings.TrimPrefix(copySource, "/")
		bucket, key, ok := strings.Cut(trimmed, "/")
		w.Header().Set("Content-Type", "application/xml")
		if !ok || bucket == "" || key == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`+
				`<Error><Code>InvalidArgument</Code><Message>Invalid copy source object key</Message></Error>`)
			return
		}
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`+
			`<CopyObjectResult>`+
			`<LastModified>2026-06-11T00:00:00Z</LastModified>`+
			`<ETag>&quot;9b2cf535f27731c974343645a3985328&quot;</ETag>`+
			`</CopyObjectResult>`)
		return
	}

	// The tests only exercise CopyObject; nothing else should be called.
	w.WriteHeader(http.StatusNotImplemented)
}

// decodeCopySource parses a x-amz-copy-source header value into its bucket
// and key the way a strict S3-compatible server does: split bucket and key
// on a literal "/", then percent-decode each path segment of the key.
func decodeCopySource(t *testing.T, src string) (string, string) {
	t.Helper()
	bucket, key, ok := strings.Cut(strings.TrimPrefix(src, "/"), "/")
	require.True(t, ok, "copy source %q does not contain a literal bucket/key separator", src)
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		decoded, err := url.PathUnescape(segment)
		require.NoError(t, err, "copy source key segment %q is not valid percent-encoding", segment)
		segments[i] = decoded
	}
	return bucket, strings.Join(segments, "/")
}

// TestS3LegacyURLCopy reproduces https://github.com/pulumi/pulumi/issues/23478:
// saving a checkpoint to an S3 DIY backend performs a blob Copy (see
// addToHistory and backupTarget in state.go), and S3-compatible servers like
// NetApp StorageGRID reject the copy source format that gocloud.dev v0.46
// produces. It also covers the AWS SDK v1-era URL parameters (disableSSL,
// scheme-less endpoint) that backends configured before the gocloud.dev
// upgrade still use.
func TestS3LegacyURLCopy(t *testing.T) {
	// No t.Parallel: subtests mutate process-wide env vars via t.Setenv.

	tests := []struct {
		name string
		// query is appended to "s3://testbucket". The %s placeholders are
		// replaced with the fake server's host (with or without scheme).
		query        string
		bareEndpoint bool
	}{
		{
			// The exact backend URL shape reported in pulumi/pulumi#23478.
			name:  "issue-23478",
			query: "?endpoint=%s&s3ForcePathStyle=true&awssdk=v2",
		},
		{
			// The same configuration without forcing an SDK version. Before
			// the gocloud.dev 0.46 upgrade this selected the AWS SDK v1 code
			// path, which was the default for S3 DIY backends.
			name:  "no-awssdk",
			query: "?endpoint=%s&s3ForcePathStyle=true",
		},
		{
			// AWS SDK v1-era parameters: disableSSL instead of
			// disable_https, and a scheme-less endpoint whose scheme was
			// implied by disableSSL.
			name:         "legacy-v1-params",
			query:        "?endpoint=%s&disableSSL=true&s3ForcePathStyle=true",
			bareEndpoint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
			t.Setenv("AWS_REGION", "us-east-1")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			// Isolate from the developer's ~/.aws configuration.
			t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))

			s3 := &strictS3Server{}
			server := httptest.NewServer(s3)
			defer server.Close()

			endpoint := server.URL
			if tt.bareEndpoint {
				endpoint = strings.TrimPrefix(endpoint, "http://")
			}
			bucketURL := "s3://testbucket" + fmt.Sprintf(tt.query, endpoint)

			// Open the bucket the same way newDIYBackend does.
			ctx := t.Context()
			u, err := massageBlobPath(bucketURL)
			require.NoError(t, err)
			bucket, err := blob.DefaultURLMux().OpenBucket(ctx, u)
			require.NoError(t, err, "opening bucket %s", bucketURL)
			defer func() {
				require.NoError(t, bucket.Close())
			}()
			wbucket := &wrappedBucket{bucket: bucket}

			// The copy addToHistory performs when saving update info, with
			// the keys from the issue report.
			err = wbucket.Copy(ctx,
				".pulumi/history/blubb/stack/stack-1780911700522805856.checkpoint.json",
				".pulumi/stacks/blubb/stack.json", nil)
			require.NoError(t, err)

			gotBucket, gotKey := decodeCopySource(t, s3.lastCopySource())
			assert.Equal(t, "testbucket", gotBucket)
			assert.Equal(t, ".pulumi/stacks/blubb/stack.json", gotKey)
		})
	}
}
