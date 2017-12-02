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
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireCurrentStack()
			if err != nil {
				return err
			}

			// First print general info about the current stack.
			fmt.Printf("Current stack is %v:\n", s.Name)
			fmt.Printf("    Managed by %s", s.Backend.Name())
			if s.Backend.IsCloud() {
				fmt.Printf(" ☁️")
			}
			fmt.Printf("\n")

			if s.Snapshot != nil {
				if t := s.Snapshot.Manifest.Time; t.IsZero() {
					fmt.Printf("    Last update time unknown\n")
				} else {
					fmt.Printf("    Last updated at %v\n", s.Snapshot.Manifest.Time)
				}
				var cliver string
				if s.Snapshot.Manifest.Version == "" {
					cliver = "?"
				} else {
					cliver = s.Snapshot.Manifest.Version
				}
				fmt.Printf("    Pulumi version %s\n", cliver)
				for _, plugin := range s.Snapshot.Manifest.Plugins {
					var plugver string
					if plugin.Version == "" {
						plugver = "?"
					} else {
						plugver = plugin.Version
					}
					fmt.Printf("    Plugin %s [%s] version %s\n", plugin.Name, plugin.Type, plugver)
				}
			} else {
				fmt.Printf("    No updates yet; run 'pulumi update'\n")
			}

			if len(s.Config) > 0 {
				fmt.Printf("    %v configuration variables set (see `pulumi config` for details)\n", len(s.Config))
			}
			fmt.Printf("\n")

			// Now show the resources.
			var rescnt int
			if s.Snapshot != nil {
				rescnt = len(s.Snapshot.Resources)
			}
			fmt.Printf("Current stack resources (%d):\n", rescnt)
			if rescnt == 0 {
				fmt.Printf("    No resources currently in this stack\n")
			} else {
				fmt.Printf("    %-48s %s\n", "TYPE", "NAME")
				var stackResource *resource.State
				for _, res := range s.Snapshot.Resources {
					if res.Type == stack.RootPulumiStackTypeName {
						stackResource = res
					} else {
						fmt.Printf("    %-48s %s\n", res.Type, res.URN.Name())

						// If the ID and/or URN is requested, show it on the following line.  It would be nice to do
						// this on a single line, but this can get quite lengthy and so this formatting is better.
						if showURNs {
							fmt.Printf("        URN: %s\n", res.URN)
						}
						if showIDs && res.ID != "" {
							fmt.Printf("        ID: %s\n", res.ID)
						}
					}
				}

				// Print out the output properties for the stack, if present.
				if stackResource != nil {
					fmt.Printf("\n")
					outputs := stack.SerializeResource(stackResource).Outputs
					fmt.Printf("Current stack outputs (%d):\n", len(outputs))
					if len(outputs) == 0 {
						fmt.Printf("    No output values currently in this stack\n")
					} else {
						fmt.Printf("    %-48s %s\n", "OUTPUT", "VALUE")
						for key, val := range outputs {
							fmt.Printf("    %-48s %s\n", key, val)
						}
					}
				}
			}
			fmt.Printf("\n")

			fmt.Printf("Use `pulumi stack select` to change stack; `pulumi stack ls` lists known ones\n")

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
