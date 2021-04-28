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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newPlanCmd() *cobra.Command {
	var showSames bool
	var stackName string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage plans",
		Long:  "Manage plans",
		Args:  cmdutil.ExactArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			planFilePath := args[0]

			s, err := requireStack(stackName, true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return err
			}

			dec, err := sm.Decrypter()
			if err != nil {
				return err
			}

			enc, err := sm.Encrypter()
			if err != nil {
				return err
			}

			plan, err := readPlan(planFilePath, dec, enc)
			if err != nil {
				return err
			}

			rows := renderPlan(plan, showSames, opts.Color)
			if len(rows) == 0 {
				fmt.Printf("No changes.\n")
				return nil
			}

			columnHeader := func(msg string) string {
				return opts.Color.Colorize(colors.Underline + colors.BrightBlue + msg + colors.Reset)
			}
			cmdutil.PrintTable(cmdutil.Table{
				Headers: []string{"", columnHeader("Type"), columnHeader("Name"), columnHeader("Plan")},
				Rows:    rows,
				Prefix:  "    ",
			})

			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")

	return cmd
}

type planNode struct {
	urn      resource.URN
	plan     *deploy.ResourcePlan
	children []*planNode
}

func renderPlanNode(node *planNode, padding, branch string, rows *[]cmdutil.TableRow, color colors.Colorization) {
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

	*rows = append(*rows, renderResourcePlan(node.urn, node.plan, padding+branch, infoPadding, color))

	for i, child := range node.children {
		childBranch := "├─ "
		if i == len(node.children)-1 {
			childBranch = "└─ "
		}
		renderPlanNode(child, childPadding, childBranch, rows, color)
	}
}

func isSame(root *planNode) bool {
	return root.plan == nil || len(root.plan.Ops) == 1 && root.plan.Ops[0] == deploy.OpSame
}

func pruneSames(root *planNode) *planNode {
	var children []*planNode
	for _, child := range root.children {
		child = pruneSames(child)
		if child != nil {
			children = append(children, child)
		}
	}

	if len(children) == 0 && isSame(root) {
		return nil
	}

	root.children = children
	return root
}

func renderPlan(plan deploy.Plan, showSames bool, color colors.Colorization) []cmdutil.TableRow {
	var root *planNode
	var orphans []*planNode
	nodes := map[resource.URN]*planNode{}
	for urn, plan := range plan {
		node, ok := nodes[urn]
		if !ok {
			node = &planNode{urn: urn, plan: plan}
			nodes[urn] = node
		} else {
			node.plan = plan
		}

		switch {
		case plan.Goal.Parent != "":
			p, ok := nodes[plan.Goal.Parent]
			if !ok {
				p = &planNode{urn: plan.Goal.Parent}
				nodes[plan.Goal.Parent] = p
			}
			p.children = append(p.children, node)
		case urn.IsValid() && urn.Type() == resource.RootStackType:
			root = node
		default:
			orphans = append(orphans, node)
		}
	}

	// If we don't have a root, synthesize one.
	if root == nil {
		root = &planNode{}
	}

	// Parent all of our orphans to the root.
	root.children = append(root.children, orphans...)

	// Remove any leaf sames unless showSames is set.
	if !showSames {
		root = pruneSames(root)
		if root == nil {
			return nil
		}
	}

	var rows []cmdutil.TableRow
	renderPlanNode(root, "", "", &rows, color)
	return rows
}

func renderResourcePlan(urn resource.URN, plan *deploy.ResourcePlan, prefix, infoPrefix string, color colors.Colorization) cmdutil.TableRow {
	displayOp := deploy.OpSame
	if plan != nil {
		for _, op := range plan.Ops {
			displayOp = op
			if op == deploy.OpReplace {
				break
			}
		}
	}

	var typ, name string
	switch {
	case urn == "":
		typ, name = "<missing>", "<missing>"
	case urn.IsValid():
		typ, name = string(urn.Type()), string(urn.Name())
	default:
		typ, name = "<invalid>", "<invalid>"
	}

	columns := []string{
		color.Colorize(displayOp.Prefix()),
		prefix + typ,
		name,
		color.Colorize(displayOp.Color() + string(displayOp) + colors.Reset),
	}
	return cmdutil.TableRow{Columns: columns}
}
