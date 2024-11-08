// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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
		Run: runCmdFunc(func(cmd *cobra.Command, args []string) error {
			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
			if err != nil {
				return err
			}

			defaultOrg, err := pkgWorkspace.GetBackendConfigDefaultOrg(project)
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
	cmd.AddCommand(newOrgListCmd())
	cmd.AddCommand(newOrgGetDefaultCmd())
	cmd.AddCommand(newOrgClearDefaultCmd())
	cmd.AddCommand(newSearchCmd())

	return cmd
}

func newOrgSetDefaultCmd() *cobra.Command {
	var orgName string

	cmd := &cobra.Command{
		Use:   "set-default [NAME]",
		Args:  cmdutil.ExactArgs(1),
		Short: "Set the local default organization for the current backend",
		Long: "Set the local default organization for the current backend.\n" +
			"\n" +
			"This command is used to set your local default organization in which to create \n" +
			"projects and stacks for the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations. " +
			"DIY backends (e.g. AWS S3 or Azure Blob Storage) do not support organizations." +
			"If you try and set a default organization for a backend that does not \n" +
			"support create organizations, then an error will be returned by the CLI",
		Run: runCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			orgName = args[0]

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, ws, DefaultLoginManager, project, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("unable to set a default organization for backend type: %s",
					currentBe.Name())
			}

			cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
			if err != nil {
				return err
			}

			return workspace.SetBackendConfigDefaultOrg(cloudURL, orgName)
		}),
	}

	return cmd
}

func newOrgListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the orgs available to the user for the current backend",
		Long: "List the orgs available to the user for the current backend.\n" +
			"\n" +
			"This command is used to list all of the organizations the user is a part of within " +
			"the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations. " +
			"DIY backends (e.g. AWS S3 or Azure Blob Storage) do not support organizations." +
			"If you try and set a default organization for a backend that does not \n" +
			"support create organizations, then an error will be returned by the CLI",
		Run: runCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, ws, DefaultLoginManager, project, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("backends of this type %q do not support organizations",
					currentBe.Name())
			}

			_, orgs, _, err := currentBe.CurrentUser()
			if err != nil {
				return err
			}

			defaultOrg, err := pkgWorkspace.GetBackendConfigDefaultOrg(project)
			if err != nil {
				return err
			}

			for _, o := range orgs {
				if defaultOrg == o {
					orgName := cmdutil.GetGlobalColorization().Colorize(colors.SpecInfo + colors.Bold + o + colors.Reset)
					fmt.Printf("%s (default)\n", orgName)
					continue
				}
				fmt.Println(o)
			}

			return nil
		}),
	}

	return cmd
}

func newOrgGetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-default",
		Short: "Get the default org for the current backend",
		Long: "Get the default org for the current backend.\n" +
			"\n" +
			"This command is used to get the default organization for which and stacks are created in " +
			"the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations. " +
			"DIY backends (e.g. AWS S3 or Azure Blob Storage) do not support organizations." +
			"If you try and set a default organization for a backend that does not \n" +
			"support create organizations, then an error will be returned by the CLI",
		Run: runCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, ws, DefaultLoginManager, project, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("backends of this type %q do not support organizations",
					currentBe.Name())
			}

			defaultOrg, err := pkgWorkspace.GetBackendConfigDefaultOrg(project)
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

func newOrgClearDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-default",
		Short: "Clear the local default organization for the current backend",
		Long: "Clear the local default organization for the current backend.\n" +
			"\n" +
			"This command is used to clear your local default organization \n" +
			"for the current backend.\n" +
			"\n" +
			"Currently, only the managed and self-hosted backends support organizations. " +
			"DIY backends (e.g. AWS S3 or Azure Blob Storage) do not support organizations." +
			"If you try and set a default organization for a backend that does not \n" +
			"support create organizations, then an error will be returned by the CLI",
		Run: runCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, ws, DefaultLoginManager, project, displayOpts)
			if err != nil {
				return err
			}
			if !currentBe.SupportsOrganizations() {
				return fmt.Errorf("unable to clear default organization for backend type: %s",
					currentBe.Name())
			}

			cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
			if err != nil {
				return err
			}

			return workspace.SetBackendConfigDefaultOrg(cloudURL, "")
		}),
	}

	return cmd
}
