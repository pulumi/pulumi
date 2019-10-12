// Copyright 2016-2018, Pulumi Corporation.
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
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/http2"

	"github.com/stretchr/testify/assert"
)

func http2ServerAndClient(handler http.Handler) (*httptest.Server, *http.Client) {
	// Create an HTTP/2 test server.
	// httptest.StartTLS will set NextProtos to ["http/1.1"] if it's unset, so we need to add
	// HTTP/2 eagerly before starting the server.
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{
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
		content, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
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
	assert.NoError(t, err)

	res, err := DoWithRetry(req, client)
	assert.NoError(t, err)
	defer res.Body.Close()

	// Check that the request succeeded on the second try.
	assert.Equal(t, 2, tries)
	assert.Equal(t, 200, res.StatusCode)
}
