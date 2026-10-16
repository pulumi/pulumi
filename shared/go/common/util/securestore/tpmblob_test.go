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

package securestore

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSealedBlobRoundTrip(t *testing.T) {
	t.Parallel()
	priv := bytes.Repeat([]byte{0xAB}, 190) // typical TPM2B_PRIVATE size
	pub := []byte{0x00, 0x0E, 0x00, 0x08, 0x00, 0x0B, 0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	blob, err := encodeSealedBlob(priv, pub)
	require.NoError(t, err)
	require.Len(t, blob, 4+len(priv)+len(pub))

	gotPriv, gotPub, err := decodeSealedBlob(blob)
	require.NoError(t, err)
	assert.Equal(t, priv, gotPriv)
	assert.Equal(t, pub, gotPub)
}

func TestSealedBlobRoundTripEmptyChunks(t *testing.T) {
	t.Parallel()
	blob, err := encodeSealedBlob(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0, 0, 0}, blob)

	gotPriv, gotPub, err := decodeSealedBlob(blob)
	require.NoError(t, err)
	assert.Empty(t, gotPriv)
	assert.Empty(t, gotPub)
}

func TestEncodeSealedBlobRejectsOversize(t *testing.T) {
	t.Parallel()
	big := make([]byte, 0x10000)
	_, err := encodeSealedBlob(big, nil)
	assert.Error(t, err)
	_, err = encodeSealedBlob(nil, big)
	assert.Error(t, err)
}

func TestDecodeSealedBlobRejectsMalformed(t *testing.T) {
	t.Parallel()
	valid, err := encodeSealedBlob([]byte{1, 2, 3}, []byte{4, 5})
	require.NoError(t, err)

	cases := map[string][]byte{
		"empty":                    {},
		"truncated first prefix":   {0x00},
		"first chunk short":        {0x00, 0x04, 0x01, 0x02},
		"missing second chunk":     {0x00, 0x01, 0xFF},
		"truncated second prefix":  {0x00, 0x01, 0xFF, 0x00},
		"second chunk short":       {0x00, 0x01, 0xFF, 0x00, 0x02, 0x01},
		"trailing garbage":         append(append([]byte{}, valid...), 0xEE),
		"prefix overflows imagine": {0xFF, 0xFF, 0x01},
	}
	for name, blob := range cases {
		_, _, err := decodeSealedBlob(blob)
		assert.Error(t, err, name)
	}
}
