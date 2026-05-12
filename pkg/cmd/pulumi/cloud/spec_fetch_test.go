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

package cloud

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// TestHostFromURL verifies the cache key is the hostname with port, scheme,
// and path stripped.
func TestHostFromURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url  string
		want string
	}{
		{"https://api.pulumi.com", "api.pulumi.com"},
		{"https://pulumi.example.com:8443", "pulumi.example.com"},
		{"http://localhost:3000", "localhost"},
		{"https://host", "host"},
		{"not a url", ""}, // no scheme → url.Parse returns no host
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, hostFromURL(tc.url))
		})
	}
}

// TestReadCachedSpec_ReportsAge verifies readCachedSpec returns a mtime-derived
// age and handles missing files without error.
func TestReadCachedSpec_ReportsAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"a":1}`), 0o600))
	past := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(path, past, past))

	data, age, ok, err := readCachedSpec(path)
	require.NoError(t, err)
	require.True(t, ok)
	assert.JSONEq(t, `{"a":1}`, string(data))
	assert.GreaterOrEqual(t, age, time.Hour)

	_, _, missingOK, err := readCachedSpec(filepath.Join(dir, "nope.json"))
	require.NoError(t, err)
	assert.False(t, missingOK)
}

// seedSpecCache writes data to the cache path that ensureSpec would use for
// cloudURL, sets its mtime to now-age, and returns the resolved path. Assumes
// PULUMI_HOME is already pointing at an isolated tempdir.
func seedSpecCache(t *testing.T, cloudURL string, data []byte, age time.Duration) string {
	t.Helper()
	path, err := specCachePath(cloudURL)
	require.NoError(t, err)
	require.NoError(t, writeCachedSpec(path, data))
	mtime := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, mtime, mtime))
	return path
}

// newSpecServer returns an httptest server that serves serveBody from the
// spec endpoint. The returned counter increments on each request.
func newSpecServer(t *testing.T, serveBody []byte) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, "/api/openapi/pulumi-spec.json", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(serveBody)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// TestEnsureSpec_CacheHitUnderTTL pins that a fresh cache hit is served
// without touching the network.
//
//nolint:paralleltest // mutates PULUMI_HOME / PULUMI_API
func TestEnsureSpec_CacheHitUnderTTL(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	cached := []byte(`{"cached":true}`)

	srv, hits := newSpecServer(t, []byte(`{"fresh":true}`))
	t.Setenv("PULUMI_API", srv.URL)

	_ = seedSpecCache(t, srv.URL, cached, time.Hour)

	data, err := ensureSpec(t.Context(), io.Discard, false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"cached":true}`, string(data))
	assert.Zero(t, hits.Load(), "cache hit under TTL must not touch the network")
}

// TestEnsureSpec_TTLExpiryTriggersFetch pins that a cache older than
// specCacheTTL forces a refresh and the new bytes are written back.
//
//nolint:paralleltest // mutates PULUMI_HOME / PULUMI_API / specCacheTTL
func TestEnsureSpec_TTLExpiryTriggersFetch(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	origTTL := specCacheTTL
	specCacheTTL = time.Hour
	t.Cleanup(func() { specCacheTTL = origTTL })

	fresh := []byte(`{"fresh":true}`)
	srv, hits := newSpecServer(t, fresh)
	t.Setenv("PULUMI_API", srv.URL)

	path := seedSpecCache(t, srv.URL, []byte(`{"stale":true}`), 2*time.Hour)

	data, err := ensureSpec(t.Context(), io.Discard, false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"fresh":true}`, string(data))
	assert.Equal(t, int64(1), hits.Load())

	onDisk, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"fresh":true}`, string(onDisk))
}

// TestEnsureSpec_RefreshForcesFetch pins that --refresh-spec bypasses a
// still-fresh cache.
//
//nolint:paralleltest // mutates PULUMI_HOME / PULUMI_API
func TestEnsureSpec_RefreshForcesFetch(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	srv, hits := newSpecServer(t, []byte(`{"fresh":true}`))
	t.Setenv("PULUMI_API", srv.URL)

	_ = seedSpecCache(t, srv.URL, []byte(`{"cached":true}`), time.Minute)

	data, err := ensureSpec(t.Context(), io.Discard, true)
	require.NoError(t, err)
	assert.JSONEq(t, `{"fresh":true}`, string(data))
	assert.Equal(t, int64(1), hits.Load())
}

// TestEnsureSpec_StaleFallbackOnFetchFailure pins that a 5xx during a TTL-driven
// refresh (refresh=false) returns the stale cached bytes with a warning.
//
//nolint:paralleltest // mutates PULUMI_HOME / PULUMI_API / specCacheTTL
func TestEnsureSpec_StaleFallbackOnFetchFailure(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	origTTL := specCacheTTL
	specCacheTTL = time.Hour
	t.Cleanup(func() { specCacheTTL = origTTL })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_API", srv.URL)

	_ = seedSpecCache(t, srv.URL, []byte(`{"stale":true}`), 2*time.Hour)

	var warnings bytes.Buffer
	data, err := ensureSpec(t.Context(), &warnings, false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"stale":true}`, string(data))
	assert.Contains(t, warnings.String(), "using cached spec")
}

// TestEnsureSpec_RefreshFlagFailsHardOnFetchError pins that an explicit
// --refresh-spec (refresh=true) returns the fetch error rather than silently
// falling back to stale cache.
//
//nolint:paralleltest // mutates PULUMI_HOME / PULUMI_API
func TestEnsureSpec_RefreshFlagFailsHardOnFetchError(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PULUMI_API", srv.URL)

	_ = seedSpecCache(t, srv.URL, []byte(`{"stale":true}`), time.Minute)

	var warnings bytes.Buffer
	_, err := ensureSpec(t.Context(), &warnings, true)
	require.Error(t, err, "refresh=true must fail hard rather than serve stale")
	assert.Empty(t, warnings.String(), "no fallback warning when refresh=true")
}

// TestFetchSpec_Unauthorized pins that a 401 from the spec endpoint produces
// an APIError with ExitAuthenticationError and a `pulumi login` suggestion.
func TestFetchSpec_Unauthorized(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	resolved := &ResolvedContext{
		Client:   client.NewClient(srv.URL, "", false, nil),
		CloudURL: srv.URL,
	}
	_, err := fetchSpec(t.Context(), resolved)
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, cmdutil.ExitAuthenticationError, apiErr.ExitCode)
	joined := fmt.Sprint(apiErr.Envelope.Error.Suggestions)
	assert.Contains(t, joined, "pulumi login")
}
