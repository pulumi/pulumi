package local

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/backend/filestate/storage"
)

const (
	// URLPrefix is a unique schema used to identify this bucket provider
	URLPrefix = "file://"
)

var _ storage.Bucket = (*Bucket)(nil) // enforces compile time check for interface compatibility

// Bucket is a blob storage implementation using the local file system
type Bucket struct{}

// NewBucket create a new Bucket instance
func NewBucket(url, accountKey string) (storage.Bucket, error) {
	return &Bucket{}, nil
}

// ListFiles returns a list of all the files in a directory matching a given prefix (directory name)
func (b *Bucket) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	files, err := ioutil.ReadDir(prefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		if f.IsDir() {
			continue // Ignore directories
		}
		names = append(names, f.Name())
	}
	return names, nil
}

// WriteFile will create any directories present in the file path and then write the file itself
func (b *Bucket) WriteFile(ctx context.Context, path string, bytes []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	return ioutil.WriteFile(path, bytes, os.ModePerm)
}

// ReadFile will read the contents of a file
func (b *Bucket) ReadFile(ctx context.Context, path string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// DeleteFile will delete a file
func (b *Bucket) DeleteFile(ctx context.Context, path string) error {
	return os.RemoveAll(path)
}

// RenameFile will rename a file
func (b *Bucket) RenameFile(ctx context.Context, path, newPath string) error {
	return os.Rename(path, newPath)
}

// DeleteFiles will delete all files under a given prefix (directory name)
func (b *Bucket) DeleteFiles(ctx context.Context, prefix string) error {
	return os.RemoveAll(prefix)
}

// IsNotExist will return true if the provided error is a file or directory not found error
func (b *Bucket) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}
