// Copyright 2016-2021, Pulumi Corporation.
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

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newOrgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage Organization configuration",
		Long: "Manage Organization configuration.\n" +
			"\n" +
			"Use this command to manage organization configuration, " +
			"e.g. setting the default organization for a backend",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cloudURL, err := workspace.GetCurrentCloudURL()
			if err != nil {
				return err
			}

			defaultOrg, err := workspace.GetBackendConfigDefaultOrg()
			if err != nil {
				return err
			}

			fmt.Printf("Current Backend: %s\n", cloudURL)
			if defaultOrg != "" {
				fmt.Printf("Default Org: %s\n", defaultOrg)
			} else {
				fmt.Println("No Default Org Specified")
			}

			return nil
		}),
	}

	cmd.AddCommand(newOrgSetDefaultCmd())
	cmd.AddCommand(newOrgGetDefaultCmd())

	return cmd
}

func newOrgSetDefaultCmd() *cobra.Command {
	var orgName string

	var cmd = &cobra.Command{
		Use:   "set-default [NAME]",
		Args:  cmdutil.ExactArgs(1),
		Short: "Set the default organization for the current backend",
		Long: "Set the default organization for the current backend.\n" +
			"\n" +
			"This command is used to set the default organization in which to create \n" +
			"projects and stacks for the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations. " +
			"If you try and set a default organization for a backend that does not \n" +
			"support create organizations, then an error will be returned by the CLI",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			orgName = args[0]

			currentBe, err := shared.CurrentBackend(ctx, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("unable to set a default organization for backend type: %s",
					currentBe.Name())
			}

			cloudURL, err := workspace.GetCurrentCloudURL()
			if err != nil {
				return err
			}
			if err := workspace.SetBackendConfigDefaultOrg(cloudURL, orgName); err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}

func newOrgGetDefaultCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "get-default",
		Short: "Get the default org for the current backend",
		Long: "Get the default org for the current backend.\n" +
			"\n" +
			"This command is used to get the default organization for which and stacks are created in " +
			"the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			currentBe, err := shared.CurrentBackend(ctx, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("backends of this type %q do not support organizations",
					currentBe.Name())
			}

			defaultOrg, err := workspace.GetBackendConfigDefaultOrg()
			if err != nil {
				return err
			}

			if defaultOrg != "" {
				fmt.Println(defaultOrg)
			} else {
				fmt.Println("No Default Org Specified")
			}

			return nil
		}),
	}

	return cmd
}
