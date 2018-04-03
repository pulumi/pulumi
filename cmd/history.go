// Copyright 2017-2018, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
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
			s, err := requireStack(tokens.QName(stack), false)
			if err != nil {
				return err
			}

			// GetHistory returns an array with the newest update first
			updates, err := s.Backend().GetHistory(s.Name())
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
		"Choose a stack other than the currently selected one")

	// pulumi/issues/496 tracks adding a --format option across all commands. Rather than expose a partial solution
	// for just `history`, we put the JSON flag behind an env var so we can use in tests w/o making public.
	if cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG_COMMANDS")) {
		cmd.PersistentFlags().BoolVar(&outputJSON, "output-json", false, "Output stack history as JSON")
	}

	return cmd
}

func printUpdateHistory(updates []backend.UpdateInfo) {
	if len(updates) == 0 {
		fmt.Println("Stack has never been updated")
		return
	}
	for _, update := range updates {
		fmt.Printf("%8v %8v %v\n", update.Kind, update.Result, update.Message)
	}
}
