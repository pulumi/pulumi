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
	"fmt"
	"sync"
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

func TestUnsupportedEnvelopeVersion(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	env, err := Seal(key, BackendMock, []byte("secret"))
	require.NoError(t, err)
	future := bytes.Replace(env, []byte(`"$pulumiSecureStore": 1`), []byte(`"$pulumiSecureStore": 2`), 1)
	require.NotEqual(t, env, future)

	assert.True(t, IsEnvelope(future), "a future-version envelope must still be detected as an envelope")
	_, err = EnvelopeBackend(future)
	assert.True(t, errors.Is(err, ErrUnsupportedVersion))
	_, err = Open(key, future)
	assert.True(t, errors.Is(err, ErrUnsupportedVersion))
}

func TestOpenRejectsUnknownAlgo(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	env, err := Seal(key, BackendMock, []byte("secret"))
	require.NoError(t, err)
	tampered := bytes.Replace(env, []byte(`"algo": "aes-256-gcm"`), []byte(`"algo": "rot13"`), 1)
	require.NotEqual(t, env, tampered)
	_, err = Open(key, tampered)
	assert.ErrorContains(t, err, "algorithm")
}

func TestOpenRejectsTamperedHeader(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	env, err := Seal(key, BackendMock, []byte("secret"))
	require.NoError(t, err)
	tampered := bytes.Replace(env, []byte(`"backend": "mock"`), []byte(`"backend": "mock-strong"`), 1)
	require.NotEqual(t, env, tampered)
	_, err = Open(key, tampered)
	assert.True(t, errors.Is(err, ErrWrongKey), "header edits must fail authentication")
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
	st, err := Resolve(ModeAuto, "")
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

	rd, err := ForBackend(BackendMock, "")
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
	st, err := Resolve(ModeAuto, "")
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

	st, err := Resolve(ModePlaintext, "")
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend())
	_, err = st.GetKey()
	assert.True(t, errors.Is(err, ErrUnavailable))
	require.NoError(t, st.DeleteKey())

	st, err = Resolve(ModeDefault, "")
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend(), "default stays plaintext until the flip")

	st, err = Resolve(ModeOS, "")
	require.NoError(t, err)
	assert.Equal(t, BackendMock, st.Backend())
}

func TestOutcomeClassification(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Ready, outcomeOf(nil))
	assert.Equal(t, Declined, outcomeOf(fmt.Errorf("wrapped: %w", ErrDeclined)))
	assert.Equal(t, Locked, outcomeOf(fmt.Errorf("wrapped: %w", ErrLocked)))
	assert.Equal(t, Absent, outcomeOf(ErrUnavailable))
	assert.Equal(t, Absent, outcomeOf(errors.New("something else")))

	assert.True(t, errors.Is(ErrLocked, ErrUnavailable), "locked still means unusable, so fallback keeps working")
	assert.False(t, errors.Is(ErrDeclined, ErrUnavailable), "a refusal must never look like absence")
}

//nolint:paralleltest // mutates the package-global resolution hook
func TestDeclinedStopsTheChain(t *testing.T) {
	fallback := &memStore{}
	prevHook := platformCandidatesHook
	platformCandidatesHook = func(bool, string) []backendImpl {
		return []backendImpl{
			{id: BackendMockStrong, store: &refusingStore{}, wrap: rawWrapper{}},
			{id: BackendMock, store: fallback, wrap: rawWrapper{}},
		}
	}
	t.Cleanup(func() { platformCandidatesHook = prevHook })

	for _, mode := range []Mode{ModeAuto, ModeOS} {
		st, err := Resolve(mode, "")
		require.Error(t, err, "a refusal must fail the resolution, not fall back")
		assert.True(t, errors.Is(err, ErrDeclined))
		assert.Nil(t, st)
	}
	_, err := fallback.get()
	assert.True(t, errors.Is(err, ErrKeyNotFound), "the next backend must not have been used")
}

//nolint:paralleltest // MockInit swaps a package-global resolver
func TestForBackendUnknown(t *testing.T) {
	MockInit(t)
	_, err := ForBackend(Backend("windows-credman"), "")
	assert.Error(t, err)

	st, err := ForBackend(BackendPlaintext, "")
	require.NoError(t, err)
	assert.Equal(t, BackendPlaintext, st.Backend())
}

// Loses a non-atomic create race: set fails, but a key is readable after.
type raceLosingStore struct {
	winner string
	gets   int
}

