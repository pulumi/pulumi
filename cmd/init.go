// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var owner string
	var name string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Pulumi repository",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			repo, err := workspace.GetRepository(cwd)
			if err != nil && err != workspace.ErrNoRepository {
				return err
			}
			if err == workspace.ErrNoRepository {
				// No existing repository, so we'll need to create one
				repo = workspace.NewRepository(cwd)

				detectedOwner, detectedName, detectErr := detectOwnerAndName(cwd)
				if detectErr != nil {
					return detectErr
				}
				repo.Owner = detectedOwner
				repo.Name = detectedName
			}

			// explicit command line arguments should overwrite any existing values
			if owner != "" {
				repo.Owner = owner
			}

			if name != "" {
				repo.Name = name
			}

			err = repo.Save()
			if err != nil {
				return err
			}

			fmt.Printf("Initialized Pulumi repository\n")

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&owner, "owner", "",
		"Override the repository owner; default is taken from current Git repository or username")
	cmd.PersistentFlags().StringVar(
		&name, "name", "",
		"Override the repository name; default is taken from current Git repository or current working directory")

	return cmd
}
