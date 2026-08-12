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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

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
		up := &fakeTaskAPI{}
		s := &acpSession{api: up, orgName: "acme"}

		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPermission, acpPermissionValueReadOnly))

		assert.Equal(t, client.NeoPermissionModeReadOnly, s.permissionMode)
		up.mu.Lock()
		assert.Empty(t, up.patches, "no task exists yet, so nothing to PATCH")
		up.mu.Unlock()
		assert.Equal(t, acpPermissionValueReadOnly,
			findOption(t, s.configOptionsSnapshot(), acpConfigPermission).CurrentValue)
	})

	t.Run("after start patches the live task", func(t *testing.T) {
		t.Parallel()
		up := &fakeTaskAPI{}
		s := &acpSession{api: up, orgName: "acme", taskID: "task_1", started: true}

		require.NoError(t, s.setConfigOption(t.Context(), acpConfigPermission, acpPermissionValueReadOnly))

		up.mu.Lock()
		defer up.mu.Unlock()
		require.Len(t, up.patches, 1)
		require.NotNil(t, up.patches[0].PermissionMode)
		assert.Equal(t, client.NeoPermissionModeReadOnly, *up.patches[0].PermissionMode)
		assert.Nil(t, up.patches[0].ApprovalMode, "approval mode is never changed")
	})

	t.Run("failed patch keeps the old mode", func(t *testing.T) {
		t.Parallel()
		up := &fakeTaskAPI{updateErr: errors.New("PATCH rejected")}
		s := &acpSession{api: up, orgName: "acme", taskID: "task_1", started: true}

		// The documented ordering: push to the live task first, store only on
		// success — a failed PATCH must not leave us advertising a mode the task
		// isn't actually in.
		err := s.setConfigOption(t.Context(), acpConfigPermission, acpPermissionValueReadOnly)
		require.ErrorContains(t, err, "PATCH rejected")

		assert.Empty(t, s.permissionMode, "a failed PATCH must not store the new mode")
		assert.Equal(t, acpPermissionValueDefault,
			findOption(t, s.configOptionsSnapshot(), acpConfigPermission).CurrentValue,
			"the advertised options must still show the mode the task is in")
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

	up := &fakeTaskAPI{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["sess_x"] = &acpSession{acpID: "sess_x", api: up, orgName: "acme"}

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
	fp := &fakeTaskAPI{}
	s := &acpSession{acpID: "sess_x", client: fc, api: fp, orgName: "acme", taskID: "task_1", planMode: true}

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
	fp := &fakeTaskAPI{}
	s := &acpSession{acpID: "sess_x", client: fc, api: fp, orgName: "acme", taskID: "task_1", planMode: true}

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
