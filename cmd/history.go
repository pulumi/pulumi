// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newHistoryCmd() *cobra.Command {
	var stack string
	var outputJSON bool // Requires PULUMI_DEBUG_COMMANDS
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

			var packageFilter *tokens.PackageName
			stackSummaries, err := b.ListStacks(commandContext(), packageFilter)
			if err != nil {
				return err
			}

			// Sort by stack name.
			sort.Slice(stackSummaries, func(i, j int) bool {
				return stackSummaries[i].Name().String() < stackSummaries[j].Name().String()
			})

			updates, err := b.GetHistory(commandContext(), s.Ref())
			if err != nil {
				return errors.Wrap(err, "getting history")
			}

			if outputJSON {
				b, err := json.MarshalIndent(updates, "", "    ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			} else {
				printUpdateHistory(updates, stackSummaries)
			}
			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	// pulumi/issues/496 tracks adding a --format option across all commands. Rather than expose a partial solution
	// for just `history`, we put the JSON flag behind an env var so we can use in tests w/o making public.
	if cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG_COMMANDS")) {
		cmd.PersistentFlags().BoolVar(&outputJSON, "output-json", false, "Output stack history as JSON")
	}
	return cmd
}
func printUpdateHistory(updates []backend.UpdateInfo, stacks []backend.StackSummary) {

	if len(updates) == 0 {
		fmt.Println("\u001b[41mStack has never been updated\033[0m")
		fmt.Println("Would you like to pick a different stack:")
		for _, value := range stacks {
			fmt.Printf("    %s", value.Name())
		}
		return
	}

	for _, update := range updates {

		fmt.Println("\033[32m-----------------------\033[0m")

		fmt.Printf("  \u001b[1mUpdateKind:\033[0m %v \u001b[1m\n  Status:\033[0m %v  m: %v \n", update.Kind, update.Result, update.Message)

		fmt.Printf("  \u001b[42m\u001b[30m+%v\033[0m\u001b[41m\u001b[30m-%v\033[0m\u001b[43m\u001b[30m~%v\033[0m\u001b[47m\u001b[30m %v\033[0m", update.ResourceChanges["create"], update.ResourceChanges["delete"], update.ResourceChanges["update"], update.ResourceChanges["same"])

		tStart := time.Unix(update.StartTime, 0)
		tCreated := humanize.Time(tStart)
		tEnd := time.Unix(update.EndTime, 0)
		duration := tEnd.Sub(tStart)

		fmt.Printf("  Updated %s took %s\n", tCreated, duration)

		if len(update.Environment) != 0 {
			fmt.Printf("    Github-Login: %s\n", update.Environment["github.login"])
			fmt.Printf("    Github-Committer: %s <%s>\n", update.Environment["git.committer"], update.Environment["git.committer.email"])
			fmt.Printf("    \u001b[33mcommit %s\033[0m \n", update.Environment["git.head"])
		}

		fmt.Println("\033[244m-----------------------\033[0m")
	}
}
