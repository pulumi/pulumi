// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
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
			stackName := tokens.QName(args[0])

			if err := backend.CreateStack(stackName, StackCreationOptions{Cloud: cloud}); err != nil {
				return err
			}

			return setCurrentStack(stackName)
		}),
	}

	// only support --cloud when the backend is Pulumi.com
	if _, ok := backend.(*pulumiCloudPulumiBackend); ok {
		cmd.PersistentFlags().StringVarP(
			&cloud, "cloud", "c", "",
			"Target cloud")
	}

	return cmd
}
