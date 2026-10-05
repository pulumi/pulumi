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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// expiryMargin is how long before expiresAt the helper's credentials are refreshed.
const expiryMargin = 30 * time.Second

// HTTPAuth holds the token and headers a credential helper supplied for one HTTP backend, and
// refreshes them. It is shared by every HTTP client of that backend. A nil HTTPAuth is valid and
// leaves requests unchanged.
type HTTPAuth struct {
	backendURL string
	origin     *url.URL
	run        runHelperFunc

	// mu is held while the helper runs, so requests that start during a refresh wait for its result.
	mu      sync.Mutex
	current *credentialhelper.Response
	// generation counts refresh attempts. A request refreshes only if no attempt was made since it
	// read the credentials; otherwise it shares that attempt's outcome.
	generation int
	refreshErr error
}

func newHTTPAuth(backendURL string, response *credentialhelper.Response, run runHelperFunc) *HTTPAuth {
	origin, _ := url.Parse(backendURL)
	return &HTTPAuth{backendURL: backendURL, origin: origin, run: run, current: response}
}

// AccessToken returns the helper's current token when it is the backend's effective token.
// PULUMI_ACCESS_TOKEN takes precedence over the helper.
func (a *HTTPAuth) AccessToken() string {
	if a == nil || env.AccessToken.Value() != "" {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current.AccessToken
}

func (a *HTTPAuth) credentials() (*credentialhelper.Response, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current, a.generation
}

func (a *HTTPAuth) refresh(ctx context.Context, generation int, reason credentialhelper.Reason) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.generation != generation {
		return a.refreshErr
	}
	logging.V(7).Infof("Refreshing credential helper HTTP credentials (%s)", reason)
	response, err := a.run(ctx, credentialhelper.Request{Reason: reason, SelectedBackendURL: a.backendURL})
	switch {
	case err != nil:
	case !response.HasHTTPCredentials():
		err = errors.New("helper returned no HTTP credentials")
	case a.current.AccessToken != "" && response.AccessToken == "":
		err = errors.New("helper must continue supplying an accessToken")
	case a.current.AccessToken == "" && response.AccessToken != "":
		err = errors.New("helper must not start supplying an accessToken on refresh")
	}
	a.generation++
	a.refreshErr = nil
	if err != nil {
		a.refreshErr = fmt.Errorf("refreshing credential helper credentials: %w", err)
		return a.refreshErr
	}
	redactHelperCredentials(response)
	a.current = response
	return nil
}

// Transport returns a transport that adds the helper's credentials to backend API requests, those
// whose context comes from WithHelperAuth, and refreshes them when they have expired or the backend
// rejects them. Other requests, such as artifact downloads and uploads, pass through unchanged.
func (a *HTTPAuth) Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if a == nil {
		return base
	}
	return &helperTransport{auth: a, base: base}
}

type helperTransport struct {
	auth *HTTPAuth
	base http.RoundTripper
}

// refreshAllowance limits helper refreshes to one, and remembers its failure.
type refreshAllowance struct {
	refreshed bool
	err       error
}

type refreshAllowanceKey struct{}

// WithHelperAuth marks the requests of one backend API operation. They carry the helper's credentials
// and share a single helper refresh, so that the limit applies to the operation, including its
// retries, rather than to each request.
func WithHelperAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, refreshAllowanceKey{}, &refreshAllowance{})
}

func (t *helperTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	allowance, ok := req.Context().Value(refreshAllowanceKey{}).(*refreshAllowance)
	if !ok {
		return t.base.RoundTrip(req)
	}
	// A retry loop above this transport must not turn a failed refresh into more helper runs.
	if allowance.err != nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, allowance.err
	}
	// The helper's credentials are for the backend's origin only; http.Client forwards
	// Authorization to subdomains when it follows a redirect.
	if !sameOrigin(t.auth.origin, req.URL) {
		req = req.Clone(req.Context())
		req.Header.Del("Authorization")
		return t.base.RoundTrip(req)
	}

	// Update lease tokens authenticate on their own; such requests only receive helper headers.
	lease := strings.HasPrefix(req.Header.Get("Authorization"), "update-token ")
	setToken := !lease && t.auth.AccessToken() != ""
	credentials, generation := t.auth.credentials()
	if !setToken && len(credentials.Headers) == 0 {
		return t.base.RoundTrip(req)
	}
	refresh := func(reason credentialhelper.Reason) error {
		allowance.refreshed = true
		allowance.err = t.auth.refresh(req.Context(), generation, reason)
		credentials, generation = t.auth.credentials()
		return allowance.err
	}
	send := func() (*http.Response, error) {
		authorized := req.Clone(req.Context())
		if setToken {
			authorized.Header.Set("Authorization", "token "+credentials.AccessToken)
		}
		for name, values := range credentials.Headers {
			for _, value := range values {
				authorized.Header.Add(name, value)
			}
		}
		return t.base.RoundTrip(authorized)
	}

	expired := credentials.ExpiresAt != nil && !credentials.ExpiresAt.After(time.Now().Add(expiryMargin))
	if expired && !allowance.refreshed {
		if err := refresh(credentialhelper.Expired); err != nil {
			if req.Body != nil {
				req.Body.Close()
			}
			return nil, err
		}
	}
	resp, err := send()
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	if err != nil || resp.StatusCode != http.StatusUnauthorized || allowance.refreshed || !replayable {
		return resp, err
	}
	resp.Body.Close()
	if err := refresh(credentialhelper.Rejected); err != nil {
		return nil, err
	}
	if req.GetBody != nil {
		req = req.Clone(req.Context())
		if req.Body, err = req.GetBody(); err != nil {
			return nil, err
		}
	}
	return send()
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
