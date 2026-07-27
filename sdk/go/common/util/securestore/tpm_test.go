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

//go:build linux || windows

package securestore

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTPMWrapperKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, wrapTPM, tpmWrapper{}.kind())
}

// TestTPMUnwrapRejectsMalformedBlob needs no TPM: framing validation fails
// before the transport is ever opened.
func TestTPMUnwrapRejectsMalformedBlob(t *testing.T) {
	t.Parallel()
	for _, blob := range [][]byte{nil, {0x01}, {0xFF, 0xFF, 0x00}} {
		_, err := tpmWrapper{}.unwrap(blob)
		assert.Error(t, err)
	}
}

// TestTPMWrapUnwrapRoundTrip exercises the real TPM where one exists (e.g.
// bare-metal Linux CI or a developer laptop) and self-skips everywhere else.
func TestTPMWrapUnwrapRoundTrip(t *testing.T) {
	t.Parallel()
	w := tpmWrapper{}
	if err := w.available(); err != nil {
		t.Skipf("no usable TPM: %v", err)
	}

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	blob, err := w.wrap(key)
	require.NoError(t, err)
	assert.NotContains(t, string(blob), string(key), "sealed blob must not embed the raw key")

	got, err := w.unwrap(blob)
	require.NoError(t, err)
	assert.Equal(t, key, got)

	// Unwrapping is repeatable: the storage primary is deterministically
	// regenerated on every operation.
	got2, err := w.unwrap(blob)
	require.NoError(t, err)
	assert.Equal(t, key, got2)
}
