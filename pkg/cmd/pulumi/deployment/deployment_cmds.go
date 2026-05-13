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

package deployment

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22988]: Not yet implemented.
func newDeploymentListCmd() *cobra.Command {
	var (
		stack    string
		page     int
		pageSize int
		sort     string
		asc      bool
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List deployments for a stack",
		Long:   "[EXPERIMENTAL] List deployments for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&page, "page", 1, "The page of results to return")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "The number of results per page (1-100)")
	cmd.Flags().StringVar(&sort, "sort", "", "The field to sort results by")
	cmd.Flags().BoolVar(&asc, "asc", false, "Sort in ascending order")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22987]: Not yet implemented.
func newDeploymentGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Get details for a specific deployment",
		Long:   "[EXPERIMENTAL] Get details for a specific deployment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22985]: Not yet implemented.
func newDeploymentCancelCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "cancel",
		Short:  "Cancel an in-progress deployment",
		Long: "[EXPERIMENTAL] Cancel an in-progress deployment.\n" +
			"\n" +
			"Canceling a deployment is a dangerous action and may leave the stack in\n" +
			"an inconsistent state if canceled during the execution of a Pulumi operation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22986]: Not yet implemented.
func newDeploymentLogCmd() *cobra.Command {
	var (
		stack  string
		job    int
		step   int
		offset int
		count  int
		token  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "log",
		Short:  "Retrieve execution logs for a deployment",
		Long:   "[EXPERIMENTAL] Retrieve execution logs for a deployment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&job, "job", -1, "The job index to fetch step-level logs for")
	cmd.Flags().IntVar(&step, "step", -1, "The step index within the job (requires --job)")
	cmd.Flags().IntVar(&offset, "offset", 0, "The byte offset within the step's logs")
	cmd.Flags().IntVar(&count, "count", 100, "The number of log lines to fetch (1-499 in step mode)")
	cmd.Flags().StringVar(&token, "continuation-token", "", "The continuation token for streaming mode")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22984]: Not yet implemented.
func newDeploymentSettingsGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Retrieve the deployment settings for a stack",
		Long:   "[EXPERIMENTAL] Retrieve the deployment settings for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22983]: Not yet implemented.
func newDeploymentSettingsEditCmd() *cobra.Command {
	var (
		stack string
		file  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Create or update deployment settings for a stack",
		Long: "[EXPERIMENTAL] Create or update deployment settings for a stack.\n" +
			"\n" +
			"If no settings exist, they are created. If settings already exist, the\n" +
			"request body is merged with the current settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&file, "file", "f", "",
		"Read settings patch from file; `-` reads stdin")

	return cmd
}
