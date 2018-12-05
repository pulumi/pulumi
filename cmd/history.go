// Copyright 2018, Pulumi Corporation.
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
	"time"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newHistoryCmd() *cobra.Command {
	var stack string
	var cmd = &cobra.Command{
		Use:        "history",
		Aliases:    []string{"hist"},
		SuggestFor: []string{"updates"},
		Short:      "Update history for a stack",
		Long: "Update history for a stack\n" +
		"\n" +
		"This command lists data about previous updates for a stack.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stack, false, opts, false /*setCurrent*/)

			if err != nil {
				return err
			}

			b := s.Backend()

			updates, err := b.GetHistory(commandContext(), s.Ref())
			if err != nil {
				return errors.Wrap(err, "getting history")
			}

			displayUpdate(updates, opts)

			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	return cmd
}

func displayUpdate(updates []backend.UpdateInfo, opts display.Options) {

	if len(updates) == 0 {
		fmt.Println("Stack has never been updated")
		return
	}

	printResourceChanges := func(backgroundColor string, textColor string, sign string, amountOfChange int, resetColor string) {
		msg := opts.Color.Colorize(fmt.Sprintf("%s%s%s%v%s", backgroundColor, textColor, sign, amountOfChange, resetColor))
		fmt.Print(msg)
	}

	for _, update := range updates {

		fmt.Print( opts.Color.Colorize( fmt.Sprintf("%s%s%s\n", colors.Green, "-----------------------", colors.Reset)))

		fmt.Printf("UpdateKind: %v\n", update.Kind)
		fmt.Printf("Status: %v m: %v \n", update.Result, update.Message)

		printResourceChanges(colors.GreenBackground, colors.Black, "+", update.ResourceChanges["create"], colors.Reset)
		printResourceChanges(colors.RedBackground, colors.Black, "-", update.ResourceChanges["delete"], colors.Reset)
		printResourceChanges(colors.YellowBackground, colors.Black, "~", update.ResourceChanges["update"], colors.Reset)
		printResourceChanges(colors.BlueBackground, colors.Black, " ", update.ResourceChanges["same"], colors.Reset)

		timeStart := time.Unix(update.StartTime, 0)
		timeCreated := humanize.Time(timeStart)
		timeEnd := time.Unix(update.EndTime, 0)
		duration := timeEnd.Sub(timeStart)

		space := 2
		fmt.Printf("%*sUpdated %s took %s\n", space, "", timeCreated, duration)

		empty := func(s string) bool {
			if len(strings.TrimSpace(s)) == 0 {
				return true
			}
			return false
		}

		if len(update.Environment) != 0 {
			indent := 4
			if !empty(update.Environment["github.login"]) && !empty(update.Environment["git.committer"]) && !empty(update.Environment["git.committer.email"]) && !empty(update.Environment["git.head"]) {
				fmt.Printf("%*sGithub-Login: %s\n", indent, "", update.Environment["github.login"])
				fmt.Printf("%*sGithub-Committer: %s <%s>\n", indent, "", update.Environment["git.committer"], update.Environment["git.committer.email"])
				fmt.Print(opts.Color.Colorize(fmt.Sprintf("%*s%scommit %s%s\n", indent, "", colors.Yellow, update.Environment["git.head"], colors.Reset)))
			}
		}

		fmt.Print( opts.Color.Colorize( fmt.Sprintf("%s%s%s\n", colors.Cyan, "-----------------------", colors.Reset)))
	}
}
