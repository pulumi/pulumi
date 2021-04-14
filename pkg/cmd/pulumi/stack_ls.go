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
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newStackLsCmd() *cobra.Command {
	var jsonOut bool
	var allStacks bool
	var orgFilter string
	var projFilter string
	var tagFilter string

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List stacks",
		Long: "List stacks\n" +
			"\n" +
			"This command lists stacks. By default only stacks with the same project name as the\n" +
			"current workspace will be returned. By passing --all, all stacks you have access to\n" +
			"will be listed.\n" +
			"\n" +
			"Results may be further filtered by passing additional flags. Tag filters may include\n" +
			"the tag name as well as the tag value, separated by an equals sign. For example\n" +
			"'environment=production' or just 'gcp:project'.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Build up the stack filters. We do not support accepting empty strings as filters
			// from command-line arguments, though the API technically supports it.
			strPtrIfSet := func(s string) *string {
				if s != "" {
					return &s
				}
				return nil
			}
			filter := backend.ListStacksFilter{
				Organization: strPtrIfSet(orgFilter),
				Project:      strPtrIfSet(projFilter),
			}
			if tagFilter != "" {
				tagName, tagValue := parseTagFilter(tagFilter)
				filter.TagName = &tagName
				filter.TagValue = tagValue
			}

			// If --all is not specified, default to filtering to just the current project.
			if !allStacks && projFilter == "" {
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
				projName := string(proj.Name)
				filter.Project = &projName
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
			stackSummaries, err := b.ListStacks(commandContext(), filter)
			if err != nil {
				return err
			}
			// Sort by stack name.
			sort.Slice(stackSummaries, func(i, j int) bool {
				return stackSummaries[i].Name().String() < stackSummaries[j].Name().String()
			})

			if jsonOut {
				return formatStackSummariesJSON(b, current, stackSummaries)
			}

			return formatStackSummariesConsole(b, current, stackSummaries)
		}),
	}
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")

	cmd.PersistentFlags().BoolVarP(
		&allStacks, "all", "a", false, "List all stacks instead of just stacks for the current project")

	cmd.PersistentFlags().StringVarP(
		&orgFilter, "organization", "o", "", "Filter returned stacks to those in a specific organization")
	cmd.PersistentFlags().StringVarP(
		&projFilter, "project", "p", "", "Filter returned stacks to those with a specific project name")
	cmd.PersistentFlags().StringVarP(
		&tagFilter, "tag", "t", "", "Filter returned stacks to those in a specific tag (tag-name or tag-name=tag-value)")

	return cmd
}

// parseTagFilter parses a tag filter into its separate name and value parts, separatedby an equal sign.
// If no "value" is provided, the second return parameter will be `nil`. Either the tag name or value can
// be omitted. e.g. "=x" returns ("", "x") and "=" returns ("", "").
func parseTagFilter(t string) (string, *string) {
	parts := strings.SplitN(t, "=", 2)
	if len(parts) == 1 {
		return parts[0], nil
	}
	return parts[0], &parts[1]
}

// stackSummaryJSON is the shape of the --json output of this command. When --json is passed, we print an array
// of stackSummaryJSON objects.  While we can add fields to this structure in the future, we should not change
// existing fields.
type stackSummaryJSON struct {
	Name             string `json:"name"`
	Current          bool   `json:"current"`
	LastUpdate       string `json:"lastUpdate,omitempty"`
	UpdateInProgress bool   `json:"updateInProgress"`
	ResourceCount    *int   `json:"resourceCount,omitempty"`
	URL              string `json:"url,omitempty"`
}

func formatStackSummariesJSON(b backend.Backend, currentStack string, stackSummaries []backend.StackSummary) error {
	output := make([]stackSummaryJSON, len(stackSummaries))
	for idx, summary := range stackSummaries {
		summaryJSON := stackSummaryJSON{
			Name:          summary.Name().String(),
			ResourceCount: summary.ResourceCount(),
			Current:       summary.Name().String() == currentStack,
		}

		if summary.LastUpdate() != nil {
			if isUpdateInProgress(summary) {
				summaryJSON.UpdateInProgress = true
			} else {
				summaryJSON.LastUpdate = summary.LastUpdate().UTC().Format(timeFormat)
			}
		}

		if httpBackend, ok := b.(httpstate.Backend); ok {
			if consoleURL, err := httpBackend.StackConsoleURL(summary.Name()); err == nil {
				summaryJSON.URL = consoleURL
			}
		}

		output[idx] = summaryJSON
	}

	return printJSON(output)
}

func formatStackSummariesConsole(b backend.Backend, currentStack string, stackSummaries []backend.StackSummary) error {
	_, showURLColumn := b.(httpstate.Backend)

	// Header string and formatting options to align columns.
	headers := []string{"NAME", "LAST UPDATE", "RESOURCE COUNT"}
	if showURLColumn {
		headers = append(headers, "URL")
	}

	rows := []cmdutil.TableRow{}

	for _, summary := range stackSummaries {
		const none = "n/a"

		// Name column
		name := summary.Name().String()
		if name == currentStack {
			name += "*"
		}

		// Last update column
		lastUpdate := none
		if stackLastUpdate := summary.LastUpdate(); stackLastUpdate != nil {
			if isUpdateInProgress(summary) {
				lastUpdate = "in progress"
			} else {
				lastUpdate = humanize.Time(*stackLastUpdate)
			}
		}

		// ResourceCount column
		resourceCount := none
		if stackResourceCount := summary.ResourceCount(); stackResourceCount != nil {
			resourceCount = strconv.Itoa(*stackResourceCount)
		}

		// Render the columns.
		columns := []string{name, lastUpdate, resourceCount}
		if showURLColumn {
			url := none
			if httpBackend, ok := b.(httpstate.Backend); ok {
				if consoleURL, err := httpBackend.StackConsoleURL(summary.Name()); err == nil {
					url = consoleURL
				}
			}

			columns = append(columns, url)
		}

		rows = append(rows, cmdutil.TableRow{Columns: columns})
	}

	cmdutil.PrintTable(cmdutil.Table{
		Headers: headers,
		Rows:    rows,
	})

	return nil
}

func isUpdateInProgress(u backend.StackSummary) bool {
	// When an update is in progress the last update time is set to zero.
	return u.LastUpdate() != nil && u.LastUpdate().Unix() == 0
}
