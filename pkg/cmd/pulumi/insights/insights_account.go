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

package insights

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newInsightsAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Pulumi Insights accounts",
		Long:  "[EXPERIMENTAL] Manage Pulumi Insights accounts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newInsightsAccountNewCmd())
	cmd.AddCommand(newInsightsAccountListCmd(nil))
	cmd.AddCommand(newInsightsAccountScanCmd())

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22979]: Not yet implemented.
func newInsightsAccountNewCmd() *cobra.Command {
	var (
		org      string
		provider string
		parent   string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new Insights account",
		Long:   "[EXPERIMENTAL] Create a new Insights account.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to create the account in")
	cmd.Flags().StringVar(&provider, "provider", "", "The cloud provider (e.g. aws, azure, gcp)")
	cmd.Flags().StringVar(&parent, "parent", "", "The parent account, if any")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22978]: Not yet implemented.
func newInsightsAccountScanCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "scan",
		Short:  "Trigger a resource discovery scan for an Insights account",
		Long:   "[EXPERIMENTAL] Trigger a resource discovery scan for an Insights account.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "account"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the account")

	cmd.AddCommand(newInsightsAccountScanLogCmd())

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22977]: Not yet implemented.
func newInsightsAccountScanLogCmd() *cobra.Command {
	var (
		org   string
		job   int
		step  int
		count int
		token string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "log",
		Short:  "Retrieve logs for an Insights scan",
		Long:   "[EXPERIMENTAL] Retrieve logs for an Insights scan.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "account"},
			{Name: "scan-id"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the account")
	cmd.Flags().IntVar(&job, "job", -1, "The job index to fetch step-level logs for")
	cmd.Flags().IntVar(&step, "step", -1, "The step index within the job (requires --job)")
	cmd.Flags().IntVar(&count, "count", 0, "The number of log lines to fetch")
	cmd.Flags().StringVar(&token, "continuation-token", "", "The continuation token for paginated retrieval")

	return cmd
}
