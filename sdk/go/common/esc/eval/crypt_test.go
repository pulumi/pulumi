// Copyright 2023, Pulumi Corporation.
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

package eval

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rot128 struct{}

func (rot128) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	for i, b := range plaintext {
		plaintext[i] = b + 128
	}
	return plaintext, nil
}

func (rot128) Decrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	for i, b := range plaintext {
		plaintext[i] = b + 128
	}
	return plaintext, nil
}

func TestCrypt(t *testing.T) {
	path := filepath.Join("testdata", "crypt")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		baseName, ok := strings.CutSuffix(e.Name(), ".plaintext.yaml")
		if !ok {
			continue
		}

		t.Run(e.Name(), func(t *testing.T) {
			plaintextPath := filepath.Join(path, e.Name())
			ciphertextPath := filepath.Join(path, baseName+".ciphertext.yaml")

			plaintextBytes, err := os.ReadFile(plaintextPath)
			require.NoError(t, err)

			if accept() {
				encrypted, err := EncryptSecrets(context.Background(), baseName, plaintextBytes, rot128{})
				require.NoError(t, err)

				err = os.WriteFile(ciphertextPath, encrypted, 0o600)
				require.NoError(t, err)

				decrypted, err := DecryptSecrets(context.Background(), baseName, encrypted, rot128{})
				require.NoError(t, err)

				err = os.WriteFile(plaintextPath, decrypted, 0o600)
				require.NoError(t, err)

				return
			}

			ciphertextBytes, err := os.ReadFile(ciphertextPath)
			require.NoError(t, err)

			encrypted, err := EncryptSecrets(context.Background(), baseName, plaintextBytes, rot128{})
			require.NoError(t, err)

			require.Equal(t, ciphertextBytes, encrypted)

			decrypted, err := DecryptSecrets(context.Background(), baseName, encrypted, rot128{})
			require.NoError(t, err)

			assert.Equal(t, plaintextBytes, decrypted)
		})
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	const doc = `values:
  password:
    fn::secret:
      ciphertext: "********"
`

	_, err := DecryptSecrets(context.Background(), "doc", []byte(doc), rot128{})
	assert.Error(t, err)
}

type broken struct{}

func (broken) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return nil, errors.New("broken")
}

func (broken) Decrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return nil, errors.New("broken")
}

func TestEncryptBroken(t *testing.T) {
	const doc = `values:
  password:
    fn::secret: hunter2
`

	_, err := EncryptSecrets(context.Background(), "doc", []byte(doc), broken{})
	assert.Error(t, err)
}

func TestDecryptBroken(t *testing.T) {
	const doc = `values:
  password:
    fn::secret:
      ciphertext: "aHVudGVyMg=="
`

	_, err := DecryptSecrets(context.Background(), "doc", []byte(doc), broken{})
	assert.Error(t, err)
}

func TestEncryptMalformedSecret(t *testing.T) {
	const doc = `values:
  password:
    fn::secret: [ array ]
`

	// Encryption and decryption ignore malformed secrets. Errors are reported if/when the environment is parsed.
	_, err := EncryptSecrets(context.Background(), "doc", []byte(doc), broken{})
	assert.NoError(t, err)
}

func TestDecryptMalformedSecret(t *testing.T) {
	const doc = `values:
  password:
    fn::secret: [ array ]
`

	// Encryption and decryption ignore malformed secrets. Errors are reported if/when the environment is parsed.
	_, err := EncryptSecrets(context.Background(), "doc", []byte(doc), broken{})
	assert.NoError(t, err)
}

func TestInvalidEnvelope(t *testing.T) {
	encodeBin := func(magic string, version uint32, ciphertext []byte) []byte {
		var b bytes.Buffer
		b.WriteString(magic)
		b.Write(binary.BigEndian.AppendUint32(nil, version))                                    // version
		b.Write(ciphertext)                                                                     // ciphertext
		b.Write(binary.BigEndian.AppendUint32(nil, crc32.Checksum(b.Bytes(), crc32.IEEETable))) // crc32
		return b.Bytes()
	}
	encode := func(magic string, version uint32, ciphertext []byte) string {
		return base64.StdEncoding.EncodeToString(encodeBin(magic, version, ciphertext))
	}

	// Short
	_, err := decodeCiphertext(base64.StdEncoding.EncodeToString([]byte("escx")))
	assert.Error(t, err)

	// Invalid magic
	_, err = decodeCiphertext(encode("xcse", envelopeVersion, []byte("foo")))
	assert.Error(t, err)

	// Invalid version
	_, err = decodeCiphertext(encode(envelopeMagic, 0, []byte("foo")))
	assert.Error(t, err)

	// Invalid checksum
	bin := encodeBin(envelopeMagic, envelopeVersion, []byte("foo"))
	bin[0] += 128
	_, err = decodeCiphertext(base64.StdEncoding.EncodeToString(bin))
	assert.Error(t, err)
}
