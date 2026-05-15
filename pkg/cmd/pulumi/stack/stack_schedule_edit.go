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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackScheduleEditClient interface {
	GetStackSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
	) (apitype.ScheduledAction, error)
	UpdateStackSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
		req apitype.CreateScheduledDeploymentRequest,
	) (apitype.ScheduledAction, error)
	UpdateStackDriftSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
		req apitype.CreateScheduledDriftDeploymentRequest,
	) (apitype.ScheduledAction, error)
	UpdateStackTTLSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
		req apitype.CreateScheduledTTLDeploymentRequest,
	) (apitype.ScheduledAction, error)
}

type stackScheduleEditClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleEditClient, client.StackIdentifier, error)

// stackScheduleEditFlags holds the resolved edit flags. The *Changed fields tell us
// whether the user explicitly passed each flag, since we always send the full request
// body and would otherwise overwrite existing values with defaults.
type stackScheduleEditFlags struct {
	cron               string
	once               string
	operation          string
	autoRemediate      bool
	deleteAfterDestroy bool

	cronChanged               bool
	onceChanged               bool
	operationChanged          bool
	autoRemediateChanged      bool
	deleteAfterDestroyChanged bool
}

func newStackScheduleEditCmd() *cobra.Command {
	return newStackScheduleEditCmdWith(nil)
}

func newStackScheduleEditCmdWith(factory stackScheduleEditClientFactory) *cobra.Command {
	var (
		stack string
		flags stackScheduleEditFlags
	)
	output := outputflag.OutputFlag[scheduleGetRenderFunc]{
		RenderForTerminal: renderScheduleGetText,
		RenderJSON:        renderScheduleGetJSON,
	}

	cmd := &cobra.Command{
		Use:   "edit <schedule-id>",
		Short: "Update the configuration of a scheduled deployment action",
		Long: "[EXPERIMENTAL] Update the configuration of a scheduled deployment action.\n\n" +
			"Only the fields you pass are changed; everything else is preserved. " +
			"Each flag is only valid for a subset of kinds:\n" +
			"  raw:   --cron, --once, --operation\n" +
			"  drift: --cron, --auto-remediate\n" +
			"  ttl:   --once, --delete-after-destroy",
		Example: "  # Switch a raw schedule to a different cron\n" +
			"  pulumi stack schedule edit <id> --cron '0 */6 * * *'\n" +
			"\n" +
			"  # Enable auto-remediation on a drift schedule\n" +
			"  pulumi stack schedule edit <id> --auto-remediate\n" +
			"\n" +
			"  # Disable delete-after-destroy on a TTL schedule\n" +
			"  pulumi stack schedule edit <id> --delete-after-destroy=false",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleEditClientFactory
			}
			flags.cronChanged = cmd.Flags().Changed("cron")
			flags.onceChanged = cmd.Flags().Changed("once")
			flags.operationChanged = cmd.Flags().Changed("operation")
			flags.autoRemediateChanged = cmd.Flags().Changed("auto-remediate")
			flags.deleteAfterDestroyChanged = cmd.Flags().Changed("delete-after-destroy")
			return runStackScheduleEdit(
				cmd.Context(), cmd.OutOrStdout(), factory, stack, args[0], flags, output.Get(),
			)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "schedule-id"}},
		Required:  1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&flags.cron, "cron", "",
		"Cron expression for recurring executions, evaluated in UTC (raw and drift)")
	cmd.Flags().StringVar(&flags.once, "once", "",
		"ISO 8601 timestamp for a one-time execution (raw and ttl)")
	cmd.Flags().StringVar(&flags.operation, "operation", "",
		"(raw only) Pulumi operation to run. One of: update, preview, refresh, destroy")
	cmd.Flags().BoolVar(&flags.autoRemediate, "auto-remediate", false,
		"(drift only) Automatically run a remediation update when drift is detected")
	cmd.Flags().BoolVar(&flags.deleteAfterDestroy, "delete-after-destroy", false,
		"(ttl only) Delete the stack from Pulumi Cloud after successfully destroying its resources")
	outputflag.VarP(cmd.Flags(), &output)

	cmd.MarkFlagsMutuallyExclusive("cron", "once")

	return cmd
}

func defaultStackScheduleEditClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleEditClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleEdit(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleEditClientFactory,
	stackFlag string,
	scheduleID string,
	flags stackScheduleEditFlags,
	render scheduleGetRenderFunc,
) error {
	if !anyEditFlagChanged(flags) {
		return errors.New(
			"at least one of --cron, --once, --operation, --auto-remediate, or --delete-after-destroy " +
				"must be provided")
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	existing, err := c.GetStackSchedule(ctx, stackID, scheduleID)
	if err != nil {
		return fmt.Errorf("reading stack schedule: %w", err)
	}

	kind := scheduleKindLabel(existing)
	if err := validateScheduleEditFlags(kind, flags); err != nil {
		return err
	}

	switch kind {
	case scheduleKindRaw:
		req, buildErr := buildRawEditRequest(existing, flags)
		if buildErr != nil {
			return buildErr
		}
		_, err = c.UpdateStackSchedule(ctx, stackID, scheduleID, req)
	case scheduleKindDrift:
		req := buildDriftEditRequest(existing, flags)
		_, err = c.UpdateStackDriftSchedule(ctx, stackID, scheduleID, req)
	case scheduleKindTTL:
		req := buildTTLEditRequest(existing, flags)
		_, err = c.UpdateStackTTLSchedule(ctx, stackID, scheduleID, req)
	default:
		return fmt.Errorf("cannot edit schedule of kind %q", kind)
	}
	if err != nil {
		return fmt.Errorf("updating stack schedule: %w", err)
	}

	// Refetch instead of using the response body: the raw update endpoint returns the
	// pre-update snapshot (it reads the schedule before the writes and never refetches).
	// Drift and TTL refetch server-side, so this extra GET is only strictly required for
	// raw, but doing it unconditionally keeps the code simple and one extra RTT is cheap.
	updated, err := c.GetStackSchedule(ctx, stackID, scheduleID)
	if err != nil {
		return fmt.Errorf("reading updated stack schedule: %w", err)
	}
	return render(w, updated)
}

func anyEditFlagChanged(flags stackScheduleEditFlags) bool {
	return flags.cronChanged ||
		flags.onceChanged ||
		flags.operationChanged ||
		flags.autoRemediateChanged ||
		flags.deleteAfterDestroyChanged
}

func validateScheduleEditFlags(kind string, flags stackScheduleEditFlags) error {
	switch kind {
	case scheduleKindRaw:
		if flags.autoRemediateChanged {
			return errors.New("--auto-remediate is only valid for drift schedules")
		}
		if flags.deleteAfterDestroyChanged {
			return errors.New("--delete-after-destroy is only valid for ttl schedules")
		}
	case scheduleKindDrift:
		if flags.onceChanged {
			return errors.New("--once is not valid for drift schedules; use --cron")
		}
		if flags.operationChanged {
			return errors.New("--operation is not valid for drift schedules")
		}
		if flags.deleteAfterDestroyChanged {
			return errors.New("--delete-after-destroy is only valid for ttl schedules")
		}
	case scheduleKindTTL:
		if flags.cronChanged {
			return errors.New("--cron is not valid for ttl schedules; use --once")
		}
		if flags.operationChanged {
			return errors.New("--operation is not valid for ttl schedules")
		}
		if flags.autoRemediateChanged {
			return errors.New("--auto-remediate is only valid for drift schedules")
		}
	}
	return nil
}

// buildRawEditRequest builds the full update body for a raw schedule. We always send the
// trigger (cron or once) the schedule should end up with, plus a full Request when the
// user touched anything inside the deployment definition.
func buildRawEditRequest(
	existing apitype.ScheduledAction, flags stackScheduleEditFlags,
) (apitype.CreateScheduledDeploymentRequest, error) {
	cron, once := existing.ScheduleCron, existing.ScheduleOnce
	if flags.cronChanged {
		cron = flags.cron
		once = ""
	} else if flags.onceChanged {
		once = flags.once
		cron = ""
	}

	existingReq, err := existingDeploymentRequest(existing)
	if err != nil {
		return apitype.CreateScheduledDeploymentRequest{}, err
	}
	op := existingReq.Op
	if flags.operationChanged {
		parsed, parseErr := parseRawOperation(flags.operation)
		if parseErr != nil {
			return apitype.CreateScheduledDeploymentRequest{}, parseErr
		}
		op = parsed
	}
	existingReq.Op = op

	return apitype.CreateScheduledDeploymentRequest{
		ScheduleCron: cron,
		ScheduleOnce: once,
		Request:      existingReq,
	}, nil
}

// buildDriftEditRequest preserves existing values for any field the user didn't change.
// The drift update endpoint rewrites the entire definition on every call.
func buildDriftEditRequest(
	existing apitype.ScheduledAction, flags stackScheduleEditFlags,
) apitype.CreateScheduledDriftDeploymentRequest {
	cron := existing.ScheduleCron
	if flags.cronChanged {
		cron = flags.cron
	}
	autoRemediate := existingAutoRemediate(existing)
	if flags.autoRemediateChanged {
		autoRemediate = flags.autoRemediate
	}
	return apitype.CreateScheduledDriftDeploymentRequest{
		ScheduleCron:  cron,
		AutoRemediate: autoRemediate,
	}
}

// buildTTLEditRequest preserves existing values for any field the user didn't change.
// The TTL update endpoint rewrites the entire definition on every call.
func buildTTLEditRequest(
	existing apitype.ScheduledAction, flags stackScheduleEditFlags,
) apitype.CreateScheduledTTLDeploymentRequest {
	once := existing.ScheduleOnce
	if flags.onceChanged {
		once = flags.once
	}
	deleteAfterDestroy := existingDeleteAfterDestroy(existing)
	if flags.deleteAfterDestroyChanged {
		deleteAfterDestroy = flags.deleteAfterDestroy
	}
	return apitype.CreateScheduledTTLDeploymentRequest{
		Timestamp:          once,
		DeleteAfterDestroy: deleteAfterDestroy,
	}
}

func parseRawOperation(s string) (apitype.PulumiOperation, error) {
	switch strings.ToLower(s) {
	case "update":
		return apitype.Update, nil
	case "preview":
		return apitype.Preview, nil
	case "refresh":
		return apitype.Refresh, nil
	case "destroy":
		return apitype.Destroy, nil
	default:
		return "", fmt.Errorf(
			"invalid --operation %q: must be one of update, preview, refresh, or destroy", s,
		)
	}
}

func existingDeploymentRequest(s apitype.ScheduledAction) (*apitype.CreateDeploymentRequest, error) {
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil {
		return nil, fmt.Errorf("parsing existing schedule definition: %w", err)
	}
	if def.Request == nil {
		return nil, errors.New("existing schedule has no deployment request")
	}
	req := *def.Request
	return &req, nil
}

func existingAutoRemediate(s apitype.ScheduledAction) bool {
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil || def.Request == nil {
		return false
	}
	if def.Request.Operation == nil || def.Request.Operation.Options == nil {
		return false
	}
	return def.Request.Operation.Options.RemediateIfDriftDetected
}

func existingDeleteAfterDestroy(s apitype.ScheduledAction) bool {
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil || def.Request == nil {
		return false
	}
	if def.Request.Operation == nil || def.Request.Operation.Options == nil {
		return false
	}
	return def.Request.Operation.Options.DeleteAfterDestroy
}
