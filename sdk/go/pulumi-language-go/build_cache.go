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

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// compileCache deduplicates `go build` invocations within one language-host
// process by hashing the source tree and reusing the previously-built binary
// when the inputs match. Conformance-test hosts call compileProgram many
// times with the same source (one Run per `pulumi up` / `preview`, repeated
// across runs of the same test); without this cache each invocation pays the
// ~1s linker cost even when Go's package cache is hot.
//
// Cached binaries live on disk at dir/<hash> (hard-linked from each
// compile's outfile) and are evicted FIFO once the entry count exceeds the
// cap. The cap is small on purpose — within one test the same source is
// built repeatedly (8+× for policy tests) so a handful of slots captures the
// win, while across tests sources differ and unbounded retention would just
// leak hundreds of MB of /tmp per host.
const compileCacheCap = 8

type compileCache struct {
	mu    sync.Mutex
	dir   string
	order []string // FIFO of cached hashes: front is the oldest
}

func newCompileCache() (*compileCache, error) {
	dir, err := os.MkdirTemp("", "pulumi-go-build-cache-*")
	if err != nil {
		return nil, err
	}
	return &compileCache{dir: dir}, nil
}

func (c *compileCache) path(hash string) string {
	return filepath.Join(c.dir, hash)
}

func (c *compileCache) lookup(hash string) string {
	if c == nil || hash == "" {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if slices.Contains(c.order, hash) {
		return c.path(hash)
	}
	return ""
}

// put stores the binary at outfile in the cache under hash, evicting the
// oldest entry when the cap is reached. Best-effort: failures leave the
// cache untouched so the next call just rebuilds.
func (c *compileCache) put(hash, outfile string) {
	if c == nil || hash == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if slices.Contains(c.order, hash) {
		return
	}
	if err := linkOrCopy(outfile, c.path(hash)); err != nil {
		return
	}
	c.order = append(c.order, hash)
	if len(c.order) > compileCacheCap {
		_ = os.Remove(c.path(c.order[0]))
		c.order = c.order[1:]
	}
}

func (c *compileCache) close() error {
	if c == nil {
		return nil
	}
	return os.RemoveAll(c.dir)
}

// hashSourceTree returns a stable hex digest of every regular file under
// root plus the withDebugFlags bit. Hidden directories (anything starting
// with ".") are skipped so transient artifacts like `.git` don't bust the
// cache, and outfile is skipped so a build target inside the source tree
// doesn't change the hash from one build to the next. Returns "" if any
// file can't be read — caller should treat that as a cache miss and just
// build normally.
//
// Note: replace directives in go.mod that point outside root are NOT
// followed. Within one short-lived host process those targets are stable
// (e.g. the conformance test's packed core SDK) so this is safe in practice.
func hashSourceTree(root, outfile string, withDebugFlags bool) string {
	h := sha256.New()
	flag := byte(0)
	if withDebugFlags {
		flag = 1
	}
	h.Write([]byte{flag})
	// Include the root path so debug builds (which don't get -trimpath) from
	// different directories don't collide on identical source content — the
	// embedded paths in their binaries would differ.
	h.Write([]byte(root))
	h.Write([]byte{0})
	// Build targets may be relative to the program directory.
	skip := filepath.Clean(outfile)
	if !filepath.IsAbs(skip) {
		skip = filepath.Join(root, skip)
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if path == skip {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		h.Write(data)
		h.Write([]byte{0})
		return nil
	})
	if walkErr != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// linkOrCopy hard-links src to dst, falling back to a byte-for-byte copy if
// hard-linking fails (e.g. across filesystems).
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
