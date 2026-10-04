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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialshelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--auth-session-helper" {
		request, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "session helper diagnostic")
		err = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"version": 1,
			"env": map[string]string{
				"HELPER_REQUEST": string(request), "HELPER_ARGUMENT": os.Args[2],
				"HELPER_INHERITED": os.Getenv("INHERITED"),
			},
		})
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPrepareBackendSelection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		environment []string
		options     BackendOptions
		helperURL   string
		selected    string
		url         string
		source      BackendSource
	}{
		{
			name: "explicit", environment: []string{"PULUMI_BACKEND_URL=https://env.example.com"},
			options: BackendOptions{
				URL: "https://explicit.example.com/", ProjectURL: "s3://project", CurrentURL: "s3://current",
			},
			selected: "https://explicit.example.com", url: "https://explicit.example.com", source: BackendSourceExplicit,
		},
		{
			name: "environment", environment: []string{"PULUMI_BACKEND_URL=s3://environment"},
			options:  BackendOptions{ProjectURL: "s3://project", CurrentURL: "s3://current"},
			selected: "s3://environment", url: "s3://environment", source: BackendSourceEnvironment,
		},
		{
			name: "project", environment: []string{"PULUMI_API=https://legacy.example.com"},
			options:  BackendOptions{ProjectURL: "s3://project", CurrentURL: "s3://current"},
			selected: "s3://project", url: "s3://project", source: BackendSourceProject,
		},
		{
			name: "current before legacy API", environment: []string{"PULUMI_API=https://legacy.example.com"},
			options:  BackendOptions{CurrentURL: "s3://current"},
			selected: "s3://current", url: "s3://current", source: BackendSourceCurrent,
		},
		{
			name: "legacy API", environment: []string{"PULUMI_API=https://legacy.example.com"},
			selected: "https://legacy.example.com", url: "https://legacy.example.com", source: BackendSourceLegacyAPI,
		},
		{
			name: "helper before default", helperURL: "https://helper.example.com/",
			url: "https://helper.example.com", source: BackendSourceHelper,
		},
		{name: "implicit cloud after decline", url: "https://api.pulumi.com", source: BackendSourceDefault},
		{
			name: "login helper before current", options: BackendOptions{Login: true, CurrentURL: "s3://current"},
			helperURL: "s3://helper", url: "s3://helper", source: BackendSourceHelper,
		},
		{
			name: "login current after decline", options: BackendOptions{Login: true, CurrentURL: "s3://current"},
			url: "s3://current", source: BackendSourceCurrent,
		},
		{
			name: "login environment before helper", environment: []string{"PULUMI_BACKEND_URL=s3://environment"},
			options:  BackendOptions{Login: true, CurrentURL: "s3://current"},
			selected: "s3://environment", url: "s3://environment", source: BackendSourceEnvironment,
		},
		{
			name: "login project before helper", options: BackendOptions{Login: true, ProjectURL: "s3://project"},
			selected: "s3://project", url: "s3://project", source: BackendSourceProject,
		},
		{
			name: "login legacy API before helper", environment: []string{"PULUMI_API=https://legacy.example.com"},
			options:  BackendOptions{Login: true, CurrentURL: "s3://current"},
			selected: "https://legacy.example.com", url: "https://legacy.example.com", source: BackendSourceLegacyAPI,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(SessionOptions{Environment: append([]string{}, tt.environment...)})
			calls := 0
			session.runHelper = func(
				_ context.Context, request credentialshelper.Request, previous *credentialshelper.Response,
			) (*credentialshelper.Response, error) {
				calls++
				assert.Equal(t, credentialshelper.Initial, request.Reason)
				assert.Equal(t, tt.selected, request.SelectedBackendURL)
				assert.Nil(t, previous)
				if tt.helperURL == "" {
					return nil, nil
				}
				return &credentialshelper.Response{BackendURL: tt.helperURL}, nil
			}
			prepared, err := session.Prepare(t.Context(), tt.options)
			require.NoError(t, err)
			assert.Equal(t, tt.url, prepared.URL)
			assert.Equal(t, tt.source, prepared.Source)
			assert.Equal(t, 1, calls)
		})
	}
}

