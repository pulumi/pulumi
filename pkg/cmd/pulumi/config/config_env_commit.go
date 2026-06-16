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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigEnvCommitCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvCommitCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Write a stack's checked-out working copy back to its remote configuration",
		Long: "Writes the local Pulumi.<stack>.local.yaml working copy back to the stack's ESC environment\n" +
			"as a single revision, then deletes the working copy and clears the checked-out state. The\n" +
			"environment's pulumiConfig is replaced exactly: keys removed locally are removed from the\n" +
			"environment. If the environment changed since checkout, commit refuses unless the overwrite is\n" +
			"confirmed (interactively, or with --force-revision).",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			impl.forced = cmd.Flags().Changed("force-revision")
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().IntVar(&impl.forceRevision, "force-revision", 0,
		"Overwrite the environment that moved since checkout, asserting it is at this revision "+
			"(force-with-lease); commit refuses if it has since moved again")

	return cmd
}

type configEnvCommitCmd struct {
	parent *configEnvCmd

	forceRevision int
	forced        bool
}

func (cmd *configEnvCommitCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmd.parent.color}

	project, stack, err := cmd.parent.resolveStack(ctx)
	if err != nil {
		return err
	}
	if !stack.ConfigLocation().IsRemote {
		return errors.New("this stack does not use remote configuration")
	}

	fqn := stack.Ref().FullyQualifiedName().String()
	marker, err := state.GetCheckout(cmd.parent.ws, fqn)
	if err != nil {
		return err
	}
	if marker == nil {
		return errors.New("this stack is not checked out; run `pulumi config env checkout` first")
	}

	if _, statErr := os.Stat(marker.FilePath); os.IsNotExist(statErr) {
		_ = state.ClearCheckout(cmd.parent.ws, fqn)
		return fmt.Errorf("the working copy %s is missing; the checkout marker has been cleared, "+
			"run `pulumi config env checkout` to start over", marker.FilePath)
	}
	ps, err := cmd.parent.loadProjectStack(ctx, cmd.parent.diags, project, stack, marker.FilePath)
	if err != nil {
		return err
	}
	if ps == nil {
		ps = &workspace.ProjectStack{}
	}

	// No-op: the working copy is unchanged from checkout, so there is nothing to write. Clean up without
	// reading or touching the environment.
	hash, err := canonicalCheckoutHash(ps)
	if err != nil {
		return err
	}
	if hash == marker.ContentHash {
		return cmd.cleanup(fqn, marker.FilePath, "No changes to commit; removed working copy %s\n")
	}

	// A real commit requires the stack still linked, unpinned, and pointing at the same environment as at
	// checkout: a pin or relink would target a different (or read-only) revision. A no-op (handled above)
	// writes nothing, so it skips these guards.
	loc := stack.ConfigLocation()
	if loc.EscEnv == nil || *loc.EscEnv == "" {
		return errors.New("this stack no longer references a backing environment")
	}
	if envRefVersion(*loc.EscEnv) != "" {
		return fmt.Errorf("the stack's configuration was pinned to %s after checkout; "+
			"run `pulumi config env pin latest` before committing", *loc.EscEnv)
	}
	if stripEnvVersion(*loc.EscEnv) != marker.EnvRef {
		return fmt.Errorf("the stack was relinked to %s since checkout (was %s); "+
			"run `pulumi config env discard` and check out again", stripEnvVersion(*loc.EscEnv), marker.EnvRef)
	}

	// Decrypt the working copy's config to plaintext-bearing values so the ESC editor can re-wrap secrets
	// as fn::secret for server-side encryption (Copy with a Nop encrypter keeps secrets as plaintext).
	// Non-secret config already holds plaintext, so skip the secrets-manager call when there are none.
	plaintextConfig := ps.Config
	if ps.Config.HasSecureValue() {
		decrypter, _, err := cmd.parent.ssml.GetDecrypter(ctx, stack, ps)
		if err != nil {
			return err
		}
		plaintextConfig, err = ps.Config.Copy(decrypter, config.NopEncrypter)
		if err != nil {
			return fmt.Errorf("decrypting working copy configuration: %w", err)
		}
	}

	// Read the environment once and write with that same etag (force-with-lease, no read-modify-write gap).
	envBackend, orgName, envProject, envName, _, err := escEnvCoordinates(stack)
	if err != nil {
		return err
	}
	def, etag, revision, err := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", false)
	if err != nil {
		return fmt.Errorf("reading environment %s/%s: %w", envProject, envName, err)
	}

	if etag != marker.Etag {
		if err := cmd.confirmOverwrite(stack, orgName, envProject, envName, marker.Revision, revision, opts); err != nil {
			return err
		}
	}

	editor, err := newESCConfigEditorFromDef(envBackend, orgName, envProject, envName, "", def, etag)
	if err != nil {
		return err
	}
	if err := editor.ReplaceConfig(ctx, plaintextConfig); err != nil {
		return err
	}

	// Reconcile imports only when the working copy's list differs from the baseline, leaving structured
	// imports (and their merge options) untouched otherwise.
	var wcImports []string
	if ps.Environment != nil {
		for _, name := range ps.Environment.Imports() {
			if name != "yaml" { // synthetic marker from a local Environment carrying values
				wcImports = append(wcImports, name)
			}
		}
	}
	if !sameImports(marker.Imports, wcImports) {
		if err := editor.ReplaceImports(wcImports); err != nil {
			return err
		}
		fmt.Fprintf(cmd.parent.stdout,
			"Warning: imports changed; any merge options on flattened imports are not preserved\n")
	}

	if err := editor.Save(ctx); err != nil {
		if errors.Is(err, backend.ErrConfigConflict) {
			return fmt.Errorf("the environment changed during commit; re-run `pulumi config env commit`: %w", err)
		}
		return err
	}

	return cmd.cleanup(fqn, marker.FilePath,
		fmt.Sprintf("Committed working copy to %s; removed %%s\n", marker.EnvRef))
}

