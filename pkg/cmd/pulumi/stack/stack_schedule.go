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

package stack

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// parseScheduleTimestamp normalizes a schedule timestamp to the ISO 8601 form the service's request expects. It accepts
// either that form (what users type into --once) or the SQL-style "2006-01-02 15:04:05.000" format the Cloud API
// returns in response bodies
func parseScheduleTimestamp(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.999", s, time.UTC); err == nil {
		return t.Format(time.RFC3339), nil
	}
	return "", fmt.Errorf(
		"invalid timestamp %q: must be ISO 8601 (e.g. 2026-12-31T23:59:00Z)", s,
	)
}

const (
	scheduleKindRaw   = "raw"
	scheduleKindDrift = "drift"
	scheduleKindTTL   = "ttl"
)

type scheduleSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Settings string `json:"settings"`
	Schedule string `json:"schedule"`
	NextRun  string `json:"nextRun,omitempty"`
	LastRun  string `json:"lastRun,omitempty"`
	Created  string `json:"created,omitempty"`
}

// scheduleKindLabel returns the user-facing schedule kind (raw / drift / ttl) matching
// the Cloud UI.
//
//   - operation == detect-drift                       -> drift
//   - operation == destroy && operationContext != nil -> ttl
//   - operation == destroy && operationContext == nil -> raw
//   - anything else                                   -> raw
func scheduleKindLabel(s apitype.ScheduledAction) string {
	if s.Kind != apitype.ScheduledActionKindDeployment {
		return string(s.Kind)
	}
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil || def.Request == nil {
		return scheduleKindRaw
	}
	req := def.Request
	if req.Op == apitype.DetectDrift {
		return scheduleKindDrift
	}
	if req.Op == apitype.Destroy && req.Operation != nil {
		return scheduleKindTTL
	}
	return scheduleKindRaw
}

func summarizeSchedule(s apitype.ScheduledAction) scheduleSummary {
	lastRun := ""
	if s.LastExecuted != nil {
		lastRun = *s.LastExecuted
	}
	return scheduleSummary{
		ID:       s.ID,
		Type:     scheduleKindLabel(s),
		Settings: scheduleSettings(s),
		Schedule: formatSchedule(s),
		NextRun:  s.NextExecution,
		LastRun:  lastRun,
		Created:  s.Created,
	}
}

// scheduleSettings returns a compact, user-readable summary of what the schedule does,
// modeled on the Cloud UI's Settings column.
//
//	"detect"                  — drift detection only
//	"detect + auto-remediate" — drift detection that auto-applies a remediation update
//	"destroy"                 — TTL destroy
//	"destroy + delete stack"  — TTL destroy that also deletes the stack from Pulumi Cloud
//	"pulumi update"           — raw schedule running a Pulumi operation
func scheduleSettings(s apitype.ScheduledAction) string {
	if s.Kind != apitype.ScheduledActionKindDeployment {
		return ""
	}
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil || def.Request == nil {
		return ""
	}
	req := def.Request
	var opts apitype.OperationContextOptions
	if req.Operation != nil && req.Operation.Options != nil {
		opts = *req.Operation.Options
	}
	//exhaustive:ignore // other operations fall through to the default branch.
	switch req.Op {
	case apitype.DetectDrift:
		parts := []string{"detect"}
		if opts.RemediateIfDriftDetected {
			parts = append(parts, "auto-remediate")
		}
		return strings.Join(parts, " + ")
	case apitype.Destroy:
		parts := []string{"destroy"}
		if opts.DeleteAfterDestroy {
			parts = append(parts, "delete stack")
		}
		return strings.Join(parts, " + ")
	default:
		return "pulumi " + string(req.Op)
	}
}

// TODO[https://github.com/pulumi/pulumi/issues/23050]: Not yet implemented.
func newStackScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "schedule",
		Short:  "Manage scheduled deployment actions for a stack",
		Long:   "[EXPERIMENTAL] Manage scheduled deployment actions for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newStackScheduleListCmd())
	cmd.AddCommand(newStackScheduleNewCmd())
	cmd.AddCommand(newStackScheduleGetCmd())
	cmd.AddCommand(newStackScheduleEditCmd())
	cmd.AddCommand(newStackScheduleRemoveCmd())
	return cmd
}
