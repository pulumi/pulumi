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
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/authtest"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const testBackendURL = "https://backend.example.com"

func TestMain(m *testing.M) {
	if code, ran := authtest.RunHelper(); ran {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// unsetEnvironment removes variables a helper is about to apply, and restores them when the test ends.
func unsetEnvironment(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		ptesting.Unsetenv(t, name)
	}
}

func TestPrepareSelection(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name            string
		url             string
		helperMaySelect bool
		response        *credentialhelper.Response
		wantURL         string
		wantSelected    string
		wantResponse    bool
		httpAuth        bool
	}{
		{
			name: "selected", url: "https://selected.example.com/",
			response: &credentialhelper.Response{AccessToken: "selected-token"},
			wantURL:  "https://selected.example.com", wantResponse: true, httpAuth: true,
		},
		{name: "selected and declined", url: "s3://selected", wantURL: "s3://selected"},
		{
			name: "helper selects", url: "https://api.pulumi.com", helperMaySelect: true,
			response: &credentialhelper.Response{BackendURL: "https://helper.example.com/"},
			wantURL:  "https://helper.example.com", wantSelected: "https://helper.example.com", wantResponse: true,
		},
		{name: "default after decline", url: "s3://current", helperMaySelect: true, wantURL: "s3://current"},
		{
			name: "credentials for the default", url: "https://api.pulumi.com/", helperMaySelect: true,
			response: &credentialhelper.Response{Headers: http.Header{"X-Gate": {"default-header"}}},
			wantURL:  "https://api.pulumi.com", wantResponse: true, httpAuth: true,
		},
		{
			name: "token for a DIY backend is unused", url: "s3://selected",
			response: &credentialhelper.Response{AccessToken: "unused-token"},
			wantURL:  "s3://selected", wantResponse: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			session := NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				calls++
				assert.Equal(t, credentialhelper.Initial, request.Reason)
				if tt.helperMaySelect {
					assert.Empty(t, request.SelectedBackendURL)
				} else {
					assert.Equal(t, tt.wantURL, request.SelectedBackendURL)
				}
				return tt.response, nil
			})
			prepare := session.PrepareBackend
			if tt.helperMaySelect {
				prepare = session.PrepareBackendWithFallback
			}
			// Repeated preparation reuses the cached helper result.
			for range 2 {
				prepared, err := prepare(t.Context(), tt.url)
				require.NoError(t, err)
				assert.Equal(t, tt.wantURL, prepared)
			}
			// Opening the backend by its URL afterwards does not run the helper again.
			prepared, err := session.PrepareBackend(t.Context(), tt.wantURL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, prepared)
			assert.Equal(t, 1, calls)
			assert.Equal(t, tt.httpAuth, session.HTTPAuth(tt.wantURL) != nil)
			assert.Equal(t, tt.wantSelected, session.SelectedBackend())
			assert.Equal(t, tt.wantResponse, session.HasHelperResponse(tt.wantURL))
			if IsHTTPBackend(tt.wantURL) {
				assert.Equal(t, tt.wantResponse, session.HasHelperResponse(tt.wantURL+"/"))
			}
		})
	}
}

func TestPrepareAnotherDefaultAfterDecline(t *testing.T) {
	t.Parallel()
	var selections []string
	session := NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		selections = append(selections, request.SelectedBackendURL)
		return nil, nil
	})
	for _, url := range []string{"s3://first", "s3://second"} {
		prepared, err := session.PrepareBackendWithFallback(t.Context(), url)
		require.NoError(t, err)
		assert.Equal(t, url, prepared)
	}
	assert.Equal(t, []string{"", "s3://second"}, selections)
}

func TestPrepareKeepsEarlierResult(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	session := NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		if request.SelectedBackendURL != "" {
			return &credentialhelper.Response{AccessToken: "selected-token"}, nil
		}
		return &credentialhelper.Response{BackendURL: testBackendURL, AccessToken: "must-not-replace-token"}, nil
	})
	_, err := session.PrepareBackend(t.Context(), testBackendURL)
	require.NoError(t, err)
	prepared, err := session.PrepareBackendWithFallback(t.Context(), "https://api.pulumi.com")
	require.NoError(t, err)
	assert.Equal(t, testBackendURL, prepared)
	assert.Equal(t, prepared, session.SelectedBackend())
	assert.True(t, session.HasHelperResponse(prepared))
	assert.Equal(t, "selected-token", session.HTTPAuth(testBackendURL).AccessToken())
}

