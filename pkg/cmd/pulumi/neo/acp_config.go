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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

// The ACP session config options the adapter exposes, mirroring the user-facing
// toggles of the `pulumi neo` TUI: a read-only permission mode (Ctrl+R) and plan
// mode (Shift+Tab). Approval mode is intentionally not exposed — it stays
// "manual" so every gated tool call surfaces as an editor permission request.
const (
	// acpConfigPermission selects between full access and read-only. Maps to
	// client.NeoPermissionMode; its values equal the mode's wire strings.
	acpConfigPermission = "permission"
	// acpConfigPlan selects plan mode vs. building directly. Maps to PlanMode.
	acpConfigPlan = "plan"

	acpPermissionValueDefault  = string(client.NeoPermissionModeDefault)
	acpPermissionValueReadOnly = string(client.NeoPermissionModeReadOnly)

	acpPlanValueBuild = "build"
	acpPlanValuePlan  = "plan"
)

// neoConfigOptions renders the adapter's config options with current values
// reflecting permissionMode and planMode. The order is the agent's preferred
// display priority (permission first).
func neoConfigOptions(permissionMode client.NeoPermissionMode, planMode bool) []acp.ConfigOption {
	permValue := string(permissionMode)
	if permValue == "" {
		permValue = acpPermissionValueDefault
	}
	planValue := acpPlanValueBuild
	if planMode {
		planValue = acpPlanValuePlan
	}
	return []acp.ConfigOption{
		{
			ID:           acpConfigPermission,
			Name:         "Permissions",
			Description:  "Limit Neo to read-only operations, or allow full access.",
			Category:     acp.ConfigCategoryMode,
			Type:         acp.ConfigOptionTypeSelect,
			CurrentValue: permValue,
			Options: []acp.ConfigOptionValue{
				{
					Value:       acpPermissionValueDefault,
					Name:        "Full access",
					Description: "Neo may change state, run `pulumi up`, and open PRs, subject to your role.",
				},
				{
					Value:       acpPermissionValueReadOnly,
					Name:        "Read-only",
					Description: "Neo may only read; state changes, deployments, and PRs are blocked.",
				},
			},
		},
		{
			ID:           acpConfigPlan,
			Name:         "Plan mode",
			Description:  "Have Neo explore and propose a plan before it makes any changes.",
			Category:     acp.ConfigCategoryMode,
			Type:         acp.ConfigOptionTypeSelect,
			CurrentValue: planValue,
			Options: []acp.ConfigOptionValue{
				{
					Value:       acpPlanValueBuild,
					Name:        "Build",
					Description: "Neo can make changes right away.",
				},
				{
					Value:       acpPlanValuePlan,
					Name:        "Plan",
					Description: "Neo proposes a plan first. Set this before the first message.",
				},
			},
		},
	}
}

// configOptionsSnapshot renders the session's config options from its current
// mode state, taking the lock.
func (s *acpSession) configOptionsSnapshot() []acp.ConfigOption {
	s.mu.Lock()
	defer s.mu.Unlock()
	return neoConfigOptions(s.permissionMode, s.planMode)
}

// setConfigOption applies a single config-option change. The permission option is
// live: once the task exists it is PATCHed so the cloud picks it up immediately;
// before the task exists the stored value is applied at creation (start). The
// plan option is fixed at task creation — PlanMode cannot change on a running task
// — so a change after the task starts is clamped (kept at its current value),
// mirroring the TUI, where Shift+Tab is frozen after the first message.
//
// startMu is held throughout so this cannot interleave with start's task
// creation: a change lands either before it (start reads the new values) or
// after it (started is true, so the permission change PATCHes the live task and
// a plan change is clamped) — never in between, where it would be stored and
// advertised but never applied.
func (s *acpSession) setConfigOption(ctx context.Context, configID, value string) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	switch configID {
	case acpConfigPermission:
		var mode client.NeoPermissionMode
		switch value {
		case acpPermissionValueDefault:
			mode = client.NeoPermissionModeDefault
		case acpPermissionValueReadOnly:
			mode = client.NeoPermissionModeReadOnly
		default:
			return fmt.Errorf("invalid value %q for config option %q", value, configID)
		}
		s.mu.Lock()
		started, taskID := s.started, s.taskID
		s.mu.Unlock()
		// Push to a live task first; only record the new value once the server has
		// accepted it, so a failed PATCH doesn't leave us advertising a mode the
		// task isn't actually in.
		if started {
			if err := s.api.UpdateNeoTask(ctx, s.orgName, taskID,
				client.UpdateNeoTaskOptions{PermissionMode: &mode}); err != nil {
				return err
			}
		}
		s.mu.Lock()
		s.permissionMode = mode
		s.mu.Unlock()
		return nil

	case acpConfigPlan:
		var plan bool
		switch value {
		case acpPlanValueBuild:
			plan = false
		case acpPlanValuePlan:
			plan = true
		default:
			return fmt.Errorf("invalid value %q for config option %q", value, configID)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// Clamp once the task exists: plan mode is established at creation and the
		// server's PlanModeTracker runs in lockstep from there.
		if !s.started {
			s.planMode = plan
		}
		return nil

	default:
		return fmt.Errorf("unknown config option %q", configID)
	}
}

// noteExitedPlanMode records that the session left plan mode (the server clears
// it when an exit_plan_mode approval is granted) and reports whether the local
// state actually changed, so the caller can emit a single config_option_update.
func (s *acpSession) noteExitedPlanMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.planMode {
		return false
	}
	s.planMode = false
	return true
}
