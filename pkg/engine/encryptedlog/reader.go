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

package encryptedlog

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// Reader reads and decrypts PLOG encrypted log files,
// returning the original plaintext log data. It tolerates missing
// end sentinels for crash resilience — all completed chunks are readable.
type Reader struct {
	cd *chunkDecrypter
}

// NewReader creates an Reader that decrypts log data from r.
// The session key stored in the file header is decrypted with dec.
func NewReader(
	ctx context.Context, r io.Reader, dec config.Decrypter,
) (*Reader, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("encryptedlog: reading magic: %w", err)
	}
	if string(magic[:]) != Magic {
		return nil, errors.New("encryptedlog: invalid magic bytes")
	}

	var versionBuf [1]byte
	if _, err := io.ReadFull(r, versionBuf[:]); err != nil {
		return nil, fmt.Errorf("encryptedlog: reading version: %w", err)
	}
	version := versionBuf[0]
	if version != Version {
		return nil, fmt.Errorf("encryptedlog: unsupported version %d", version)
	}

	var keyLenBuf [2]byte
	if _, err := io.ReadFull(r, keyLenBuf[:]); err != nil {
		return nil, fmt.Errorf("encryptedlog: reading key length: %w", err)
	}
	keyLen := binary.BigEndian.Uint16(keyLenBuf[:])

	encryptedKeyBytes := make([]byte, keyLen)
	if _, err := io.ReadFull(r, encryptedKeyBytes); err != nil {
		return nil, fmt.Errorf("encryptedlog: reading encrypted key: %w", err)
	}

	encodedKey, err := dec.DecryptValue(ctx, string(encryptedKeyBytes))
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: decrypting session key: %w", err)
	}
	sessionKey, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: decoding session key: %w", err)
	}
	if len(sessionKey) != keySize {
		return nil, fmt.Errorf("encryptedlog: invalid session key size %d", len(sessionKey))
	}

	// Set up AES-256-GCM from the session key.
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: creating cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: creating GCM: %w", err)
	}

	return &Reader{cd: &chunkDecrypter{r: r, aesgcm: aesgcm}}, nil
}

// Read reads decrypted, decompressed log data.
func (elr *Reader) Read(p []byte) (int, error) {
	return elr.cd.Read(p)
}

// chunkDecrypter is an io.Reader that reads encrypted chunks from the
// underlying reader, decrypts them, and serves the plaintext sequentially.
type chunkDecrypter struct {
	r       io.Reader
	aesgcm  cipher.AEAD
	buf     []byte // remaining plaintext from the current chunk
	counter uint64
	done    bool
}

func (cd *chunkDecrypter) Read(p []byte) (int, error) {
	// Serve any buffered plaintext first.
	if len(cd.buf) > 0 {
		n := copy(p, cd.buf)
		cd.buf = cd.buf[n:]
		return n, nil
	}
	if cd.done {
		return 0, io.EOF
	}

	// Read the next chunk's payload length.
	var lenBuf [4]byte
	if _, err := io.ReadFull(cd.r, lenBuf[:]); err != nil {
		// Treat EOF at a chunk boundary as end of stream — all
		// previously completed chunks are still readable.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			cd.done = true
			return 0, io.EOF
		}
		return 0, fmt.Errorf("encryptedlog: reading chunk length: %w", err)
	}
	payloadLen := binary.BigEndian.Uint32(lenBuf[:])
	if payloadLen > uint32(maxPayloadLen) {
		return 0, fmt.Errorf(
			"encryptedlog: chunk payload too large (%d bytes)", payloadLen)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(cd.r, payload); err != nil {
		return 0, fmt.Errorf("encryptedlog: reading chunk data: %w", err)
	}
	minPayload := nonceSize + cd.aesgcm.Overhead()
	if len(payload) < minPayload {
		return 0, fmt.Errorf("encryptedlog: chunk payload too small (%d bytes, need at least %d)", len(payload), minPayload)
	}

	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]

	// Verify the nonce matches the expected counter.
	cd.counter++
	expected := makeNonce(cd.counter)
	if !bytes.Equal(nonce, expected[:]) {
		return 0, errors.New("encryptedlog: nonce counter mismatch")
	}

	compressed, err := cd.aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, fmt.Errorf("encryptedlog: chunk decryption failed: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return 0, fmt.Errorf("encryptedlog: decompressing chunk: %w", err)
	}
	plaintext, err := io.ReadAll(gz)
	if err != nil {
		return 0, fmt.Errorf("encryptedlog: decompressing chunk: %w", err)
	}
	if err := gz.Close(); err != nil {
		return 0, fmt.Errorf("encryptedlog: decompressing chunk: %w", err)
	}

	n := copy(p, plaintext)
	cd.buf = plaintext[n:]
	return n, nil
}
