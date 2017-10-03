// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known environments",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			currentEnv, err := getCurrentEnv()
			if err != nil {
				// If we couldn't figure out the current environment, just don't print the '*' later
				// on instead of failing.
				currentEnv = tokens.QName("")
			}

			envs, err := lumiEngine.GetEnvironments()
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT")
			for _, env := range envs {
				// Now print out the name, last deployment time (if any), and resources (if any).
				lastDeploy := "n/a"
				resourceCount := "n/a"
				if env.Checkpoint.Latest != nil {
					lastDeploy = env.Checkpoint.Latest.Time.String()
				}
				if env.Snapshot != nil {
					resourceCount = strconv.Itoa(len(env.Snapshot.Resources))
				}
				display := env.Name
				if env.Name == currentEnv {
					display += "*" // fancify the current environment.
				}
				fmt.Printf("%-20s %-48s %-12s\n", display, lastDeploy, resourceCount)
			}

			return nil
		}),
	}
}
