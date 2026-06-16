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
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func newConfigEnvCheckoutCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvCheckoutCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Materialize a stack's remote configuration into a local working copy for editing",
		Long: "Materializes the ESC environment backing a remote-config stack into a local\n" +
			"Pulumi.<stack>.local.yaml working copy and marks the stack checked out. While checked out,\n" +
			"`pulumi config` and `pulumi up`/`preview` operate on the working copy instead of the remote\n" +
			"environment, so changes can be previewed without mutating the shared environment. Use\n" +
			"`pulumi config env commit` to write the working copy back to the environment, or\n" +
			"`pulumi config env discard` to drop it.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&impl.secretsProvider, "secrets-provider", "",
		"The secrets provider to use for re-encrypting secrets in the working copy "+
			"(default, passphrase, or a cloud KMS URL such as awskms://...)")

	return cmd
}

type configEnvCheckoutCmd struct {
	parent *configEnvCmd

	secretsProvider string
}

func (cmd *configEnvCheckoutCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmd.parent.color}

	// resolveStack does not apply the working-copy redirect, so checkout operates on the remote
	// environment and the marker directly.
	_, stack, err := cmd.parent.resolveStack(ctx)
	if err != nil {
		return err
	}

	if !stack.ConfigLocation().IsRemote {
		return errors.New("this stack does not use remote configuration; there is nothing to check out")
	}
	loc := stack.ConfigLocation()
	if loc.EscEnv == nil || *loc.EscEnv == "" {
		return errors.New("this stack does not reference a backing environment")
	}
	// A checkout snapshots the latest revision and commit writes the latest; a pinned stack would make
	// both ambiguous, so require unpinning first.
	if envRefVersion(*loc.EscEnv) != "" {
		return fmt.Errorf("the stack's configuration is pinned to %s; "+
			"run `pulumi config env pin latest` before checking out", *loc.EscEnv)
	}

	fqn := stack.Ref().FullyQualifiedName().String()
	if existing, err := state.GetCheckout(cmd.parent.ws, fqn); err != nil {
		return err
	} else if existing != nil {
		return errors.New("this stack is already checked out; " +
			"run `pulumi config env commit` or `pulumi config env discard` first")
	}

	path, err := cmdStack.WorkingCopyPath(stack.Ref().Name().Q())
	if err != nil {
		return err
	}
	// Refuse to overwrite a working copy we did not create (the file-without-marker matrix case).
	if _, statErr := os.Stat(path); statErr == nil {
		return fmt.Errorf("found %s but this stack is not checked out on this machine; "+
			"delete it or move it aside before checking out", path)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("checking working copy %s: %w", path, statErr)
	}

	envBackend, orgName, envProject, envName, _, err := escEnvCoordinates(stack)
	if err != nil {
		return err
	}
	def, etag, revision, err := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", true)
	if err != nil {
		return fmt.Errorf("getting environment %s/%s: %w", envProject, envName, err)
	}

	ps, imports, structured, err := materializeProjectStack(
		ctx, cmd.parent.ssml, stack, def, envProject, envName, cmd.secretsProvider, cmd.parent.interactive, opts)
	if err != nil {
		return err
	}
	if len(structured) > 0 {
		fmt.Fprintf(cmd.parent.stdout,
			"Warning: import(s) %s had merge options that are not preserved in the local working copy; "+
				"only the environment name is kept\n", strings.Join(structured, ", "))
	}

	banner := fmt.Sprintf(
		"# Local working copy created by `pulumi config env checkout` from %s@rev%d.\n"+
			"# This file is for temporary, local usage only; do not commit it.\n"+
			"# Run `pulumi config env commit` to save changes, or `pulumi config env discard` to drop them.\n",
		stripEnvVersion(*loc.EscEnv), revision)

	if err := writeWorkingCopy(ps, path, banner); err != nil {
		return err
	}

	hash, err := canonicalCheckoutHash(ps)
	if err != nil {
		_ = os.Remove(path)
		return err
	}

	marker := pkgWorkspace.Checkout{
		EnvRef:      stripEnvVersion(*loc.EscEnv),
		Etag:        etag,
		Revision:    revision,
		FilePath:    path,
		ContentHash: hash,
		Imports:     imports,
	}
	if err := state.SetCheckout(cmd.parent.ws, fqn, marker); err != nil {
		// Keep the matrix consistent: a file without a marker is a hard error on the next config read.
		_ = os.Remove(path)
		return err
	}

	fmt.Fprintf(cmd.parent.stdout, "Checked out %s@rev%d to %s\n", stripEnvVersion(*loc.EscEnv), revision, path)

	fmt.Fprint(cmd.parent.stdout,
		"Edit configuration with `pulumi config set`; run `pulumi config env commit` to save changes "+
			"or `pulumi config env discard` to drop them.\n")
	return nil
}
