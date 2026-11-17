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
		mu         sync.Mutex
		refreshed  bool
		refreshErr error
	}
)

// WithOperation shares one credential refresh attempt and its error across retries of a backend API operation.
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
			for name := range r.snapshot().credentials.Headers {
				req.Header.Del(name)
			}
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

type refreshAttempt struct {
	once sync.Once
	done chan struct{}
	err  error
}

// credentialState pairs immutable credentials with their shared refresh attempt.
type credentialState struct {
	credentials response
	refresh     refreshAttempt
}

func (r *Result) snapshot() *credentialState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (s *credentialState) expired() bool {
	return s.credentials.ExpiresAt != nil && !s.credentials.ExpiresAt.After(time.Now().Add(30*time.Second))
}

func (s *credentialState) apply(req *http.Request) *http.Request {
	clone := req.Clone(req.Context())
	if !strings.HasPrefix(req.Header.Get("Authorization"), "update-token ") {
		clone.Header.Del("Authorization")
		if s.credentials.AccessToken != "" {
			clone.Header.Set("Authorization", "token "+s.credentials.AccessToken)
		}
	}
	for name, values := range s.credentials.Headers {
		clone.Header.Del(name)
		for _, v := range values {
			clone.Header.Add(name, v)
		}
	}
	return clone
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
	if op.refreshErr != nil {
		return nil, op.refreshErr
	}
	r := t.result
	state := r.snapshot()
	if state.expired() && !op.refreshed {
		op.refreshed = true
		op.refreshErr = r.refresh(req.Context(), state, "expired")
		if op.refreshErr != nil {
			return nil, op.refreshErr
		}
	}
	state = r.snapshot()
	resp, err := t.base.RoundTrip(state.apply(req))
	if err != nil || resp.StatusCode != http.StatusUnauthorized || op.refreshed {
		return resp, err
	}
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	op.refreshed = true
	op.refreshErr = r.refresh(req.Context(), state, "rejected")
	if op.refreshErr != nil {
		resp.Body.Close()
		return nil, op.refreshErr
	}
	retry := r.snapshot().apply(req)
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

// refresh shares an attempt's outcome while allowing each caller to cancel its wait.
func (r *Result) refresh(ctx context.Context, state *credentialState, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	attempt := &state.refresh
	attempt.once.Do(func() {
		attempt.done = make(chan struct{})
		refreshCtx := context.WithoutCancel(ctx)
		go func() {
			credentials, err := r.fetchCredentials(refreshCtx, reason)
			if err != nil {
				credentials = state.credentials
			}
			r.mu.Lock()
			r.state = &credentialState{credentials: credentials}
			r.mu.Unlock()
			attempt.err = err
			close(attempt.done)
		}()
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-attempt.done:
		return attempt.err
	}
}

func (r *Result) fetchCredentials(ctx context.Context, reason string) (response, error) {
	r.mu.Lock()
	helper, url, session, hasToken := r.Helper, r.url, r.session, r.hasToken
	r.mu.Unlock()
	resp, err := invoke(ctx, helper, request{Version: 1, Reason: reason, SelectedBackendURL: url}, session)
	if err != nil {
		return response{}, err
	}
	if resp.BackendURL != nil || resp.Env != nil {
		return response{}, errors.New("credentials helper refresh must not return backendUrl or env")
	}
	if resp.AccessToken == "" && len(resp.Headers) == 0 {
		return response{}, errors.New("credentials helper refresh requires HTTP credentials")
	}
	if (resp.AccessToken != "") != hasToken {
		return response{}, errors.New("credentials helper refresh cannot change whether an access token is supplied")
	}
	return resp, nil
}
