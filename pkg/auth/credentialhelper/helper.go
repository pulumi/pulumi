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

// Package credentialhelper runs local credential helpers and keeps their credentials in memory.
//
// Helpers run directly, without a shell. Each invocation reads one JSON request
// from stdin, writes one JSON response to stdout, and exits successfully.
// Stderr carries diagnostic messages. Invocations have a one-minute timeout.
//
// Requests contain version (1), reason ("initial", "expired", or "rejected"),
// and selectedBackendUrl when a backend has already been selected. For example:
//
//	{"version":1,"reason":"initial","selectedBackendUrl":"https://api.pulumi.com"}
//
// An empty response object declines an initial request. Otherwise, responses
// require version:1 and may contain accessToken, headers (a map of string arrays),
// expiresAt (an RFC3339 timestamp), backendUrl, and env (a map of strings):
//
//	{"version":1,"accessToken":"my-token","headers":{"X-Session":["my-session"]}}
//
// Only initial responses may supply backendUrl or env, and backendUrl is allowed
// only when no backend was selected. Refresh responses must supply HTTP credentials
// and preserve whether an access token is supplied. Unknown fields, invalid JSON,
// unsupported versions, and nonzero exit codes are errors.
package credentialhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"golang.org/x/net/http/httpguts"
)

// Session owns helper configuration, initial calls, and environment changes for one command.
type Session struct {
	mu         sync.Mutex
	configured bool
	config     *workspace.CredentialHelper
	source     string
	original   map[string]string
	applied    map[string]string
	results    map[string]*Result
	current    *Result
	outputMu   sync.Mutex
	output     func(string)
}

// NewSession creates a lazy helper session for one command without discovering or running a helper.
func NewSession() *Session {
	original := map[string]string{}
	for _, entry := range os.Environ() {
		k, v, _ := strings.Cut(entry, "=")
		original[environmentKey(k)] = v
	}
	return &Session{original: original, applied: map[string]string{}, results: map[string]*Result{}}
}

// Configure selects a login flag override without executing it.
func (s *Session) Configure(path string, args []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := resolve(path, args)
	if err != nil {
		return err
	}
	s.configured = true
	s.config = c
	s.source = "login flags"
	return nil
}

func resolve(path string, args []string) (*workspace.CredentialHelper, error) {
	p, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("finding credentials helper: %w", err)
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return nil, err
	}
	return &workspace.CredentialHelper{Path: p, Args: args}, nil
}

// Configuration discovers the configured helper without running it.
func (s *Session) Configuration() (*workspace.CredentialHelper, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configuration()
}

func (s *Session) configuration() (*workspace.CredentialHelper, string, error) {
	if s.configured {
		return s.config, s.source, nil
	}
	var path, source string
	var args []string
	if p := env.CredentialHelper.Value(); p != "" {
		if p == "none" {
			s.configured = true
			return nil, "disabled", nil
		}
		path, source = p, "environment configuration"
		if a, ok := env.CredentialHelperArgs.Underlying(); ok {
			if err := json.Unmarshal([]byte(a), &args); err != nil || args == nil {
				return nil, "", fmt.Errorf("%s must be a JSON array of strings", env.CredentialHelperArgs.Var().Name())
			}
		}
	} else {
		creds, err := workspace.GetStoredCredentials()
		if err != nil && workspace.AgentCredentialsFallbackEnabled() {
			agentCreds, agentErr := workspace.GetAgentStoredCredentials()
			if agentErr != nil {
				return nil, "", errors.Join(err, agentErr)
			}
			if !workspace.IsUndecryptableCredentials(err) || agentCreds.Current != "" {
				creds, err = workspace.Credentials{}, nil
			}
		}
		if err != nil {
			return nil, "", err
		}
		if creds.CredentialHelper != nil {
			path, args, source = creds.CredentialHelper.Path, creds.CredentialHelper.Args, "saved configuration"
		}
	}
	if path == "" {
		name := "pulumi-credential-helper"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		executable, err := os.Executable()
		if err != nil {
			return nil, "", err
		}
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return nil, "", err
		}
		beside := filepath.Join(filepath.Dir(executable), name)
		if _, err := os.Stat(beside); err == nil {
			path, source = beside, "beside the CLI"
		} else if !os.IsNotExist(err) {
			return nil, "", err
		} else if p, err := exec.LookPath(name); err == nil {
			path, source = p, "$PATH"
		}
	}
	if path != "" {
		c, err := resolve(path, args)
		if err != nil {
			return nil, "", err
		}
		s.config = c
	}
	s.configured = true
	s.source = source
	return s.config, s.source, nil
}

type request struct {
	Version            int    `json:"version"`
	Reason             string `json:"reason"`
	SelectedBackendURL string `json:"selectedBackendUrl,omitempty"`
}

