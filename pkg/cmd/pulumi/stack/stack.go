// Copyright 2016-2024, Pulumi Corporation.
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

package stack

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackArgs struct {
	showIDs                bool
	showURNs               bool
	showSecrets            bool
	startTime              string
	showStackName          bool
	fullyQualifyStackNames bool
}

func NewStackCmd() *cobra.Command {
	var stackName string
	args := stackArgs{}

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks and view stack state",
		Long: "Manage stacks and view stack state\n" +
			"\n" +
			"A stack is a named update target, and a single project may have many of them.\n" +
			"Each stack has a configuration and update history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good update.\n",
		Args: cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				stackName,
				OfferNew,
				opts,
			)
			if err != nil {
				return err
			}

			args.fullyQualifyStackNames = cmdutil.FullyQualifyStackNames
			return runStack(ctx, s, os.Stdout, args)
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(
		&args.showIDs, "show-ids", "i", false, "Display each resource's provider-assigned unique ID")
	cmd.Flags().BoolVarP(
		&args.showURNs, "show-urns", "u", false, "Display each resource's Pulumi-assigned globally unique URN")
	cmd.Flags().BoolVar(
		&args.showSecrets, "show-secrets", false, "Display stack outputs which are marked as secret in plaintext")
	cmd.Flags().BoolVar(
		&args.showStackName, "show-name", false, "Display only the stack name")

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
	cmd.AddCommand(newStackUnselectCmd())

	return cmd
}

func runStack(ctx context.Context, s backend.Stack, out io.Writer, args stackArgs) error {
	if args.showStackName {
		if args.fullyQualifyStackNames {
			fmt.Fprintln(out, s.Ref().String())
		} else {
			fmt.Fprintln(out, s.Ref().Name())
		}
		return nil
	}

	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Current stack is %s:\n", s.Ref())

	be := s.Backend()
	cloudBe, isCloud := be.(httpstate.Backend)
	if !isCloud || cloudBe.CloudURL() != httpstate.PulumiCloudURL {
		fmt.Fprintf(out, "    Managed by %s\n", be.Name())
	}
	if isCloud {
		if cs, ok := s.(httpstate.Stack); ok {
			fmt.Fprintf(out, "    Owner: %s\n", cs.OrgName())

			if currentOp := cs.CurrentOperation(); currentOp != nil {
				fmt.Fprintf(out, "    Update in progress:\n")
				args.startTime = humanize.Time(time.Unix(currentOp.Started, 0))
				fmt.Fprintf(out, "	Started: %v\n", args.startTime)
				fmt.Fprintf(out, "	Requested By: %s\n", currentOp.Author)
			}
		}
	}

	if snap != nil {
		t := snap.Manifest.Time.Local()
		if args.startTime == "" {
			if !t.IsZero() && t.Before(time.Now()) {
				fmt.Fprintf(out, "    Last updated: %s (%v)\n", humanize.Time(t), t)
			}
		}
		if snap.Manifest.Version != "" {
			fmt.Fprintf(out, "    Pulumi version used: %s\n", snap.Manifest.Version)
		}
		for _, p := range snap.Manifest.Plugins {
			var pluginVersion string
			if p.Version == nil {
				pluginVersion = "?"
			} else {
				pluginVersion = p.Version.String()
			}
			fmt.Fprintf(out, "    Plugin %s [%s] version: %s\n", p.Name, p.Kind, pluginVersion)
		}
	} else {
		fmt.Fprintf(out, "    No updates yet; run `pulumi up`\n")
	}

	var resourceCount int
	if snap != nil {
		resourceCount = len(snap.Resources)
	}
	fmt.Fprintf(out, "Current stack resources (%d):\n", resourceCount)
	if resourceCount == 0 {
		fmt.Fprintf(out, "    No resources currently in this stack\n")
	} else {
		rows, ok := renderTree(snap, args.showURNs, args.showIDs)
		if !ok {
			for _, res := range snap.Resources {
				rows = append(rows, renderResourceRow(res, "", "    ", args.showURNs, args.showIDs))
			}
		}

		ui.PrintTable(cmdutil.Table{
			Headers: []string{"TYPE", "NAME"},
			Rows:    rows,
			Prefix:  "    ",
		}, nil)

		outputs, err := getStackOutputs(snap, args.showSecrets)
		if err == nil {
			fmt.Fprintf(out, "\n")
			_ = fprintStackOutputs(os.Stdout, outputs)
		}

		if args.showSecrets {
			Log3rdPartySecretsProviderDecryptionEvent(ctx, s, "", "pulumi stack")
		}
	}

	if isCloud {
		if consoleURL, err := cloudBe.StackConsoleURL(s.Ref()); err == nil {
			fmt.Fprintf(out, "\n")
			fmt.Fprintf(out, "More information at: %s\n", consoleURL)
		}
	}

	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Use `pulumi stack select` to change stack; `pulumi stack ls` lists known ones\n")

	return nil
}

func fprintStackOutputs(w io.Writer, outputs map[string]interface{}) error {
	_, err := fmt.Fprintf(w, "Current stack outputs (%d):\n", len(outputs))
	if err != nil {
		return err
	}

	if len(outputs) == 0 {
		_, err = fmt.Fprintf(w, "    No output values currently in this stack\n")
		return err
	}

	outKeys := slice.Prealloc[string](len(outputs))
	for v := range outputs {
		outKeys = append(outKeys, v)
	}
	sort.Strings(outKeys)

	rows := []cmdutil.TableRow{}
	for _, key := range outKeys {
		rows = append(rows, cmdutil.TableRow{Columns: []string{key, stringifyOutput(outputs[key])}})
	}

	return cmdutil.FprintTable(w, cmdutil.Table{
		Headers: []string{"OUTPUT", "VALUE"},
		Rows:    rows,
		Prefix:  "    ",
	})
}

// stringifyOutput formats an output value for presentation to a user. We use JSON formatting, except in the case
// of top level strings, where we just return the raw value.
func stringifyOutput(v interface{}) string {
	s, ok := v.(string)
	if ok {
		return s
	}

	o, err := ui.MakeJSONString(v, false /* single line */)
	if err != nil {
		return "error: could not format value"
	}

	return o
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
		case res.Type == resource.RootStackType && res.Parent == "":
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
	columns := []string{prefix + string(res.Type), res.URN.Name()}
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
