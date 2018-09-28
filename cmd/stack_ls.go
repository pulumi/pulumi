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

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
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
			var packageFilter *tokens.PackageName
			if !allStacks {
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
				packageFilter = &proj.Name
			}

			// Get the current backend.
			b, err := currentBackend(display.Options{Color: cmdutil.GetGlobalColorization()})
			if err != nil {
				return err
			}

			// Get the current stack so we can print a '*' next to it.
			var current string
			if s, _ := state.CurrentStack(commandContext(), b); s != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later on instead of failing.
				current = s.Ref().String()
			}

			// List all of the stacks available.
			stackSummaries, err := b.ListStacks(commandContext(), packageFilter)
			if err != nil {
				return err
			}
			// Sort by stack name.
			sort.Slice(stackSummaries, func(i, j int) bool {
				return stackSummaries[i].Name().String() < stackSummaries[j].Name().String()
			})

			_, showURLColumn := b.(httpstate.Backend)

			// Devote 48 characters to the name width, unless there is a longer name.
			maxName := 47
			for _, summary := range stackSummaries {
				name := summary.Name().String()
				if len(name) > maxName {
					maxName = len(name)
				}
			}
			maxName++ // Account for adding the '*' to the currently selected stack.

			// Header string and formatting options to align columns.
			formatDirective := "%-" + strconv.Itoa(maxName) + "s %-24s %-18s"
			headers := []interface{}{"NAME", "LAST UPDATE", "RESOURCE COUNT"}
			if showURLColumn {
				formatDirective += " %s"
				headers = append(headers, "URL")
			}
			formatDirective = formatDirective + "\n"

			fmt.Printf(formatDirective, headers...)
			for _, summary := range stackSummaries {
				const none = "n/a"

				// Name column
				name := summary.Name().String()
				if name == current {
					name += "*"
				}

				// Last update column
				lastUpdate := none
				if stackLastUpdate := summary.LastUpdate(); stackLastUpdate != nil {
					lastUpdate = humanize.Time(*stackLastUpdate)
				}

				// ResourceCount column
				resourceCount := none
				if stackResourceCount := summary.ResourceCount(); stackResourceCount != nil {
					resourceCount = strconv.Itoa(*stackResourceCount)
				}

				// Render the columns.
				values := []interface{}{name, lastUpdate, resourceCount}
				if showURLColumn {
					var url string
					if httpBackend, ok := b.(httpstate.Backend); ok {
						if nameSuffix, err := httpBackend.StackConsoleURL(summary.Name()); err != nil {
							url = none
						} else {
							url = fmt.Sprintf("%s/%s", httpBackend.CloudURL(), nameSuffix)
						}
					}
					values = append(values, url)
				}

				fmt.Printf(formatDirective, values...)
			}

			return nil
		}),
	}
	cmd.PersistentFlags().BoolVarP(
		&allStacks, "all", "a", false, "List all stacks instead of just stacks for the current project")

	return cmd
}