type response struct {
	Version     *int              `json:"version,omitempty"`
	BackendURL  *string           `json:"backendUrl,omitempty"`
	AccessToken string            `json:"accessToken,omitempty"`
	Headers     http.Header       `json:"headers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	ExpiresAt   *time.Time        `json:"expiresAt,omitempty"`
}

// Result holds the resolved backend and its in-memory HTTP credentials.
type Result struct {
	url             string
	Helper          *workspace.CredentialHelper
	mu              sync.Mutex
	state           *credentialState
	hasToken        bool
	selectedBackend bool
	session         *Session
}

// BackendURL returns the backend selected for this result.
func (r *Result) BackendURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.url
}

// SelectedBackend reports whether the helper supplied the backend URL.
func (r *Result) SelectedBackend() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.selectedBackend
}

// HasHTTP reports whether the helper owns HTTP authentication for this backend.
func (r *Result) HasHTTP() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hasToken || len(r.state.credentials.Headers) > 0
}

// Token returns the current helper access token.
func (r *Result) Token() string {
	return r.snapshot().credentials.AccessToken
}

// PluginEnv snapshots helper HTTP credentials for a newly launched plugin.
func (r *Result) PluginEnv() map[string]string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.hasToken && len(r.state.credentials.Headers) == 0 {
		return nil
	}
	return map[string]string{"PULUMI_API": r.url, "PULUMI_ACCESS_TOKEN": r.state.credentials.AccessToken}
}

// Current returns the explicitly selected backend, or nil if none has been selected.
func (s *Session) Current() *Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Resolve invokes the helper for backendURL and caches the result.
// If backendURL is empty, the helper may supply a backend URL.
func (s *Session) Resolve(ctx context.Context, backendURL string) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.results[strings.TrimSuffix(backendURL, "/")]; ok {
		return r, nil
	}
	c, _, err := s.configuration()
	if err != nil {
		return nil, err
	}
	r := &Result{url: backendURL, Helper: c, session: s, state: &credentialState{}}
	if c != nil {
		resp, err := invoke(ctx, c, request{Version: 1, Reason: "initial", SelectedBackendURL: backendURL}, s)
		if err != nil {
			return nil, err
		}
		if resp.BackendURL != nil {
			if backendURL != "" {
				return nil, errors.New("credentials helper cannot return backendUrl when a backend is selected")
			}
			if err := validateURL(*resp.BackendURL); err != nil {
				return nil, err
			}
			r.url = *resp.BackendURL
			r.selectedBackend = true
			logging.AddGlobalSecretFilter(httputil.URLSecrets(r.url), "[credential]")
		}
		if (resp.AccessToken != "" || len(resp.Headers) > 0) && r.url != "" && !isHTTP(r.url) {
			return nil, errors.New("credentials helper HTTP credentials require an HTTP backend")
		}
		for k, v := range resp.Env {
			if old, ok := s.applied[environmentKey(k)]; ok && old != v {
				return nil, fmt.Errorf("credentials helpers returned conflicting values for %s", k)
			}
		}
		for k, v := range resp.Env {
			s.applied[environmentKey(k)] = v
			if _, ok := s.original[environmentKey(k)]; ok {
				logging.V(7).Infof("Skipping credentials helper environment variable %s", k)
				continue
			}
			if err := os.Setenv(k, v); err != nil {
				return nil, err
			}
			logging.V(7).Infof("Applied credentials helper environment variable %s", k)
		}
		r.state = &credentialState{credentials: resp}
		r.hasToken = resp.AccessToken != ""
	}
	s.results[backendURL] = r
	if r.url != "" {
		s.results[strings.TrimSuffix(r.url, "/")] = r
	}
	return r, nil
}

func isHTTP(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("credentials helper returned an invalid backend URL")
	}
	switch u.Scheme {
	case "http", "https":
		if u.Host != "" {
			return nil
		}
	case "file", "s3", "gs", "azblob", "postgres", "postgresql":
		if u.Host != "" || u.Path != "" {
			return nil
		}
	}
	return errors.New("credentials helper returned an unsupported or malformed backend URL")
}

// SetCurrent saves only the current backend selection.
func SetCurrent(raw string) error {
	c, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}
	c.Current = raw
	c.Version = 1
	return workspace.StoreCredentials(c)
}

// SaveConfiguration persists a credential helper.
func (s *Session) SaveConfiguration() error {
	c, _, err := s.Configuration()
	if err != nil || c == nil {
		return err
	}
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}
	creds.Version = 1
	creds.CredentialHelper = c
	return workspace.StoreCredentials(creds)
}

func invoke(ctx context.Context, c *workspace.CredentialHelper, req request, session *Session) (response, error) {
	logging.V(7).Infof("Invoking credentials helper %s (%s)", c.Path, req.Reason)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	data, _ := json.Marshal(req)
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.WaitDelay = time.Second
	cmd.Stdin = bytes.NewReader(data)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	diagnostics := newDiagnostics(session)
	defer diagnostics.close()
	cmd.Stderr = diagnostics
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return response{}, fmt.Errorf("credentials helper %s timed out: %w", c.Path, ctx.Err())
		}
		if ctx.Err() != nil {
			return response{}, fmt.Errorf("credentials helper %s failed: %w", c.Path, ctx.Err())
		}
		return response{}, fmt.Errorf("credentials helper %s failed: %w", c.Path, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil || fields == nil {
		return response{}, errors.New("credentials helper response must be a JSON object")
	}
	for name, v := range fields {
		switch name {
		case "version", "backendUrl", "accessToken", "headers", "env", "expiresAt":
		default:
			return response{}, errors.New("credentials helper returned an unknown field")
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return response{}, errors.New("credentials helper fields must not be null")
		}

		if name == "env" {
			var values map[string]*string
			if err := json.Unmarshal(v, &values); err != nil {
				return response{}, errors.New("credentials helper env must contain strings")
			}
			for _, value := range values {
				if value == nil {
					return response{}, errors.New("credentials helper env must contain strings")
				}
			}
		}
		if name == "headers" {
			var values map[string][]*string
			if err := json.Unmarshal(v, &values); err != nil {
				return response{}, errors.New("credentials helper headers must contain arrays of strings")
			}
			for _, entries := range values {
				if entries == nil {
					return response{}, errors.New("credentials helper headers must contain arrays of strings")
				}
				for _, entry := range entries {
					if entry == nil {
						return response{}, errors.New("credentials helper headers must contain arrays of strings")
					}
				}
			}
		}
	}
	var resp response
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&resp); err != nil {
		return response{}, errors.New("credentials helper returned invalid JSON or fields")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return response{}, errors.New("credentials helper must return exactly one JSON response")
	}
	if resp.Version != nil && *resp.Version != 1 {
		return response{}, errors.New("unsupported credentials helper protocol version")
	}
	hasFields := resp.BackendURL != nil || resp.AccessToken != "" || len(resp.Headers) > 0 ||
		len(resp.Env) > 0 || resp.ExpiresAt != nil
	if resp.Version == nil && hasFields {
		return response{}, errors.New("credentials helper response requires version 1")
	}
	canonical := http.Header{}
	for k, values := range resp.Headers {
		for _, v := range values {
			canonical.Add(k, v)
		}
		switch strings.ToLower(k) {
		case "authorization", "connection", "content-length", "host", "keep-alive",
			"proxy-connection", "te", "trailer", "transfer-encoding", "upgrade":
			return response{}, fmt.Errorf("credentials helper returned prohibited header %s", k)
		}
		if !httpguts.ValidHeaderFieldName(k) {
			return response{}, errors.New("credentials helper returned invalid header name")
		}
		for _, v := range values {
			if !httpguts.ValidHeaderFieldValue(v) {
				return response{}, errors.New("credentials helper returned invalid header value")
			}
		}
	}
	resp.Headers = canonical
	if !httpguts.ValidHeaderFieldValue(resp.AccessToken) {
		return response{}, errors.New("credentials helper returned invalid access token")
	}
	for k, v := range resp.Env {
		switch environmentKey(k) {
		case "PULUMI_ACCESS_TOKEN", "PULUMI_BACKEND_URL", "PULUMI_API", "PULUMI_CREDENTIAL_HELPER":
			return response{}, fmt.Errorf("credentials helper returned prohibited environment variable %s", k)
		}
		// Names cannot contain '=' and neither names nor values can contain NUL.
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return response{}, errors.New("credentials helper returned invalid environment variable")
		}
	}
	var secrets []string
	if resp.AccessToken != "" {
		secrets = append(secrets, resp.AccessToken)
	}
	for _, vs := range resp.Headers {
		for _, v := range vs {
			if v != "" {
				secrets = append(secrets, v)
			}
		}
	}
	for _, v := range resp.Env {
		if v != "" {
			secrets = append(secrets, v)
		}
	}
	logging.AddGlobalSecretFilter(secrets, "[credential]")
	if resp.BackendURL == nil && resp.AccessToken == "" && len(resp.Headers) == 0 && len(resp.Env) == 0 {
		logging.V(7).Infof("Credentials helper declined")
	}
	return resp, nil
}

// SelectBackend records the selected backend URL and makes the result current without invoking the helper.
func (s *Session) SelectBackend(r *Result, raw string) error {
	if r.HasHTTP() && !isHTTP(raw) {
		return errors.New("credentials helper HTTP credentials require an HTTP backend")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.url == "" {
		r.url = raw
	}
	s.results[strings.TrimSuffix(raw, "/")] = r
	s.current = r
	return nil
}

// Lookup returns credentials already resolved for this backend without running a helper.
func (s *Session) Lookup(raw string) *Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.results[strings.TrimSuffix(raw, "/")]
}

// Contributed reports whether the helper supplied credentials or environment variables.
func (r *Result) Contributed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hasToken || len(r.state.credentials.Headers) > 0 || len(r.state.credentials.Env) > 0
}

// ClearConfiguration removes the saved helper while preserving backend accounts.
func ClearConfiguration() error {
	c, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}
	c.CredentialHelper = nil
	return workspace.StoreCredentials(c)
}

// DisplayDiagnostics routes this backend's helper messages to its interactive display.
func (r *Result) DisplayDiagnostics(send func(string)) func() {
	return r.session.DisplayDiagnostics(send)
}

func environmentKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}
