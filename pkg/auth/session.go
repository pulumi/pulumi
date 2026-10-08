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
	"io"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type runHelperFunc func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error)

// Session runs the credential helper for the backends opened by one CLI invocation.
type Session struct {
	mu sync.Mutex
	// find initializes run lazily. It is nil when run was supplied directly.
	find     func() error
	run      runHelperFunc
	resolved *credentialhelper.Resolved
	stderr   io.Writer
	// helperEnv holds the variables applied so far, to detect conflicts between backends.
	helperEnv map[string]string
	// prepared holds the outcome for each backend the helper ran for.
	prepared map[string]preparedBackend
	// unselected is the outcome of the request that let the helper select a backend.
	unselected *preparedBackend
	// validateBackend rejects a backend the helper selected that the caller cannot open.
	validateBackend func(backendURL string) error
	httpAuth        map[string]*HTTPAuth
}

type preparedBackend struct {
	url               string
	selectedByHelper  bool
	hasHelperResponse bool
	err               error
}

var (
	defaultSession     *Session
	defaultSessionOnce sync.Once
)

// DefaultSession returns the session of the running CLI process. It finds and runs the credential
// helper only when a backend is first prepared.
func DefaultSession() *Session {
	defaultSessionOnce.Do(func() { defaultSession = NewSession(nil) })
	return defaultSession
}

// NewSession returns a session that finds its helper from the environment, the saved configuration, or
// automatic discovery when the first backend is prepared. Helper diagnostics go to stderr.
func NewSession(stderr io.Writer) *Session {
	session := NewSessionWithHelperFunc(nil)
	session.stderr = stderr
	session.find = sync.OnceValue(func() error {
		// Environment configuration overrides the saved helper, so the credentials file is not read for it.
		var saved *workspace.CredentialHelper
		if env.CredentialHelper.Value() == "" {
			var err error
			if saved, err = SavedHelper(); err != nil {
				return err
			}
		}
		resolved, err := credentialhelper.ResolveHelper(credentialhelper.ResolveHelperOptions{Saved: saved})
		if err != nil || resolved == nil {
			return err
		}
		session.resolved = resolved
		session.run = session.executableRunner(resolved)
		return nil
	})
	return session
}

// NewSessionWithHelperFunc returns a session that uses run directly, without helper discovery.
// A nil run disables the helper.
func NewSessionWithHelperFunc(run runHelperFunc) *Session {
	return &Session{
		run:       run,
		helperEnv: map[string]string{},
		prepared:  map[string]preparedBackend{},
		httpAuth:  map[string]*HTTPAuth{},
	}
}

func (s *Session) executableRunner(resolved *credentialhelper.Resolved) runHelperFunc {
	return func(ctx context.Context, request credentialhelper.Request) (*credentialhelper.Response, error) {
		runner := credentialhelper.Runner{Path: resolved.Path, Args: resolved.Args, Stderr: s.stderr}
		return runner.Run(ctx, request)
	}
}

// UseHelper selects an explicitly configured helper before any backends are prepared.
func (s *Session) UseHelper(config workspace.CredentialHelper) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.prepared) != 0 || s.unselected != nil {
		return errors.New("credential helper cannot be changed after preparing a backend")
	}
	resolved, err := credentialhelper.ResolveHelper(credentialhelper.ResolveHelperOptions{Explicit: &config})
	if err != nil {
		return err
	}
	s.resolved = resolved
	s.run = s.executableRunner(resolved)
	s.find = nil
	return nil
}

// Helper finds the configured helper without executing it. Nil means no helper is in use.
func (s *Session) Helper() (*credentialhelper.Resolved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.find != nil {
		if err := s.find(); err != nil {
			return nil, err
		}
	}
	return s.resolved, nil
}

// SetBackendValidator registers a function that checks whether the caller can open a backend URL
// selected by the helper.
func (s *Session) SetBackendValidator(validate func(backendURL string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.validateBackend = validate
}

// PrepareBackend runs the helper for selectedURL, applies its environment variables, and returns the
// normalized URL. selectedURL must be non-empty. the helper cannot select a different backend.
func (s *Session) PrepareBackend(ctx context.Context, selectedURL string) (string, error) {
	return s.prepare(ctx, selectedURL, false)
}

// PrepareBackendWithFallback lets the helper select a backend and applies its environment variables.
// fallbackURL must be non-empty. The helper receives no selected URL. Ff it does not select a backend,
// fallbackURL is used. The returned URL is normalized.
func (s *Session) PrepareBackendWithFallback(ctx context.Context, fallbackURL string) (string, error) {
	return s.prepare(ctx, fallbackURL, true)
}

// SelectedBackend returns the URL selected by the helper, or an empty string if it selected none.
func (s *Session) SelectedBackend() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unselected != nil && s.unselected.err == nil && s.unselected.selectedByHelper {
		return s.unselected.url
	}
	return ""
}

