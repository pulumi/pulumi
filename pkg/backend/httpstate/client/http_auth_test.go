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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
	return session.HelperHTTPAuth(backendURL)
}

func TestHTTPAuthRequestScope(t *testing.T) {
	t.Parallel()
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
		updateTokenStaticSource("lease"), httpCallOptions{}))
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

func TestHTTPAuthRefreshError(t *testing.T) {
	t.Parallel()
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
