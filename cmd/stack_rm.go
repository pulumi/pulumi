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
	if usePulumiCloudCommands() {
		return newCloudStackRmCmd()
	}
	return newFAFStackRmCmd()
}

func newFAFStackRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm <stack>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove an stack and its configuration",
		Long: "Remove an stack and its configuration\n" +
			"\n" +
			"This command removes an stack and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the stack will no longer be available for updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(args[0])

			// Ensure the user really wants to do this.
			if yes ||
				confirmPrompt("This will permanently remove the '%v' stack!", stackName.String()) {

				name, _, snapshot, err := getStack(stackName)
				if err != nil {
					return err
				}

				// Don't remove stacks that still have resources.
				if !force && snapshot != nil && len(snapshot.Resources) > 0 {
					return errors.Errorf(
						"'%v' still has resources; removal rejected; pass --force to override", stackName)
				}

				err = removeStack(name)
				if err != nil {
					return err
				}
				printStackRemoved(stackName)
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

func newCloudStackRmCmd() *cobra.Command {
	var yes bool

	var cmd = &cobra.Command{
		Use:   "rm <stack>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove an stack and its configuration",
		Long: "Remove an stack and its configuration\n" +
			"\n" +
			"This command removes an stack and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the stack will no longer be available for updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName := tokens.QName(args[0])

			// Ensure the user really wants to do this.
			if yes || confirmPrompt("This will permanently remove the '%v' stack!", stackName.String()) {
				// Look up the owner, repository, and project from the workspace and nearest package.
				w, err := newWorkspace()
				if err != nil {
					return err
				}
				projID, err := getCloudProjectIdentifier(w)
				if err != nil {
					return err
				}

				// Query all stacks for the project on Pulumi.
				path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s",
					projID.Owner, projID.Repository, projID.Project, string(stackName))
				if err = pulumiRESTCall("DELETE", path, nil, nil); err != nil {
					return err
				}

				// Delete the reference to the stack in the current workspace settings.
				err = removeStack(stackName)
				if err != nil {
					return err
				}
				printStackRemoved(stackName)
			}

			return nil
		}),
	}

	// Unlike the local variant of this command, there is no --force argument. You cannot delete
	// a stack hosted by Pulumi without first removing its resources.
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}

func printStackRemoved(stackName tokens.QName) {
	msg := fmt.Sprintf("%sStack '%s' has been removed!%s", colors.SpecAttention, stackName, colors.Reset)
	fmt.Println(colors.ColorizeText(msg))
}
