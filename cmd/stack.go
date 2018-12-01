// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
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
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireCurrentStack(true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}
			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			// First print general info about the current stack.
			fmt.Printf("Current stack is %s:\n", s.Ref())

			be := s.Backend()
			cloudBe, isCloud := be.(httpstate.Backend)
			if !isCloud || cloudBe.CloudURL() != httpstate.PulumiCloudURL {
				fmt.Printf("    Managed by %s\n", be.Name())
			}
			if isCloud {
				if cs, ok := s.(httpstate.Stack); ok {
					fmt.Printf("    Owner: %s\n", cs.OrgName())
				}
			}

			if snap != nil {
				if t := snap.Manifest.Time; t.IsZero() {
					fmt.Printf("    Last update time unknown\n")
				} else {
					fmt.Printf("    Last updated: %s (%v)\n", humanize.Time(t), t)
				}
				var cliver string
				if snap.Manifest.Version == "" {
					cliver = "?"
				} else {
					cliver = snap.Manifest.Version
				}
				fmt.Printf("    Pulumi version: %s\n", cliver)
				for _, plugin := range snap.Manifest.Plugins {
					var plugver string
					if plugin.Version == nil {
						plugver = "?"
					} else {
						plugver = plugin.Version.String()
					}
					fmt.Printf("    Plugin %s [%s] version: %s\n", plugin.Name, plugin.Kind, plugver)
				}
			} else {
				fmt.Printf("    No updates yet; run 'pulumi update'\n")
			}

			cfg := s.Config()
			if cfg != nil && len(cfg) > 0 {
				fmt.Printf("    %v configuration variables set (see `pulumi config` for details)\n", len(cfg))
			}
			fmt.Printf("\n")

			// Now show the resources.
			var rescnt int
			if snap != nil {
				rescnt = len(snap.Resources)
			}
			fmt.Printf("Current stack resources (%d):\n", rescnt)
			if rescnt == 0 {
				fmt.Printf("    No resources currently in this stack\n")
			} else {
				table := []cmdutil.TableRow{}
				table = append(table, cmdutil.TableRow{Columns: []string{"TYPE", "NAME"}})

				for _, res := range snap.Resources {
					columns := []string{string(res.Type), string(res.URN.Name())}
					additionalInfo := ""

					// If the ID and/or URN is requested, show it on the following line.  It would be nice to do
					// this on a single line, but this can get quite lengthy and so this formatting is better.
					if showURNs {
						additionalInfo += fmt.Sprintf("        URN: %s\n", res.URN)
					}
					if showIDs && res.ID != "" {
						additionalInfo += fmt.Sprintf("        ID: %s\n", res.ID)
					}

					table = append(table, cmdutil.TableRow{Columns: columns, AdditionalInfo: additionalInfo})
				}

				cmdutil.PrintTable(table)

				// Print out the output properties for the stack, if present.
				if res, outputs := stack.GetRootStackResource(snap); res != nil {
					fmt.Printf("\n")
					printStackOutputs(outputs)
				}
			}

			// Add a link to the pulumi.com console page for this stack, if it has one.
			if cs, ok := s.(httpstate.Stack); ok {
				if consoleURL, err := cs.ConsoleURL(); err == nil {
					fmt.Printf("\n")
					fmt.Printf("More information at: %s\n", consoleURL)
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

	cmd.AddCommand(newStackExportCmd())
	cmd.AddCommand(newStackGraphCmd())
	cmd.AddCommand(newStackImportCmd())
	cmd.AddCommand(newStackInitCmd())
	cmd.AddCommand(newStackLsCmd())
	cmd.AddCommand(newStackOutputCmd())
	cmd.AddCommand(newStackRmCmd())
	cmd.AddCommand(newStackSelectCmd())

	return cmd
}

func printStackOutputs(outputs map[string]interface{}) {
	fmt.Printf("Current stack outputs (%d):\n", len(outputs))
	if len(outputs) == 0 {
		fmt.Printf("    No output values currently in this stack\n")
	} else {
		maxkey := 48
		var outkeys []string
		for outkey := range outputs {
			if len(outkey) > maxkey {
				maxkey = len(outkey)
			}
			outkeys = append(outkeys, outkey)
		}
		sort.Strings(outkeys)
		fmt.Printf("    %-"+strconv.Itoa(maxkey)+"s %s\n", "OUTPUT", "VALUE")
		for _, key := range outkeys {
			fmt.Printf("    %-"+strconv.Itoa(maxkey)+"s %s\n", key, stringifyOutput(outputs[key]))
		}
	}
}

// stringifyOutput formats an output value for presentation to a user. We use JSON formatting, except in the case
// of top level strings, where we just return the raw value.
func stringifyOutput(v interface{}) string {
	s, ok := v.(string)
	if ok {
		return s
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "error: could not format value"
	}

	return string(b)
}
