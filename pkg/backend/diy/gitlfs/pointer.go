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
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// LFSVersion is the Git LFS pointer file version
	LFSVersion = "https://git-lfs.github.com/spec/v1"

	// LFSHashAlgo is the hash algorithm used for LFS object IDs
	LFSHashAlgo = "sha256"

	// maxPointerSize is the maximum size of a valid LFS pointer file
	// Pointer files are small text files, typically under 200 bytes
	maxPointerSize = 1024
)

var (
	// ErrInvalidPointer indicates the data is not a valid LFS pointer
	ErrInvalidPointer = errors.New("invalid LFS pointer file")

	// ErrMissingVersion indicates the pointer file is missing the version line
	ErrMissingVersion = errors.New("missing version in LFS pointer")

	// ErrMissingOID indicates the pointer file is missing the oid line
	ErrMissingOID = errors.New("missing oid in LFS pointer")

	// ErrMissingSize indicates the pointer file is missing the size line
	ErrMissingSize = errors.New("missing size in LFS pointer")

	// ErrInvalidOID indicates the oid format is invalid
	ErrInvalidOID = errors.New("invalid oid format in LFS pointer")

	// oidPattern matches a valid LFS oid (sha256:64-hex-chars)
	oidPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

// Pointer represents a Git LFS pointer file
type Pointer struct {
	// Version is the LFS specification version
	Version string

	// OID is the object identifier in the format "sha256:<64-hex-chars>"
	OID string

	// Size is the size of the actual content in bytes
	Size int64
}

// NewPointer creates a new LFS pointer from content data
func NewPointer(data []byte) *Pointer {
	hash := sha256.Sum256(data)
	return &Pointer{
		Version: LFSVersion,
		OID:     fmt.Sprintf("%s:%s", LFSHashAlgo, hex.EncodeToString(hash[:])),
		Size:    int64(len(data)),
	}
}

// NewPointerFromOID creates a new LFS pointer from an existing OID and size
func NewPointerFromOID(oid string, size int64) *Pointer {
	return &Pointer{
		Version: LFSVersion,
		OID:     oid,
		Size:    size,
	}
}

// Parse parses an LFS pointer file from bytes
func Parse(data []byte) (*Pointer, error) {
	if len(data) > maxPointerSize {
		return nil, ErrInvalidPointer
	}

	pointer := &Pointer{}
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: invalid line format: %q", ErrInvalidPointer, line)
		}

		key, value := parts[0], parts[1]
		switch key {
		case "version":
			pointer.Version = value
		case "oid":
			pointer.OID = value
		case "size":
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid size: %v", ErrInvalidPointer, err)
			}
			pointer.Size = size
		default:
			// Ignore unknown keys for forward compatibility
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading pointer: %w", err)
	}

	// Validate required fields
	if pointer.Version == "" {
		return nil, ErrMissingVersion
	}
	if pointer.OID == "" {
		return nil, ErrMissingOID
	}
	if pointer.Size < 0 {
		return nil, ErrMissingSize
	}

	// Validate OID format
	if !oidPattern.MatchString(pointer.OID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidOID, pointer.OID)
	}

	return pointer, nil
}

// IsPointer returns true if data looks like an LFS pointer file
func IsPointer(data []byte) bool {
	if len(data) > maxPointerSize {
		return false
	}

	// Quick check: must start with "version "
	if !bytes.HasPrefix(data, []byte("version ")) {
		return false
	}

	// Try to parse it
	_, err := Parse(data)
	return err == nil
}

// Bytes returns the pointer file content as bytes
func (p *Pointer) Bytes() []byte {
	// Git LFS pointer format:
	// version https://git-lfs.github.com/spec/v1
	// oid sha256:...
	// size ...
	return []byte(fmt.Sprintf("version %s\noid %s\nsize %d\n", p.Version, p.OID, p.Size))
}

// String returns a string representation of the pointer
func (p *Pointer) String() string {
	return string(p.Bytes())
}

// SHA256 returns just the hex hash portion of the OID (without the "sha256:" prefix)
func (p *Pointer) SHA256() string {
	if strings.HasPrefix(p.OID, "sha256:") {
		return strings.TrimPrefix(p.OID, "sha256:")
	}
	return p.OID
}

// Validate checks if the pointer is valid
func (p *Pointer) Validate() error {
	if p.Version == "" {
		return ErrMissingVersion
	}
	if p.OID == "" {
		return ErrMissingOID
	}
	if p.Size < 0 {
		return ErrMissingSize
	}
	if !oidPattern.MatchString(p.OID) {
		return fmt.Errorf("%w: %q", ErrInvalidOID, p.OID)
	}
	return nil
}

// ComputeOID computes the LFS OID for given data
func ComputeOID(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%s:%s", LFSHashAlgo, hex.EncodeToString(hash[:]))
}