//nolint:paralleltest // The test checks a process-wide environment variable.
func TestPrepareRejectsSelection(t *testing.T) {
	unsetEnvironment(t, "HELPER_TEST_REJECTED")
	rejected := errors.New("unsupported backend")
	session := NewSessionWithHelperFunc(func(
		context.Context, credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		return &credentialhelper.Response{
			BackendURL: "unsupported://backend", Env: map[string]string{"HELPER_TEST_REJECTED": "applied"},
		}, nil
	})
	session.SetBackendValidator(func(string) error { return rejected })
	prepared, err := session.PrepareBackendWithFallback(t.Context(), "https://api.pulumi.com")
	require.ErrorIs(t, err, rejected)
	assert.Equal(t, "https://api.pulumi.com", prepared)
	assert.Empty(t, session.SelectedBackend())
	assert.False(t, session.HasHelperResponse(prepared))
	assert.Empty(t, os.Getenv("HELPER_TEST_REJECTED"))
}

func TestPrepareRequiresURL(t *testing.T) {
	t.Parallel()
	_, err := NewSessionWithHelperFunc(nil).PrepareBackendWithFallback(t.Context(), "")
	require.Error(t, err)
	_, err = NewSessionWithHelperFunc(nil).PrepareBackend(t.Context(), "")
	require.Error(t, err)
}

func TestPrepareEnvironmentAcrossBackends(t *testing.T) {
	t.Setenv("HELPER_TEST_INHERITED", "original")
	t.Setenv("HELPER_TEST_EMPTY", "")
	unsetEnvironment(t, "HELPER_TEST_PROFILE", "HELPER_TEST_SOURCE", "HELPER_TEST_TARGET")
	responses := map[string]*credentialhelper.Response{
		"s3://source": {Env: map[string]string{
			"HELPER_TEST_INHERITED": "ignored-source", "HELPER_TEST_EMPTY": "ignored-empty",
			"HELPER_TEST_PROFILE": "shared-profile", "HELPER_TEST_SOURCE": "source-value",
		}},
		"s3://target": {Env: map[string]string{
			"HELPER_TEST_INHERITED": "ignored-target", "HELPER_TEST_PROFILE": "shared-profile",
			"HELPER_TEST_TARGET": "target-value",
		}},
		"s3://conflict": {Env: map[string]string{"HELPER_TEST_PROFILE": "conflicting-profile"}},
	}
	session := NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		return responses[request.SelectedBackendURL], nil
	})
	for _, url := range []string{"s3://source", "s3://target"} {
		_, err := session.PrepareBackend(t.Context(), url)
		require.NoError(t, err)
	}
	for name, value := range map[string]string{
		"HELPER_TEST_INHERITED": "original", "HELPER_TEST_EMPTY": "", "HELPER_TEST_PROFILE": "shared-profile",
		"HELPER_TEST_SOURCE": "source-value", "HELPER_TEST_TARGET": "target-value",
	} {
		assert.Equal(t, value, os.Getenv(name), name)
	}
	prepared, err := session.PrepareBackend(t.Context(), "s3://conflict")
	require.ErrorContains(t, err, "conflicting values for environment variable HELPER_TEST_PROFILE")
	assert.NotContains(t, err.Error(), "shared-profile")
	assert.NotContains(t, err.Error(), "conflicting-profile")
	assert.Equal(t, "s3://conflict", prepared)
	assert.True(t, session.HasHelperResponse(prepared))
	assert.Equal(t, "shared-profile", os.Getenv("HELPER_TEST_PROFILE"))
	// Environment values can be credentials, such as a cloud provider's secret key.
	assert.Equal(t, "[credential] [credential]", logging.FilterString("source-value conflicting-profile"))
}

func TestPrepareEnvironmentNameCase(t *testing.T) {
	unsetEnvironment(t, "HELPER_TEST_CASE")
	t.Setenv("Helper_Test_Case", "inherited")
	session := NewSessionWithHelperFunc(func(
		context.Context, credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		return &credentialhelper.Response{Env: map[string]string{"HELPER_TEST_CASE": "helper"}}, nil
	})
	_, err := session.PrepareBackend(t.Context(), "s3://bucket")
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Equal(t, "inherited", os.Getenv("HELPER_TEST_CASE"))
	} else {
		assert.Equal(t, "helper", os.Getenv("HELPER_TEST_CASE"))
	}
}

func TestPrepareConcurrentCalls(t *testing.T) {
	t.Parallel()
	calls := 0
	session := NewSessionWithHelperFunc(func(
		context.Context, credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		calls++
		return &credentialhelper.Response{AccessToken: "concurrent-session-token"}, nil
	})
	results := make([]struct {
		url string
		err error
	}, 20)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			results[i].url, results[i].err = session.PrepareBackend(t.Context(), testBackendURL)
		})
	}
	wg.Wait()
	for _, result := range results {
		require.NoError(t, result.err)
		assert.Equal(t, testBackendURL, result.url)
	}
	assert.True(t, session.HasHelperResponse(testBackendURL))
	assert.Equal(t, 1, calls)
}

func TestPrepareRemembersFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"selected", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			failure := errors.New("helper failed")
			calls := 0
			session := NewSessionWithHelperFunc(func(
				context.Context, credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				calls++
				return nil, failure
			})
			prepare := session.PrepareBackend
			if mode == "fallback" {
				prepare = session.PrepareBackendWithFallback
			}
			for range 2 {
				_, err := prepare(t.Context(), testBackendURL)
				require.ErrorIs(t, err, failure)
			}
			assert.Equal(t, 1, calls)
		})
	}
}

func TestPrepareRedactsHTTPCredentials(t *testing.T) {
	t.Parallel()
	session := NewSessionWithHelperFunc(func(
		context.Context, credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		return &credentialhelper.Response{
			AccessToken: "redacted-session-token", Headers: http.Header{"X-Session": {"redacted-session-header"}},
		}, nil
	})
	_, err := session.PrepareBackend(t.Context(), testBackendURL)
	require.NoError(t, err)
	assert.Equal(t, "[credential] [credential]", logging.FilterString("redacted-session-token redacted-session-header"))
}

func TestSessionWithoutHelper(t *testing.T) {
	t.Parallel()
	session := NewSessionWithHelperFunc(nil)
	prepared, err := session.PrepareBackendWithFallback(t.Context(), "https://api.pulumi.com")
	require.NoError(t, err)
	assert.Equal(t, "https://api.pulumi.com", prepared)
	assert.Empty(t, session.SelectedBackend())
	assert.False(t, session.HasHelperResponse(prepared))
	assert.Nil(t, session.HTTPAuth("https://api.pulumi.com"))
	helper, err := session.Helper()
	require.NoError(t, err)
	assert.Nil(t, helper)
}

//nolint:paralleltest // The helper applies process-wide environment variables.
func TestSessionRunsConfiguredHelper(t *testing.T) {
	unsetEnvironment(t, "HELPER_APPLIED")
	fake := authtest.NewHelper(t, map[string]any{
		"initial": authtest.Reply{
			Response: map[string]any{"version": 1, "env": map[string]string{"HELPER_APPLIED": "yes"}},
			Stderr:   "session helper diagnostic\n",
		},
	})
	var stderr bytes.Buffer
	session := NewSession(&stderr)
	config := workspace.CredentialHelper{Path: fake.Path, Args: fake.Args}
	require.NoError(t, session.UseHelper(config))
	helper, err := session.Helper()
	require.NoError(t, err)
	require.NotNil(t, helper)
	prepared, err := session.PrepareBackend(t.Context(), "s3://bucket")
	require.NoError(t, err)
	assert.Equal(t, "s3://bucket", prepared)
	assert.True(t, session.HasHelperResponse(prepared))
	assert.Equal(t, "yes", os.Getenv("HELPER_APPLIED"))
	assert.Equal(t, "session helper diagnostic\n", stderr.String())
	require.ErrorContains(t, session.UseHelper(config), "cannot be changed after preparing a backend")
}

func TestSessionFindsSavedHelperWithoutExecuting(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
	path := filepath.Join(t.TempDir(), "helper")
	require.NoError(t, os.WriteFile(path, []byte("not an executable program"), 0o700)) //nolint:gosec // Discovery fixture.
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
		CredentialHelper: &workspace.CredentialHelper{Path: path},
	}))
	helper, err := NewSession(io.Discard).Helper()
	require.NoError(t, err)
	resolvedPath, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	require.NotNil(t, helper)
	assert.Equal(t, resolvedPath, helper.Path)
	assert.Equal(t, credentialhelper.SourceSaved, helper.Source)
}

func TestSessionDisabledHelper(t *testing.T) {
	ptesting.IsolateCredentials(t)
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
		CredentialHelper: &workspace.CredentialHelper{Path: filepath.Join(t.TempDir(), "missing-helper")},
	}))
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
	session := NewSession(io.Discard)
	helper, err := session.Helper()
	require.NoError(t, err)
	assert.Nil(t, helper)
	prepared, err := session.PrepareBackend(t.Context(), testBackendURL)
	require.NoError(t, err)
	assert.Equal(t, testBackendURL, prepared)
	assert.Empty(t, session.SelectedBackend())
	assert.False(t, session.HasHelperResponse(prepared))
}
