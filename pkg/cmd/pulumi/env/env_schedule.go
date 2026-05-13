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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23036]: Not yet implemented.
func newEnvScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "schedule",
		Short:  "Manage environment scheduled actions",
		Long:   "[EXPERIMENTAL] Manage environment scheduled actions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvScheduleListCmd())
	cmd.AddCommand(newEnvScheduleNewCmd())
	cmd.AddCommand(newEnvSchedulePauseCmd())
	cmd.AddCommand(newEnvScheduleResumeCmd())
	cmd.AddCommand(newEnvScheduleRemoveCmd())
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

// TODO[https://github.com/pulumi/pulumi/issues/23035]: Not yet implemented.
func newEnvScheduleListCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List scheduled actions configured for an environment",
		Long:   "[EXPERIMENTAL] List scheduled actions configured for an environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envScheduleEnvArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23034]: Not yet implemented.
func newEnvScheduleNewCmd() *cobra.Command {
	var (
		org    string
		cron   string
		once   string
		action string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new scheduled action for an environment",
		Long:   "[EXPERIMENTAL] Create a new scheduled action for an environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envScheduleEnvArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&cron, "cron", "", "The cron expression for recurring executions")
	cmd.Flags().StringVar(&once, "once", "", "The ISO 8601 timestamp for a one-time execution")
	cmd.Flags().StringVar(&action, "action", "", "The action to perform on each execution")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23033]: Not yet implemented.
func newEnvSchedulePauseCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "pause",
		Short:  "Pause a scheduled action",
		Long:   "[EXPERIMENTAL] Pause a scheduled action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envScheduleEnvWithIDArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23032]: Not yet implemented.
func newEnvScheduleResumeCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "resume",
		Short:  "Resume a previously paused scheduled action",
		Long:   "[EXPERIMENTAL] Resume a previously paused scheduled action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envScheduleEnvWithIDArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23031]: Not yet implemented.
func newEnvScheduleRemoveCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Permanently delete a scheduled action",
		Long:   "[EXPERIMENTAL] Permanently delete a scheduled action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envScheduleEnvWithIDArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}
