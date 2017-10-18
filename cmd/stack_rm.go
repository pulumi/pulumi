// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm <stack>",
		Short: "Remove an stack and its configuration",
		Long: "Remove an stack and its configuration\n" +
			"\n" +
			"This command removes an stack and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the stack will no longer be available for deployments.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 || args[0] == "" {
				return errors.Errorf("missing required stack name")
			}

			stackName := tokens.QName(args[0])

			// Ensure the user really wants to do this.
			if yes ||
				confirmPrompt("This will permanently remove the '%v' stack!", stackName.String()) {

				target, snapshot, err := getStack(stackName)
				if err != nil {
					return err
				}

				// Don't remove stacks that still have resources.
				if !force && snapshot != nil && len(snapshot.Resources) > 0 {
					return errors.Errorf(
						"'%v' still has resources; removal rejected; pass --force to override", stackName)
				}

				err = removeStack(target)
				if err != nil {
					return err
				}

				msg := fmt.Sprintf("%sStack '%s' has been removed!%s", colors.SpecAttention, stackName, colors.Reset)
				fmt.Println(colors.ColorizeText(msg))
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"By default, removal of a stack with resources will be rejected; this forces it")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
