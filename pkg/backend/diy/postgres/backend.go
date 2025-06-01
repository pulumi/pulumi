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

	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	// PostgresScheme is the scheme for PostgreSQL backend URLs
	PostgresScheme = "postgres"
)

func init() {
	// Register the PostgreSQL bucket provider with the default blob.URLMux
	// This allows the default muxer to recognize PostgreSQL URLs automatically
	_ = PostgresSchemeMux{}.RegisterBucketMux(blob.DefaultURLMux())
}

// PostgresSchemeMux is a URL opener for PostgreSQL URLs.
type PostgresSchemeMux struct{}

// OpenBucketURL implements blob.BucketURLOpener.
func (p PostgresSchemeMux) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	config := u.String()
	pg, err := NewPostgresBucket(ctx, config)
	if err != nil {
		return nil, err
	}
	return pg.Bucket(), nil
}

// RegisterBucketMux registers the blob.URLMux provider for PostgreSQL.
func (p PostgresSchemeMux) RegisterBucketMux(mux *blob.URLMux) error {
	// Check if the scheme is already registered to avoid double registration
	if mux.ValidBucketScheme(PostgresScheme) {
		return nil
	}

	mux.RegisterBucket(PostgresScheme, p)
	return nil
}

// IsPostgresBackendURL returns true if the URL is a PostgreSQL backend URL.
func IsPostgresBackendURL(urlstr string) bool {
	u, err := url.Parse(urlstr)
	if err != nil {
		return false
	}
	return u.Scheme == PostgresScheme
}

// New creates a new DIY backend using PostgreSQL as the storage layer.
func New(ctx context.Context, d diag.Sink, postgresURL string, project *workspace.Project) (diy.Backend, error) {
	// The PostgreSQL bucket provider is automatically registered via init()
	// Configure and initialize the PostgreSQL bucket
	return diy.New(ctx, d, postgresURL, project)
}

// Login creates or connects to a DIY backend using PostgreSQL as the storage layer.
func Login(ctx context.Context, d diag.Sink, postgresURL string, project *workspace.Project) (diy.Backend, error) {
	// The PostgreSQL bucket provider is automatically registered via init()
	be, err := diy.New(ctx, d, postgresURL, project)
	if err != nil {
		return nil, err
	}
	return be, workspace.StoreAccount(be.URL(), workspace.Account{}, true)
}