// cleanup deletes the working copy and clears the checkout marker, printing msg (which must contain a
// single %s for the working-copy path).
func (cmd *configEnvCommitCmd) cleanup(fqn, path, msg string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := state.ClearCheckout(cmd.parent.ws, fqn); err != nil {
		return err
	}
	fmt.Fprintf(cmd.parent.stdout, msg, path)
	return nil
}

// confirmOverwrite handles the drift case where the environment moved since checkout. With
// --force-revision it asserts the environment is still at the reviewed revision (force-with-lease);
// interactively it prompts; non-interactively without the flag it refuses.
func (cmd *configEnvCommitCmd) confirmOverwrite(
	stack backend.Stack, orgName, envProject, envName string, fromRev, curRev int, opts display.Options,
) error {
	drift := fmt.Sprintf("the environment moved from revision %d to %d since checkout", fromRev, curRev)
	if url := envConsoleURL(stack, orgName, envProject, envName); url != "" {
		drift += "; review it at " + url
	}

	if cmd.forced {
		if cmd.forceRevision != curRev {
			return fmt.Errorf("the environment is now at revision %d, not %d; re-review and retry", curRev, cmd.forceRevision)
		}
		return nil
	}
	if !cmd.parent.interactive {
		return fmt.Errorf("%s; re-review and pass --force-revision %d to overwrite", drift, curRev)
	}

	fmt.Fprintf(cmd.parent.stdout, "Warning: %s\n", drift)
	if !ui.ConfirmPrompt(
		fmt.Sprintf("Overwrite the environment, discarding the changes in revision %d?", curRev), "yes", opts) {
		return errors.New("commit canceled")
	}
	return nil
}

// envConsoleURL returns the Pulumi Cloud console URL for the environment, or "" if the backend does not
// provide one.
func envConsoleURL(stack backend.Stack, orgName, envProject, envName string) string {
	provider, ok := stack.Backend().(consoleURLProvider)
	if !ok {
		return ""
	}
	return provider.CloudConsoleURL(orgName, "esc", envProject, envName)
}

// sameImports reports whether two import lists are identical in both contents and order. Order matters:
// ESC resolves imports in sequence, so a reorder is a semantic change that commit must propagate.
func sameImports(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
