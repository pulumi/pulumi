// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Import a deployment from standard in into an existing stack.\n",
		Long: "Import a deployment from standard in into an existing stack.\n" +
			"\n" +
			"A deployment that was exported from a stack using `pulumi stack export` and\n" +
			"hand-edited to correct inconsistencies due to failed updates, manual changes\n" +
			"to cloud resources, etc. can be reimported to the stack using this command.\n" +
			"The updated deployment will be read from standard in.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Fetch the current stack and import a deployment.
			s, err := requireCurrentStack()
			if err != nil {
				return err
			}

			var deployment json.RawMessage
			if err := json.NewDecoder(os.Stdin).Decode(&deployment); err != nil {
				return err
			}

			if err = s.ImportDeployment(deployment); err != nil {
				return errors.Wrap(err, "could not import deployment")
			}
			fmt.Printf("Import successful.\n")
			return nil
		}),
	}
}
