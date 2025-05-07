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

package state

import (
	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func NewStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Edit the current stack's state",
		Long: `Edit the current stack's state

Subcommands of this command can be used to surgically edit parts of a stack's state. These can be useful when
troubleshooting a stack or when performing specific edits that otherwise would require editing the state file by hand.`,
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newStateEditCommand())
	cmd.AddCommand(newStateDeleteCommand(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newStateUnprotectCommand())
	cmd.AddCommand(newStateProtectCommand())
	cmd.AddCommand(newStateRenameCommand())
	cmd.AddCommand(newStateUpgradeCommand(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newStateMoveCommand())
	cmd.AddCommand(newStateRepairCommand())
	return cmd
}
