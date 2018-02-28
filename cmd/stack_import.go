// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackImportCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "import",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Import a deployment from standard in into an existing stack",
		Long: "Import a deployment from standard in into an existing stack.\n" +
			"\n" +
			"A deployment that was exported from a stack using `pulumi stack export` and\n" +
			"hand-edited to correct inconsistencies due to failed updates, manual changes\n" +
			"to cloud resources, etc. can be reimported to the stack using this command.\n" +
			"The updated deployment will be read from standard in.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Fetch the current stack and import a deployment.
			s, err := requireCurrentStack(false)
			if err != nil {
				return err
			}

			// Read the checkpoint from stdin.
			var deployment apitype.Deployment
			if err = json.NewDecoder(os.Stdin).Decode(&deployment); err != nil {
				return err
			}

			// Check to see if the checkpoint contains resources with names other than the selected stack.  This can
			// catch errors wherein someone imports the wrong stack's checkpoint (which can seriously hork things).
			var result error
			for _, res := range deployment.Resources {
				if res.URN.Stack() != s.Name() {
					msg := fmt.Sprintf("resource '%s' is from a different stack (%s != %s)",
						res.URN, res.URN.Stack(), s.Name())
					if force {
						// If --force was passed, just issue a warning and proceed anyway.
						cmdutil.Diag().Warningf(diag.Message(msg))
					} else {
						// Otherwise, gather up an error so that we can quit before doing damage.
						msg += "; importing this could be dangerous, pass --force to proceed anyway"
						result = multierror.Append(result, errors.New(msg))
					}
				}
			}
			if result != nil {
				return result
			}

			// Now perform the deployment.
			if err = s.ImportDeployment(&deployment); err != nil {
				return errors.Wrap(err, "could not import deployment")
			}
			fmt.Printf("Import successful.\n")
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Force the import to occur, even if apparent errors are discovered beforehand (not recommended)")

	return cmd
}
