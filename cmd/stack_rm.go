// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm [<stack-name>]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Remove an stack and its configuration",
		Long: "Remove an stack and its configuration\n" +
			"\n" +
			"This command removes an stack and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the stack will no longer be available for updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Use the stack provided or, if missing, default to the current one.
			var stack tokens.QName
			if len(args) > 0 {
				stack = tokens.QName(args[0])
			}
			s, err := requireStack(stack, false)
			if err != nil {
				return err
			}

			// Ensure the user really wants to do this.
			if !yes && !confirmPrompt("This will permanently remove the '%v' stack!", string(s.Name())) {
				return errors.New("confirmation declined")
			}

			hasResources, err := s.Remove(force)
			if err != nil {
				if hasResources {
					return errors.Errorf(
						"'%v' still has resources; removal rejected; pass --force to override", s.Name())
				}
				return err
			}

			err = deleteAllStackConfiguration(s.Name())
			if err != nil {
				return err
			}

			msg := fmt.Sprintf("%sStack '%s' has been removed!%s", colors.SpecAttention, s.Name(), colors.Reset)
			fmt.Println(colors.ColorizeText(msg))

			return state.SetCurrentStack("")
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
