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
	"fmt"
	"sort"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
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

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Get a list of all known backends, as we will query them all.
			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			// Get the current stack so we can print a '*' next to it.
			var current string
			if s, _ := state.CurrentStack(commandContext(), b); s != nil {
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
			bs, err := b.ListStacks(commandContext(), packageFilter)
			if err != nil {
				return err
			}
			_, showURLColumn := b.(httpstate.Backend)

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

			// We have to fault in snapshots for all the stacks we are going to list here, because that's the easiest
			// way to get the last update time and the resource count.  Since this is an expensive operation, we'll
			// do it before printing any output so the latency happens all at once instead of line by line.
			//
			// TODO[pulumi/pulumi-service#1530]: We need a lighterweight way of fetching just the specific information
			// we want to display here.
			for _, name := range stackNames {
				stack := stacks[name]
				_, err := stack.Snapshot(commandContext())
				contract.IgnoreError(err) // If we couldn't get snapshot for the stack don't fail the overall listing.
			}

			formatDirective := "%-" + strconv.Itoa(maxname) + "s %-24s %-18s"
			headers := []interface{}{"NAME", "LAST UPDATE", "RESOURCE COUNT"}

			if showURLColumn {
				formatDirective += " %s"
				headers = append(headers, "URL")
			}

			formatDirective = formatDirective + "\n"

			fmt.Printf(formatDirective, headers...)
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
				snap, err := stack.Snapshot(commandContext())
				contract.IgnoreError(err) // If we couldn't get snapshot for the stack don't fail the overall listing.

				if snap != nil {
					if t := snap.Manifest.Time; !t.IsZero() {
						lastUpdate = humanize.Time(t)
					}
					resourceCount = strconv.Itoa(len(snap.Resources))
				}

				values := []interface{}{name, lastUpdate, resourceCount}
				if showURLColumn {
					var url string
					if cs, ok := stack.(httpstate.Stack); ok {
						if u, urlErr := cs.ConsoleURL(); urlErr == nil {
							url = u
						}
					}
					if url == "" {
						url = none
					}
					values = append(values, url)
				}

				fmt.Printf(formatDirective, values...)
			}

			return result
		}),
	}
	cmd.PersistentFlags().BoolVarP(
		&allStacks, "all", "a", false, "List all stacks instead of just stacks for the current project")

	return cmd
}
