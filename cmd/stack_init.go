// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/resource/deploy"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
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

			if _, _, err := getStack(stackName); err == nil {
				return fmt.Errorf("stack '%v' already exists", stackName)

			}

			target := deploy.Target{Name: stackName}

			err := saveStack(&target, nil)
			if err != nil {
				return err
			}

			return setCurrentStack(stackName, false)
		}),
	}
}
