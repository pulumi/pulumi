// Copyright 2016, Pulumi Corporation.
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

package httputil

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func http2ServerAndClient(handler http.Handler) (*httptest.Server, *http.Client) {
	// Create an HTTP/2 test server.
	// httptest.StartTLS will set NextProtos to ["http/1.1"] if it's unset, so we need to add
	// HTTP/2 eagerly before starting the server.
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{ //nolint:gosec

		NextProtos: []string{http2.NextProtoTLS},
	}
	server.StartTLS()

	// Create a client for the test server that will use HTTP/2.
	// We need a client that will (a) upgrade to HTTP/2 and (b) trust the test server's certs.
	// In order to satisfy (b), httptest sets Transport to an `http.Transport`, breaking (a),
	// so we have to manually create an `http2.Transport` and copy over the `tls.Config`.
	tlsConfig := server.Client().Transport.(*http.Transport).TLSClientConfig
	client := &http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return server, client
}

// Test that DoWithRetry rewinds and resends the request body when retrying POSTs over HTTP/2.
func TestRetryPostHTTP2(t *testing.T) {
	t.Parallel()

	tries := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		tries++
		t.Logf("try %d", tries)

		assert.Equal(t, "HTTP/2.0", r.Proto)

		// Check that the body's content length matches the sent data.
		defer r.Body.Close()
		content, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, strconv.Itoa(len(content)), r.Header.Get("Content-Length"))

		// Check the message matches.
		assert.Equal(t, string(content), "hello, server")

		// Fail the first try with 500, which will prompt a retry.
		switch tries {
		case 1:
			w.WriteHeader(500)
		default:
			w.WriteHeader(200)
		}
	}

	server, client := http2ServerAndClient(http.HandlerFunc(handler))

	req, err := http.NewRequest("POST", server.URL, strings.NewReader("hello, server"))
	require.NoError(t, err)

	res, err := DoWithRetry(req, client)
	require.NoError(t, err)
	defer res.Body.Close()

	// Check that the request succeeded on the second try.
	assert.Equal(t, 2, tries)
	assert.Equal(t, 200, res.StatusCode)
}

func TestRetry429UnderHandshakeTimeoutsOnly(t *testing.T) {
	t.Parallel()

	tries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		if tries < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, strings.NewReader("hello, server"))
	require.NoError(t, err)

	delay := time.Millisecond
	res, err := DoWithRetryOpts(req, server.Client(), RetryOpts{
		Delay:                 &delay,
		HandshakeTimeoutsOnly: true,
	})
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, 3, tries)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestRetry429Exhausted(t *testing.T) {
	t.Parallel()

	tries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, strings.NewReader("hello, server"))
	require.NoError(t, err)

	delay := time.Millisecond
	maxRetryCount := 3
	res, err := DoWithRetryOpts(req, server.Client(), RetryOpts{
		Delay:         &delay,
		MaxRetryCount: &maxRetryCount,
	})
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, 3, tries)
	assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
}

func TestRetry429HonorsRetryAfter(t *testing.T) {
	t.Parallel()

	tries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		if tries == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, strings.NewReader("hello, server"))
	require.NoError(t, err)

	delay := time.Millisecond
	start := time.Now()
	res, err := DoWithRetryOpts(req, server.Client(), RetryOpts{Delay: &delay})
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, 2, tries)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.GreaterOrEqual(t, time.Since(start), time.Second)
}

func TestRetry429ContextCanceledDuringWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader("hello, server"))
	require.NoError(t, err)

	delay := time.Millisecond
	start := time.Now()
	_, err = DoWithRetryOpts(req, server.Client(), RetryOpts{Delay: &delay}) //nolint:bodyclose // no response on error
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestRetryAfterDelay(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mk := func(value string) *http.Response {
		res := &http.Response{Header: http.Header{}}
		if value != "" {
			res.Header.Set("Retry-After", value)
		}
		return res
	}

	assert.Equal(t, time.Duration(0), retryAfterDelay(mk(""), now))
	assert.Equal(t, 2*time.Second, retryAfterDelay(mk("2"), now))
	assert.Equal(t, time.Duration(0), retryAfterDelay(mk("0"), now))
	assert.Equal(t, time.Duration(0), retryAfterDelay(mk("-3"), now))
	assert.Equal(t, time.Duration(0), retryAfterDelay(mk("garbage"), now))
	assert.Equal(t, maxRetryAfterDelay, retryAfterDelay(mk("3000"), now))

	// HTTP-date form. The date has second precision, so allow rounding slack.
	future := mk(now.Add(5 * time.Second).UTC().Format(http.TimeFormat))
	assert.InDelta(t, 5, retryAfterDelay(future, now).Seconds(), 1.1)
	past := mk(now.Add(-5 * time.Second).UTC().Format(http.TimeFormat))
	assert.Equal(t, time.Duration(0), retryAfterDelay(past, now))
}
