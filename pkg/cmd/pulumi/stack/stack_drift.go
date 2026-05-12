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

// TODO[https://github.com/pulumi/pulumi/issues/23053]: Not yet implemented.
func newStackDriftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "drift",
		Short:  "Inspect stack drift detection results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newStackDriftListCmd())
	cmd.AddCommand(newStackDriftGetCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23051]: Not yet implemented.
func newStackDriftListCmd() *cobra.Command {
	var (
		stack    string
		page     int
		pageSize int
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List drift detection runs for a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&page, "page", 1, "The page of results to return")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "The number of results per page (1-100)")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23052]: Not yet implemented.
func newStackDriftGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Retrieve the current drift detection status for a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
