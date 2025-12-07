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
	"net/url"

	"gocloud.dev/blob"
)

const (
	// Scheme is the URL scheme for Git LFS backend
	Scheme = "gitlfs"
)

func init() {
	// Register the Git LFS bucket provider with the default blob.URLMux
	// This allows the default muxer to recognize gitlfs:// URLs automatically
	mux := blob.DefaultURLMux()

	// Check if the scheme is already registered to avoid double registration
	if !mux.ValidBucketScheme(Scheme) {
		mux.RegisterBucket(Scheme, URLHandler{})
	}
}

// URLHandler is a URL opener for Git LFS URLs.
type URLHandler struct{}

// OpenBucketURL implements blob.BucketURLOpener.
// It opens a bucket from a URL in the format:
//
//	gitlfs://host/owner/repo[?ref=branch&path=subdir&lfs_threshold=bytes]
//
// Examples:
//   - gitlfs://github.com/myorg/pulumi-state
//   - gitlfs://gitlab.com/myorg/infra-state?ref=main
//   - gitlfs://gitea.example.com/team/state?path=production
//
// Query parameters:
//   - ref: Git branch to use (default: "main")
//   - path: Subdirectory within the repository
//   - lfs_threshold: Size threshold in bytes for using LFS (default: 102400)
func (h URLHandler) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	bucket, err := NewLFSBucket(ctx, u)
	if err != nil {
		return nil, err
	}
	return bucket.Bucket(), nil
}
