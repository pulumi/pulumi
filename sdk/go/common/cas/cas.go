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

// Package cas implements a content-addressed store (CAS): blobs keyed by the
// lowercase-hex SHA256 digest of their contents. Identical contents are stored
// once and shared, and a blob can be referenced by its digest alone and
// materialized on demand.
//
// The only backend today is a plain local filesystem directory. A richer remote
// backend (an org-wide cache served by Pulumi Cloud) is a separate, future
// protocol that will sit behind this same Store interface; nothing here assumes
// the filesystem, so it can be swapped without touching callers.
package cas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store is a content-addressed blob store. Blobs are keyed by the lowercase-hex
// SHA256 digest of their contents, which makes Put idempotent: writing the same
// contents twice stores them once.
type Store interface {
	// Has reports whether a blob with the given digest is present.
	Has(ctx context.Context, digest string) (bool, error)
	// Get opens the blob with the given digest for reading. The caller must Close it.
	Get(ctx context.Context, digest string) (io.ReadCloser, error)
	// Put stores the contents read from r under the given digest. It is idempotent
	// (a no-op if the digest is already present) and verifies that the contents
	// actually hash to digest, so a corrupt or mislabeled blob never lands.
	Put(ctx context.Context, digest string, r io.Reader) error
}

// fsStore is a Store backed by a plain local filesystem directory. Blobs live at
// <root>/<digest[:2]>/<digest> (two-char sharding to avoid huge directories) and
// are written atomically via a temp file + rename, so readers never observe a
// partial blob.
type fsStore struct {
	root string
}

// NewFSStore returns a filesystem-backed Store rooted at dir, creating it if needed.
func NewFSStore(dir string) (Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating CAS root %q: %w", dir, err)
	}
	return &fsStore{root: dir}, nil
}

func (s *fsStore) blobPath(digest string) string {
	return filepath.Join(s.root, digest[:2], digest)
}

func (s *fsStore) Has(_ context.Context, digest string) (bool, error) {
	if err := validateDigest(digest); err != nil {
		return false, err
	}
	switch _, err := os.Stat(s.blobPath(digest)); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func (s *fsStore) Get(_ context.Context, digest string) (io.ReadCloser, error) {
	if err := validateDigest(digest); err != nil {
		return nil, err
	}
	f, err := os.Open(s.blobPath(digest))
	if err != nil {
		return nil, fmt.Errorf("reading blob %s from CAS: %w", digest, err)
	}
	return f, nil
}

func (s *fsStore) Put(ctx context.Context, digest string, r io.Reader) error {
	if err := validateDigest(digest); err != nil {
		return err
	}
	if has, err := s.Has(ctx, digest); err != nil || has {
		return err // already present (idempotent) or stat failed
	}

	final := s.blobPath(digest)
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return err
	}

	// Stage to a temp file in the destination directory while verifying the digest,
	// then rename into place atomically.
	tmp, err := os.CreateTemp(filepath.Dir(final), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), r); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != digest {
		return fmt.Errorf("CAS digest mismatch: declared %s but contents hash to %s", digest, got)
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, final); err != nil {
		// A concurrent writer may have committed the same digest first; that's fine.
		if has, herr := s.Has(ctx, digest); herr == nil && has {
			committed = true
			return nil
		}
		return err
	}
	committed = true
	return nil
}

// validateDigest checks that digest is a 64-character lowercase-hex SHA256.
func validateDigest(digest string) error {
	if len(digest) != sha256.Size*2 {
		return fmt.Errorf("invalid CAS digest %q: expected %d hex chars", digest, sha256.Size*2)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("invalid CAS digest %q: %w", digest, err)
	}
	return nil
}
