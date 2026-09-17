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

// Package atomicinstall installs directories so that no process ever observes a partial install as complete.
//
// A [Store] owns a root directory. Each entry is a child directory of the root, and a key names it. The on-disk
// protocol uses two files next to the entry directory:
//
//   - "<key>.lock" is the target of an exclusive file lock. Each writer holds the lock for its full duration. No
//     operation deletes this file: a writer that holds a lock on a deleted file does not exclude a writer that
//     creates the file again.
//   - "<key>.partial" marks the entry as not installed. A writer creates the marker before its first write and
//     removes the marker after its last write. A process that dies between the two leaves the marker behind, so
//     the entry stays not installed and the next writer discards it.
//
// Readers take no lock. The result of [Store.Reinstall] or [Store.Remove] on an entry that is in use is undefined.
package atomicinstall

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

const (
	lockSuffix    = ".lock"
	partialSuffix = ".partial"
)

// Store is a directory of entries that are installed with the protocol of this package.
type Store struct{ root string }

// NewStore returns the store that keeps its entries in rootDir. The store creates rootDir on the first write.
func NewStore(rootDir string) Store {
	contract.Requiref(rootDir != "", "rootDir", "must not be empty")
	return Store{root: rootDir}
}

// PopulateFunc writes the full content of an entry into dir, which is empty. The entry counts as installed only
// if PopulateFunc returns nil, so each step that can fail (extract, dependency install, validation) belongs here.
type PopulateFunc = func(ctx context.Context, dir string) error

// Lookup returns the directory of key if key is installed.
func (s Store) Lookup(key string) (string, bool, error) {
	e, err := s.entry(key)
	if err != nil {
		return "", false, err
	}
	ok, err := e.installed()
	if err != nil || !ok {
		return "", false, err
	}
	return e.dir, true, nil
}