// HasHelperResponse reports whether a helper response was accepted for url.
func (s *Session) HasHelperResponse(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepared[normalizeBackendURL(url)].hasHelperResponse
}

func (s *Session) prepare(ctx context.Context, url string, helperMaySelect bool) (string, error) {
	if url == "" {
		return "", errors.New("a backend URL is required to prepare a backend")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	url = normalizeBackendURL(url)
	if helperMaySelect && s.unselected != nil {
		if s.unselected.err != nil || s.unselected.selectedByHelper {
			return s.unselected.url, s.unselected.err
		}
		// The helper already declined to select a backend, so url is the selection.
		helperMaySelect = false
	}
	if !helperMaySelect {
		if result, ok := s.prepared[url]; ok {
			return result.url, result.err
		}
	}

	request := credentialhelper.Request{Reason: credentialhelper.Initial}
	if !helperMaySelect {
		request.SelectedBackendURL = url
	}
	var response *credentialhelper.Response
	var err error
	if s.find != nil {
		err = s.find()
	}
	if err == nil && s.run != nil {
		logging.V(7).Infof("Invoking credential helper for initial backend preparation")
		response, err = s.run(ctx, request)
		if err == nil && response == nil {
			logging.V(7).Infof("Credential helper declined the backend")
		}
	}
	result := preparedBackend{url: url, hasHelperResponse: response != nil, err: err}
	if helperMaySelect && response != nil && response.BackendURL != "" {
		selected := normalizeBackendURL(response.BackendURL)
		if s.validateBackend != nil {
			err = s.validateBackend(selected)
		}
		if err != nil {
			result, response = preparedBackend{url: url, err: err}, nil
		} else {
			result.url, result.selectedByHelper = selected, true
		}
	}
	// A backend the command already opened by URL keeps the credentials it was given then.
	if earlier, ok := s.prepared[result.url]; ok && err == nil {
		result.hasHelperResponse, result.err, response = earlier.hasHelperResponse, earlier.err, nil
	}
	if response != nil {
		redactHelperCredentials(response)
		result.err = s.applyEnvironment(response.Env)
		if result.err == nil && response.HasHTTPCredentials() && IsHTTPBackend(result.url) {
			s.httpAuth[result.url] = newHTTPAuth(result.url, response, s.run)
		}
	}
	if helperMaySelect {
		s.unselected = &result
	}
	if !helperMaySelect || result.err == nil {
		s.prepared[result.url] = result
	}
	return result.url, result.err
}

// HTTPAuth returns the helper's HTTP credentials for a prepared backend, or nil if it supplied none.
func (s *Session) HTTPAuth(backendURL string) *HTTPAuth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.httpAuth[normalizeBackendURL(backendURL)]
}

func redactHelperCredentials(response *credentialhelper.Response) {
	secrets := []string{response.AccessToken}
	for _, values := range response.Headers {
		secrets = append(secrets, values...)
	}
	for _, value := range response.Env {
		secrets = append(secrets, value)
	}
	logging.AddGlobalSecretFilter(secrets, "[credential]")
}

// applyEnvironment sets helper variables for the CLI and its child processes. Variables the CLI
// inherited win; a variable two helper responses disagree on is an error.
func (s *Session) applyEnvironment(environment map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(environment)) {
		key, value := name, environment[name]
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(name)
		}
		if applied, ok := s.helperEnv[key]; ok {
			if applied != value {
				return fmt.Errorf("credential helpers supplied conflicting values for environment variable %s", name)
			}
			continue
		}
		if _, inherited := os.LookupEnv(name); inherited {
			logging.V(7).Infof("Skipping credential helper environment variable %s: inherited value takes precedence", name)
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("applying credential helper environment variable %s: %w", name, err)
		}
		s.helperEnv[key] = value
		logging.V(7).Infof("Applied credential helper environment variable %s", name)
	}
	return nil
}

// IsHTTPBackend reports whether url names an HTTP backend, such as Pulumi Cloud, rather than a DIY one.
func IsHTTPBackend(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

func normalizeBackendURL(url string) string {
	if IsHTTPBackend(url) {
		return strings.TrimSuffix(url, "/")
	}
	return url
}
