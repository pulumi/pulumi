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

package stackenv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const publishedDefinition = "values:\n  a: 1\n"

type fakeService struct {
	mu           sync.Mutex
	requests     []string
	queries      map[string]string
	notFound     bool
	createStatus int
}

func (f *fakeService) newClient(t *testing.T) client.Client {
	f.queries = map[string]string{}
	mux := http.NewServeMux()
	const base = "/api/esc/environments/org/proj/env"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		if f.notFound {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(`{"code":404,"message":"not found"}`))
			require.NoError(t, err)
			return
		}
		w.Header().Set("ETag", "etag-1")
		w.Header().Set("Pulumi-ESC-Revision", "7")
		_, err := w.Write([]byte(publishedDefinition))
		require.NoError(t, err)
	})
	mux.HandleFunc("POST "+base+"/versions/7/open", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"id": "open-published"}))
	})
	mux.HandleFunc("POST "+base+"/drafts", func(w http.ResponseWriter, r *http.Request) {
		if f.createStatus != 0 {
			w.WriteHeader(f.createStatus)
			_, err := fmt.Fprintf(w, `{"code":%d,"message":"forbidden"}`, f.createStatus)
			require.NoError(t, err)
			return
		}
		// A concurrent PATCH may have made the create response report a later revision.
		require.NoError(t, json.NewEncoder(w).Encode(client.CreateEnvironmentDraftResponse{
			ChangeRequestID:      "cr-1",
			LatestRevisionNumber: 1,
		}))
	})
	mux.HandleFunc("POST "+base+"/drafts/cr-1/open", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"id": "open-draft"}))
	})
	mux.HandleFunc("GET "+base+"/open/{id}", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(esc.Environment{
			Properties: map[string]esc.Value{"id": {Value: r.PathValue("id")}},
		}))
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.queries[r.URL.Path] = r.URL.RawQuery
		f.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return client.New("test-agent", server.URL, "test-token", false)
}

func openedID(t *testing.T, e *Evaluation) string {
	t.Helper()
	require.NotNil(t, e.Environment)
	id, ok := e.Environment.Properties["id"].Value.(string)
	require.True(t, ok)
	return id
}

func TestEvaluateUnchanged(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	c := svc.newClient(t)

	got, diags, err := Evaluate(t.Context(), c, "org", "proj", "env", []byte("values:\n    a:   1\n"), time.Hour)
	require.NoError(t, err)
	require.Empty(t, diags)
	assert.Equal(t, 7, got.BaseRevision)
	assert.Empty(t, got.ChangeRequestID)
	assert.Equal(t, "open-published", openedID(t, got))
	assert.Equal(t, []string{
		"GET /api/esc/environments/org/proj/env",
		"POST /api/esc/environments/org/proj/env/versions/7/open",
		"GET /api/esc/environments/org/proj/env/open/open-published",
	}, svc.requests)
}

func TestEvaluateChanged(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	c := svc.newClient(t)

	got, diags, err := Evaluate(t.Context(), c, "org", "proj", "env", []byte("values:\n  a: 2\n"), time.Hour)
	require.NoError(t, err)
	require.Empty(t, diags)
	assert.Equal(t, 7, got.BaseRevision)
	assert.Equal(t, "cr-1", got.ChangeRequestID)
	assert.Equal(t, 0, got.DraftRevision)
	assert.Equal(t, "open-draft", openedID(t, got))
	assert.Contains(t, svc.queries["/api/esc/environments/org/proj/env/drafts/cr-1/open"], "revision=0")
}

func TestEvaluateErrors(t *testing.T) {
	t.Parallel()

	t.Run("plaintext secret", func(t *testing.T) {
		t.Parallel()
		svc := &fakeService{}
		c := svc.newClient(t)
		_, _, err := Evaluate(t.Context(), c, "org", "proj", "env",
			[]byte("values:\n  token:\n    fn::secret: hunter2\n"), time.Hour)
		require.ErrorContains(t, err, "replace plaintext secret at values.token")
		assert.Empty(t, svc.requests)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc := &fakeService{notFound: true}
		c := svc.newClient(t)
		_, _, err := Evaluate(t.Context(), c, "org", "proj", "env", []byte(publishedDefinition), time.Hour)
		require.ErrorContains(t, err, "managed environment proj/env was not found; the stack cannot load its config")
	})

	t.Run("draft create fails", func(t *testing.T) {
		t.Parallel()
		svc := &fakeService{createStatus: http.StatusForbidden}
		c := svc.newClient(t)
		_, _, err := Evaluate(t.Context(), c, "org", "proj", "env", []byte("values:\n  a: 2\n"), time.Hour)
		require.ErrorContains(t, err, "creating a draft of managed environment proj/env: ")
		var apiErr *client.EnvironmentErrorResponse
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusForbidden, apiErr.Code)
	})
}

func TestDefinitionsEqual(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		local     string
		published string
		want      bool
	}{
		{"identical", "values:\n  a: 1\n", "values:\n  a: 1\n", true},
		{"indentation", "values:\n    a: 1\n", "values:\n  a: 1\n", true},
		{
			"ciphertext kept", "values:\n  s:\n    fn::secret:\n      ciphertext: abc\n",
			"values:\n  s:\n    fn::secret:\n      ciphertext: abc\n", true,
		},
		{"value change", "values:\n  a: 2\n", "values:\n  a: 1\n", false},
		{"comment change", "# note\nvalues:\n  a: 1\n", "values:\n  a: 1\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DefinitionsEqual([]byte(tc.local), []byte(tc.published))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
