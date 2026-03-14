// Copyright 2016-2025, Pulumi Corporation.
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

package neo

import (
	"strings"

	"github.com/spf13/cobra"
)

// NewNeoCommand creates the `pulumi neo` command and its subcommands.
func NewNeoCommand() *cobra.Command {
	var org string
	var nonInteractive bool
	var jsonOutput bool
	var approvalMode string

	cmd := &cobra.Command{
		Use:   "neo [prompt]",
		Short: "AI-powered infrastructure agent",
		Long: "Connect to Pulumi Neo, an AI agent that understands your infrastructure.\n" +
			"Run with a prompt for one-shot mode, or without arguments for interactive mode.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := !nonInteractive && len(args) == 0

			prompt := ""
			if len(args) > 0 {
				prompt = strings.Join(args, " ")
			}

			return runNeo(ctx, prompt, org, interactive, jsonOutput, approvalMode)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Target organization (default: inferred from project/stack)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Disable interactive prompts")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output events as JSON lines")
	cmd.Flags().StringVar(&approvalMode, "approval-mode", "balanced",
		"Approval mode: auto, balanced, or manual")

	cmd.AddCommand(newAttachCommand(&org, &nonInteractive, &jsonOutput, &approvalMode))
	cmd.AddCommand(newSessionsCommand(&org))

	return cmd
}

func newAttachCommand(org *string, nonInteractive *bool, jsonOutput *bool, approvalMode *string) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to an existing Neo session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runAttach(ctx, args[0], *org, !*nonInteractive, *jsonOutput, *approvalMode)
		},
	}
}

func newSessionsCommand(org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List recent Neo sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runListSessions(ctx, *org)
		},
	}
}
