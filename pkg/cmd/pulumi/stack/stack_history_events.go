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

package stack

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newStackHistoryEventsCmd() *cobra.Command {
	var (
		stack               string
		eventTypes          []string
		resourceURN         string
		includeNonActivated bool
		token               string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "events",
		Short:  "Retrieve engine events for an update",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "update-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringArrayVar(&eventTypes, "event-type", nil,
		"Filter by engine event type code (repeatable)")
	cmd.Flags().StringVar(&resourceURN, "urn", "", "Filter by resource URN")
	cmd.Flags().BoolVar(&includeNonActivated, "include-non-activated", false,
		"Include events not yet marked as activated")
	cmd.Flags().StringVar(&token, "continuation-token", "",
		"The continuation token for paginated retrieval")

	return cmd
}
