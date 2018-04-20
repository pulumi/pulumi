// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newStackLsCmd() *cobra.Command {
	var allStacks bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Ensure we are in a project; if not, we will fail.
			projPath, err := workspace.DetectProjectPath()
			if err != nil {
				return errors.Wrapf(err, "could not detect current project")
			} else if projPath == "" {
				return errors.New("no Pulumi.yaml found; please run this command in a project directory")
			}

			proj, err := workspace.LoadProject(projPath)
			if err != nil {
				return errors.Wrap(err, "could not load current project")
			}

			// Get a list of all known backends, as we will query them all.
			b, err := currentBackend()
			if err != nil {
				return err
			}

			// Get the current stack so we can print a '*' next to it.
			var current string
			if s, _ := state.CurrentStack(b); s != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later on instead of failing.
				current = s.Name().String()
			}

			var packageFilter *tokens.PackageName
			if !allStacks {
				packageFilter = &proj.Name
			}

			// Now produce a list of summaries, and enumerate them sorted by name.
			var result error
			var stackNames []string
			stacks := make(map[string]backend.Stack)
			bs, err := b.ListStacks(packageFilter)
			if err != nil {
				return err
			}
			for _, stack := range bs {
				name := stack.Name().String()
				stacks[name] = stack
				stackNames = append(stackNames, name)
			}
			sort.Strings(stackNames)

			// Devote 48 characters to the name width, unless there is a longer name.
			maxname := 48
			for _, name := range stackNames {
				if len(name) > maxname {
					maxname = len(name)
				}
			}

			fmt.Printf("%-"+strconv.Itoa(maxname)+"s %-24s %-18s %-25s\n",
				"NAME", "LAST UPDATE", "RESOURCE COUNT", "CLOUD")
			for _, name := range stackNames {
				// Mark the name as current '*' if we've selected it.
				stack := stacks[name]
				if name == current {
					name += "*"
				}

				// Get last deployment info, provided that it exists.
				none := "n/a"
				lastUpdate := none
				resourceCount := none
				if snap := stack.Snapshot(); snap != nil {
					if t := snap.Manifest.Time; !t.IsZero() {
						lastUpdate = humanize.Time(t)
					}
					resourceCount = strconv.Itoa(len(snap.Resources))
				}

				// Print out the cloud URL.
				var cloudInfo string
				if cs, ok := stack.(cloud.Stack); ok {
					cloudInfo = fmt.Sprintf("%s:%s/%s", cs.CloudURL(), cs.OrgName(), cs.CloudName())
				} else {
					cloudInfo = none
				}

				fmt.Printf("%-"+strconv.Itoa(maxname)+"s %-24s %-18s %-25s\n",
					name, lastUpdate, resourceCount, cloudInfo)
			}

			return result
		}),
	}
	cmd.PersistentFlags().BoolVarP(
		&allStacks, "all", "a", false, "List all stacks instead of just stacks for the current project")

	return cmd
}
