// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackCmd() *cobra.Command {
	var showIDs bool
	var showURNs bool
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
		Long: "Manage stacks\n" +
			"\n" +
			"An stack is a named update target, and a single project may have many of them.\n" +
			"Each stack has a configuration and update history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good update.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName, err := getCurrentStack()
			if err != nil {
				return err
			}

			_, config, snapshot, err := getStack(stackName)
			if err != nil {
				return err
			}

			fmt.Printf("Current stack is %v\n", stackName)
			fmt.Printf("    (use `pulumi stack select` to change stack; `pulumi stack ls` lists known ones)\n")

			if err != nil {
				return err
			}
			if snapshot != nil {
				fmt.Printf("Last update at %v\n", snapshot.Time)
			}
			if len(config) > 0 {
				fmt.Printf("%v configuration variables set (see `pulumi config` for details)\n", len(config))
			}
			var stackResource *resource.State
			if snapshot == nil || len(snapshot.Resources) == 0 {
				fmt.Printf("No resources currently in this stack\n")
			} else {
				fmt.Printf("%v resources currently in this stack:\n", len(snapshot.Resources))
				fmt.Printf("\n")
				fmt.Printf("%-48s %s\n", "TYPE", "NAME")
				for _, res := range snapshot.Resources {
					if res.Type == stack.RootPulumiStackTypeName {
						stackResource = res
						continue
					}
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
				if stackResource != nil {
					fmt.Printf("\n")
					// Note: Currently, components place their output properties into the `Inputs` of the resource, so
					// we need to extract the outputs from the `Inputs`.
					outputs := stack.SerializeResource(stackResource).Inputs
					if len(outputs) == 0 {
						fmt.Printf("No output values currently in this stack\n")
					} else {
						fmt.Printf("%v output values currently in this stack:\n", len(outputs))
						fmt.Printf("\n")
						fmt.Printf("%-48s %s\n", "OUTPUT", "VALUE")
						for key, val := range outputs {
							fmt.Printf("%-48s %s\n", key, val)
						}
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

	cmd.AddCommand(newStackInitCmd())
	cmd.AddCommand(newStackLsCmd())
	cmd.AddCommand(newStackRmCmd())
	cmd.AddCommand(newStackSelectCmd())

	return cmd
}
