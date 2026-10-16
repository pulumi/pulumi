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

package credentialhelper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type (
	operationKey struct{}
	operation    struct {
		mu        sync.Mutex
		refreshed bool
	}
)

// WithOperation bounds credential refresh to once across retries of a backend API operation.
func WithOperation(ctx context.Context) context.Context {
	return context.WithValue(ctx, operationKey{}, &operation{})
}

func sameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// HTTPClient adds backend-scoped helper credentials without changing the supplied client.
func (r *Result) HTTPClient(base *http.Client) *http.Client {
	c := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	origin, _ := url.Parse(r.BackendURL())
	c.Transport = &helperTransport{base: transport, result: r, origin: origin}
	previous := base.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !sameOrigin(origin, req.URL) {
			req.Header.Del("Authorization")
			// Redirects copy headers from the original request, so remove every helper header.
			r.mu.Lock()
			for name := range r.credentials.Headers {
				req.Header.Del(name)
			}
			r.mu.Unlock()
		}
		if previous != nil {
			return previous(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &c
}

type helperTransport struct {
	base   http.RoundTripper
	result *Result
	origin *url.URL
}

func (t *helperTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !sameOrigin(t.origin, req.URL) {
		return t.base.RoundTrip(req)
	}
	op, _ := req.Context().Value(operationKey{}).(*operation)
	if op == nil {
		op = &operation{}
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	r := t.result
	r.mu.Lock()
	if r.credentials.ExpiresAt != nil && !r.credentials.ExpiresAt.After(time.Now().Add(30*time.Second)) && !op.refreshed {
		op.refreshed = true
		if err := r.refresh(req.Context(), "expired"); err != nil {
			r.mu.Unlock()
			return nil, err
		}
	}
	generation := r.generation
	auth := req.Header.Get("Authorization")
	lease := strings.HasPrefix(auth, "update-token ")
	apply := func(request *http.Request) *http.Request {
		clone := request.Clone(request.Context())
		if !lease {
			clone.Header.Del("Authorization")
			if r.credentials.AccessToken != "" {
				clone.Header.Set("Authorization", "token "+r.credentials.AccessToken)
			}
		}
		for name, values := range r.credentials.Headers {
			clone.Header.Del(name)
			for _, v := range values {
				clone.Header.Add(name, v)
			}
		}
		return clone
	}
	outgoing := apply(req)
	r.mu.Unlock()
	resp, err := t.base.RoundTrip(outgoing)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || op.refreshed {
		return resp, err
	}
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	r.mu.Lock()
	op.refreshed = true
	if r.generation == generation {
		if err = r.refresh(req.Context(), "rejected"); err != nil {
			r.mu.Unlock()
			resp.Body.Close()
			return nil, err
		}
	}
	retry := apply(req)
	r.mu.Unlock()
	if req.Body != nil {
		retry.Body, err = req.GetBody()
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return t.base.RoundTrip(retry)
}

// refresh is called with the credential lock held, so concurrent REST and ESC requests share a refresh.
func (r *Result) refresh(ctx context.Context, reason string) error {
	resp, err := invoke(ctx, r.Helper, request{Version: 1, Reason: reason, SelectedBackendURL: r.url}, r.session)
	if err != nil {
		return err
	}
	if resp.BackendURL != nil || resp.Env != nil {
		return errors.New("credentials helper refresh must not return backendUrl or env")
	}
	if resp.AccessToken == "" && len(resp.Headers) == 0 {
		return errors.New("credentials helper refresh requires HTTP credentials")
	}
	if (resp.AccessToken != "") != r.hasToken {
		return errors.New("credentials helper refresh cannot change whether an access token is supplied")
	}
	r.credentials = resp
	r.generation++
	return nil
}
