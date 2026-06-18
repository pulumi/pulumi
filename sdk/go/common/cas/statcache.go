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

package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// StatCache maps a file's identity (path, size, modification time) to the content
// digest computed for it, so an unchanged file is never re-read and re-hashed across
// operations. It is the "build-grade" half of content-addressed hashing: the cost of
// hashing a tree becomes proportional to what changed, not to its total size.
type StatCache interface {
	// Get returns the cached digest for a file whose current identity matches the
	// cached one, or ok=false if absent or stale.
	Get(path string, info os.FileInfo) (digest string, ok bool)
	// Put records the digest for the file's current identity.
	Put(path string, info os.FileInfo, digest string) error
}

type statEntry struct {
	Size    int64  `json:"size"`
	MtimeNs int64  `json:"mtimeNs"`
	Digest  string `json:"digest"`
}

type fsStatCache struct{ dir string }

// NewFSStatCache returns a StatCache backed by a directory of entries keyed on the
// absolute file path.
func NewFSStatCache(dir string) (StatCache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &fsStatCache{dir: dir}, nil
}

func (c *fsStatCache) entry(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

func (c *fsStatCache) Get(path string, info os.FileInfo) (string, bool) {
	data, err := os.ReadFile(c.entry(path))
	if err != nil {
		return "", false
	}
	var e statEntry
	if json.Unmarshal(data, &e) != nil {
		return "", false
	}
	if e.Size == info.Size() && e.MtimeNs == info.ModTime().UnixNano() {
		return e.Digest, true
	}
	return "", false
}

func (c *fsStatCache) Put(path string, info os.FileInfo, digest string) error {
	data, err := json.Marshal(statEntry{
		Size:    info.Size(),
		MtimeNs: info.ModTime().UnixNano(),
		Digest:  digest,
	})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.dir, "tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), c.entry(path))
}
