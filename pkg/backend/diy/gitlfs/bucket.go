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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
)

const (
	// defaultLFSSizeThreshold is the default size threshold for using LFS (100KB)
	defaultLFSSizeThreshold = 100 * 1024

	// pushRetryCount is the number of times to retry a push on conflict
	pushRetryCount = 3
)

// Bucket implements blob storage using Git LFS
type Bucket struct {
	repo       *Repository
	lfsClient  *Client
	bucket     *blob.Bucket
	mu         sync.RWMutex
	threshold  int64 // Size threshold for using LFS
	autoCommit bool  // Whether to auto-commit after writes
}

// BucketOptions contains options for creating an LFS bucket
type BucketOptions struct {
	// Branch is the Git branch to use (default: "main")
	Branch string

	// Subdir is a subdirectory within the repo to use
	Subdir string

	// LFSThreshold is the size threshold in bytes for using LFS
	// Files larger than this will be stored in LFS
	LFSThreshold int64

	// AutoCommit controls whether to auto-commit after each write
	AutoCommit bool
}

// NewLFSBucket creates a new Git LFS bucket from a URL
// URL format: gitlfs://host/owner/repo[?ref=branch&path=subdir&lfs_threshold=bytes]
func NewLFSBucket(ctx context.Context, u *url.URL) (*Bucket, error) {
	// Parse URL components
	host := u.Host
	pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid gitlfs URL: expected gitlfs://host/owner/repo, got %s", u.String())
	}
	owner := pathParts[0]
	repo := pathParts[1]

	// Parse query parameters
	query := u.Query()
	branch := query.Get("ref")
	if branch == "" {
		branch = defaultBranch
	}
	subdir := query.Get("path")

	threshold := int64(defaultLFSSizeThreshold)
	if thresholdStr := query.Get("lfs_threshold"); thresholdStr != "" {
		if _, err := fmt.Sscanf(thresholdStr, "%d", &threshold); err != nil {
			return nil, fmt.Errorf("invalid lfs_threshold: %w", err)
		}
	}

	// Also check environment variable for threshold
	if envThreshold := os.Getenv("PULUMI_DIY_BACKEND_GITLFS_SIZE_THRESHOLD"); envThreshold != "" {
		var envThresholdVal int64
		if _, err := fmt.Sscanf(envThreshold, "%d", &envThresholdVal); err == nil {
			threshold = envThresholdVal
		}
	}

	opts := &BucketOptions{
		Branch:       branch,
		Subdir:       subdir,
		LFSThreshold: threshold,
		AutoCommit:   true,
	}

	return NewLFSBucketWithOptions(ctx, host, owner, repo, opts)
}

// NewLFSBucketWithOptions creates a new Git LFS bucket with explicit options
func NewLFSBucketWithOptions(ctx context.Context, host, owner, repo string, opts *BucketOptions) (*Bucket, error) {
	if opts == nil {
		opts = &BucketOptions{}
	}
	if opts.Branch == "" {
		opts.Branch = defaultBranch
	}
	if opts.LFSThreshold == 0 {
		opts.LFSThreshold = defaultLFSSizeThreshold
	}

	// Build remote URL
	remote := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

	// Create/open the repository
	repository, err := NewRepository(ctx, remote, opts.Branch, opts.Subdir)
	if err != nil {
		return nil, fmt.Errorf("initializing repository: %w", err)
	}

	// Create authenticator
	path := fmt.Sprintf("%s/%s", owner, repo)
	auth, err := NewAuthenticator(ctx, host, path)
	if err != nil {
		return nil, fmt.Errorf("creating authenticator: %w", err)
	}

	// Create LFS client
	lfsURL := BuildLFSURL(host, owner, repo)
	lfsClient := NewClient(lfsURL, auth)

	bucket := &Bucket{
		repo:       repository,
		lfsClient:  lfsClient,
		threshold:  opts.LFSThreshold,
		autoCommit: opts.AutoCommit,
	}

	// Create the driver and wrap with blob.Bucket
	drv := &lfsBucketDriver{bucket: bucket}
	bucket.bucket = blob.NewBucket(drv)

	return bucket, nil
}

// Bucket returns the blob.Bucket
func (b *Bucket) Bucket() *blob.Bucket {
	return b.bucket
}

// Close closes the bucket
func (b *Bucket) Close() error {
	if b.bucket != nil {
		if err := b.bucket.Close(); err != nil {
			return err
		}
	}
	if b.repo != nil {
		return b.repo.Close()
	}
	return nil
}

