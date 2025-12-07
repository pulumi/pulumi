// Copyright 2025, Pulumi Corporation.
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

package gitlfs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// Authenticator provides authentication for LFS requests
type Authenticator interface {
	// Authenticate adds authentication to an HTTP request
	Authenticate(req *http.Request) error
}

// TokenAuth uses a bearer token for authentication
type TokenAuth struct {
	token string
}

// NewTokenAuth creates a new TokenAuth
func NewTokenAuth(token string) *TokenAuth {
	return &TokenAuth{token: token}
}

// Authenticate implements Authenticator
func (a *TokenAuth) Authenticate(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// BasicAuth uses username/password for authentication
type BasicAuth struct {
	username string
	password string
}

// NewBasicAuth creates a new BasicAuth
func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{
		username: username,
		password: password,
	}
}

// Authenticate implements Authenticator
func (a *BasicAuth) Authenticate(req *http.Request) error {
	credentials := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
	req.Header.Set("Authorization", "Basic "+credentials)
	return nil
}

// GitCredentialAuth uses git-credential helper for authentication
type GitCredentialAuth struct {
	host     string
	protocol string
	path     string

	// Cached credentials
	username string
	password string
	cached   bool
}

// NewGitCredentialAuth creates a new GitCredentialAuth
func NewGitCredentialAuth(host, protocol, path string) *GitCredentialAuth {
	if protocol == "" {
		protocol = "https"
	}
	return &GitCredentialAuth{
		host:     host,
		protocol: protocol,
		path:     path,
	}
}

// Authenticate implements Authenticator
func (a *GitCredentialAuth) Authenticate(req *http.Request) error {
	if !a.cached {
		if err := a.fetchCredentials(); err != nil {
			return fmt.Errorf("git credential fill: %w", err)
		}
	}

	if a.username != "" && a.password != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
		req.Header.Set("Authorization", "Basic "+credentials)
	}

	return nil
}

// fetchCredentials fetches credentials from git-credential helper
func (a *GitCredentialAuth) fetchCredentials() error {
	// Build the credential input
	input := fmt.Sprintf("protocol=%s\nhost=%s\n", a.protocol, a.host)
	if a.path != "" {
		input += fmt.Sprintf("path=%s\n", a.path)
	}
	input += "\n"

	// Run git credential fill
	cmd := exec.Command("git", "credential", "fill")
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.Output()
	if err != nil {
		// If git credential fails, it might just mean no credentials are stored
		// This is not necessarily an error - we can proceed without auth
		a.cached = true
		return nil
	}

	// Parse the output
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "username":
			a.username = value
		case "password":
			a.password = value
		}
	}

	a.cached = true
	return nil
}

// NoAuth provides no authentication
type NoAuth struct{}

// Authenticate implements Authenticator
func (a *NoAuth) Authenticate(req *http.Request) error {
	return nil
}

// NewAuthenticator creates an Authenticator based on available credentials
// Priority:
//  1. Environment token (PULUMI_DIY_BACKEND_GITLFS_TOKEN or PULUMI_GITLFS_TOKEN)
//  2. Environment username/password
//  3. Git credential helper
func NewAuthenticator(ctx context.Context, host, path string) (Authenticator, error) {
	// Check for bearer token in environment
	token := getEnvWithFallback("PULUMI_DIY_BACKEND_GITLFS_TOKEN", "PULUMI_GITLFS_TOKEN")
	if token != "" {
		return NewTokenAuth(token), nil
	}

	// Check for username/password in environment
	username := getEnvWithFallback("PULUMI_DIY_BACKEND_GITLFS_USERNAME", "PULUMI_GITLFS_USERNAME")
	password := getEnvWithFallback("PULUMI_DIY_BACKEND_GITLFS_PASSWORD", "PULUMI_GITLFS_PASSWORD")
	if username != "" && password != "" {
		return NewBasicAuth(username, password), nil
	}

	// Fall back to git credential helper
	return NewGitCredentialAuth(host, "https", path), nil
}

// getEnvWithFallback returns the first non-empty environment variable value
func getEnvWithFallback(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// AuthFromGitConfig attempts to get auth from git config for a specific remote
func AuthFromGitConfig(ctx context.Context, remote string) (Authenticator, error) {
	// Try to get the credential helper from git config
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "credential.helper")
	output, err := cmd.Output()
	if err != nil {
		// No credential helper configured, try git credential fill directly
		return NewGitCredentialAuth("", "https", ""), nil
	}

	helper := strings.TrimSpace(string(output))
	if helper == "" {
		return &NoAuth{}, nil
	}

	// Use git credential with the configured helper
	return NewGitCredentialAuth("", "https", ""), nil
}
