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
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// Writer writes AES-256-GCM encrypted log data in the PLOG format.
// Each chunk is independently gzip-compressed for crash resilience — all
// completed chunks on disk are decodable even if the process crashes.
// It is safe for concurrent use.
type Writer struct {
	mu        sync.Mutex
	w         io.Writer
	aesgcm    cipher.AEAD
	buf       bytes.Buffer // buffered plaintext
	counter   uint64
	chunkSize int
	closed    bool
}

// NewWriter creates an Writer that encrypts log data to w.
// A random session key is generated and encrypted with enc for the file header.
func NewWriter(
	ctx context.Context, w io.Writer, enc config.Encrypter,
) (*Writer, error) {
	var sessionKey [keySize]byte
	if _, err := rand.Read(sessionKey[:]); err != nil {
		return nil, fmt.Errorf("encryptedlog: generating session key: %w", err)
	}

	// Encrypt the base64-encoded session key via the caller's secrets provider.
	encodedKey := base64.StdEncoding.EncodeToString(sessionKey[:])
	encryptedKey, err := enc.EncryptValue(ctx, encodedKey)
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: encrypting session key: %w", err)
	}

	encryptedKeyBytes := []byte(encryptedKey)
	if len(encryptedKeyBytes) > 65535 {
		return nil, fmt.Errorf("encryptedlog: encrypted key too large (%d bytes)", len(encryptedKeyBytes))
	}

	// Write the PLOG header: magic + version + key length + key.
	header := make([]byte, 0, len(Magic)+1+2+len(encryptedKeyBytes))
	header = append(header, Magic...)
	header = append(header, Version)
	//nolint:gosec // bounded by 65535 check above
	header = binary.BigEndian.AppendUint16(header, uint16(len(encryptedKeyBytes)))
	header = append(header, encryptedKeyBytes...)
	if _, err := w.Write(header); err != nil {
		return nil, fmt.Errorf("encryptedlog: writing header: %w", err)
	}

	// Set up AES-256-GCM from the session key.
	block, err := aes.NewCipher(sessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: creating cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryptedlog: creating GCM: %w", err)
	}

	elw := &Writer{
		w:         w,
		aesgcm:    aesgcm,
		chunkSize: DefaultChunkSize,
	}

	return elw, nil
}

// Write buffers plaintext log data, flushing compressed and encrypted chunks
// to the underlying writer as the buffer fills.
func (elw *Writer) Write(p []byte) (int, error) {
	elw.mu.Lock()
	defer elw.mu.Unlock()

	if elw.closed {
		return 0, errors.New("encryptedlog: writer is closed")
	}

	n, err := elw.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("encryptedlog: buffering: %w", err)
	}

	if err := elw.flushChunks(); err != nil {
		return n, err
	}
	return n, nil
}

// Close flushes all remaining data.
func (elw *Writer) Close() error {
	elw.mu.Lock()
	defer elw.mu.Unlock()

	if elw.closed {
		return nil
	}
	elw.closed = true

	// Flush complete chunks.
	if err := elw.flushChunks(); err != nil {
		return err
	}

	// Write any remaining partial chunk.
	if elw.buf.Len() > 0 {
		if err := elw.writeChunk(elw.buf.Bytes()); err != nil {
			return err
		}
		elw.buf.Reset()
	}

	return nil
}

// flushChunks writes all complete chunk-sized blocks from the buffer.
func (elw *Writer) flushChunks() error {
	for elw.buf.Len() >= elw.chunkSize {
		if err := elw.writeChunk(elw.buf.Next(elw.chunkSize)); err != nil {
			return err
		}
	}
	return nil
}

// writeChunk gzip-compresses plaintext, encrypts the compressed data, and writes
// the chunk (length + nonce + ciphertext) to the output. Each chunk is independently
// compressed for crash resilience.
func (elw *Writer) writeChunk(plaintext []byte) error {
	// Compress the chunk independently.
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(plaintext); err != nil {
		return fmt.Errorf("encryptedlog: gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("encryptedlog: gzip close: %w", err)
	}

	elw.counter++
	nonce := makeNonce(elw.counter)

	ciphertext := elw.aesgcm.Seal(nil, nonce[:], compressed.Bytes(), nil)

	// Chunk frame: 4-byte payload length, then nonce, then ciphertext+tag.
	payloadLen := uint32(nonceSize + len(ciphertext)) //nolint:gosec // bounded by chunk size
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], payloadLen)

	if _, err := elw.w.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("encryptedlog: writing chunk length: %w", err)
	}
	if _, err := elw.w.Write(nonce[:]); err != nil {
		return fmt.Errorf("encryptedlog: writing chunk nonce: %w", err)
	}
	if _, err := elw.w.Write(ciphertext); err != nil {
		return fmt.Errorf("encryptedlog: writing chunk data: %w", err)
	}
	return nil
}