// Pull pulls latest changes from remote
func (b *Bucket) Pull(ctx context.Context) error {
	return b.repo.Pull(ctx)
}

// CommitAndPush commits pending changes and pushes to remote
func (b *Bucket) CommitAndPush(ctx context.Context, message string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.repo.Commit(ctx, message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := b.repo.PushWithRetry(ctx, pushRetryCount); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	return nil
}

// lfsBucketDriver implements driver.Bucket for Git LFS
type lfsBucketDriver struct {
	bucket *Bucket
}

// As implements driver.Bucket.As
func (d *lfsBucketDriver) As(i any) bool {
	p, ok := i.(**Bucket)
	if !ok {
		return false
	}
	*p = d.bucket
	return true
}

// ErrorAs implements driver.Bucket.ErrorAs
func (d *lfsBucketDriver) ErrorAs(err error, i any) bool {
	return false
}

// ErrorCode implements driver.Bucket.ErrorCode
func (d *lfsBucketDriver) ErrorCode(err error) gcerrors.ErrorCode {
	if errors.Is(err, fs.ErrNotExist) {
		return gcerrors.NotFound
	}
	if errors.Is(err, ErrNotFound) {
		return gcerrors.NotFound
	}
	if errors.Is(err, ErrUnauthorized) {
		return gcerrors.PermissionDenied
	}
	if errors.Is(err, ErrForbidden) {
		return gcerrors.PermissionDenied
	}
	return gcerrors.Unknown
}

// ListPaged implements driver.Bucket.ListPaged
func (d *lfsBucketDriver) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {
	// Pull latest before listing
	// Ignore errors - we might be offline
	_ = d.bucket.repo.Pull(ctx)

	prefix := opts.Prefix
	delimiter := opts.Delimiter

	files, err := d.bucket.repo.ListFilesWithInfo(prefix)
	if err != nil {
		return nil, err
	}

	page := &driver.ListPage{}
	seenDirs := make(map[string]bool)

	for _, file := range files {
		key := file.Key

		// Handle delimiter for "virtual directories"
		if delimiter != "" {
			keyAfterPrefix := strings.TrimPrefix(key, prefix)
			delimiterIndex := strings.Index(keyAfterPrefix, delimiter)
			if delimiterIndex >= 0 {
				// This is a "directory"
				dirPrefix := key[:len(prefix)+delimiterIndex+1]
				if seenDirs[dirPrefix] {
					continue
				}
				seenDirs[dirPrefix] = true

				page.Objects = append(page.Objects, &driver.ListObject{
					Key:   dirPrefix,
					IsDir: true,
				})
				continue
			}
		}

		// Skip directories when not using delimiter
		if file.IsDir && delimiter == "" {
			continue
		}

		page.Objects = append(page.Objects, &driver.ListObject{
			Key:     key,
			ModTime: file.ModTime,
			Size:    file.Size,
			IsDir:   file.IsDir,
		})
	}

	// Handle pagination if requested
	if opts.PageSize > 0 && len(page.Objects) > opts.PageSize {
		page.Objects = page.Objects[:opts.PageSize]
		// We don't implement true pagination for simplicity
		// The file list is typically small enough
	}

	return page, nil
}

// Attributes implements driver.Bucket.Attributes
func (d *lfsBucketDriver) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	path := d.bucket.repo.FilePath(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %w", fs.ErrNotExist)
		}
		return nil, err
	}

	return &driver.Attributes{
		ContentType: "application/octet-stream",
		ModTime:     info.ModTime(),
		Size:        info.Size(),
	}, nil
}

// NewRangeReader implements driver.Bucket.NewRangeReader
func (d *lfsBucketDriver) NewRangeReader(
	ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions,
) (driver.Reader, error) {
	// Pull latest before reading
	// Ignore errors - file might exist locally
	_ = d.bucket.repo.Pull(ctx)

	// Read the file
	data, err := d.bucket.repo.ReadFile(key)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("key not found: %w", err)
		}
		return nil, err
	}

	// Check if it's an LFS pointer
	if IsPointer(data) {
		pointer, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing LFS pointer: %w", err)
		}

		// Download from LFS
		data, err = d.bucket.lfsClient.Download(ctx, pointer.OID, pointer.Size)
		if err != nil {
			return nil, fmt.Errorf("downloading from LFS: %w", err)
		}
	}

	// Apply offset and length
	size := int64(len(data))
	if offset < 0 || offset > size {
		return nil, fmt.Errorf("invalid offset: %d, size: %d", offset, size)
	}

	if length < 0 {
		length = size - offset
	} else if offset+length > size {
		length = size - offset
	}

	data = data[offset : offset+length]

	return &lfsReader{
		data:    data,
		offset:  0,
		modTime: time.Now(),
	}, nil
}

