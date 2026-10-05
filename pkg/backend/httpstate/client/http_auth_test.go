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

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
)

func helperAuth(
	t *testing.T, backendURL string, initial *credentialhelper.Response, refreshErr error,
) *auth.HTTPAuth {
	t.Helper()
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	session := auth.NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		if request.Reason == credentialhelper.Initial {
			return initial, nil
		}
		return nil, refreshErr
	})
	_, err := session.PrepareBackend(t.Context(), backendURL)
	require.NoError(t, err)
	return session.HTTPAuth(backendURL)
}

//nolint:paralleltest // PULUMI_ACCESS_TOKEN is a process-wide environment variable.
func TestHTTPAuthRequestScope(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/api/lease":
			assert.Equal(t, "update-token lease", r.Header.Get("Authorization"))
			assert.Equal(t, "secret", r.Header.Get("X-Gate"))
		case "/presigned", "/policy-pack":
			assert.Empty(t, r.Header.Get("X-Gate"))
			assert.Empty(t, r.Header.Get("Authorization"))
		default:
			assert.Equal(t, "token helper", r.Header.Get("Authorization"))
			assert.Equal(t, "secret", r.Header.Get("X-Gate"))
		}
		_, err := w.Write([]byte(`{}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	a := helperAuth(t, server.URL,
		&credentialhelper.Response{AccessToken: "helper", Headers: http.Header{"X-Gate": {"secret"}}}, nil)
	pc := NewClient(server.URL, "stored", false, diagtest.LogSink(t)).WithHTTPAuth(a)
	require.NoError(t, pc.updateRESTCall(t.Context(), http.MethodPost, "/api/lease", nil, nil, nil,
		updateAccessToken(updateTokenStaticSource("lease")), httpCallOptions{}))
	_, err := pc.GetCapabilities(t.Context())
	require.NoError(t, err)
	_, err = pc.GetTemplate(t.Context(), "private", "publisher", "name", nil)
	require.NoError(t, err)
	body, err := pc.DownloadTemplate(t.Context(), server.URL+"/template.tar.gz")
	require.NoError(t, err)
	require.NoError(t, body.Close())
	body, err = pc.downloadWithRawClient(t.Context(), server.URL+"/presigned?X-Amz-Expires=60")
	require.NoError(t, err)
	require.NoError(t, body.Close())
	body, _, err = pc.DownloadPolicyPack(t.Context(), server.URL+"/policy-pack")
	require.NoError(t, err)
	require.NoError(t, body.Close())

	// API requests that are not sent through the REST call path carry the credentials too.
	events, err := pc.StreamNeoTaskEvents(t.Context(), "org", "task", "")
	require.NoError(t, err)
	// Drain the stream so its goroutine finishes.
	for range events {
	}
	// The fake backend's reply is not a Copilot answer; only the request matters here.
	_, _ = pc.callCopilot(t.Context(), map[string]string{})
	mu.Lock()
	defer mu.Unlock()
	assert.Subset(t, paths, []string{"/api/preview/agents/org/tasks/task/events/stream", "/api/ai/chat/preview"})
}

//nolint:paralleltest // PULUMI_ACCESS_TOKEN is a process-wide environment variable.
func TestHTTPAuthRefreshError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	a := helperAuth(t, server.URL,
		&credentialhelper.Response{Headers: http.Header{"X-Gate": {"secret"}}}, context.DeadlineExceeded)
	pc := NewClient(server.URL, "stored", false, diagtest.LogSink(t)).WithHTTPAuth(a)
	_, err := pc.GetCapabilities(t.Context())
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestHTTPAuthRefreshesOncePerOperation(t *testing.T) {
	var apiCalls, exchanges, helperRefreshes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			exchanges.Add(1)
			fmt.Fprint(w, `{"access_token":"exchanged","refresh_token":"rotated"}`)
			return
		}
		apiCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	session := auth.NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		if request.Reason != credentialhelper.Initial {
			helperRefreshes.Add(1)
		}
		return &credentialhelper.Response{Headers: http.Header{"X-Gate": {"secret"}}}, nil
	})
	_, err := session.PrepareBackend(t.Context(), server.URL)
	require.NoError(t, err)
	pc := NewClient(server.URL, "stored", false, diagtest.LogSink(t)).
		WithRefresh("refresh-token", func(string, time.Time, string) error { return nil }).
		WithHTTPAuth(session.HTTPAuth(server.URL))
	_, err = pc.GetCapabilities(t.Context())
	require.Error(t, err)
	// The first attempt, its retry after the helper refresh, and the retry after the OAuth exchange.
	assert.EqualValues(t, 3, apiCalls.Load())
	assert.EqualValues(t, 1, helperRefreshes.Load())
	assert.EqualValues(t, 1, exchanges.Load())
}
