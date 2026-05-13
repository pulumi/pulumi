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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23050]: Not yet implemented.
func newStackScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "schedule",
		Short:  "Manage scheduled deployment actions for a stack",
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
