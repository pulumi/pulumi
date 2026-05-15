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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

var rawOperations = []string{"update", "preview", "refresh", "destroy"}

type stackScheduleNewClient interface {
	CreateStackSchedule(
		ctx context.Context, stackID client.StackIdentifier, req apitype.CreateScheduledDeploymentRequest,
	) (apitype.ScheduledAction, error)
	CreateStackDriftSchedule(
		ctx context.Context, stackID client.StackIdentifier, req apitype.CreateScheduledDriftDeploymentRequest,
	) (apitype.ScheduledAction, error)
	CreateStackTTLSchedule(
		ctx context.Context, stackID client.StackIdentifier, req apitype.CreateScheduledTTLDeploymentRequest,
	) (apitype.ScheduledAction, error)
}

type stackScheduleNewClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleNewClient, client.StackIdentifier, error)

type stackScheduleNewArgs struct {
	stack              string
	kind               string
	cron               string
	once               string
	operation          string
	autoRemediate      bool
	deleteAfterDestroy bool
}

func newStackScheduleNewCmd() *cobra.Command {
	return newStackScheduleNewCmdWith(nil)
}

func newStackScheduleNewCmdWith(factory stackScheduleNewClientFactory) *cobra.Command {
	args := stackScheduleNewArgs{}
	var yes bool
	output := outputflag.OutputFlag[scheduleGetRenderFunc]{
		RenderForTerminal: renderScheduleGetText,
		RenderJSON:        renderScheduleGetJSON,
	}

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a scheduled deployment action for a stack",
		Long: "[EXPERIMENTAL] Create a scheduled deployment action for a stack.\n\n" +
			"The --kind flag selects the schedule type:\n" +
			"  raw   — runs a Pulumi operation on a cron or one-time schedule.\n" +
			"          Requires --operation and exactly one of --cron or --once.\n" +
			"  drift — runs drift detection on a cron schedule.\n" +
			"          Requires --cron. Use --auto-remediate to fix detected drift.\n" +
			"  ttl   — destroys the stack at a one-time timestamp.\n" +
			"          Requires --once. Use --delete-after-destroy to also delete\n" +
			"          the stack from Pulumi Cloud after destroy.\n\n" +
			"When run interactively, prompts for required values that aren't provided\n" +
			"via flags. Pass --yes to fail fast on missing values instead of prompting.\n\n" +
			"The stack must have deployment settings configured before a schedule can be created.",
		Example: "  # Walk through prompts to create a schedule\n" +
			"  pulumi stack schedule new\n" +
			"\n" +
			"  # Raw: refresh every 4 hours\n" +
			"  pulumi stack schedule new --kind raw --operation refresh --cron '0 */4 * * *'\n" +
			"\n" +
			"  # Drift detection every 4 hours, auto-remediating any drift found\n" +
			"  pulumi stack schedule new --kind drift --cron '0 */4 * * *' --auto-remediate\n" +
			"\n" +
			"  # TTL: destroy the stack on Jan 1 and remove it from Pulumi Cloud after\n" +
			"  pulumi stack schedule new --kind ttl --once 2027-01-01T00:00:00Z --delete-after-destroy\n",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if factory == nil {
				factory = defaultStackScheduleNewClientFactory
			}
			skipPrompts := yes || !cmdutil.Interactive()
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}
			resolved, err := resolveScheduleNewArgs(skipPrompts, args, opts)
			if err != nil {
				return err
			}
			return runStackScheduleNew(cmd.Context(), cmd.OutOrStdout(), factory, resolved, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&args.kind, "kind", "",
		"Schedule kind (required). One of: raw, drift, ttl")
	cmd.Flags().StringVar(&args.cron, "cron", "",
		"Cron expression for recurring executions, evaluated in UTC (e.g. '0 */4 * * *')")
	cmd.Flags().StringVar(&args.once, "once", "",
		"ISO 8601 timestamp for a one-time execution (e.g. '2026-12-31T23:59:00Z')")
	cmd.Flags().StringVar(&args.operation, "operation", "",
		"(raw only) Pulumi operation to run. One of: update, preview, refresh, destroy")
	cmd.Flags().BoolVar(&args.autoRemediate, "auto-remediate", false,
		"(drift only) Automatically run a remediation update when drift is detected")
	cmd.Flags().BoolVar(&args.deleteAfterDestroy, "delete-after-destroy", false,
		"(ttl only) Delete the stack from Pulumi Cloud after successfully destroying its resources")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip prompts; fail if any required value is missing")
	outputflag.VarP(cmd.Flags(), &output)

	cmd.MarkFlagsMutuallyExclusive("cron", "once")
	cmd.MarkFlagsMutuallyExclusive("operation", "auto-remediate")
	cmd.MarkFlagsMutuallyExclusive("operation", "delete-after-destroy")
	cmd.MarkFlagsMutuallyExclusive("auto-remediate", "delete-after-destroy")

	return cmd
}

