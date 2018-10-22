package local

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/pulumi/pulumi/pkg/backend/filestate/storage"
)

const (
	// URLPrefix is a unique schema used to identify this bucket provider
	URLPrefix = "file://"
)

var _ storage.Bucket = (*bucket)(nil) // enforces compile time check for interface compatibility

// bucket is a blob storage implementation using the local file system
type bucket struct {
	lock localLock
}

type localLock struct {
	flock *flock.Flock
	mu    sync.Mutex
}

// NewBucket create a new Bucket instance
func NewBucket(url, accountKey string) (storage.Bucket, error) {
	return &bucket{}, nil
}

// ListFiles returns a list of all the files in a directory matching a given prefix (directory name)
func (b *bucket) ListFiles(ctx context.Context, prefix string) ([]string, error) {
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
func (b *bucket) WriteFile(ctx context.Context, path string, bytes []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	return ioutil.WriteFile(path, bytes, os.ModePerm)
}

// ReadFile will read the contents of a file
func (b *bucket) ReadFile(ctx context.Context, path string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// DeleteFile will delete a file
func (b *bucket) DeleteFile(ctx context.Context, path string) error {
	return os.RemoveAll(path)
}

// RenameFile will rename a file
func (b *bucket) RenameFile(ctx context.Context, path, newPath string) error {
	return os.Rename(path, newPath)
}

// DeleteFiles will delete all files under a given prefix (directory name)
func (b *bucket) DeleteFiles(ctx context.Context, prefix string) error {
	return os.RemoveAll(prefix)
}

// IsNotExist will return true if the provided error is a file or directory not found error
func (b *bucket) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Lock is a blocking call to try and take the lock for this
// stack. Once it has the lock it will return a unlocker
// function that can then be used by the client.
func (b *bucket) Lock(ctx context.Context, stackName string) (storage.UnlockFn, error) {
	if b.lock.flock == nil {
		b.lock.flock = flock.New(fmt.Sprintf("/var/lock/pulumi-%s.lock", stackName))
	}
	// Get the mutex first because this ensures
	// we can control data access within this
	// process, across multiple goroutines.
	b.lock.mu.Lock()

	// Get the file lock to ensure only one
	// process has write access to the state
	// files at any time.
	//
	// TODO: Should we add an additional timeout
	// or notify the user whilst waiting for the
	// file lock?
	locked, err := b.lock.flock.TryLockContext(ctx, time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to take lock for stack %s", stackName)
	}
	if !locked {
		// If we couldn't take the file lock but we didn't error
		// the context's done channel has closed.
		b.lock.mu.Unlock()
		return nil, fmt.Errorf("context done channel closed, unable to take lock")
	}
	unlocker := func() error {
		flocked := b.lock.flock.Locked()
		// Only unlock the mutex once the file lock has been released
		defer func() {
			if flocked {
				// If we had the file lock, we are guaranteed
				// to have the mutex lock so unlock it.
				b.lock.mu.Unlock()
			}
		}()
		return b.lock.flock.Unlock()
	}
	return unlocker, nil
}
