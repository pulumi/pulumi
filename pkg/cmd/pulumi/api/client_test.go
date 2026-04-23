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

package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIClientDo_GzipRequestCompression pins that POST bodies are gzip
// compressed on the wire and Content-Encoding: gzip is set.
func TestAPIClientDo_GzipRequestCompression(t *testing.T) {
	t.Parallel()
	var gotEncoding string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		gotBody, err = io.ReadAll(gr)
		require.NoError(t, err)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "")
	resp, err := c.Do(t.Context(), http.MethodPost, "/x", nil,
		strings.NewReader(`{"name":"acme"}`), "application/json", "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "gzip", gotEncoding)
	assert.JSONEq(t, `{"name":"acme"}`, string(gotBody))
}

// TestAPIClientDo_GzipResponseDecompression pins that gzip-encoded response
// bodies are transparently decoded and that Content-Encoding and
// Content-Length headers are removed (both applied to the compressed bytes
// and would mislead any consumer that inspects them after decompression).
func TestAPIClientDo_GzipResponseDecompression(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		encoding string
	}{
		{"gzip", "gzip"},
		{"x-gzip", "x-gzip"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := `{"ok":true,"msg":"hello"}`
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			_, err := gw.Write([]byte(payload))
			require.NoError(t, err)
			require.NoError(t, gw.Close())

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", tc.encoding)
				w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
				_, _ = w.Write(buf.Bytes())
			}))
			t.Cleanup(srv.Close)

			c := NewAPIClient(srv.URL, "")
			resp, err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil, "", "application/json", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.JSONEq(t, payload, string(body))
			assert.Empty(t, resp.Header.Get("Content-Encoding"),
				"Content-Encoding must be stripped so consumers don't double-decode")
			assert.Empty(t, resp.Header.Get("Content-Length"),
				"Content-Length applied to the compressed bytes and is meaningless after decompression")
		})
	}
}

// TestAPIClientDo_WarningHeaderPassthrough pins that X-Pulumi-Warning response
// headers are surfaced as stderr warnings.
//
//nolint:paralleltest // mutates os.Stderr
func TestAPIClientDo_WarningHeaderPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Pulumi-Warning", "deprecated field in use")
		w.Header().Add("X-Pulumi-Warning", "rate limit approaching")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "")
	stderr := captureStderr(t, func() {
		resp, err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil, "", "application/json", nil)
		require.NoError(t, err)
		resp.Body.Close()
	})
	assert.Contains(t, stderr, "warning: deprecated field in use")
	assert.Contains(t, stderr, "warning: rate limit approaching")
}

// trackingReadCloser records whether Close has been called, so tests can
// verify gzipReadCloser forwards Close to the underlying body.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error { t.closed = true; return nil }

// TestGzipReadCloser_ClosesUnderlyingBody pins that closing the wrapper
// closes both the gzip reader and the underlying response body.
func TestGzipReadCloser_ClosesUnderlyingBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte(`{"ok":true}`))
	require.NoError(t, gw.Close())

	body := &trackingReadCloser{Reader: bytes.NewReader(buf.Bytes())}
	gr, err := gzip.NewReader(body)
	require.NoError(t, err)

	wrapper := &gzipReadCloser{gzip: gr, body: body}
	_, err = io.ReadAll(wrapper)
	require.NoError(t, err)
	require.NoError(t, wrapper.Close())
	assert.True(t, body.closed, "underlying body must be closed when the wrapper is closed")
}
