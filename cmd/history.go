// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

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
				printUpdateHistory(updates, opts)
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
func printUpdateHistory(updates []backend.UpdateInfo, opts display.Options) {

	if len(updates) == 0 {
		fmt.Println("Stack has never been updated")
		return
	}

	for _, update := range updates {

		fmt.Print(
			opts.Color.Colorize(
				fmt.Sprintf("%s%s%s\n", colors.Green, "-----------------------", colors.Reset)))

		fmt.Printf("  UpdateKind: %v \n  Status: %v  m: %v \n", update.Kind, update.Result, update.Message)
		fmt.Print(
			opts.Color.Colorize(
				fmt.Sprintf("  %s%s+%v%s%s%s-%v%s%s%s~%v%s%s%s %v%s", colors.GreenBg, colors.Black, update.ResourceChanges["create"], colors.Reset,
																		colors.RedBg, colors.Black, update.ResourceChanges["delete"], colors.Reset,
																		colors.YellowBg, colors.Black, update.ResourceChanges["update"], colors.Reset,
																		colors.BlueBg, colors.Black, update.ResourceChanges["same"], colors.Reset)))

		tStart := time.Unix(update.StartTime, 0)
		tCreated := humanize.Time(tStart)
		tEnd := time.Unix(update.EndTime, 0)
		duration := tEnd.Sub(tStart)

		fmt.Printf("  Updated %s took %s\n", tCreated, duration)

		if len(update.Environment) != 0 {
			fmt.Printf("    Github-Login: %s\n", update.Environment["github.login"])
			fmt.Printf("    Github-Committer: %s <%s>\n", update.Environment["git.committer"], update.Environment["git.committer.email"])
			fmt.Print(
				opts.Color.Colorize(
					fmt.Sprintf("%s    commit %s%s\n", colors.Yellow, update.Environment["git.head"], colors.Reset)))
		}

		fmt.Print(
			opts.Color.Colorize(
				fmt.Sprintf("%s%s%s\n", colors.Cyan, "-----------------------", colors.Reset)))
	}
}
