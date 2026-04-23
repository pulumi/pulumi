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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// NewEventsCmd is the parent of the `pulumi events` subcommand tree. It does not perform any
// work on its own — running `pulumi events` without a subcommand prints the command's help text.
// Behaviour lives on the subcommands (currently `filter`).
//
// Hidden from `--help` while the interface is being developed.
func NewEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "events",
		Short:  "Work with engine event streams",
		Long:   "Commands for working with Pulumi engine event streams.\n",
		Hidden: true,
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.AddCommand(NewFilterCmd())
	return cmd
}