// resolveScheduleNewArgs prompts for any required values not provided via flags. When
// skipPrompts is true (--yes or non-interactive), missing values return an error.
func resolveScheduleNewArgs(
	skipPrompts bool, args stackScheduleNewArgs, opts display.Options,
) (stackScheduleNewArgs, error) {
	if args.kind == "" {
		if skipPrompts {
			return args, errors.New("--kind is required")
		}
		args.kind = ui.PromptUserSkippable(
			false, "Schedule kind",
			[]string{scheduleKindRaw, scheduleKindDrift, scheduleKindTTL},
			scheduleKindRaw, opts.Color)
	}
	args.kind = strings.ToLower(strings.TrimSpace(args.kind))

	switch args.kind {
	case scheduleKindRaw:
		if args.operation == "" && !skipPrompts {
			args.operation = ui.PromptUserSkippable(
				false, "Pulumi operation", rawOperations, rawOperations[0], opts.Color)
		}
		if args.cron == "" && args.once == "" && !skipPrompts {
			triggerCron := "cron (recurring)"
			triggerOnce := "once (one-time)"
			trigger := ui.PromptUserSkippable(
				false, "Schedule trigger",
				[]string{triggerCron, triggerOnce}, triggerCron, opts.Color)
			if trigger == triggerOnce {
				once, err := ui.PromptForValue(
					false, "Timestamp (ISO 8601, e.g. 2026-12-31T23:59:00Z)", "", false,
					nonEmptyValidator("a timestamp"), opts)
				if err != nil {
					return args, err
				}
				args.once = once
			} else {
				cron, err := ui.PromptForValue(
					false, "Cron expression (e.g. '0 */4 * * *')", "", false,
					nonEmptyValidator("a cron expression"), opts)
				if err != nil {
					return args, err
				}
				args.cron = cron
			}
		}
	case scheduleKindDrift:
		if args.cron == "" && !skipPrompts {
			cron, err := ui.PromptForValue(
				false, "Cron expression (e.g. '0 */4 * * *')", "", false,
				nonEmptyValidator("a cron expression"), opts)
			if err != nil {
				return args, err
			}
			args.cron = cron
		}
		if !args.autoRemediate && !skipPrompts {
			choice := ui.PromptUserSkippable(
				false, "Automatically run `pulumi up` to remediate detected drift",
				[]string{"no", "yes"}, "no", opts.Color)
			args.autoRemediate = choice == "yes"
		}
	case scheduleKindTTL:
		if args.once == "" && !skipPrompts {
			once, err := ui.PromptForValue(
				false, "Timestamp (ISO 8601, e.g. 2026-12-31T23:59:00Z)", "", false,
				nonEmptyValidator("a timestamp"), opts)
			if err != nil {
				return args, err
			}
			args.once = once
		}
		if !args.deleteAfterDestroy && !skipPrompts {
			choice := ui.PromptUserSkippable(
				false, "Delete the stack from Pulumi Cloud after destroy completes",
				[]string{"no", "yes"}, "no", opts.Color)
			args.deleteAfterDestroy = choice == "yes"
		}
	}

	return args, nil
}

