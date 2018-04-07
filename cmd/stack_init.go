// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackInitCmd() *cobra.Command {
	var ppc string
	cmd := &cobra.Command{
		Use:   "init <stack-name>",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			b, err := currentBackend()
			if err != nil {
				return err
			}

			var opts interface{}
			if _, ok := b.(cloud.Backend); ok {
				opts = cloud.CreateStackOptions{CloudName: ppc}
			}

			var stackName tokens.QName
			if len(args) > 0 {
				stackName = tokens.QName(args[0])
			} else if cmdutil.Interactive() {
				name, nameErr := cmdutil.ReadConsole("Enter a stack name")
				if nameErr != nil {
					return nameErr
				}
				stackName = tokens.QName(name)
			}

			if stackName == "" {
				return errors.New("missing stack name")
			}

			_, err = createStack(b, stackName, opts)
			return err
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&ppc, "ppc", "p", "", "A Pulumi Private Cloud (PPC) name to initialize this stack in (if not --local)")
	return cmd
}
