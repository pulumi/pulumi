// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
	if usePulumiCloudCommands() {
		return newCloudStackInitCmd()
	}
	return newFAFStackInitCmd()
}

func newFAFStackInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <stack>",
		Args:  cobra.ExactArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var backend pulumiBackend = &localPulumiBackend{}

			stackName := tokens.QName(args[0])

			if err := backend.CreateStack(stackName); err != nil {
				return err
			}

			return setCurrentStack(stackName)
		}),
	}
}

func newCloudStackInitCmd() *cobra.Command {
	var cloud string

	cmd := &cobra.Command{
		Use:   "init <stack>",
		Args:  cobra.ExactArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Look up the owner, repository, and project from the workspace and nearest package.
			w, err := newWorkspace()
			if err != nil {
				return err
			}
			projID, err := getCloudProjectIdentifier(w)
			if err != nil {
				return err
			}

			stackName := args[0]
			createStackReq := apitype.CreateStackRequest{
				CloudName: cloud,
				StackName: stackName,
			}

			var createStackResp apitype.CreateStackResponse
			path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
			if err := pulumiRESTCall("POST", path, &createStackReq, &createStackResp); err != nil {
				return err
			}
			fmt.Printf("Created Stack '%s' hosted in Cloud '%s'\n", stackName, createStackResp.CloudName)

			stackQName := tokens.QName(stackName)
			return setCurrentStack(stackQName)
		}),
	}

	// If not set will use the "default" cloud for the organization.
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "",
		"Target cloud")

	return cmd
}
