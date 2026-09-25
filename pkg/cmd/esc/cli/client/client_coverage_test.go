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
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRevisionNumber(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("empty version defaults to latest tag lookup", func(t *testing.T) {
		t.Parallel()

		expected := EnvironmentRevisionTag{Name: "latest", Revision: 42}
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags/latest",
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		rev, err := client.GetRevisionNumber(ctx, "org", "proj", "env", "")
		require.NoError(t, err)
		assert.Equal(t, 42, rev)
	})

	t.Run("numeric version parsed directly", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/unused",
			func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("should not make API call for numeric version")
			})
		rev, err := client.GetRevisionNumber(ctx, "org", "proj", "env", "7")
		require.NoError(t, err)
		assert.Equal(t, 7, rev)
	})

	t.Run("invalid numeric version", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/unused",
			func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("should not make API call")
			})
		_, err := client.GetRevisionNumber(ctx, "org", "proj", "env", "99999999999999999999")
		assert.ErrorContains(t, err, "invalid revision number")
	})

	t.Run("tag version lookup", func(t *testing.T) {
		t.Parallel()

		expected := EnvironmentRevisionTag{Name: "stable", Revision: 10}
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		rev, err := client.GetRevisionNumber(ctx, "org", "proj", "env", "stable")
		require.NoError(t, err)
		assert.Equal(t, 10, rev)
	})

	t.Run("tag not found", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags/missing",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		_, err := client.GetRevisionNumber(ctx, "org", "proj", "env", "missing")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestCloneEnvironment(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		destEnv := CloneEnvironmentRequest{
			Project:         "dest-proj",
			Name:            "dest-env",
			PreserveHistory: true,
		}
		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/clone",
			func(w http.ResponseWriter, r *http.Request) {
				var req CloneEnvironmentRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, destEnv, req)
				w.WriteHeader(http.StatusOK)
			})
		err := client.CloneEnvironment(ctx, "org", "proj", "env", destEnv)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/clone",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		err := client.CloneEnvironment(ctx, "org", "proj", "env", CloneEnvironmentRequest{Name: "x"})
		assert.ErrorContains(t, err, "not found")
	})
}

func TestGetEnvironmentDraft(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		yamlContent := []byte("values:\n  key: val\n")
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/drafts/cr-123",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", "etag-abc")
				_, err := w.Write(yamlContent)
				require.NoError(t, err)
			})
		yaml, etag, err := client.GetEnvironmentDraft(ctx, "org", "proj", "env", "cr-123")
		require.NoError(t, err)
		assert.Equal(t, yamlContent, yaml)
		assert.Equal(t, "etag-abc", etag)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/drafts/cr-bad",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "draft not found"})
				require.NoError(t, err)
			})
		_, _, err := client.GetEnvironmentDraft(ctx, "org", "proj", "env", "cr-bad")
		assert.ErrorContains(t, err, "draft not found")
	})
}

func TestUpdateEnvironmentDraft(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/drafts/cr-123",
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "etag-1", r.Header.Get("If-Match"))
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, `"values:\n  key: val\n"`, string(body))
				err = json.NewEncoder(w).Encode(UpdateEnvironmentDraftResponse{ChangeRequestID: "cr-123"})
				require.NoError(t, err)
			})
		diags, err := client.UpdateEnvironmentDraft(
			ctx, "org", "proj", "env", "cr-123", []byte(`"values:\n  key: val\n"`), "etag-1")
		require.NoError(t, err)
		assert.Nil(t, diags)
	})

	t.Run("diagnostics error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/drafts/cr-123",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				err := json.NewEncoder(w).Encode(EnvironmentErrorResponse{
					Code:    400,
					Message: "bad",
					Diagnostics: []EnvironmentDiagnostic{
						{Summary: "invalid yaml"},
					},
				})
				require.NoError(t, err)
			})
		diags, err := client.UpdateEnvironmentDraft(ctx, "org", "proj", "env", "cr-123", []byte(`bad`), "etag-1")
		require.NoError(t, err)
		require.Len(t, diags, 1)
		assert.Equal(t, "invalid yaml", diags[0].Summary)
	})
}

func TestGetEnvironmentRevision(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		expected := EnvironmentRevision{Number: 5, CreatorLogin: "user"}
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions",
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "6", r.URL.Query().Get("before"))
				assert.Equal(t, "1", r.URL.Query().Get("count"))
				err := json.NewEncoder(w).Encode([]EnvironmentRevision{expected})
				require.NoError(t, err)
			})
		rev, err := client.GetEnvironmentRevision(ctx, "org", "proj", "env", 5)
		require.NoError(t, err)
		require.NotNil(t, rev)
		assert.Equal(t, 5, rev.Number)
	})

	t.Run("not found returns nil", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions",
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode([]EnvironmentRevision{})
				require.NoError(t, err)
			})
		rev, err := client.GetEnvironmentRevision(ctx, "org", "proj", "env", 999)
		require.NoError(t, err)
		assert.Nil(t, rev)
	})
}

func TestListEnvironmentRevisions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		now := time.Now().Truncate(time.Second)
		expected := []EnvironmentRevision{
			{Number: 3, Created: now, CreatorLogin: "alice"},
			{Number: 2, Created: now, CreatorLogin: "bob"},
		}
		before := 4
		count := 2
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions",
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "4", r.URL.Query().Get("before"))
				assert.Equal(t, "2", r.URL.Query().Get("count"))
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		revs, err := client.ListEnvironmentRevisions(ctx, "org", "proj", "env", ListEnvironmentRevisionsOptions{
			Before: &before,
			Count:  &count,
		})
		require.NoError(t, err)
		require.Len(t, revs, 2)
		assert.Equal(t, 3, revs[0].Number)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 500, Message: "server error"})
				require.NoError(t, err)
			})
		_, err := client.ListEnvironmentRevisions(ctx, "org", "proj", "env", ListEnvironmentRevisionsOptions{})
		assert.ErrorContains(t, err, "server error")
	})
}

