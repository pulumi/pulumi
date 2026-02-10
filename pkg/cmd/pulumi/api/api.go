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

package api

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/spf13/cobra"
)

// NewCloudCmd creates the top-level "pulumi cloud" command with the "api" subcommand.
func NewCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Pulumi Cloud commands",
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.AddCommand(newAPICmd())
	return cmd
}

// newAPICmd creates the "pulumi cloud api" command.
// Subcommands are built dynamically from the embedded OpenAPI spec.
func newAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Interact with the Pulumi Cloud API",
		Long: "Interact with the Pulumi Cloud API.\n\n" +
			"Commands are built from the Pulumi Cloud OpenAPI specification.\n" +
			"Use 'pulumi cloud api <group> <operation> --help' for details on any operation.\n\n" +
			"Authentication is automatically resolved from your Pulumi credentials.\n" +
			"The organization is resolved from your default org (set with 'pulumi org set-default')\n" +
			"or can be overridden with the --org flag on any operation.",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	groups, err := parseEmbeddedSpec()
	if err != nil {
		cmd.RunE = func(*cobra.Command, []string) error {
			return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
		}
		return cmd
	}

	for _, g := range groups {
		groupCmd := &cobra.Command{
			Use:   g.Slug,
			Short: g.Name + " operations",
		}
		constrictor.AttachArguments(groupCmd, constrictor.NoArgs)

		for _, op := range g.Operations {
			groupCmd.AddCommand(buildOperationCmd(op))
		}
		cmd.AddCommand(groupCmd)
	}

	return cmd
}
