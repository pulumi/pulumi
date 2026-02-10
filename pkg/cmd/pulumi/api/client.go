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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

// APIClient is a thin HTTP client for Pulumi Cloud API calls.
type APIClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client.
func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: http.DefaultClient,
	}
}

// Do executes an API request and returns the raw response.
// The contentType parameter sets the Content-Type header for requests with a body.
// The accept parameter sets the Accept header for the response.
func (c *APIClient) Do(ctx context.Context, method, path string, query url.Values, body io.Reader,
	contentType, accept string,
) (*http.Response, error) {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	// Buffer and gzip-compress the request body if present.
	var reqBody io.Reader
	if body != nil {
		raw, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(raw); err != nil {
			return nil, fmt.Errorf("compressing request body: %w", err)
		}
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("closing gzip writer: %w", err)
		}
		// Use bytes.NewReader so http.NewRequestWithContext sets GetBody
		// automatically, which is required for retry support.
		reqBody = bytes.NewReader(buf.Bytes())
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("User-Agent", client.UserAgent())

	// Set Accept header: operation-specific content type + Pulumi API version.
	if accept != "" {
		req.Header.Set("Accept", accept+", application/vnd.pulumi+8")
	} else {
		req.Header.Set("Accept", "application/json, application/vnd.pulumi+8")
	}

	req.Header.Set("Accept-Encoding", "gzip")

	if body != nil {
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Retry with the same parameters as the backend client:
	// 1s initial delay, 2x backoff, 30s max delay, 4 retries.
	// Only retry GETs fully; for other methods only retry TLS handshake timeouts.
	resp, err := httputil.DoWithRetryOpts(req, c.HTTPClient, httputil.RetryOpts{
		Delay:                 durationPtr(time.Second),
		Backoff:               float64Ptr(2.0),
		MaxDelay:              durationPtr(30 * time.Second),
		MaxRetryCount:         intPtr(4),
		HandshakeTimeoutsOnly: method != http.MethodGet,
	})
	if err != nil {
		return nil, err
	}

	// Display any X-Pulumi-Warning headers.
	if warnings, ok := resp.Header["X-Pulumi-Warning"]; ok {
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}

	// Transparently decompress gzip responses.
	// The HTTP/1.1 spec recommends treating x-gzip as an alias of gzip.
	if ce := resp.Header.Get("Content-Encoding"); ce == "gzip" || ce == "x-gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decompressing gzip response: %w", err)
		}
		resp.Body = &gzipReadCloser{gzip: gr, body: resp.Body}
		resp.Header.Del("Content-Encoding")
	}

	return resp, nil
}

func durationPtr(d time.Duration) *time.Duration { return &d }
func float64Ptr(f float64) *float64               { return &f }
func intPtr(i int) *int                            { return &i }

// gzipReadCloser wraps a gzip reader and the underlying body so both get closed.
type gzipReadCloser struct {
	gzip *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gzip.Read(p)
}

func (g *gzipReadCloser) Close() error {
	g.gzip.Close()
	return g.body.Close()
}
