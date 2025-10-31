package postgres

import postgres "github.com/pulumi/pulumi/sdk/v3/pkg/backend/diy/postgres"

// Bucket implements blob.Bucket storage using PostgreSQL.
type Bucket = postgres.Bucket

// NewPostgresBucket creates a new Bucket.
func NewPostgresBucket(ctx context.Context, u *url.URL) (*Bucket, error) {
	return postgres.NewPostgresBucket(ctx, u)
}