func (r *raceLosingStore) available() (Outcome, error) { return Ready, nil }

func (r *raceLosingStore) get() (string, error) {
	r.gets++
	if r.gets == 1 {
		// Mirrors `security find-generic-password` reporting "not found" just
		// before `add-generic-password` collides.
		return "", ErrKeyNotFound
	}
	if r.gets == 2 {
		return "", ErrKeyNotFound
	}
	return r.winner, nil
}

func (r *raceLosingStore) set(value string) error {
	return errors.New("exit status 45")
}

func (r *raceLosingStore) delete() error { return nil }

//nolint:paralleltest // mutates the package-global resolution hook
func TestGetOrCreateKeyReconcilesWhenSetLosesRace(t *testing.T) {
	winnerKey := testKey(t)
	store := &raceLosingStore{winner: formatItem(wrapRaw, winnerKey)}
	prevHook := platformCandidatesHook
	platformCandidatesHook = func(bool, string) []backendImpl {
		return []backendImpl{{id: BackendMock, store: store, wrap: rawWrapper{}}}
	}
	t.Cleanup(func() { platformCandidatesHook = prevHook })

	st, err := Resolve(ModeAuto, "")
	require.NoError(t, err)
	key, err := st.GetOrCreateKey()
	require.NoError(t, err, "losing the create race must reconcile silently, not fail")
	assert.Equal(t, winnerKey, key, "the persisted (winning) key must be adopted")
}

//nolint:paralleltest // mutates the package-global resolution hook
func TestGetOrCreateKeyConcurrent(t *testing.T) {
	MockInit(t)
	st, err := Resolve(ModeAuto, "")
	require.NoError(t, err)

	const n = 16
	keys := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			keys[i], errs[i] = st.GetOrCreateKey()
		}(i)
	}
	wg.Wait()
	for i := range n {
		require.NoError(t, errs[i])
		assert.Equal(t, keys[0], keys[i], "all concurrent callers must converge on one key")
	}
}

// "Seals" by prefixing a marker, so wrapped and raw payloads differ.
type fakeTPMWrapper struct{}

func (fakeTPMWrapper) kind() wrapKind   { return wrapTPM }
func (fakeTPMWrapper) available() error { return nil }
func (fakeTPMWrapper) wrap(key []byte) ([]byte, error) {
	return append([]byte("sealed:"), key...), nil
}

func (fakeTPMWrapper) unwrap(blob []byte) ([]byte, error) {
	if !bytes.HasPrefix(blob, []byte("sealed:")) {
		return nil, errors.New("not a sealed blob")
	}
	return bytes.TrimPrefix(blob, []byte("sealed:")), nil
}

//nolint:paralleltest // mutates the package-global resolution hook
func TestGetOrCreateKeyUpgradesRawItemToTPM(t *testing.T) {
	rawKey := testKey(t)
	mem := &memStore{}
	require.NoError(t, mem.set(formatItem(wrapRaw, rawKey)))
	prevHook := platformCandidatesHook
	platformCandidatesHook = func(bool, string) []backendImpl {
		return []backendImpl{{id: BackendMock, store: mem, wrap: fakeTPMWrapper{}}}
	}
	t.Cleanup(func() { platformCandidatesHook = prevHook })

	st, err := Resolve(ModeAuto, "")
	require.NoError(t, err)
	key, err := st.GetOrCreateKey()
	require.NoError(t, err, "raw->tpm upgrade must succeed")
	assert.Equal(t, rawKey, key, "the existing key must be preserved, not regenerated")

	value, err := mem.get()
	require.NoError(t, err)
	kind, blob, err := parseItem(value)
	require.NoError(t, err)
	assert.Equal(t, wrapTPM, kind, "stored item must now be TPM-wrapped")
	assert.Equal(t, append([]byte("sealed:"), rawKey...), blob)

	got, err := st.GetKey()
	require.NoError(t, err)
	assert.Equal(t, rawKey, got)
}

func TestMemoizePrecheckAsksOncePerPolicy(t *testing.T) {
	t.Parallel()
	calls := 0
	precheck := memoizePrecheck(func(bool) (Outcome, error) {
		calls++
		return Ready, nil
	})

	for range 3 {
		_, _ = precheck(true)
	}
	assert.Equal(t, 1, calls, "repeated probes must not re-ask, or a declined dialog would reappear")

	_, _ = precheck(false)
	assert.Equal(t, 2, calls, "the silent policy is a separate question")
}
