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
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListStackSchedules(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		want := apitype.ListScheduledActionsResponse{
			Schedules: []apitype.ScheduledAction{
				{
					ID:            "bb61b60a-a313-46cb-b4ab-9d42dce46de8",
					OrgID:         "feacc792-460f-4525-a091-e8de1f6ef34c",
					ScheduleOnce:  "2026-05-13 10:51:00.000",
					NextExecution: "2026-05-13 10:51:00.000",
					Paused:        false,
					Kind:          apitype.ScheduledActionKindDeployment,
					Definition: json.RawMessage(
						`{"programID":"5f337707","request":{"operation":"destroy","inheritSettings":true}}`),
					Created:  "2026-05-13 08:51:42.176",
					Modified: "2026-05-13 08:51:42.176",
				},
				{
					ID:            "abc-cron",
					OrgID:         "feacc792-460f-4525-a091-e8de1f6ef34c",
					ScheduleCron:  "0 */4 * * *",
					NextExecution: "2026-05-13 12:00:00.000",
					Paused:        true,
					Kind:          apitype.ScheduledActionKindDeployment,
					Definition:    json.RawMessage(`{"request":{"operation":"refresh","inheritSettings":true}}`),
				},
			},
		}

		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(want)
			require.NoError(t, err)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.ListStackSchedules(t.Context(), stackID)
		require.NoError(t, err)

		assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/schedules", gotPath)
		assert.Equal(t, want.Schedules, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusInternalServerError, `{"message":"internal error"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.ListStackSchedules(t.Context(), stackID)
		assert.Error(t, err)
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"schedules":[]}`))
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.ListStackSchedules(t.Context(), stackID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestGetStackSchedule(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}
	const scheduleID = "bb61b60a-a313-46cb-b4ab-9d42dce46de8"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		want := apitype.ScheduledAction{
			ID:            scheduleID,
			OrgID:         "feacc792-460f-4525-a091-e8de1f6ef34c",
			ScheduleCron:  "12 16 * * *",
			NextExecution: "2026-05-13 16:12:00.000",
			Paused:        false,
			Kind:          apitype.ScheduledActionKindDeployment,
			Definition: json.RawMessage(
				`{"programID":"5f337707","request":{"operation":"detect-drift","inheritSettings":true}}`),
			Created:  "2026-05-13 08:51:42.176",
			Modified: "2026-05-13 09:47:37.982",
		}

		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(want)
			require.NoError(t, err)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.GetStackSchedule(t.Context(), stackID, scheduleID)
		require.NoError(t, err)

		assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/schedules/"+scheduleID, gotPath)
		assert.Equal(t, want, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusNotFound, `{"message":"not found"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.GetStackSchedule(t.Context(), stackID, scheduleID)
		assert.Error(t, err)
	})
}

func TestCreateStackSchedule(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		req := apitype.CreateScheduledDeploymentRequest{
			ScheduleCron: "0 */4 * * *",
			Request: &apitype.CreateDeploymentRequest{
				Op:              apitype.Update,
				InheritSettings: true,
			},
		}
		want := apitype.ScheduledAction{
			ID:           "bb61b60a-a313-46cb-b4ab-9d42dce46de8",
			OrgID:        "feacc792-460f-4525-a091-e8de1f6ef34c",
			ScheduleCron: "0 */4 * * *",
			Paused:       false,
			Kind:         apitype.ScheduledActionKindDeployment,
			Definition: json.RawMessage(
				`{"request":{"operation":"update","inheritSettings":true}}`),
			Created:  "2026-05-13 10:00:00.000",
			Modified: "2026-05-13 10:00:00.000",
		}

		var (
			gotPath   string
			gotMethod string
			gotBody   apitype.CreateScheduledDeploymentRequest
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &gotBody))
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.CreateStackSchedule(t.Context(), stackID, req)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/schedules", gotPath)
		assert.Equal(t, req, gotBody)
		assert.Equal(t, want, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusBadRequest, `{"message":"deployment settings not configured"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.CreateStackSchedule(t.Context(), stackID, apitype.CreateScheduledDeploymentRequest{
			ScheduleCron: "0 */4 * * *",
			Request:      &apitype.CreateDeploymentRequest{Op: apitype.Update, InheritSettings: true},
		})
		assert.Error(t, err)
	})
}

func TestCreateStackDriftSchedule(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	req := apitype.CreateScheduledDriftDeploymentRequest{
		ScheduleCron:  "0 */4 * * *",
		AutoRemediate: true,
	}
	want := apitype.ScheduledAction{
		ID:           "drift-id",
		ScheduleCron: "0 */4 * * *",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition: json.RawMessage(
			`{"request":{"operation":"detect-drift","inheritSettings":true,` +
				`"operationContext":{"options":{"remediateIfDriftDetected":true}}}}`),
	}

	var (
		gotPath   string
		gotMethod string
		gotBody   apitype.CreateScheduledDriftDeploymentRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	c := newMockClient(srv)
	got, err := c.CreateStackDriftSchedule(t.Context(), stackID, req)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/drift/schedules", gotPath)
	assert.Equal(t, req, gotBody)
	assert.Equal(t, want, got)
}

func TestCreateStackTTLSchedule(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	req := apitype.CreateScheduledTTLDeploymentRequest{
		Timestamp:          "2026-12-31T23:59:00Z",
		DeleteAfterDestroy: true,
	}
	want := apitype.ScheduledAction{
		ID:           "ttl-id",
		ScheduleOnce: "2026-12-31T23:59:00Z",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition: json.RawMessage(
			`{"request":{"operation":"destroy","inheritSettings":true,` +
				`"operationContext":{"options":{"deleteAfterDestroy":true}}}}`),
	}

	var (
		gotPath   string
		gotMethod string
		gotBody   apitype.CreateScheduledTTLDeploymentRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	c := newMockClient(srv)
	got, err := c.CreateStackTTLSchedule(t.Context(), stackID, req)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/ttl/schedules", gotPath)
	assert.Equal(t, req, gotBody)
	assert.Equal(t, want, got)
}

func TestDeleteStackSchedule(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}
	const scheduleID = "bb61b60a-a313-46cb-b4ab-9d42dce46de8"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var gotPath, gotMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		err := c.DeleteStackSchedule(t.Context(), stackID, scheduleID)
		require.NoError(t, err)

		assert.Equal(t, http.MethodDelete, gotMethod)
		assert.Equal(t, "/api/stacks/my-org/my-project/dev/deployments/schedules/"+scheduleID, gotPath)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusNotFound, `{"message":"not found"}`)
		defer srv.Close()

		c := newMockClient(srv)
		err := c.DeleteStackSchedule(t.Context(), stackID, scheduleID)
		assert.Error(t, err)
	})
}
