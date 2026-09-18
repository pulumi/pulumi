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

package httpstate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestSetupPerStackSnapshotsAbortsOnSnapshotLoadError is the regression test for
// SetupPerStackSnapshots failing open: a snapshot-load failure (authorization, decryption,
// transport, corrupt checkpoint) for ANY member must abort the whole setup -- calling cleanup()
// and returning a wrapped error naming the failing stack's FQN -- rather than continuing with a
// nil snapshot for that member, which would make the update run as though the stack were empty.
func TestSetupPerStackSnapshotsAbortsOnSnapshotLoadError(t *testing.T) {
	t.Parallel()

	emptyDeployment, err := stack.SerializeUntypedDeployment(t.Context(), &deploy.Snapshot{}, nil)
	require.NoError(t, err)

	var completedUpdates []struct {
		stack  string
		status apitype.UpdateStatus
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/capabilities":
			require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
		case r.URL.Path == "/api/user":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"githubLogin":   "owner",
				"organizations": []map[string]string{},
			}))
		case r.URL.Path == "/api/user/organizations/default":
			require.NoError(t, json.NewEncoder(w).Encode(apitype.GetDefaultOrganizationResponse{GitHubLogin: "owner"}))
		case r.URL.Path == "/api/stacks/owner/project" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{}"))
			require.NoError(t, err)
		case r.URL.Path == "/api/stacks/owner/project/stack1/update" && r.Method == http.MethodPost:
			require.NoError(t, json.NewEncoder(w).Encode(apitype.UpdateProgramResponse{UpdateID: "update-1"}))
		case r.URL.Path == "/api/stacks/owner/project/stack2/update" && r.Method == http.MethodPost:
			require.NoError(t, json.NewEncoder(w).Encode(apitype.UpdateProgramResponse{UpdateID: "update-2"}))
		case r.URL.Path == "/api/stacks/owner/project/stack1/update/update-1" && r.Method == http.MethodPost:
			// A real lease token (renewed on an 8th-of-5-minutes ticker, far outside this test's
			// runtime, so no renewal call happens) so completeUpdate has a real tokenSource to
			// close during cleanup(), rather than the nil one an empty token would leave behind.
			require.NoError(t, json.NewEncoder(w).Encode(apitype.StartUpdateResponse{Version: 1, Token: "lease-1"}))
		case r.URL.Path == "/api/stacks/owner/project/stack2/update/update-2" && r.Method == http.MethodPost:
			require.NoError(t, json.NewEncoder(w).Encode(apitype.StartUpdateResponse{Version: 1, Token: "lease-2"}))
		case r.URL.Path == "/api/stacks/owner/project/stack1/export":
			// stack1 loads successfully.
			require.NoError(t, json.NewEncoder(w).Encode(apitype.ExportStackResponse(*emptyDeployment)))
		case r.URL.Path == "/api/stacks/owner/project/stack2/export":
			// stack2's snapshot load fails: an authorization/transport/corrupt-checkpoint failure.
			http.Error(w, "internal error exporting stack2", http.StatusInternalServerError)
		case r.URL.Path == "/api/stacks/owner/project/stack1/update/update-1/complete" && r.Method == http.MethodPost:
			completedUpdates = append(completedUpdates, struct {
				stack  string
				status apitype.UpdateStatus
			}{"stack1", readCompleteStatus(t, r)})
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/stacks/owner/project/stack2/update/update-2/complete" && r.Method == http.MethodPost:
			completedUpdates = append(completedUpdates, struct {
				stack  string
				status apitype.UpdateStatus
			}{"stack2", readCompleteStatus(t, r)})
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	ctx := t.Context()
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	be, err := New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)
	b, ok := be.(*cloudBackend)
	require.True(t, ok)

	ref1, err := b.ParseStackReference("owner/project/stack1")
	require.NoError(t, err)
	stack1, err := b.CreateStack(ctx, ref1, "", nil, nil)
	require.NoError(t, err)

	ref2, err := b.ParseStackReference("owner/project/stack2")
	require.NoError(t, err)
	stack2, err := b.CreateStack(ctx, ref2, "", nil, nil)
	require.NoError(t, err)

	sm := b64.NewBase64SecretsManager()
	opFor := func(root string) backend.UpdateOperation {
		return backend.UpdateOperation{
			Proj: &workspace.Project{
				Name:    "project",
				Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
			},
			Root: root,
			M:    &backend.UpdateMetadata{},
			Opts: backend.UpdateOptions{
				Display: display.Options{Color: colors.Never, Stdout: io.Discard, Stderr: io.Discard},
			},
			StackConfiguration: backend.StackConfiguration{Config: config.Map{}, Decrypter: sm.Decrypter()},
			SecretsManager:     sm,
			SecretsProvider:    b64.Base64SecretsProvider,
		}
	}

	entries := []backend.MultistackEntry{
		{Stack: stack1, Op: opFor(t.TempDir())},
		{Stack: stack2, Op: opFor(t.TempDir())},
	}

	managers, snapshots, complete, setupErr := b.SetupPerStackSnapshots(ctx, apitype.UpdateUpdate, entries, false)

	require.Error(t, setupErr, "a snapshot-load failure for any member must abort the whole setup")
	assert.Contains(t, setupErr.Error(), "stack2")
	assert.Nil(t, managers)
	assert.Nil(t, snapshots)
	assert.Nil(t, complete)

	// stack1's update, which had already started, must have been completed as failed rather than
	// left dangling -- this is what cleanup() does.
	require.Len(t, completedUpdates, 1)
	assert.Equal(t, "stack1", completedUpdates[0].stack)
	assert.Equal(t, apitype.UpdateStatusFailed, completedUpdates[0].status)
}

func readCompleteStatus(t *testing.T, r *http.Request) apitype.UpdateStatus {
	t.Helper()
	var req apitype.CompleteUpdateRequest
	require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
	return req.Status
}
