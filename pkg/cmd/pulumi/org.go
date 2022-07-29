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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newOrgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   orgText.Use,
		Short: orgText.Short,
		Long:  orgText.Long,
		Args:  cmdutil.NoArgs,
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
		Use:   orgSetDefaultText.Use,
		Args:  cmdutil.ExactArgs(1),
		Short: orgSetDefaultText.Short,
		Long:  orgSetDefaultText.Long,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			orgName = args[0]

			currentBe, err := currentBackend(displayOpts)
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
		Use:   orgGetDefault.Use,
		Short: orgGetDefault.Short,
		Long:  orgGetDefault.Long,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			currentBe, err := currentBackend(displayOpts)
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
