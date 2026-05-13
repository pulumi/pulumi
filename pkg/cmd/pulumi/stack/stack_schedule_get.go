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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackScheduleGetClient interface {
	GetStackSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
	) (apitype.ScheduledAction, error)
}

type stackScheduleGetClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleGetClient, client.StackIdentifier, error)

func newStackScheduleGetCmd() *cobra.Command {
	return newStackScheduleGetCmdWith(nil)
}

func newStackScheduleGetCmdWith(factory stackScheduleGetClientFactory) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "get <schedule-id>",
		Short: "Retrieve the configuration of a scheduled action",
		Long:  "[EXPERIMENTAL] Retrieve the configuration of a scheduled action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleGetClientFactory
			}
			return runStackScheduleGet(cmd.Context(), cmd.OutOrStdout(), factory, stack, args[0], output)
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
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable text) or json")

	return cmd
}

func defaultStackScheduleGetClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleGetClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleGet(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleGetClientFactory,
	stackFlag, scheduleID, output string,
) error {
	renderer, err := ui.Renderer(output, ui.OutputRenderers[scheduleGetRenderFunc]{
		Default: renderScheduleGetText,
		JSON:    renderScheduleGetJSON,
	})
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	schedule, err := c.GetStackSchedule(ctx, stackID, scheduleID)
	if err != nil {
		return fmt.Errorf("getting stack schedule: %w", err)
	}

	return renderer(w, schedule)
}

type scheduleGetRenderFunc func(w io.Writer, schedule apitype.ScheduledAction) error

func renderScheduleGetJSON(w io.Writer, schedule apitype.ScheduledAction) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(schedule)
}

func renderScheduleGetText(w io.Writer, s apitype.ScheduledAction) error {
	line := func(label, value string) {
		fmt.Fprintf(w, "%-22s %s\n", label+":", value)
	}
	optLine := func(label, value string) {
		if value != "" {
			line(label, value)
		}
	}

	line("ID", s.ID)
	line("Kind", string(s.Kind))
	optLine("Cron", s.ScheduleCron)
	optLine("Once", s.ScheduleOnce)
	optLine("Next execution", s.NextExecution)
	if s.LastExecuted != nil && *s.LastExecuted != "" {
		line("Last executed", *s.LastExecuted)
	} else {
		line("Last executed", "(never)")
	}
	line("Paused", strconv.FormatBool(s.Paused))
	optLine("Created", s.Created)
	optLine("Modified", s.Modified)

	if s.Kind == apitype.ScheduledActionKindDeployment {
		var def apitype.ScheduledDeploymentDefinition
		if err := json.Unmarshal(s.Definition, &def); err == nil {
			renderScheduledDeploymentText(def, line)
		}
	}
	return nil
}

func renderScheduledDeploymentText(
	def apitype.ScheduledDeploymentDefinition,
	line func(label, value string),
) {
	req := def.Request
	if req == nil {
		return
	}
	line("Operation", string(req.Op))
	line("Inherit settings", strconv.FormatBool(req.InheritSettings))

	var opts apitype.OperationContextOptions
	if opCtx := req.Operation; opCtx != nil && opCtx.Options != nil {
		opts = *opCtx.Options
	}
	// The schedule-creation flows in the backend pair RemediateIfDriftDetected with
	// DetectDrift (drift schedules) and DeleteAfterDestroy with Destroy (TTL schedules);
	// see pulumi-service/cmd/service/api/scheduled_{drift,ttl}_deployments.go. Other
	// operations don't carry a kind-specific toggle we need to surface.
	switch req.Op {
	case apitype.DetectDrift:
		line("Remediate on drift", strconv.FormatBool(opts.RemediateIfDriftDetected))
	case apitype.Destroy:
		line("Delete after destroy", strconv.FormatBool(opts.DeleteAfterDestroy))
	case apitype.Update, apitype.Preview, apitype.Refresh, apitype.RemediateDrift:
		// No kind-specific option for these operations.
	}
}
