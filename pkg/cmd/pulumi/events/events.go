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
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func NewEventsCmd() *cobra.Command {
	var changesOnly bool

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Operate on engine event streams",
		Long: "Operate on engine event streams.\n" +
			"\n" +
			"Reads an engine event stream from stdin and writes it to stdout.\n" +
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

// runChangesOnly reads a JSONL engine event stream from in, applies the `--changes-only` filter
// to each event, and writes the surviving events back as JSONL to out. Decoding errors are
// surfaced to the caller so that malformed input fails fast rather than silently dropping events.
func runChangesOnly(in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)
	for {
		var evt apitype.EngineEvent
		if err := dec.Decode(&evt); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decoding event: %w", err)
		}
		filtered := filterChangesOnly(evt)
		if filtered == nil {
			continue
		}
		if err := enc.Encode(filtered); err != nil {
			return fmt.Errorf("encoding event: %w", err)
		}
	}
}