func TestPrepareReusesInitialCredentials(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		options  BackendOptions
		response *credentialshelper.Response
		url      string
	}{
		{name: "selected", options: BackendOptions{URL: "https://api.example.com/"}, url: "https://api.example.com"},
		{
			name: "helper selected", response: &credentialshelper.Response{
				BackendURL: "https://api.example.com", AccessToken: "session-cached-token",
			}, url: "https://api.example.com",
		},
		{name: "declined default", url: "https://api.pulumi.com"},
		{
			name: "declined login", options: BackendOptions{Login: true, CurrentURL: "s3://current"}, url: "s3://current",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(SessionOptions{Environment: []string{}})
			calls := 0
			session.runHelper = func(
				context.Context, credentialshelper.Request, *credentialshelper.Response,
			) (*credentialshelper.Response, error) {
				calls++
				return tt.response, nil
			}
			for _, options := range []BackendOptions{tt.options, tt.options, {URL: tt.url}} {
				prepared, err := session.Prepare(t.Context(), options)
				require.NoError(t, err)
				assert.Equal(t, tt.url, prepared.URL)
				assert.Equal(t, tt.response, prepared.HelperResponse)
				if options.URL == tt.url {
					assert.Equal(t, BackendSourceExplicit, prepared.Source)
				}
			}
			assert.Equal(t, 1, calls)
		})
	}
}

func TestPrepareEnvironmentAcrossBackends(t *testing.T) {
	t.Parallel()
	inherited := []string{"INHERITED=original", "EMPTY="}
	applied := map[string]string{}
	session := NewSession(SessionOptions{
		Environment: inherited,
		Setenv: func(name, value string) error {
			applied[name] = value
			return nil
		},
	})
	inherited[0] = "INHERITED=changed"
	responses := map[string]*credentialshelper.Response{
		"s3://source": {Env: map[string]string{
			"INHERITED": "ignored-source", "EMPTY": "ignored-empty", "PROFILE": "shared-profile", "SOURCE": "source-value",
		}},
		"s3://target": {Env: map[string]string{
			"INHERITED": "ignored-target", "PROFILE": "shared-profile", "TARGET": "target-value",
		}},
		"s3://conflict": {Env: map[string]string{
			"A_NEW": "must-not-be-applied", "PROFILE": "conflicting-profile",
		}},
	}
	calls := map[string]int{}
	session.runHelper = func(
		_ context.Context, request credentialshelper.Request, _ *credentialshelper.Response,
	) (*credentialshelper.Response, error) {
		calls[request.SelectedBackendURL]++
		return responses[request.SelectedBackendURL], nil
	}
	for _, url := range []string{"s3://source", "s3://target", "s3://source", "s3://target"} {
		_, err := session.Prepare(t.Context(), BackendOptions{URL: url})
		require.NoError(t, err)
	}
	assert.Equal(t, map[string]int{"s3://source": 1, "s3://target": 1}, calls)
	assert.Equal(t, map[string]string{
		"PROFILE": "shared-profile", "SOURCE": "source-value", "TARGET": "target-value",
	}, applied)
	prepared, err := session.Prepare(t.Context(), BackendOptions{URL: "s3://conflict"})
	require.ErrorContains(t, err, "conflicting values for environment variable PROFILE")
	assert.NotContains(t, err.Error(), "shared-profile")
	assert.NotContains(t, err.Error(), "conflicting-profile")
	assert.Empty(t, prepared)
	assert.NotContains(t, applied, "A_NEW")
	assert.Equal(t, "shared-profile", applied["PROFILE"])
}

