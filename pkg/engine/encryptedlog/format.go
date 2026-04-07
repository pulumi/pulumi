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

// Package encryptedlog implements chunked AES-256-GCM envelope encryption for
// Pulumi engine log files using the PLOG (Pulumi log) binary format.
//
// The format consists of a file header in the following format, followed by a sequence of encrypted chunks:
//
//   - Magic (4 bytes): the ASCII string "PLOG"
//   - Version (1 byte): the format version (currently 0x01)
//   - Key length (2 bytes, big-endian uint16): the length of the encrypted session key
//   - Encrypted session key (variable length): the base64-encoded session key, encrypted with the configured encrypter
//
// After the header, the file contains a sequence of encrypted chunks. Each chunk has the following format:
//
//   - Payload length (4 bytes, big-endian uint32)
//   - Nonce (12 bytes)
//   - Ciphertext (variable length, as specified by the payload length)
//
// Each chunk is independently encrypted and gzip-compressed. This way we have crash resilience, as all
// completed chunks on disk are decodable if the process crashes.
//
// The nonce is constructed from a counter that increments with each chunk that is written. This guarantees
// that each chunk has a unique nonce, which is a requirement for AES-GCM security.
package encryptedlog

import "encoding/binary"

const (
	// Magic is the 4-byte file signature for encrypted log files.
	Magic = "PLOG"

	// Version is the current format version.
	Version = 0x01

	// DefaultChunkSize is the default plaintext chunk size in bytes (64 KB).
	DefaultChunkSize = 64 * 1024

	// nonceSize is the AES-256-GCM nonce size in bytes.
	nonceSize = 12

	// keySize is the AES-256 key size in bytes.
	keySize = 32

	// maxPayloadLen is the maximum allowed chunk payload size on read.
	// Set generously above any legitimate chunk to guard against OOM
	// from corrupt or malicious files.
	maxPayloadLen = DefaultChunkSize + 1024
)

// makeNonce builds a 12-byte GCM nonce by zero-padding a uint64 counter.
func makeNonce(counter uint64) [nonceSize]byte {
	var nonce [nonceSize]byte
	binary.BigEndian.PutUint64(nonce[4:], counter)
	return nonce
}
