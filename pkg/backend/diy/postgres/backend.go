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

package postgres

import (
	"context"
	"net/url"

	_ "github.com/lib/pq" // Import PostgreSQL driver
	"gocloud.dev/blob"
)

const (
	// PostgresScheme is the scheme for PostgreSQL backend URLs
	PostgresScheme = "postgres"
)

func init() {
	// Register the PostgreSQL bucket provider with the default blob.URLMux
	// This allows the default muxer to recognize PostgreSQL URLs automatically
	mux := blob.DefaultURLMux()

	// Check if the scheme is already registered to avoid double registration
	if !mux.ValidBucketScheme(PostgresScheme) {
		mux.RegisterBucket(PostgresScheme, URLHandler{})
	}
}

// URLHandler is a URL opener for PostgreSQL URLs.
type URLHandler struct{}

// OpenBucketURL implements blob.BucketURLOpener.
func (p URLHandler) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	pg, err := NewPostgresBucket(ctx, u)
	if err != nil {
		return nil, err
	}
	return pg.Bucket(), nil
}
