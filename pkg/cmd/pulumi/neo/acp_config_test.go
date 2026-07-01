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

package neo

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// fakeUpdater records the UpdateNeoTask PATCHes a session issues for live mode
// changes (the read-only toggle on a running task).
type fakeUpdater struct {
	mu    sync.Mutex
	calls []client.UpdateNeoTaskOptions
}

func (u *fakeUpdater) UpdateNeoTask(
	_ context.Context, _, _ string, opts client.UpdateNeoTaskOptions,
) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, opts)
	return nil
}

// findOption returns the config option with the given id, failing the test if it
// is absent.
func findOption(t *testing.T, opts []acp.ConfigOption, id string) acp.ConfigOption {
	t.Helper()
	for _, o := range opts {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("config option %q not advertised", id)
	return acp.ConfigOption{}
}

func TestConfigOptionsDefaults(t *testing.T) {
	t.Parallel()

	s := &acpSession{}
	opts := s.configOptionsSnapshot()

	// permission first (the agent's preferred display priority), then plan.
	require.Len(t, opts, 2)
	assert.Equal(t, acpConfigPermission, opts[0].ID)
	assert.Equal(t, acpConfigPlan, opts[1].ID)

	perm := findOption(t, opts, acpConfigPermission)
	assert.Equal(t, acp.ConfigOptionTypeSelect, perm.Type)
	assert.Equal(t, acp.ConfigCategoryMode, perm.Category)
	assert.Equal(t, acpPermissionValueDefault, perm.CurrentValue, "default is full access, not read-only")

	plan := findOption(t, opts, acpConfigPlan)
	assert.Equal(t, acpPlanValueBuild, plan.CurrentValue, "plan mode is off by default")
}

func TestSetConfigOptionPermission(t *testing.T) {
	t.Parallel()

	t.Run("before start stores value without patching", func(t *testing.T) {
		t.Parallel()
		up := &fakeUpdater{}
		s := &acpSession{updater: up, orgName: "acme"}

		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPermission, acpPermissionValueReadOnly))

		assert.Equal(t, client.NeoPermissionModeReadOnly, s.permissionMode)
		up.mu.Lock()
		assert.Empty(t, up.calls, "no task exists yet, so nothing to PATCH")
		up.mu.Unlock()
		assert.Equal(t, acpPermissionValueReadOnly,
			findOption(t, s.configOptionsSnapshot(), acpConfigPermission).CurrentValue)
	})

	t.Run("after start patches the live task", func(t *testing.T) {
		t.Parallel()
		up := &fakeUpdater{}
		s := &acpSession{updater: up, orgName: "acme", taskID: "task_1", started: true}

		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPermission, acpPermissionValueReadOnly))

		up.mu.Lock()
		defer up.mu.Unlock()
		require.Len(t, up.calls, 1)
		require.NotNil(t, up.calls[0].PermissionMode)
		assert.Equal(t, client.NeoPermissionModeReadOnly, *up.calls[0].PermissionMode)
		assert.Nil(t, up.calls[0].ApprovalMode, "approval mode is never changed")
	})
}

func TestSetConfigOptionPlan(t *testing.T) {
	t.Parallel()

	t.Run("before start stores value", func(t *testing.T) {
		t.Parallel()
		s := &acpSession{}

		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPlan, acpPlanValuePlan))

		assert.True(t, s.planMode)
		assert.Equal(t, acpPlanValuePlan,
			findOption(t, s.configOptionsSnapshot(), acpConfigPlan).CurrentValue)
	})

	t.Run("after start is clamped", func(t *testing.T) {
		t.Parallel()
		s := &acpSession{started: true}

		// PlanMode is fixed at task creation; a later switch must not change it.
		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPlan, acpPlanValuePlan))

		assert.False(t, s.planMode, "plan mode cannot be enabled once the task exists")
		assert.Equal(t, acpPlanValueBuild,
			findOption(t, s.configOptionsSnapshot(), acpConfigPlan).CurrentValue)
	})
}

func TestSetConfigOptionRejectsInvalid(t *testing.T) {
	t.Parallel()

	s := &acpSession{}
	require.Error(t, s.setConfigOption(t.Context(), acpConfigPermission, "bogus"))
	require.Error(t, s.setConfigOption(t.Context(), acpConfigPlan, "bogus"))
	require.Error(t, s.setConfigOption(t.Context(), "unknown", "x"))
}

func TestDelegateSetConfigOption(t *testing.T) {
	t.Parallel()

	up := &fakeUpdater{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["sess_x"] = &acpSession{acpID: "sess_x", updater: up, orgName: "acme"}

	res, err := d.SetConfigOption(t.Context(), acp.SetConfigOptionParams{
		SessionID: "sess_x", ConfigID: acpConfigPermission, Value: acpPermissionValueReadOnly,
	})
	require.NoError(t, err)
	assert.Equal(t, acpPermissionValueReadOnly,
		findOption(t, res.ConfigOptions, acpConfigPermission).CurrentValue,
		"the response carries the complete, updated option list")

	_, err = d.SetConfigOption(t.Context(), acp.SetConfigOptionParams{SessionID: "missing"})
	require.ErrorContains(t, err, "unknown session")
}

func TestRequestPermissionPlanExitEmitsConfigUpdate(t *testing.T) {
	t.Parallel()

	fc := &fakeACPClient{permResult: acp.RequestPermissionResult{
		Outcome: acp.PermissionOutcome{Outcome: "selected", OptionID: "allow"},
	}}
	fp := &fakePoster{}
	s := &acpSession{acpID: "sess_x", client: fc, poster: fp, orgName: "acme", taskID: "task_1", planMode: true}

	s.requestPermission(t.Context(), UIApprovalRequest{
		ApprovalID:   "appr_1",
		ApprovalType: approvalTypePlanExit,
		Message:      "approve the plan?",
	})

	assert.False(t, s.planMode, "approving an exit_plan_mode request clears plan mode")

	fc.mu.Lock()
	defer fc.mu.Unlock()
	var update *acp.ConfigOptionUpdate
	for _, n := range fc.notifications {
		if n.method != "session/update" {
			continue
		}
		if sn, ok := n.params.(acp.SessionNotification); ok {
			if u, ok := sn.Update.(acp.ConfigOptionUpdate); ok {
				update = &u
			}
		}
	}
	require.NotNil(t, update, "a config_option_update must announce leaving plan mode")
	assert.Equal(t, acpPlanValueBuild, findOption(t, update.ConfigOptions, acpConfigPlan).CurrentValue)
}

func TestRequestPermissionPlanExitRejectedKeepsPlanMode(t *testing.T) {
	t.Parallel()

	fc := &fakeACPClient{permResult: acp.RequestPermissionResult{
		Outcome: acp.PermissionOutcome{Outcome: "selected", OptionID: "reject"},
	}}
	fp := &fakePoster{}
	s := &acpSession{acpID: "sess_x", client: fc, poster: fp, orgName: "acme", taskID: "task_1", planMode: true}

	s.requestPermission(t.Context(), UIApprovalRequest{
		ApprovalID:   "appr_1",
		ApprovalType: approvalTypePlanExit,
		Message:      "approve the plan?",
	})

	assert.True(t, s.planMode, "rejecting the plan keeps plan mode on")
	fc.mu.Lock()
	defer fc.mu.Unlock()
	for _, n := range fc.notifications {
		if sn, ok := n.params.(acp.SessionNotification); ok {
			_, isCfg := sn.Update.(acp.ConfigOptionUpdate)
			assert.False(t, isCfg, "no config_option_update when plan mode is unchanged")
		}
	}
}
