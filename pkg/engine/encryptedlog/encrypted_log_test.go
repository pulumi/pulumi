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
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	input := "Hello, encrypted world!\nLine two.\n"

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 32

	_, err = w.Write([]byte(input))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.NotContains(t, buf.String(), input)

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, input, string(got))
}

func TestEmptyLog(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Empty(t, got)
}

func TestLargeLog(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	input := make([]byte, 2*1024*1024)
	_, err := rand.Read(input)
	require.NoError(t, err)

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	_, err = w.Write(input)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, input, got)
}

func TestSingleByteWrites(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	input := "one byte at a time"

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 16

	for i := range len(input) {
		_, err = w.Write([]byte{input[i]})
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	assert.NotContains(t, buf.String(), input)

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, input, string(got))
}

func TestMultipleWritesBeforeClose(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 32

	_, err = w.Write([]byte("first chunk "))
	require.NoError(t, err)
	_, err = w.Write([]byte("second chunk "))
	require.NoError(t, err)
	_, err = w.Write([]byte("third chunk"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.NotContains(t, buf.String(), "first chunk second chunk third chunk")

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, "first chunk second chunk third chunk", string(got))
}

func TestCorruptMagic(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	_, err = w.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data := buf.Bytes()
	data[0] = 'X'

	_, err = NewReader(ctx, bytes.NewReader(data), config.Base64Crypter)
	assert.ErrorContains(t, err, "invalid magic bytes")
}

func TestCorruptVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	_, err = w.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data := buf.Bytes()
	data[4] = 0xFF

	_, err = NewReader(ctx, bytes.NewReader(data), config.Base64Crypter)
	assert.ErrorContains(t, err, "unsupported version")
}

func TestCorruptChunkCiphertext(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 32
	_, err = w.Write([]byte(strings.Repeat("A", 200)))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data := buf.Bytes()
	headerEnd := 4 + 1 + 2 + int(data[5])<<8 + int(data[6])
	corruptIdx := headerEnd + 4 + 20
	if corruptIdx < len(data) {
		data[corruptIdx] ^= 0xFF
	}

	r, err := NewReader(ctx, bytes.NewReader(data), config.Base64Crypter)
	if err != nil {
		return
	}
	_, err = io.ReadAll(r)
	assert.ErrorContains(t, err, "chunk decryption failed")
}

func TestTruncatedFile(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 32
	_, err = w.Write([]byte(strings.Repeat("B", 200)))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data := buf.Bytes()
	truncated := data[:len(data)-10]

	r, err := NewReader(ctx, bytes.NewReader(truncated), config.Base64Crypter)
	if err != nil {
		return
	}
	_, err = io.ReadAll(r)
	assert.ErrorContains(t, err, "reading chunk data")
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 64

	const goroutines = 10
	const writesPerGoroutine = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines*writesPerGoroutine)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range writesPerGoroutine {
				msg := fmt.Sprintf("[%d:%d]", id, j)
				if _, werr := w.Write([]byte(msg)); werr != nil {
					errs <- werr
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for werr := range errs {
		require.NoError(t, werr)
	}
	require.NoError(t, w.Close())
	assert.NotContains(t, buf.String(), "[0:0]")

	r, err := NewReader(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)

	output := string(got)
	for i := range goroutines {
		for j := range writesPerGoroutine {
			assert.Contains(t, output, fmt.Sprintf("[%d:%d]", i, j))
		}
	}
}

func TestWriteAfterClose(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, err = w.Write([]byte("should fail"))
	assert.ErrorContains(t, err, "writer is closed")
}

func TestChunkReorderDetected(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 16
	_, err = w.Write([]byte(strings.Repeat("X", 500)))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data := buf.Bytes()

	keyLen := int(data[5])<<8 | int(data[6])
	headerEnd := 4 + 1 + 2 + keyLen

	c1Start := headerEnd
	c1PayloadLen := int(binary.BigEndian.Uint32(data[c1Start : c1Start+4]))
	c1End := c1Start + 4 + c1PayloadLen

	c2Start := c1End
	c2PayloadLen := int(binary.BigEndian.Uint32(data[c2Start : c2Start+4]))
	c2End := c2Start + 4 + c2PayloadLen

	require.Greater(t, c1PayloadLen, 0, "need at least two chunks")
	require.Greater(t, c2PayloadLen, 0, "need at least two chunks")

	swapped := make([]byte, len(data))
	copy(swapped, data[:headerEnd])
	copy(swapped[headerEnd:], data[c2Start:c2End])
	copy(swapped[headerEnd+(c2End-c2Start):], data[c1Start:c1End])
	copy(swapped[headerEnd+(c2End-c2Start)+(c1End-c1Start):], data[c2End:])

	r, err := NewReader(ctx, bytes.NewReader(swapped), config.Base64Crypter)
	if err != nil {
		return
	}
	_, err = io.ReadAll(r)
	assert.ErrorContains(t, err, "nonce counter mismatch")
}

func TestCrashTolerance(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var buf bytes.Buffer
	w, err := NewWriter(ctx, &buf, config.Base64Crypter)
	require.NoError(t, err)
	w.chunkSize = 16

	input := "chunk1 data!!!!!" + "chunk2 data!!!!!" + "chunk3 data!!!!!"
	_, err = w.Write([]byte(input))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// Find the end of the second chunk, then truncate mid-way through the third.
	data := buf.Bytes()
	keyLen := int(data[5])<<8 | int(data[6])
	pos := 4 + 1 + 2 + keyLen
	for i := 0; i < 2; i++ {
		chunkPayloadLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4 + chunkPayloadLen
	}
	truncated := data[:pos+10]

	r, err := NewReader(ctx, bytes.NewReader(truncated), config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	assert.ErrorContains(t, err, "reading chunk data")
	assert.Equal(t, "chunk1 data!!!!!chunk2 data!!!!!", string(got))
}

func TestRapidRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	rapid.Check(t, func(t *rapid.T) {
		var expected, buf bytes.Buffer
		w, err := NewWriter(ctx, &buf, config.Base64Crypter)
		require.NoError(t, err)
		w.chunkSize = rapid.IntRange(1, 1024).Draw(t, "writerChunkSize")

		for _, data := range rapid.SliceOf(rapid.SliceOf(rapid.Byte())).Draw(t, "writes") {
			expected.Write(data)
			_, err := w.Write(data)
			require.NoError(t, err)
		}

		require.NoError(t, w.Close())

		r, err := NewReader(ctx, &buf, config.Base64Crypter)
		require.NoError(t, err)

		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, expected.String(), string(got))
	})
}

func TestV1GoldenData(t *testing.T) {
	t.Parallel()

	// This blob was generated by NewWriter with config.Base64Crypter and contains
	// the plaintext "Hello, PLOG v1!\n". If the format ever changes, this test
	// will catch it.
	golden, err := base64.StdEncoding.DecodeString(
		"UExPRwEAPFNrcG9TRGRNY0RJclpDOUVTR1E1WmpGVU0wdDBjMnBFVUROV2JTOVNiVGx2WVdwT1dt" +
			"RnVNbUZqY3owPQAAAEQAAAAAAAAAAAAAAAHQ5ZUrfOfE0r/+aPKp7NRX2gpCrBGIJgj3Pl2Ztxn3" +
			"tLKmracwfcAaqfmwsL4mbrKen2vYLziniQ==")
	require.NoError(t, err)

	ctx := t.Context()
	r, err := NewReader(ctx, bytes.NewReader(golden), config.Base64Crypter)
	require.NoError(t, err)

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "Hello, PLOG v1!\n", string(got))
}
