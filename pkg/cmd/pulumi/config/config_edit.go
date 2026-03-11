// Copyright 2024, Pulumi Corporation.
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEditCmd(ws pkgWorkspace.Context, stackRef *string) *cobra.Command {
	impl := &configEditCmd{
		ws:       ws,
		stackRef: stackRef,
	}

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit stack config in $EDITOR",
		Long: `Opens the stack's configuration in $EDITOR.

For service-backed stacks, the ESC environment definition YAML is downloaded,
opened for editing, and uploaded on save. Changes are rejected if the environment
was concurrently modified (optimistic concurrency via etag).

For local stacks, the Pulumi.<stack>.yaml file is opened directly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().BoolVar(&impl.showSecrets, "show-secrets", false,
		"Decrypt secrets in plaintext when downloading the ESC environment (service-backed stacks only)")

	return cmd
}

type configEditCmd struct {
	ws          pkgWorkspace.Context
	stackRef    *string
	showSecrets bool

	// openEditor is overridable for testing.
	openEditor func(filename string) error
}

func (cmd *configEditCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	stack, err := cmdStack.RequireStack(
		ctx,
		cmdutil.Diag(),
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	openFn := cmd.openEditor
	if openFn == nil {
		openFn = openInEditorFromEnv
	}

	loc := stack.ConfigLocation()
	if loc.IsRemote && loc.EscEnv != nil {
		return cmd.editRemote(ctx, stack, openFn)
	}
	return cmd.editLocal(stack, openFn)
}

func (cmd *configEditCmd) editRemote(ctx context.Context, stack backend.Stack, openFn func(string) error) error {
	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	orgName := stack.(interface{ OrgName() string }).OrgName()
	loc := stack.ConfigLocation()
	envProject, envName, _ := strings.Cut(*loc.EscEnv, "/")

	yamlBytes, etag, _, err := envBackend.GetEnvironment(ctx, orgName, envProject, envName, "", cmd.showSecrets)
	if err != nil {
		return fmt.Errorf("loading ESC environment: %w", err)
	}

	// Write to a temp file for editing.
	tmpFile, err := os.CreateTemp("", "pulumi-config-edit-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	if err := openFn(tmpPath); err != nil {
		return err
	}

	modified, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading modified file: %w", err)
	}

	if bytes.Equal(yamlBytes, modified) {
		fmt.Println("No changes made.")
		return nil
	}

	diags, err := envBackend.UpdateEnvironmentWithProject(ctx, orgName, envProject, envName, modified, etag)
	if err != nil {
		// Surface etag conflict as a human-readable error.
		if isHTTPConflict(err) {
			return errors.New("the ESC environment was modified concurrently; please re-run `pulumi config edit` to retry")
		}
		return fmt.Errorf("saving ESC environment: %w", err)
	}
	if len(diags) > 0 {
		return fmt.Errorf("ESC environment has errors:\n%s", formatEnvDiags(diags))
	}

	fmt.Printf("Saved changes to ESC environment %s.\n", *stack.ConfigLocation().EscEnv)
	return nil
}

func (cmd *configEditCmd) editLocal(stack backend.Stack, openFn func(string) error) error {
	_, configFilePath, err := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
	if err != nil {
		return fmt.Errorf("locating config file: %w", err)
	}
	return openFn(configFilePath)
}

// openInEditorFromEnv opens the given file in the editor specified by $EDITOR.
func openInEditorFromEnv(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("no EDITOR environment variable set")
	}

	args, err := shlex.Split(editor)
	if err != nil {
		return fmt.Errorf("parsing EDITOR: %w", err)
	}
	args = append(args, filename)

	//nolint:gosec
	c := exec.Command(args[0], args[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