// List returns the keys of all installed entries, sorted.
func (s Store) List() ([]string, error) {
	files, err := os.ReadDir(s.root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var keys []string
	for _, file := range files {
		if validateKey(file.Name()) != nil {
			continue
		}
		_, ok, err := s.Lookup(file.Name())
		if err != nil {
			return nil, err
		}
		if ok {
			keys = append(keys, file.Name())
		}
	}
	return keys, nil
}

// Install makes sure that key is installed and returns its directory. It calls populate only if key is not
// installed at the time that Install holds the lock.
func (s Store) Install(ctx context.Context, key string, populate PopulateFunc) (_ string, err error) {
	if path, installed, err := s.Lookup(key); err != nil {
		return "", err
	} else if installed {
		return path, nil
	}

	// The key isn't installed, so add it.

	p, err := s.Mutate(ctx, key)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	// If s.Mutate blocked while the key was being installed, it might be installed now, so check again.
	if dir, ok := p.Installed(); ok {
		return dir, nil
	}
	return p.fill(ctx, populate)
}

// Reinstall replaces the content of key, installed or not, and returns its directory.
func (s Store) Reinstall(ctx context.Context, key string, populate PopulateFunc) (_ string, err error) {
	p, err := s.Mutate(ctx, key)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	return p.fill(ctx, populate)
}

// Remove deletes key. It is not an error if key is not installed.
func (s Store) Remove(ctx context.Context, key string) error {
	p, err := s.Mutate(ctx, key)
	if err != nil {
		return err
	}
	if _, ok := p.Installed(); ok {
		_, err = p.Reset()
	}

	return errors.Join(err, p.Close())
}

// Mutate takes the exclusive lock on key and returns a [Pending] that holds it. Mutate waits until the lock is
// free or ctx is done. It discards what a failed or interrupted install of key left behind.
//
// The caller must call [Pending.Close].
func (s Store) Mutate(ctx context.Context, key string) (*Pending, error) {
	e, err := s.entry(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return nil, err
	}
	unlock, err := lock(ctx, key, e.lock)
	if err != nil {
		return nil, err
	}

	installed, err := e.discardIfPartial()
	if err != nil {
		return nil, errors.Join(err, unlock())
	}
	return &Pending{entry: e, unlock: unlock, installed: installed}, nil
}

// lock takes the exclusive file lock at path and returns the function that releases it.
func lock(ctx context.Context, key, path string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mutex := fsutil.NewFileMutex(path)
	acquired := make(chan error, 1)
	go func() { acquired <- mutex.Lock() }()

	select {
	case err := <-acquired:
		if err != nil {
			return nil, err
		}
		return mutex.Unlock, nil
	case <-ctx.Done():
		// The OS call that waits for the lock cannot be interrupted, so release the lock when it arrives.
		go func() {
			if err := <-acquired; err == nil {
				contract.IgnoreError(mutex.Unlock())
			}
		}()
		return nil, fmt.Errorf("wait for the install lock on %q: %w", key, ctx.Err())
	}
}

// Pending is exclusive write access to one entry. It is safe for concurrent use.
type Pending struct {
	entry
	unlock func() error

	m         sync.Mutex
	installed bool // The entry is complete on disk.
	dirty     bool // Reset created the marker and Commit has not removed it.
	closed    bool
}

// Installed returns the directory of the entry if the entry is installed. It reports false between [Pending.Reset]
// and [Pending.Commit].
func (p *Pending) Installed() (string, bool) {
	p.m.Lock()
	defer p.m.Unlock()
	if !p.installed {
		return "", false
	}
	return p.dir, true
}

// Reset marks the entry as not installed, deletes its content, and returns the empty directory of the entry. The
// caller writes the new content there and then calls [Pending.Commit].
func (p *Pending) Reset() (string, error) {
	p.m.Lock()
	defer p.m.Unlock()
	if p.closed {
		return "", errors.New("atomicinstall: Reset of a closed Pending")
	}

	if err := writeMarker(p.partial); err != nil {
		return "", err
	}
	p.installed, p.dirty = false, true

	if err := os.RemoveAll(p.dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.dir, 0o700); err != nil {
		return "", err
	}
	return p.dir, nil
}

// Commit marks the entry as installed and releases the lock. It is valid only after [Pending.Reset].
func (p *Pending) Commit() error {
	p.m.Lock()
	defer p.m.Unlock()
	if p.closed {
		return errors.New("atomicinstall: Commit of a closed Pending")
	}
	if !p.dirty {
		return errors.New("atomicinstall: Commit without Reset")
	}

	// ponytail: Commit does not flush the content of the entry to disk. A power loss soon after Commit can leave
	// an installed entry with unwritten file data. Flush the tree here (syncfs on Linux, a walk with fsync
	// elsewhere) if that matters more than install time.
	if err := os.Remove(p.partial); err != nil {
		return err
	}
	p.installed, p.dirty, p.closed = true, false, true
	return p.unlock()
}

// Close releases the lock. If [Pending.Reset] was called without a successful [Pending.Commit], Close first
// discards the entry. Close after Commit or after Close does nothing, so `defer p.Close()` is always correct.
func (p *Pending) Close() error {
	p.m.Lock()
	defer p.m.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	var err error
	if p.dirty {
		err = p.discard()
	}
	return errors.Join(err, p.unlock())
}

func (p *Pending) fill(ctx context.Context, populate PopulateFunc) (string, error) {
	dir, err := p.Reset()
	if err != nil {
		return "", err
	}
	if err := populate(ctx, dir); err != nil {
		return "", err
	}
	if err := p.Commit(); err != nil {
		return "", err
	}
	return dir, nil
}

// entry holds the paths of one entry.
type entry struct{ dir, lock, partial string }

func (s Store) entry(key string) (entry, error) {
	contract.Assertf(s.root != "", "a Store must come from NewStore")
	if err := validateKey(key); err != nil {
		return entry{}, err
	}
	dir := filepath.Join(s.root, key)
	return entry{dir: dir, lock: dir + lockSuffix, partial: dir + partialSuffix}, nil
}

func validateKey(key string) error {
	if key == "." || !filepath.IsLocal(key) || strings.ContainsAny(key, `/\`) {
		return fmt.Errorf("atomicinstall: invalid key %q: a key is one path element", key)
	}
	if strings.HasSuffix(key, lockSuffix) || strings.HasSuffix(key, partialSuffix) {
		return fmt.Errorf("atomicinstall: invalid key %q: a key cannot end in %q or %q", key, lockSuffix, partialSuffix)
	}
	return nil
}

// installed reports if the entry is complete. It must look at the directory before the marker: a writer creates
// the marker before the directory, so the opposite order can miss the marker of an install that starts between
// the two checks.
func (e entry) installed() (bool, error) {
	info, err := os.Stat(e.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	_, err = os.Stat(e.partial)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	return false, err
}

// discardIfPartial discards the entry if a marker exists, and then reports if the entry is installed. The caller holds
// the lock, so a marker can only come from a writer that failed or died.
func (e entry) discardIfPartial() (bool, error) {
	_, err := os.Stat(e.partial)
	if err == nil {
		if err := e.discard(); err != nil {
			return false, fmt.Errorf("discard the partial install at %q: %w", e.dir, err)
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return e.installed()
}

// discard deletes the entry. The marker goes last, so a failure leaves the entry marked as not installed.
func (e entry) discard() error {
	if err := os.RemoveAll(e.dir); err != nil {
		return err
	}
	if err := os.Remove(e.partial); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// writeMarker creates the marker and flushes it to disk. Without the flush, a power loss can keep the writes to
// the entry directory and lose the marker, and the partial entry then counts as installed.
func writeMarker(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// syncDir flushes the directory entries of dir to disk.
func syncDir(dir string) error {
	// Windows has no operation to flush a directory.
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(d)
	// fsync(2) returns EINVAL for a file system that does not support the operation.
	if err := d.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
		return err
	}
	return nil
}
