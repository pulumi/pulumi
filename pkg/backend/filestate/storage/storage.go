package storage

import (
	"context"
)

// BucketCreater should be implemented by each provider to instantiate
// a new specialized bucket instance.
type BucketCreater func(url, accessToken string) (Bucket, error)

// Bucket defines an interface for bucket level operations
// to be implemented by specialized file providers.
type Bucket interface {
	DeleteFiles(ctx context.Context, prefix string) error
	ListFiles(ctx context.Context, prefix string) ([]string, error)

	WriteFile(ctx context.Context, path string, bytes []byte) error
	ReadFile(ctx context.Context, path string) ([]byte, error)
	DeleteFile(ctx context.Context, path string) error
	RenameFile(ctx context.Context, path, newPath string) error
	IsNotExist(err error) bool
}
