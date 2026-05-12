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

func newEnvReferrerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "referrer",
		Short:  "Inspect entities that reference an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvReferrerListCmd())
	return cmd
}

func newEnvReferrerListCmd() *cobra.Command {
	var (
		org                    string
		count                  int
		allRevisions           bool
		latestStackVersionOnly bool
		token                  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List entities that reference an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().IntVar(&count, "count", 0, "The maximum number of results to return (1-500)")
	cmd.Flags().BoolVar(&allRevisions, "all-revisions", false,
		"Include references across all revisions")
	cmd.Flags().BoolVar(&latestStackVersionOnly, "latest-stack-version-only", false,
		"Return only the latest stack version for each referring stack")
	cmd.Flags().StringVar(&token, "continuation-token", "",
		"The continuation token for paginated retrieval")

	return cmd
}
