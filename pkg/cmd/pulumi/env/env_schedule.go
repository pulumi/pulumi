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

package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// envScheduleClient is the API surface used by `env schedule *`.
type envScheduleClient interface {
	ListEnvironmentSchedules(ctx context.Context, org, project, env string) (apitype.ListScheduledActionsResponse, error)
	CreateEnvironmentSchedule(
		ctx context.Context, org, project, env string, req apitype.CreateEnvironmentScheduleRequest,
	) (apitype.ScheduledAction, error)
	PauseEnvironmentSchedule(ctx context.Context, org, project, env, id string) error
	ResumeEnvironmentSchedule(ctx context.Context, org, project, env, id string) error
	DeleteEnvironmentSchedule(ctx context.Context, org, project, env, id string) error
}

type envScheduleFactory func(ctx context.Context, orgOverride string) (envScheduleClient, string, error)

func newEnvScheduleCmd() *cobra.Command {
	return newEnvScheduleCmdWith(nil)
}

func newEnvScheduleCmdWith(factory envScheduleFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage environment scheduled actions",
		Long:  "[EXPERIMENTAL] Manage environment scheduled actions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvScheduleListCmd(factory))
	cmd.AddCommand(newEnvScheduleNewCmd(factory))
	cmd.AddCommand(newEnvSchedulePauseCmd(factory))
	cmd.AddCommand(newEnvScheduleResumeCmd(factory))
	cmd.AddCommand(newEnvScheduleRemoveCmd(factory))
	return cmd
}

func envScheduleEnvArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	}
}

func envScheduleEnvWithIDArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
			{Name: "schedule-id"},
		},
		Required: 3,
	}
}

// --- list ---

func newEnvScheduleListCmd(factory envScheduleFactory) *cobra.Command {
	var (
		org    string
		output string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List scheduled actions configured for an environment",
		Long:  "[EXPERIMENTAL] List scheduled actions configured for an environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			render, err := envScheduleListRenderer(output)
			if err != nil {
				return err
			}
			resolveFactory := factory
			if resolveFactory == nil {
				resolveFactory = defaultEnvScheduleFactory
			}
			c, resolvedOrg, err := resolveFactory(cmd.Context(), org)
			if err != nil {
				return err
			}
			resp, err := c.ListEnvironmentSchedules(cmd.Context(), resolvedOrg, args[0], args[1])
			if err != nil {
				return fmt.Errorf("listing schedules for %s/%s/%s: %w", resolvedOrg, args[0], args[1], err)
			}
			return render(cmd.OutOrStdout(), resp)
		},
	}
	constrictor.AttachArguments(cmd, envScheduleEnvArg())
	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable) or json")
	return cmd
}

// --- new ---

func newEnvScheduleNewCmd(factory envScheduleFactory) *cobra.Command {
	var (
		org, cron, once, action string
		output                  string
	)
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new scheduled action for an environment",
		Long: "[EXPERIMENTAL] Create a new scheduled action for an environment.\n" +
			"\n" +
			"Exactly one of --cron or --once must be specified. The --action flag\n" +
			"selects the action kind; only 'rotate-secrets' is supported today.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (cron == "") == (once == "") {
				return errors.New("specify exactly one of --cron or --once")
			}
			req := apitype.CreateEnvironmentScheduleRequest{
				ScheduleCron: cron,
				ScheduleOnce: once,
			}
			switch action {
			case "", "rotate-secrets":
				req.SecretRotationRequest = &apitype.CreateEnvironmentSecretRotationScheduleRequest{}
			default:
				return fmt.Errorf("unsupported --action %q (only 'rotate-secrets' is supported)", action)
			}
			render, err := envScheduleRenderer(output)
			if err != nil {
				return err
			}
			resolveFactory := factory
			if resolveFactory == nil {
				resolveFactory = defaultEnvScheduleFactory
			}
			c, resolvedOrg, err := resolveFactory(cmd.Context(), org)
			if err != nil {
				return err
			}
			sched, err := c.CreateEnvironmentSchedule(cmd.Context(), resolvedOrg, args[0], args[1], req)
			if err != nil {
				return fmt.Errorf("creating schedule for %s/%s/%s: %w", resolvedOrg, args[0], args[1], err)
			}
			return render(cmd.OutOrStdout(), sched)
		},
	}
	constrictor.AttachArguments(cmd, envScheduleEnvArg())
	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&cron, "cron", "", "The cron expression for recurring executions")
	cmd.Flags().StringVar(&once, "once", "", "The ISO 8601 timestamp for a one-time execution")
	cmd.Flags().StringVar(&action, "action", "rotate-secrets", "The action to perform on each execution")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable) or json")
	return cmd
}

