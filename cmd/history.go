// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newHistoryCmd() *cobra.Command {
	var stack string
	var outputJSON bool

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
			s, err := requireStack(tokens.QName(stack))
			if err != nil {
				return err
			}

			b := s.Backend()
			updates, err := b.GetHistory(s.Name())
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
				printUpdateHistory(updates)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	// See pulumi/issues/496, which tracks adding a --format option across all commands.
	cmd.PersistentFlags().BoolVar(
		&outputJSON, "output-json", false,
		"Output stack history as JSON")

	return cmd
}

func printUpdateHistory(updates []backend.UpdateInfo) {
	if len(updates) == 0 {
		fmt.Println("Stack has never been updated")
		return
	}
	for _, update := range updates {
		fmt.Printf("%-3d %8v %8v %v\n", update.Version, update.Kind, update.Result, update.Message)
	}
}
