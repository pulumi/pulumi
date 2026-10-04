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
	"fmt"
	"io"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialshelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// BackendSource identifies how a backend URL was selected.
type BackendSource string

const (
	BackendSourceExplicit    BackendSource = "explicit"
	BackendSourceEnvironment BackendSource = "environment"
	BackendSourceProject     BackendSource = "project"
	BackendSourceCurrent     BackendSource = "current"
	BackendSourceLegacyAPI   BackendSource = "legacy-api"
	BackendSourceHelper      BackendSource = "helper"
	BackendSourceDefault     BackendSource = "default"
)

// BackendOptions supplies the command's backend choices before opening a backend.
type BackendOptions struct {
	// URL is an explicit command argument or an interactively selected account URL.
	URL        string
	ProjectURL string
	CurrentURL string
	// Login lets the helper select a backend before falling back to CurrentURL.
	Login bool
}

// PreparedBackend contains the selected URL and the helper's initial credentials.
type PreparedBackend struct {
	URL            string
	Source         BackendSource
	Helper         *credentialshelper.Resolved
	HelperResponse *credentialshelper.Response
}

// SessionOptions configures helper execution and environment application for one CLI invocation.
type SessionOptions struct {
	// Environment is a snapshot of the inherited environment; nil uses os.Environ.
	Environment []string
	// Helper is the previously resolved helper; nil disables helper execution.
	Helper *credentialshelper.Resolved
	Stderr io.Writer
	// Setenv applies helper variables before opening a backend; nil uses os.Setenv.
	Setenv func(string, string) error
}

// Session prepares backends and coordinates helper environment variables within one CLI invocation.
type Session struct {
	mu        sync.Mutex
	inherited map[string]string
	helperEnv map[string]string
	helper    *credentialshelper.Resolved
	setenv    func(string, string) error
	runHelper func(
		context.Context, credentialshelper.Request, *credentialshelper.Response,
	) (*credentialshelper.Response, error)
	initial map[string]initialCredentials
}

type initialCredentials struct {
	url      string
	source   BackendSource
	response *credentialshelper.Response
	err      error
}

// NewSession captures the inherited environment without running a helper or reading stored accounts.
func NewSession(options SessionOptions) *Session {
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
	}
	inherited := make(map[string]string, len(environment))
	for _, entry := range environment {
		if key, value, ok := strings.Cut(entry, "="); ok {
			inherited[environmentKey(key)] = value
		}
	}
	setenv := options.Setenv
	if setenv == nil {
		setenv = os.Setenv
	}
	session := &Session{
		inherited: inherited,
		helperEnv: map[string]string{},
		helper:    cloneHelper(options.Helper),
		setenv:    setenv,
		initial:   map[string]initialCredentials{},
	}
	if session.helper != nil {
		runner := credentialshelper.Runner{
			Path: session.helper.Path, Args: session.helper.Args,
			Env: append([]string{}, environment...), Stderr: options.Stderr,
		}
		session.runHelper = runner.Run
	}
	return session
}

