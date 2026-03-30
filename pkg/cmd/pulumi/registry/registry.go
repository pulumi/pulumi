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

package registry

import (
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func NewRegistryCmd() *cobra.Command {
	// Create the ls command first so we can delegate to it.
	lsCmd := newRegistryLsCmd()

	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Explore and inspect Pulumi Registry packages",
		Long: `Explore and inspect Pulumi Registry packages.

Search for packages, view package metadata, and inspect the resources
and functions defined by packages in the Pulumi Registry.

When run with no subcommand in interactive mode, browses the package registry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmdutil.Interactive() {
				lsCmd.SetContext(cmd.Context())
				return lsCmd.RunE(lsCmd, args)
			}
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newRegistryPackageParentCmd(),
		newRegistryResourceCmd(),
		newRegistryFunctionCmd(),
		newRegistryExampleCmd(),
	)

	return cmd
}

func newRegistryPackageParentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Inspect packages in the Pulumi Registry",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newRegistryLsCmd(),
		newRegistryPackageGetCmd(),
		newRegistryPackageConfigCmd(),
		newRegistryPackageHowtoCmd(),
		newRegistryPackageInstallGuideCmd(),
	)

	return cmd
}

func newRegistryExampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Browse and retrieve code examples",
		Long: `Browse and retrieve code examples for packages, resources, and functions.

Examples are available for resource types, functions, and packages.
Specify a type token (e.g., aws:ec2/instance:Instance) or a package name.`,
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newRegistryExampleLsCmd(),
		newRegistryExampleGetCmd(),
	)

	return cmd
}

func newRegistryResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Inspect resources in a Pulumi Registry package",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newRegistryResourceLsCmd(),
		newRegistryResourceGetCmd(),
	)

	return cmd
}

func newRegistryFunctionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "function",
		Short: "Inspect functions in a Pulumi Registry package",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(
		newRegistryFunctionLsCmd(),
		newRegistryFunctionGetCmd(),
	)

	return cmd
}
