// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvCmd() *cobra.Command {
	var showIDs bool
	var showURNs bool
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage target environments",
		Long: "Manage target environments\n" +
			"\n" +
			"An environment is a named update target, and a single project may have many of them.\n" +
			"Each environment has a configuration and update history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good update.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			envName, err := getCurrentEnv()
			if err != nil {
				return err
			}

			envInfo, err := lumiEngine.GetEnvironmentInfo(envName)
			if err != nil {
				return err
			}
			config, err := getConfiguration(envName)
			if err != nil {
				return err
			}

			checkpoint := envInfo.Checkpoint
			snapshot := envInfo.Snapshot

			fmt.Printf("Current environment is %v\n", envInfo.Name)
			fmt.Printf("    (use `pulumi env select` to change environments; `pulumi env ls` lists known ones)\n")

			if err != nil {
				return err
			}
			if checkpoint.Latest != nil {
				fmt.Printf("Last update at %v\n", checkpoint.Latest.Time)
				if checkpoint.Latest.Info != nil {
					info, err := json.MarshalIndent(checkpoint.Latest.Info, "    ", "    ")
					if err != nil {
						return err
					}
					fmt.Printf("Additional update info:\n    %s\n", string(info))
				}
			}
			if len(config) > 0 {
				fmt.Printf("%v configuration variables set (see `pulumi config` for details)\n", len(config))
			}
			if snapshot == nil || len(snapshot.Resources) == 0 {
				fmt.Printf("No resources currently in this environment\n")
			} else {
				fmt.Printf("%v resources currently in this environment:\n", len(snapshot.Resources))
				fmt.Printf("\n")
				fmt.Printf("%-48s %s\n", "TYPE", "NAME")
				for _, res := range snapshot.Resources {
					fmt.Printf("%-48s %s\n", res.Type, res.URN.Name())

					// If the ID and/or URN is requested, show it on the following line.  It would be nice to do this
					// on a single line, but they can get quite lengthy and so this formatting makes more sense.
					if showIDs {
						fmt.Printf("\tID: %s\n", res.ID)
					}
					if showURNs {
						fmt.Printf("\tURN: %s\n", res.URN)
					}
				}
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&showIDs, "show-ids", "i", false, "Display each resource's provider-assigned unique ID")
	cmd.PersistentFlags().BoolVarP(
		&showURNs, "show-urns", "u", false, "Display each resource's Pulumi-assigned globally unique URN")

	cmd.AddCommand(newEnvInitCmd())
	cmd.AddCommand(newEnvLsCmd())
	cmd.AddCommand(newEnvRmCmd())
	cmd.AddCommand(newEnvSelectCmd())

	return cmd
}