func TestPrepareRetainsPreviouslySelectedCredentials(t *testing.T) {
	t.Parallel()
	for _, initial := range []*credentialshelper.Response{nil, {AccessToken: "previously-selected-session-token"}} {
		name := "credentials"
		if initial == nil {
			name = "decline"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(SessionOptions{Environment: []string{}})
			session.runHelper = func(
				_ context.Context, request credentialshelper.Request, _ *credentialshelper.Response,
			) (*credentialshelper.Response, error) {
				if request.SelectedBackendURL != "" {
					return initial, nil
				}
				return &credentialshelper.Response{
					BackendURL: "https://api.example.com", AccessToken: "must-not-replace-selected-token",
				}, nil
			}
			_, err := session.Prepare(t.Context(), BackendOptions{URL: "https://api.example.com"})
			require.NoError(t, err)
			prepared, err := session.Prepare(t.Context(), BackendOptions{})
			require.NoError(t, err)
			assert.Equal(t, "https://api.example.com", prepared.URL)
			assert.Equal(t, BackendSourceHelper, prepared.Source)
			assert.Equal(t, initial, prepared.HelperResponse)
		})
	}
}

func TestPrepareConcurrentCalls(t *testing.T) {
	t.Parallel()
	session := NewSession(SessionOptions{Environment: []string{}})
	calls := 0
	session.runHelper = func(
		context.Context, credentialshelper.Request, *credentialshelper.Response,
	) (*credentialshelper.Response, error) {
		calls++
		return &credentialshelper.Response{AccessToken: "concurrent-session-token"}, nil
	}
	results := make([]struct {
		prepared PreparedBackend
		err      error
	}, 20)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			results[i].prepared, results[i].err = session.Prepare(
				t.Context(), BackendOptions{URL: "https://api.example.com"})
		})
	}
	wg.Wait()
	for _, result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.prepared.HelperResponse)
		assert.Equal(t, "concurrent-session-token", result.prepared.HelperResponse.AccessToken)
	}
	assert.Equal(t, 1, calls)
}

func TestPrepareDoesNotExposeCachedState(t *testing.T) {
	t.Parallel()
	helper := &credentialshelper.Resolved{
		CredentialHelper: workspace.CredentialHelper{Path: "/unused", Args: []string{"original"}},
		Source:           credentialshelper.SourceSaved,
	}
	session := NewSession(SessionOptions{Environment: []string{}, Helper: helper})
	helper.Args[0] = "changed"
	expiresAt := time.Now().Add(time.Hour)
	session.runHelper = func(
		context.Context, credentialshelper.Request, *credentialshelper.Response,
	) (*credentialshelper.Response, error) {
		return &credentialshelper.Response{
			AccessToken: "cached-session-token", Headers: http.Header{"X-Session": {"cached-session-header"}},
			ExpiresAt: &expiresAt,
		}, nil
	}
	options := BackendOptions{URL: "https://api.example.com"}
	first, err := session.Prepare(t.Context(), options)
	require.NoError(t, err)
	first.Helper.Args[0] = "mutated"
	first.HelperResponse.AccessToken = "mutated"
	first.HelperResponse.Headers.Set("X-Session", "mutated")
	*first.HelperResponse.ExpiresAt = time.Time{}
	second, err := session.Prepare(t.Context(), options)
	require.NoError(t, err)
	assert.Equal(t, "original", second.Helper.Args[0])
	assert.Equal(t, "cached-session-token", second.HelperResponse.AccessToken)
	assert.Equal(t, "cached-session-header", second.HelperResponse.Headers.Get("X-Session"))
	assert.Equal(t, expiresAt, *second.HelperResponse.ExpiresAt)
	assert.Equal(t, "[credential] [credential]", logging.FilterString("cached-session-token cached-session-header"))
}

