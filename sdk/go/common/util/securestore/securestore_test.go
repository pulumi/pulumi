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
	"crypto/rand"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	plaintext := []byte(`{"current":"https://api.pulumi.com"}`)

	env, err := Seal(key, BackendMock, plaintext)
	require.NoError(t, err)
	assert.True(t, IsEnvelope(env))
	assert.False(t, bytes.Contains(env, []byte("api.pulumi.com")))

	backend, err := EnvelopeBackend(env)
	require.NoError(t, err)
	assert.Equal(t, BackendMock, backend)

	got, err := Open(key, env)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestOpenWrongKey(t *testing.T) {
	t.Parallel()
	env, err := Seal(testKey(t), BackendMock, []byte("secret"))
	require.NoError(t, err)
	_, err = Open(testKey(t), env)
	assert.True(t, errors.Is(err, ErrWrongKey))
}

func TestOpenTamperedCiphertext(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	env, err := Seal(key, BackendMock, []byte("secret"))
	require.NoError(t, err)
	tampered := bytes.Replace(env, []byte(`"data": "`), []byte(`"data": "A`), 1)
	_, err = Open(key, tampered)
	assert.Error(t, err)
}

func TestIsEnvelopeRejectsLegacyCredentials(t *testing.T) {
	t.Parallel()
	assert.False(t, IsEnvelope([]byte(`{"current":"x","accessTokens":{"x":"tok"}}`)))
	assert.False(t, IsEnvelope([]byte("")))
	assert.False(t, IsEnvelope([]byte("not json")))
}

func TestSealRejectsBadKeyLength(t *testing.T) {
	t.Parallel()
	_, err := Seal([]byte("short"), BackendMock, []byte("x"))
	assert.Error(t, err)
}

func TestItemFormatRoundTrip(t *testing.T) {
	t.Parallel()
	blob := []byte{0x00, 0x01, 0xFF, 0xFE}
	kind, got, err := parseItem(formatItem(wrapTPM, blob))
	require.NoError(t, err)
	assert.Equal(t, wrapTPM, kind)
	assert.Equal(t, blob, got)

	_, _, err = parseItem("not-an-item")
	assert.Error(t, err)
	_, _, err = parseItem(itemPrefix + ":weird:AAAA")
	assert.Error(t, err)
}

//nolint:paralleltest // MockInit swaps a package-global resolver
func TestGetOrCreateKeyCreatesOnceAndIsStable(t *testing.T) {
	MockInit(t)
	st, err := Resolve(ModeAuto)
	require.NoError(t, err)
	require.Equal(t, BackendMock, st.Backend())

	_, err = st.GetKey()
	assert.True(t, errors.Is(err, ErrKeyNotFound), "GetKey must not create")

	k1, err := st.GetOrCreateKey()
	require.NoError(t, err)
	require.Len(t, k1, 32)

	k2, err := st.GetOrCreateKey()
	require.NoError(t, err)
	assert.Equal(t, k1, k2)

	// ForBackend on the recorded backend reaches the same key.
	rd, err := ForBackend(BackendMock)
	require.NoError(t, err)
	k3, err := rd.GetKey()
	require.NoError(t, err)
	assert.Equal(t, k1, k3)

	require.NoError(t, st.DeleteKey())
	require.NoError(t, st.DeleteKey(), "delete is idempotent")
	_, err = st.GetKey()
	assert.True(t, errors.Is(err, ErrKeyNotFound))
}

//nolint:paralleltest // MockInit swaps a package-global resolver
func TestCorruptStoredItemSurfacesErrorNotRegeneration(t *testing.T) {
	MockInit(t)
	st, err := Resolve(ModeAuto)
	require.NoError(t, err)
	require.NoError(t, st.b.store.set("garbage"))

	_, err = st.GetOrCreateKey()
	assert.Error(t, err, "corrupt stored key must surface an error, never be silently replaced")
	value, err := st.b.store.get()
	require.NoError(t, err)
	assert.Equal(t, "garbage", value, "corrupt item must be left untouched")
}

//nolint:paralleltest // MockInit swaps a package-global resolver
func TestResolveModes(t *testing.T) {
	MockInit(t)

	st, err := Resolve(ModePlaintext)
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend())
	_, err = st.GetKey()
	assert.True(t, errors.Is(err, ErrUnavailable))
	require.NoError(t, st.DeleteKey())

	st, err = Resolve(ModeDefault)
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend(), "default stays plaintext until the flip")

	st, err = Resolve(ModeOS)
	require.NoError(t, err)
	assert.Equal(t, BackendMock, st.Backend())
}

//nolint:paralleltest // MockInit swaps a package-global resolver
func TestForBackendUnknown(t *testing.T) {
	MockInit(t)
	_, err := ForBackend(Backend("windows-credman"))
	assert.Error(t, err)

	st, err := ForBackend(BackendPlaintext)
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend())
}
