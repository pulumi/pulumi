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
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

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

// TODO[https://github.com/pulumi/pulumi/issues/23048]: Not yet implemented.
func newStackScheduleNewCmd() *cobra.Command {
	var (
		stack     string
		cron      string
		once      string
		operation string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a custom deployment schedule for a stack",
		Long:   "[EXPERIMENTAL] Create a custom deployment schedule for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&cron, "cron", "",
		"A cron expression for recurring executions (e.g. '0 */4 * * *')")
	cmd.Flags().StringVar(&once, "once", "",
		"An ISO 8601 timestamp for a one-time execution")
	cmd.Flags().StringVar(&operation, "operation", "",
		"The Pulumi operation to perform: update, preview, refresh, or destroy")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23046]: Not yet implemented.
func newStackScheduleEditCmd() *cobra.Command {
	var (
		stack     string
		cron      string
		once      string
		operation string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Update the configuration of a custom deployment schedule",
		Long:   "[EXPERIMENTAL] Update the configuration of a custom deployment schedule.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "schedule-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&cron, "cron", "",
		"A cron expression for recurring executions")
	cmd.Flags().StringVar(&once, "once", "",
		"An ISO 8601 timestamp for a one-time execution")
	cmd.Flags().StringVar(&operation, "operation", "",
		"The Pulumi operation to perform: update, preview, refresh, or destroy")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23045]: Not yet implemented.
func newStackScheduleRemoveCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Permanently delete a scheduled deployment action",
		Long:   "[EXPERIMENTAL] Permanently delete a scheduled deployment action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "schedule-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