// Prepare selects a backend, invokes its helper once, and applies helper environment variables.
// A helper-selected URL should be saved as current only after the caller successfully opens the backend.
func (s *Session) Prepare(ctx context.Context, options BackendOptions) (PreparedBackend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return PreparedBackend{}, err
	}

	selected, source := s.selectBackend(options)
	selected = normalizeBackendURL(selected)
	result, cached := s.initial[selected]
	if !cached {
		result.url, result.source = selected, source
		if s.runHelper != nil {
			logging.V(7).Infof("Invoking credentials helper for initial backend preparation")
			result.response, result.err = s.runHelper(ctx, credentialshelper.Request{
				Reason: credentialshelper.Initial, SelectedBackendURL: selected,
			}, nil)
		}
		if result.err == nil {
			if selected == "" {
				switch {
				case result.response != nil && result.response.BackendURL != "":
					result.url, result.source = result.response.BackendURL, BackendSourceHelper
				case options.Login && options.CurrentURL != "":
					result.url, result.source = options.CurrentURL, BackendSourceCurrent
				default:
					result.url, result.source = "https://api.pulumi.com", BackendSourceDefault
				}
				result.url = normalizeBackendURL(result.url)
				if previous, prepared := s.initial[result.url]; prepared {
					result.response, result.err = previous.response, previous.err
				}
			}
			if result.err == nil && result.response != nil {
				redactHelperCredentials(result.response)
				result.err = s.applyEnvironment(result.response.Env)
			} else if result.err == nil && s.runHelper != nil {
				logging.V(7).Infof("Credentials helper declined the backend")
			}
		}
		s.initial[selected] = result
		if selected == "" && result.err == nil {
			s.initial[result.url] = result
		}
	}
	if result.err != nil {
		return PreparedBackend{}, result.err
	}
	if selected == "" {
		source = result.source
	}
	return PreparedBackend{
		URL: result.url, Source: source,
		Helper: cloneHelper(s.helper), HelperResponse: cloneHelperResponse(result.response),
	}, nil
}

func (s *Session) selectBackend(options BackendOptions) (string, BackendSource) {
	backendURL, legacyURL := s.inherited["PULUMI_BACKEND_URL"], s.inherited["PULUMI_API"]
	switch {
	case options.URL != "":
		return options.URL, BackendSourceExplicit
	case backendURL != "":
		return backendURL, BackendSourceEnvironment
	case options.ProjectURL != "":
		return options.ProjectURL, BackendSourceProject
	case !options.Login && options.CurrentURL != "":
		return options.CurrentURL, BackendSourceCurrent
	case legacyURL != "":
		return legacyURL, BackendSourceLegacyAPI
	default:
		return "", ""
	}
}

func (s *Session) applyEnvironment(environment map[string]string) error {
	names := slices.Sorted(maps.Keys(environment))
	pending := map[string]string{}
	for _, name := range names {
		key := environmentKey(name)
		if _, inherited := s.inherited[key]; inherited {
			continue
		}
		value := environment[name]
		previous, applied := s.helperEnv[key]
		other, duplicate := pending[key]
		if (applied && previous != value) || (duplicate && other != value) {
			return fmt.Errorf("credentials helpers supplied conflicting values for environment variable %s", name)
		}
		pending[key] = value
	}
	for _, name := range names {
		key := environmentKey(name)
		if _, inherited := s.inherited[key]; inherited {
			logging.V(7).Infof("Skipping credentials helper environment variable %s: inherited value takes precedence", name)
			continue
		}
		if _, applied := s.helperEnv[key]; applied {
			continue
		}
		if err := s.setenv(name, environment[name]); err != nil {
			return fmt.Errorf("applying credentials helper environment variable %s: %w", name, err)
		}
		s.helperEnv[key] = environment[name]
		logging.V(7).Infof("Applied credentials helper environment variable %s", name)
	}
	return nil
}

func environmentKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}

func normalizeBackendURL(url string) string {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return strings.TrimSuffix(url, "/")
	}
	return url
}

func cloneHelper(helper *credentialshelper.Resolved) *credentialshelper.Resolved {
	if helper == nil {
		return nil
	}
	copy := *helper
	copy.Args = slices.Clone(helper.Args)
	return &copy
}

func cloneHelperResponse(response *credentialshelper.Response) *credentialshelper.Response {
	if response == nil {
		return nil
	}
	copy := *response
	copy.Env = maps.Clone(response.Env)
	copy.Headers = response.Headers.Clone()
	if response.ExpiresAt != nil {
		expiresAt := *response.ExpiresAt
		copy.ExpiresAt = &expiresAt
	}
	return &copy
}

func redactHelperCredentials(response *credentialshelper.Response) {
	secrets := []string{response.AccessToken}
	for _, values := range response.Headers {
		secrets = append(secrets, values...)
	}
	for _, value := range response.Env {
		secrets = append(secrets, value)
	}
	logging.AddGlobalSecretFilter(secrets, "[credential]")
}
