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

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackCmd() *cobra.Command {
	var showIDs bool
	var showURNs bool
	var showSecrets bool
	var stackName string
	var startTime string
	var showStackName bool

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
		Long: "Manage stacks\n" +
			"\n" +
			"A stack is a named update target, and a single project may have many of them.\n" +
			"Each stack has a configuration and update history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good update.\n",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stackName, true, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}
			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			if showStackName {
				fmt.Printf("%s\n", s.Ref().Name())
				return nil
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
					// If there is an in-flight operation, provide info.
					if currentOp := cs.CurrentOperation(); currentOp != nil {
						fmt.Printf("    Update in progress:\n")
						startTime = humanize.Time(time.Unix(currentOp.Started, 0))
						fmt.Printf("	Started: %v\n", startTime)
						fmt.Printf("	Requested By: %s\n", currentOp.Author)
					}
				}
			}

			if snap != nil {
				if t := snap.Manifest.Time; t.IsZero() && startTime == "" {
					fmt.Printf("    Last update time unknown\n")
				} else if startTime == "" {
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
				fmt.Printf("    No updates yet; run 'pulumi up'\n")
			}

			// Now show the resources.
			var rescnt int
			if snap != nil {
				rescnt = len(snap.Resources)
			}
			fmt.Printf("Current stack resources (%d):\n", rescnt)
			if rescnt == 0 {
				fmt.Printf("    No resources currently in this stack\n")
			} else {
				rows, ok := renderTree(snap, showURNs, showIDs)
				if !ok {
					for _, res := range snap.Resources {
						rows = append(rows, renderResourceRow(res, "", "    ", showURNs, showIDs))
					}
				}

				cmdutil.PrintTable(cmdutil.Table{
					Headers: []string{"TYPE", "NAME"},
					Rows:    rows,
					Prefix:  "    ",
				})

				outputs, err := getStackOutputs(snap, showSecrets)
				if err == nil {
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
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(
		&showIDs, "show-ids", "i", false, "Display each resource's provider-assigned unique ID")
	cmd.Flags().BoolVarP(
		&showURNs, "show-urns", "u", false, "Display each resource's Pulumi-assigned globally unique URN")
	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false, "Display stack outputs which are marked as secret in plaintext")
	cmd.Flags().BoolVar(
		&showStackName, "show-name", false, "Display only the stack name")

	cmd.AddCommand(newStackExportCmd())
	cmd.AddCommand(newStackGraphCmd())
	cmd.AddCommand(newStackImportCmd())
	cmd.AddCommand(newStackInitCmd())
	cmd.AddCommand(newStackLsCmd())
	cmd.AddCommand(newStackOutputCmd())
	cmd.AddCommand(newStackRmCmd())
	cmd.AddCommand(newStackSelectCmd())
	cmd.AddCommand(newStackTagCmd())
	cmd.AddCommand(newStackRenameCmd())
	cmd.AddCommand(newStackChangeSecretsProviderCmd())
	cmd.AddCommand(newStackHistoryCmd())

	return cmd
}

func printStackOutputs(outputs map[string]interface{}) {
	fmt.Printf("Current stack outputs (%d):\n", len(outputs))
	if len(outputs) == 0 {
		fmt.Printf("    No output values currently in this stack\n")
	} else {
		var outkeys []string
		for outkey := range outputs {
			outkeys = append(outkeys, outkey)
		}
		sort.Strings(outkeys)

		rows := []cmdutil.TableRow{}

		for _, key := range outkeys {
			rows = append(rows, cmdutil.TableRow{Columns: []string{key, stringifyOutput(outputs[key])}})
		}

		cmdutil.PrintTable(cmdutil.Table{
			Headers: []string{"OUTPUT", "VALUE"},
			Rows:    rows,
			Prefix:  "    ",
		})
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

type treeNode struct {
	res      *resource.State
	children []*treeNode
}

func renderNode(node *treeNode, padding, branch string, showURNs, showIDs bool, rows *[]cmdutil.TableRow) {
	padBranch := ""
	switch branch {
	case "├─ ":
		padBranch = "│  "
	case "└─ ":
		padBranch = "   "
	}
	childPadding := padding + padBranch

	infoBranch := "   "
	if len(node.children) > 0 {
		infoBranch = "│  "
	}
	infoPadding := childPadding + infoBranch

	*rows = append(*rows, renderResourceRow(node.res, padding+branch, infoPadding, showURNs, showIDs))

	for i, child := range node.children {
		childBranch := "├─ "
		if i == len(node.children)-1 {
			childBranch = "└─ "
		}
		renderNode(child, childPadding, childBranch, showURNs, showIDs, rows)
	}
}

func renderTree(snap *deploy.Snapshot, showURNs, showIDs bool) ([]cmdutil.TableRow, bool) {
	var root *treeNode
	var orphans []*treeNode
	nodes := make(map[resource.URN]*treeNode)
	for _, res := range snap.Resources {
		node, ok := nodes[res.URN]
		if !ok {
			node = &treeNode{res: res}
			nodes[res.URN] = node
		} else {
			node.res = res
		}

		switch {
		case res.Parent != "":
			p, ok := nodes[res.Parent]
			if !ok {
				p = &treeNode{}
				nodes[res.Parent] = p
			}
			p.children = append(p.children, node)
		case res.Type == resource.RootStackType:
			root = node
		default:
			orphans = append(orphans, node)
		}
	}

	// If we don't have a root, we can't display the tree.
	if root == nil {
		return nil, false
	}

	// Make sure all of our nodes have states.
	for _, n := range nodes {
		if n.res == nil {
			return nil, false
		}
	}

	// Parent all of our orphans to the root.
	root.children = append(root.children, orphans...)

	var rows []cmdutil.TableRow
	renderNode(root, "", "", showURNs, showIDs, &rows)
	return rows, true
}

func renderResourceRow(res *resource.State, prefix, infoPrefix string, showURN, showID bool) cmdutil.TableRow {
	columns := []string{prefix + string(res.Type), string(res.URN.Name())}
	additionalInfo := ""

	// If the ID and/or URN is requested, show it on the following line.  It would be nice to do
	// this on a single line, but this can get quite lengthy and so this formatting is better.
	if showURN {
		additionalInfo += fmt.Sprintf("    %sURN: %s\n", infoPrefix, res.URN)
	}
	if showID && res.ID != "" {
		additionalInfo += fmt.Sprintf("    %sID: %s\n", infoPrefix, res.ID)
	}

	return cmdutil.TableRow{Columns: columns, AdditionalInfo: additionalInfo}
}
