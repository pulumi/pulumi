// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
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
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the stack to use.
			if len(args) == 0 {
				return errors.New("missing required stack name")
			}

			stackName := tokens.QName(args[0])

			if _, _, _, err := getStack(stackName); err == nil {
				return fmt.Errorf("stack '%v' already exists", stackName)

			}

			err := saveStack(stackName, nil, nil)
			if err != nil {
				return err
			}

			return setCurrentStack(stackName, false)
		}),
	}
}

func newCloudStackInitCmd() *cobra.Command {
	var org string
	var cloud string
	var repo string
	var project string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the stack to use.
			if len(args) == 0 {
				return errors.New("missing required stack name")
			}

			stackName := args[0]
			createStackReq := apitype.CreateStackRequest{
				CloudName: cloud,
				StackName: stackName,
			}

			var createStackResp apitype.CreateStackResponse
			path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", org, repo, project)
			if err := pulumiRESTCall("POST", path, &createStackReq, &createStackResp); err != nil {
				return err
			}
			fmt.Printf("Created Stack '%s' hosted in Cloud '%s'\n", stackName, createStackResp.CloudName)

			stackQName := tokens.QName(stackName)
			if err := saveStack(stackQName, nil, nil); err != nil {
				return err
			}
			return setCurrentStack(stackQName, false)
		}),
	}

	// "cloud" is not a persistent flag. If not set will use the "default" cloud for the organization.
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "",
		"Target cloud")

	cmd.PersistentFlags().StringVarP(
		&org, "organization", "o", "",
		"Target organization")
	cmd.PersistentFlags().StringVarP(
		&repo, "repo", "r", "",
		"Target Pulumi repo")
	cmd.PersistentFlags().StringVarP(
		&project, "project", "p", "",
		"Target Pulumi project")

	// We need all of these flags to be set. In the future we'll get some of these from the .pulumi folder, and have
	// a meaningful default for others. So in practice users won't need to specify all of these. (Ideally none.)
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("organization"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("repo"))
	contract.AssertNoError(cmd.MarkPersistentFlagRequired("project"))

	return cmd
}
