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

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const expiryMargin = 30 * time.Second

// ErrCannotRefresh leaves a rejected request's response for the caller to handle.
var ErrCannotRefresh = errors.New("credentials cannot be refreshed")

type refreshReason string

const (
	credentialsExpired  refreshReason = "expired"
	credentialsRejected refreshReason = "rejected"
)

type httpCredentials struct {
	accessToken string
	headers     http.Header
	expiresAt   *time.Time
}

// HTTPAuth holds the credentials for one backend. Clients that share it also share refreshed credentials.
type HTTPAuth struct {
	origin  *url.URL
	refresh func(context.Context, refreshReason) (httpCredentials, error)

	mu         sync.Mutex
	current    httpCredentials
	generation int
	refreshErr error
	refreshing *pendingRefresh
}

type pendingRefresh struct {
	done chan struct{}
	err  error
}

// NewHTTPAuth creates backend authentication with an access token and an optional refresh operation.
func NewHTTPAuth(backendURL, accessToken string, refresh func(context.Context) (string, error)) *HTTPAuth {
	origin, _ := url.Parse(backendURL)
	a := &HTTPAuth{origin: origin, current: httpCredentials{accessToken: accessToken}}
	if refresh != nil {
		a.refresh = func(ctx context.Context, _ refreshReason) (httpCredentials, error) {
			token, err := refresh(ctx)
			return httpCredentials{accessToken: token}, err
		}
	}
	return a
}

// AccessToken returns the current backend token, or an empty string if none is available.
func (a *HTTPAuth) AccessToken() string {
	if a == nil {
		return ""
	}
	credentials, _ := a.credentials()
	return credentials.accessToken
}

func (s *HTTPAuth) credentials() (httpCredentials, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, s.generation
}

func (s *HTTPAuth) renew(ctx context.Context, generation int, reason refreshReason) error {
	s.mu.Lock()
	if s.generation != generation {
		err := s.refreshErr
		s.mu.Unlock()
		return err
	}
	if pending := s.refreshing; pending != nil {
		s.mu.Unlock()
		select {
		case <-pending.done:
			return pending.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	pending := &pendingRefresh{done: make(chan struct{})}
	s.refreshing = pending
	s.mu.Unlock()

	credentials, err := s.refresh(ctx, reason)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.current = credentials
	}
	s.generation++
	s.refreshErr = err
	s.refreshing = nil
	pending.err = err
	close(pending.done)
	return err
}

type operation struct {
	auth      *HTTPAuth
	refreshed bool
	err       error
}

type operationKey struct{}

// WithHTTPAuth starts a backend API operation. Its requests and retries share at most one refresh.
func WithHTTPAuth(ctx context.Context, a *HTTPAuth) context.Context {
	return context.WithValue(ctx, operationKey{}, &operation{auth: a})
}

// Transport authenticates backend API operations marked by WithHTTPAuth. Other requests pass through unchanged.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &httpTransport{base: base}
}

type httpTransport struct {
	base http.RoundTripper
}

func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	op, _ := req.Context().Value(operationKey{}).(*operation)
	if op == nil || op.auth == nil {
		return t.base.RoundTrip(req)
	}
	if op.err != nil {
		closeRequestBody(req)
		return nil, op.err
	}
	if !sameOrigin(op.auth.origin, req.URL) {
		req = req.Clone(req.Context())
		req.Header.Del("Authorization")
		return t.base.RoundTrip(req)
	}

	credentials, generation := op.auth.credentials()
	useToken := !strings.HasPrefix(req.Header.Get("Authorization"), "update-token ")
	if !useToken && len(credentials.headers) == 0 {
		return t.base.RoundTrip(req)
	}
	refresh := func(reason refreshReason) error {
		op.refreshed = true
		op.err = op.auth.renew(req.Context(), generation, reason)
		credentials, generation = op.auth.credentials()
		return op.err
	}
	send := func() (*http.Response, error) {
		authorized := req.Clone(req.Context())
		if useToken {
			authorized.Header.Del("Authorization")
			if credentials.accessToken != "" {
				authorized.Header.Set("Authorization", "token "+credentials.accessToken)
			}
		}
		for name, values := range credentials.headers {
			for _, value := range values {
				authorized.Header.Add(name, value)
			}
		}
		return t.base.RoundTrip(authorized)
	}

	if op.auth.refresh != nil && !op.refreshed && credentials.expiresAt != nil &&
		!credentials.expiresAt.After(time.Now().Add(expiryMargin)) {
		if err := refresh(credentialsExpired); err != nil {
			closeRequestBody(req)
			return nil, err
		}
	}
	resp, err := send()
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	if err != nil || resp.StatusCode != http.StatusUnauthorized || !replayable ||
		op.auth.refresh == nil || op.refreshed {
		return resp, err
	}
	if err := refresh(credentialsRejected); err != nil {
		if errors.Is(err, ErrCannotRefresh) {
			op.err = nil
			return resp, nil
		}
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()
	if req.GetBody != nil {
		req = req.Clone(req.Context())
		if req.Body, err = req.GetBody(); err != nil {
			return nil, err
		}
	}
	return send()
}

func closeRequestBody(req *http.Request) {
	if req.Body != nil {
		req.Body.Close()
	}
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
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}