// NewTypedWriter implements driver.Bucket.NewTypedWriter
func (d *lfsBucketDriver) NewTypedWriter(
	ctx context.Context, key string, contentType string, opts *driver.WriterOptions,
) (driver.Writer, error) {
	return &lfsWriter{
		ctx:         ctx,
		bucket:      d.bucket,
		key:         key,
		contentType: contentType,
	}, nil
}

// Copy implements driver.Bucket.Copy
func (d *lfsBucketDriver) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	// Read source
	data, err := d.bucket.repo.ReadFile(srcKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("source key not found: %w", err)
		}
		return err
	}

	// Write destination
	if err := d.bucket.repo.WriteFile(dstKey, data); err != nil {
		return err
	}

	// Commit and push if auto-commit is enabled
	if d.bucket.autoCommit {
		message := fmt.Sprintf("pulumi: copy %s to %s", filepath.Base(srcKey), filepath.Base(dstKey))
		if err := d.bucket.CommitAndPush(ctx, message); err != nil {
			return err
		}
	}

	return nil
}

// Delete implements driver.Bucket.Delete
func (d *lfsBucketDriver) Delete(ctx context.Context, key string) error {
	if err := d.bucket.repo.DeleteFile(key); err != nil {
		return err
	}

	// Commit and push if auto-commit is enabled
	if d.bucket.autoCommit {
		message := "pulumi: delete " + filepath.Base(key)
		if err := d.bucket.CommitAndPush(ctx, message); err != nil {
			return err
		}
	}

	return nil
}

// SignedURL implements driver.Bucket.SignedURL
func (d *lfsBucketDriver) SignedURL(
	ctx context.Context, key string, opts *driver.SignedURLOptions,
) (string, error) {
	// Git LFS doesn't support signed URLs in the same way as cloud storage
	// Return the raw git URL for the file
	return "", errors.New("signed URLs not supported with Git LFS backend")
}

// Close implements driver.Bucket.Close
func (d *lfsBucketDriver) Close() error {
	return nil
}

// lfsReader implements driver.Reader
type lfsReader struct {
	data    []byte
	offset  int64
	modTime time.Time
}

// Read implements io.Reader
func (r *lfsReader) Read(p []byte) (int, error) {
	if r.offset >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += int64(n)
	return n, nil
}

// Close implements io.Closer
func (r *lfsReader) Close() error {
	return nil
}

// Attributes implements driver.Reader.Attributes
func (r *lfsReader) Attributes() *driver.ReaderAttributes {
	return &driver.ReaderAttributes{
		ContentType: "application/octet-stream",
		ModTime:     r.modTime,
		Size:        int64(len(r.data)),
	}
}

// As implements driver.Reader.As
func (r *lfsReader) As(i any) bool {
	return false
}

// lfsWriter implements driver.Writer
type lfsWriter struct {
	ctx         context.Context
	bucket      *Bucket
	key         string
	contentType string
	buf         []byte
}

// Write implements io.Writer
func (w *lfsWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// Close implements io.Closer
func (w *lfsWriter) Close() error {
	data := w.buf

	// Determine if we should use LFS based on size
	if int64(len(data)) > w.bucket.threshold {
		// Upload to LFS
		oid := ComputeOID(data)
		if err := w.bucket.lfsClient.Upload(w.ctx, oid, data); err != nil {
			return fmt.Errorf("uploading to LFS: %w", err)
		}

		// Create pointer file
		pointer := NewPointerFromOID(oid, int64(len(data)))
		data = pointer.Bytes()
	}

	// Write to repository
	if err := w.bucket.repo.WriteFile(w.key, data); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	// Commit and push if auto-commit is enabled
	if w.bucket.autoCommit {
		message := "pulumi: update " + filepath.Base(w.key)
		if err := w.bucket.CommitAndPush(w.ctx, message); err != nil {
			return err
		}
	}

	return nil
}

// As implements driver.Writer.As
func (w *lfsWriter) As(i any) bool {
	return false
}
