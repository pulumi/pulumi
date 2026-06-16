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

package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigEnvDiscardCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvDiscardCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "discard",
		Short: "Discard a stack's checked-out working copy without writing it back",
		Long: "Deletes the local Pulumi.<stack>.local.yaml working copy and clears the checked-out state\n" +
			"without writing anything back to the stack's ESC environment. When the working copy is unchanged\n" +
			"from checkout it is removed without confirmation; otherwise confirmation is required.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().BoolVarP(&impl.yes, "yes", "y", false, "True to proceed without prompting")

	return cmd
}

type configEnvDiscardCmd struct {
	parent *configEnvCmd

	yes bool
}

func (cmd *configEnvDiscardCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmd.parent.color}

	project, stack, err := cmd.parent.resolveStack(ctx)
	if err != nil {
		return err
	}

	fqn := stack.Ref().FullyQualifiedName().String()
	marker, err := state.GetCheckout(cmd.parent.ws, fqn)
	if err != nil {
		return err
	}
	if marker == nil {
		return errors.New("this stack is not checked out; there is nothing to discard")
	}

	// A missing working copy means there is nothing to drop; just clear the marker.
	if _, statErr := os.Stat(marker.FilePath); os.IsNotExist(statErr) {
		if err := state.ClearCheckout(cmd.parent.ws, fqn); err != nil {
			return err
		}
		fmt.Fprintf(cmd.parent.stdout, "Working copy already gone; cleared checked-out state for %s\n", marker.EnvRef)
		return nil
	}

	// Unchanged working copies have nothing to lose, so skip confirmation. A working copy that no longer
	// parses (the user hand-edited it into invalid YAML) is treated as changed so discard can still drop
	// it — discard must never be blocked by the file it is removing.
	changed := true
	if ps, err := cmd.parent.loadProjectStack(ctx, cmd.parent.diags, project, stack, marker.FilePath); err == nil {
		if ps == nil {
			ps = &workspace.ProjectStack{}
		}
		if hash, herr := canonicalCheckoutHash(ps); herr == nil {
			changed = hash != marker.ContentHash
		}
	}

	if changed && !cmd.yes {
		if !cmd.parent.interactive {
			return backenderr.ErrNonInteractiveRequiresYes
		}
		if !ui.ConfirmPrompt(
			fmt.Sprintf("Discard local changes to %s and revert to remote configuration?", marker.EnvRef),
			"yes", opts) {
			return errors.New("discard canceled")
		}
	}

	if err := os.Remove(marker.FilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := state.ClearCheckout(cmd.parent.ws, fqn); err != nil {
		return err
	}
	fmt.Fprintf(cmd.parent.stdout, "Discarded working copy %s\n", marker.FilePath)
	return nil
}
