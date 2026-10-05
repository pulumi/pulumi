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

// Session discovers and runs an optional credential helper, sharing its results across one CLI invocation.
// Callers resolve account authentication when the helper supplies no HTTP credentials.
type Session struct {
	mu sync.Mutex
	// discoverHelper initializes runHelper lazily. It is nil when runHelper was supplied directly.
	discoverHelper func() error
	runHelper      runHelperFunc
	stderr         io.Writer
	// helperEnv holds the variables applied so far, to detect conflicts between backends.
	helperEnv map[string]string
	// prepared caches each backend's preparation result, including when no helper is available.
	prepared map[string]preparedBackend
	// unselected is the outcome of the request that let the helper select a backend.
	unselected *preparedBackend
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

// DefaultSession returns the session of the running CLI process, with lazy helper discovery shared across backends.
func DefaultSession() *Session {
	defaultSessionOnce.Do(func() { defaultSession = NewSession(nil) })
	return defaultSession
}

// NewSession returns a session that finds its helper from the environment, the saved configuration, or
// automatic discovery when the first backend is prepared. Helper diagnostics go to stderr.
func NewSession(stderr io.Writer) *Session {
	session := NewSessionWithHelperFunc(nil)
	session.stderr = stderr
	session.discoverHelper = sync.OnceValue(func() error {
		resolved, err := FindHelper()
		if err != nil || resolved == nil {
			return err
		}
		session.runHelper = session.executableRunner(resolved)
		return nil
	})
	return session
}

// FindHelper resolves the configured helper without executing it.
func FindHelper() (*credentialhelper.Resolved, error) {
	var saved *workspace.CredentialHelper
	if env.CredentialHelper.Value() == "" {
		var err error
		if saved, err = SavedHelper(); err != nil {
			return nil, err
		}
	}
	return credentialhelper.ResolveHelper(credentialhelper.ResolveHelperOptions{Saved: saved})
}

// NewSessionWithHelperFunc returns a session that uses run directly, without helper discovery.
// A nil run disables the helper.
func NewSessionWithHelperFunc(run runHelperFunc) *Session {
	return &Session{
		runHelper: run,
		helperEnv: map[string]string{},
		prepared:  map[string]preparedBackend{},
	}
}

func (s *Session) executableRunner(resolved *credentialhelper.Resolved) runHelperFunc {
	return func(ctx context.Context, request credentialhelper.Request) (*credentialhelper.Response, error) {
		runner := credentialhelper.Runner{Path: resolved.Path, Args: resolved.Args, Stderr: s.stderr}
		return runner.Run(ctx, request)
	}
}

// PrepareBackend runs the helper if available, applies its environment variables, and returns the normalized URL.
// selectedURL must be non-empty; the helper cannot select a different backend.
func (s *Session) PrepareBackend(ctx context.Context, selectedURL string) (string, error) {
	return s.prepare(ctx, selectedURL, false, nil)
}

// SelectBackend asks the optional helper to select a backend, validating its URL before applying its environment.
// It returns the selected URL, or fallbackURL (which must be non-empty) if the helper selects none.
func (s *Session) SelectBackend(
	ctx context.Context, fallbackURL string, validate func(string) error,
) (string, error) {
	return s.prepare(ctx, fallbackURL, true, validate)
}

func (s *Session) prepare(
	ctx context.Context, url string, helperMaySelect bool, validate func(string) error,
) (string, error) {
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
	if s.discoverHelper != nil {
		err = s.discoverHelper()
	}
	if err == nil && s.runHelper != nil {
		logging.V(7).Infof("Invoking credential helper for initial backend preparation")
		response, err = s.runHelper(ctx, request)
		if err == nil && response == nil {
			logging.V(7).Infof("Credential helper declined the backend")
		}
	}
	result := preparedBackend{url: url, hasHelperResponse: response != nil, err: err}
	if helperMaySelect && response != nil && response.BackendURL != "" {
		selected := normalizeBackendURL(response.BackendURL)
		if validate != nil {
			err = validate(selected)
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
	}
	if helperMaySelect {
		s.unselected = &result
	}
	if !helperMaySelect || result.err == nil {
		s.prepared[result.url] = result
	}
	return result.url, result.err
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

func normalizeBackendURL(url string) string {
	if IsHTTPBackend(url) {
		return strings.TrimSuffix(url, "/")
	}
	return url
}