func nonEmptyValidator(what string) func(string) error {
	return func(v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s is required", what)
		}
		return nil
	}
}

func timestampValidator(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("a timestamp is required")
	}
	_, err := parseScheduleTimestamp(v)
	return err
}

func defaultStackScheduleNewClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleNewClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleNew(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleNewClientFactory,
	args stackScheduleNewArgs,
	render scheduleGetRenderFunc,
) error {
	kind := strings.ToLower(strings.TrimSpace(args.kind))
	if err := validateScheduleNewArgs(kind, args); err != nil {
		return err
	}

	if args.once != "" {
		normalized, err := parseScheduleTimestamp(args.once)
		if err != nil {
			return fmt.Errorf("--once %w", err)
		}
		args.once = normalized
	}

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	var schedule apitype.ScheduledAction
	switch kind {
	case scheduleKindRaw:
		op, opErr := parseScheduleOperation(args.operation)
		if opErr != nil {
			return opErr
		}
		schedule, err = c.CreateStackSchedule(ctx, stackID, apitype.CreateScheduledDeploymentRequest{
			ScheduleCron: args.cron,
			ScheduleOnce: args.once,
			Request: &apitype.CreateDeploymentRequest{
				Op:              op,
				InheritSettings: true,
			},
		})
	case scheduleKindDrift:
		schedule, err = c.CreateStackDriftSchedule(ctx, stackID, apitype.CreateScheduledDriftDeploymentRequest{
			ScheduleCron:  args.cron,
			AutoRemediate: args.autoRemediate,
		})
	case scheduleKindTTL:
		schedule, err = c.CreateStackTTLSchedule(ctx, stackID, apitype.CreateScheduledTTLDeploymentRequest{
			Timestamp:          args.once,
			DeleteAfterDestroy: args.deleteAfterDestroy,
		})
	}
	if err != nil {
		return fmt.Errorf("creating stack schedule: %w", err)
	}

	return render(w, schedule)
}

func validateScheduleNewArgs(kind string, args stackScheduleNewArgs) error {
	switch kind {
	case scheduleKindRaw:
		if args.cron == "" && args.once == "" {
			return errors.New("one of --cron or --once is required for --kind=raw")
		}
		if args.operation == "" {
			return errors.New("--operation is required for --kind=raw")
		}
		if args.autoRemediate {
			return errors.New("--auto-remediate is only valid for --kind=drift")
		}
		if args.deleteAfterDestroy {
			return errors.New("--delete-after-destroy is only valid for --kind=ttl")
		}
	case scheduleKindDrift:
		if args.cron == "" {
			return errors.New("--cron is required for --kind=drift")
		}
		if args.once != "" {
			return errors.New("--once is not valid for --kind=drift; use --cron")
		}
		if args.operation != "" {
			return errors.New("--operation is not valid for --kind=drift")
		}
		if args.deleteAfterDestroy {
			return errors.New("--delete-after-destroy is only valid for --kind=ttl")
		}
	case scheduleKindTTL:
		if args.once == "" {
			return errors.New("--once is required for --kind=ttl")
		}
		if args.cron != "" {
			return errors.New("--cron is not valid for --kind=ttl; use --once")
		}
		if args.operation != "" {
			return errors.New("--operation is not valid for --kind=ttl")
		}
		if args.autoRemediate {
			return errors.New("--auto-remediate is only valid for --kind=drift")
		}
	default:
		return fmt.Errorf("invalid --kind %q: must be one of raw, drift, ttl", args.kind)
	}
	return nil
}

func parseScheduleOperation(s string) (apitype.PulumiOperation, error) {
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
			"invalid --operation %q: must be one of update, preview, refresh, or destroy", s)
	}
}