// --- pause / resume / remove (share a body) ---

func newEnvSchedulePauseCmd(factory envScheduleFactory) *cobra.Command {
	return newEnvScheduleSimpleCmd(factory, "pause",
		"Pause a scheduled action", "Paused schedule",
		func(c envScheduleClient, ctx context.Context, org, project, env, id string) error {
			return c.PauseEnvironmentSchedule(ctx, org, project, env, id)
		})
}

func newEnvScheduleResumeCmd(factory envScheduleFactory) *cobra.Command {
	return newEnvScheduleSimpleCmd(factory, "resume",
		"Resume a previously paused scheduled action", "Resumed schedule",
		func(c envScheduleClient, ctx context.Context, org, project, env, id string) error {
			return c.ResumeEnvironmentSchedule(ctx, org, project, env, id)
		})
}

func newEnvScheduleRemoveCmd(factory envScheduleFactory) *cobra.Command {
	return newEnvScheduleSimpleCmd(factory, "remove",
		"Permanently delete a scheduled action", "Deleted schedule",
		func(c envScheduleClient, ctx context.Context, org, project, env, id string) error {
			return c.DeleteEnvironmentSchedule(ctx, org, project, env, id)
		})
}

func newEnvScheduleSimpleCmd(
	factory envScheduleFactory, use, short, successVerb string,
	op func(envScheduleClient, context.Context, string, string, string, string) error,
) *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  "[EXPERIMENTAL] " + short + ".",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolveFactory := factory
			if resolveFactory == nil {
				resolveFactory = defaultEnvScheduleFactory
			}
			c, resolvedOrg, err := resolveFactory(cmd.Context(), org)
			if err != nil {
				return err
			}
			if err := op(c, cmd.Context(), resolvedOrg, args[0], args[1], args[2]); err != nil {
				return fmt.Errorf("%s %s: %w", use, args[2], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s on %s/%s/%s\n",
				successVerb, args[2], resolvedOrg, args[0], args[1])
			return nil
		},
	}
	constrictor.AttachArguments(cmd, envScheduleEnvWithIDArg())
	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	return cmd
}

// --- rendering ---

type (
	envScheduleListRenderFunc func(io.Writer, apitype.ListScheduledActionsResponse) error
	envScheduleRenderFunc     func(io.Writer, apitype.ScheduledAction) error
)

func envScheduleListRenderer(format string) (envScheduleListRenderFunc, error) {
	switch format {
	case "", "default":
		return renderScheduleListText, nil
	case "json":
		return func(w io.Writer, r apitype.ListScheduledActionsResponse) error {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		}, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func envScheduleRenderer(format string) (envScheduleRenderFunc, error) {
	switch format {
	case "", "default":
		return renderScheduleText, nil
	case "json":
		return func(w io.Writer, r apitype.ScheduledAction) error {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		}, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderScheduleListText(w io.Writer, r apitype.ListScheduledActionsResponse) error {
	if len(r.Schedules) == 0 {
		fmt.Fprintln(w, "No schedules configured for this environment.")
		return nil
	}
	for i, s := range r.Schedules {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if err := renderScheduleText(w, s); err != nil {
			return err
		}
	}
	return nil
}

func renderScheduleText(w io.Writer, s apitype.ScheduledAction) error {
	fmt.Fprintf(w, "ID:        %s\n", s.ID)
	fmt.Fprintf(w, "Kind:      %s\n", s.Kind)
	if s.ScheduleCron != "" {
		fmt.Fprintf(w, "Cron:      %s\n", s.ScheduleCron)
	}
	if s.ScheduleOnce != "" {
		fmt.Fprintf(w, "Once:      %s\n", s.ScheduleOnce)
	}
	fmt.Fprintf(w, "Paused:    %t\n", s.Paused)
	if s.NextExecution != "" {
		fmt.Fprintf(w, "Next run:  %s\n", s.NextExecution)
	}
	if s.LastExecuted != "" {
		fmt.Fprintf(w, "Last run:  %s\n", s.LastExecuted)
	}
	return nil
}

// --- factory ---

func defaultEnvScheduleFactory(
	ctx context.Context, orgOverride string,
) (envScheduleClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}
	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}
	return resolved.Client, org, nil
}
