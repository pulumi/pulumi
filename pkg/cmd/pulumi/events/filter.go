// Copyright 2026, Pulumi Corporation.
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

package events

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// NewFilterCmd builds the `pulumi events filter` command. With no flags it is effectively `cat`:
// it reads a JSONL engine event stream from stdin and writes it back to stdout unchanged. Flags
// select concrete filters — currently `--changes-only`.
func NewFilterCmd() *cobra.Command {
	var changesOnly bool

	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Filter an engine event stream",
		Long: "Filter an engine event stream.\n" +
			"\n" +
			"Reads an engine event stream from stdin and writes it to stdout. With no flags\n" +
			"the stream is passed through unchanged.\n" +
			"\n" +
			"With --changes-only, events are filtered down to only resource changes, and\n" +
			"each event's state metadata is restricted to the properties that changed.\n",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !changesOnly {
				_, err := io.Copy(cmd.OutOrStdout(), cmd.InOrStdin())
				return err
			}
			return runChangesOnly(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.Flags().BoolVar(
		&changesOnly, "changes-only", false,
		"Keep only resource-change events and strip each event down to the properties that changed.")
	return cmd
}