func TestRetractEnvironmentRevision(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		replacement := 4
		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/versions/5/retract",
			func(w http.ResponseWriter, r *http.Request) {
				var req RetractEnvironmentRevisionRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, &replacement, req.Replacement)
				assert.Equal(t, "bad revision", req.Reason)
				w.WriteHeader(http.StatusOK)
			})
		err := client.RetractEnvironmentRevision(ctx, "org", "proj", "env", "5", &replacement, "bad revision")
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/versions/5/retract",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "revision not found"})
				require.NoError(t, err)
			})
		err := client.RetractEnvironmentRevision(ctx, "org", "proj", "env", "5", nil, "")
		assert.ErrorContains(t, err, "revision not found")
	})
}

func TestCreateEnvironmentRevisionTag(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		revision := 3
		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/versions/tags",
			func(w http.ResponseWriter, r *http.Request) {
				var req CreateEnvironmentRevisionTagRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, "stable", req.Name)
				assert.Equal(t, &revision, req.Revision)
				w.WriteHeader(http.StatusOK)
			})
		err := client.CreateEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable", &revision)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPost, "/api/esc/environments/org/proj/env/versions/tags",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 409, Message: "tag already exists"})
				require.NoError(t, err)
			})
		err := client.CreateEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable", nil)
		assert.ErrorContains(t, err, "tag already exists")
	})
}

func TestGetEnvironmentRevisionTag(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		expected := EnvironmentRevisionTag{Name: "stable", Revision: 5}
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		tag, err := client.GetEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable")
		require.NoError(t, err)
		assert.Equal(t, "stable", tag.Name)
		assert.Equal(t, 5, tag.Revision)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags/missing",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "tag not found"})
				require.NoError(t, err)
			})
		_, err := client.GetEnvironmentRevisionTag(ctx, "org", "proj", "env", "missing")
		assert.ErrorContains(t, err, "tag not found")
	})
}

func TestUpdateEnvironmentRevisionTag(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		revision := 7
		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				var req UpdateEnvironmentRevisionTagRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, &revision, req.Revision)
				w.WriteHeader(http.StatusOK)
			})
		err := client.UpdateEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable", &revision)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		err := client.UpdateEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable", nil)
		assert.ErrorContains(t, err, "not found")
	})
}

func TestDeleteEnvironmentRevisionTag(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodDelete, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		err := client.DeleteEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable")
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodDelete, "/api/esc/environments/org/proj/env/versions/tags/stable",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		err := client.DeleteEnvironmentRevisionTag(ctx, "org", "proj", "env", "stable")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestListEnvironmentRevisionTags(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		expected := ListEnvironmentRevisionTagsResponse{
			Tags: []EnvironmentRevisionTag{
				{Name: "latest", Revision: 10},
				{Name: "stable", Revision: 8},
			},
			NextToken: "next",
		}
		count := 10
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags",
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "prev", r.URL.Query().Get("after"))
				assert.Equal(t, "10", r.URL.Query().Get("count"))
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		tags, err := client.ListEnvironmentRevisionTags(ctx, "org", "proj", "env", ListEnvironmentRevisionTagsOptions{
			After: "prev",
			Count: &count,
		})
		require.NoError(t, err)
		require.Len(t, tags, 2)
		assert.Equal(t, "latest", tags[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/versions/tags",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 500, Message: "server error"})
				require.NoError(t, err)
			})
		_, err := client.ListEnvironmentRevisionTags(ctx, "org", "proj", "env", ListEnvironmentRevisionTagsOptions{})
		assert.ErrorContains(t, err, "server error")
	})
}

func TestEnvironmentExists(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		exists, err := client.EnvironmentExists(ctx, "org", "proj", "env")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		exists, err := client.EnvironmentExists(ctx, "org", "proj", "env")
		assert.Error(t, err)
		assert.False(t, exists)
	})
}

func TestGetEnvironmentSettings(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		expected := EnvironmentSettings{DeletionProtected: true}
		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/settings",
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(expected)
				require.NoError(t, err)
			})
		settings, err := client.GetEnvironmentSettings(ctx, "org", "proj", "env")
		require.NoError(t, err)
		assert.True(t, settings.DeletionProtected)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodGet, "/api/esc/environments/org/proj/env/settings",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 404, Message: "not found"})
				require.NoError(t, err)
			})
		_, err := client.GetEnvironmentSettings(ctx, "org", "proj", "env")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestPatchEnvironmentSettings(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		prot := true
		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/settings",
			func(w http.ResponseWriter, r *http.Request) {
				var req PatchEnvironmentSettingsRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, &prot, req.DeletionProtected)
				w.WriteHeader(http.StatusOK)
			})
		err := client.PatchEnvironmentSettings(ctx, "org", "proj", "env", PatchEnvironmentSettingsRequest{
			DeletionProtected: &prot,
		})
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.MethodPatch, "/api/esc/environments/org/proj/env/settings",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				err := json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 403, Message: "forbidden"})
				require.NoError(t, err)
			})
		err := client.PatchEnvironmentSettings(ctx, "org", "proj", "env", PatchEnvironmentSettingsRequest{})
		assert.ErrorContains(t, err, "forbidden")
	})
}