func TestPrepareFailures(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"helper", "environment"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			failure := errors.New("operation failed")
			session := NewSession(SessionOptions{Environment: []string{}, Setenv: func(string, string) error { return failure }})
			calls := 0
			session.runHelper = func(
				context.Context, credentialshelper.Request, *credentialshelper.Response,
			) (*credentialshelper.Response, error) {
				calls++
				if operation == "helper" {
					return nil, failure
				}
				return &credentialshelper.Response{Env: map[string]string{"PROFILE": "session-failure-profile"}}, nil
			}
			for range 2 {
				prepared, err := session.Prepare(t.Context(), BackendOptions{})
				require.ErrorIs(t, err, failure)
				assert.Empty(t, prepared)
			}
			assert.Equal(t, 1, calls)
		})
	}
	t.Run("canceled", func(t *testing.T) {
		t.Parallel()
		session := NewSession(SessionOptions{Environment: []string{}})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := session.Prepare(ctx, BackendOptions{})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestSessionUsesResolvedHelper(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	require.NoError(t, err)
	argument := "literal argument; $(not-a-shell)"
	helper, err := credentialshelper.Resolve(credentialshelper.ResolveOptions{
		Environment: env.NewEnv(env.MapStore{}),
		Saved: &workspace.CredentialHelper{
			Path: executable, Args: []string{"--auth-session-helper", argument},
		},
	})
	require.NoError(t, err)
	var stderr bytes.Buffer
	applied := map[string]string{}
	inherited := []string{"INHERITED=original"}
	session := NewSession(SessionOptions{
		Environment: inherited, Helper: helper, Stderr: &stderr,
		Setenv: func(name, value string) error { applied[name] = value; return nil },
	})
	inherited[0] = "INHERITED=changed"
	prepared, err := session.Prepare(t.Context(), BackendOptions{URL: "s3://bucket"})
	require.NoError(t, err)
	assert.Equal(t, "s3://bucket", prepared.URL)
	assert.Equal(t, helper, prepared.Helper)
	assert.Equal(t, argument, applied["HELPER_ARGUMENT"])
	assert.Equal(t, "original", applied["HELPER_INHERITED"])
	assert.JSONEq(t, `{"version":1,"reason":"initial","selectedBackendUrl":"s3://bucket"}`, applied["HELPER_REQUEST"])
	assert.Equal(t, "session helper diagnostic\n", stderr.String())
}

func TestPrepareWithoutHelper(t *testing.T) {
	t.Parallel()
	session := NewSession(SessionOptions{Environment: []string{"PULUMI_BACKEND_URL=s3://configured"}})
	prepared, err := session.Prepare(t.Context(), BackendOptions{})
	require.NoError(t, err)
	assert.Equal(t, "s3://configured", prepared.URL)
	assert.Nil(t, prepared.Helper)
	assert.Nil(t, prepared.HelperResponse)
}

func TestSessionAppliesProcessEnvironment(t *testing.T) {
	t.Setenv("PULUMI_SESSION_TEST_ENV", "before")
	session := NewSession(SessionOptions{Environment: []string{}})
	session.runHelper = func(
		context.Context, credentialshelper.Request, *credentialshelper.Response,
	) (*credentialshelper.Response, error) {
		return &credentialshelper.Response{Env: map[string]string{"PULUMI_SESSION_TEST_ENV": "after"}}, nil
	}
	_, err := session.Prepare(t.Context(), BackendOptions{URL: "s3://bucket"})
	require.NoError(t, err)
	assert.Equal(t, "after", os.Getenv("PULUMI_SESSION_TEST_ENV"))
}

func TestPrepareEnvironmentNameCase(t *testing.T) {
	t.Parallel()
	applied := map[string]string{}
	session := NewSession(SessionOptions{
		Environment: []string{"Profile=inherited"},
		Setenv:      func(name, value string) error { applied[name] = value; return nil },
	})
	session.runHelper = func(
		context.Context, credentialshelper.Request, *credentialshelper.Response,
	) (*credentialshelper.Response, error) {
		return &credentialshelper.Response{Env: map[string]string{"PROFILE": "helper-profile"}}, nil
	}
	_, err := session.Prepare(t.Context(), BackendOptions{URL: "s3://bucket"})
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Empty(t, applied)
	} else {
		assert.Equal(t, map[string]string{"PROFILE": "helper-profile"}, applied)
	}
}
