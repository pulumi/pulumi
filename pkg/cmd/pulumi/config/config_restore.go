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
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newConfigRestoreCmd(ws pkgWorkspace.Context, stackRef *string) *cobra.Command {
	impl := &configRestoreCmd{
		ws:       ws,
		stackRef: stackRef,
		stdout:   os.Stdout,
	}

	cmd := &cobra.Command{
		Use:   "restore <revision>",
		Short: "Restore config to a previous revision (creates new revision)",
		Long: `Restore config to a previous revision by creating a new revision with the old content.

This is not a rollback — the specified revision's content is read and written as a new
revision, preserving the full revision history. This command is only supported for
remote config stacks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return impl.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "revision"},
		},
		Required: 1,
	})

	return cmd
}

type configRestoreCmd struct {
	ws       pkgWorkspace.Context
	stackRef *string
	stdout   io.Writer

	getEnvironment func(
		ctx context.Context, eb backend.EnvironmentsBackend,
		org, project, name, version string, decrypt bool,
	) ([]byte, string, int, error)
	updateEnvironmentWithProject func(
		ctx context.Context, eb backend.EnvironmentsBackend,
		org, project, name string, yaml []byte, etag string,
	) error
}

func (cmd *configRestoreCmd) run(ctx context.Context, revisionArg string) error {
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

	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return errors.New("config restore is only supported for remote config stacks; this stack uses local config")
	}

	return cmd.restoreRevision(ctx, stack, *loc.EscEnv, revisionArg)
}

func (cmd *configRestoreCmd) restoreRevision(
	ctx context.Context,
	stack backend.Stack,
	escEnv string,
	revisionArg string,
) error {
	baseRef := stripEscEnvVersion(escEnv)

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %q does not support ESC environments", stack.Backend().Name())
	}

	orgName, err := stackOrgName(stack)
	if err != nil {
		return err
	}

	envProject, envName, err := parseEscEnvRef(baseRef)
	if err != nil {
		return err
	}

	getFn := cmd.getEnvironment
	if getFn == nil {
		getFn = func(
			ctx context.Context, eb backend.EnvironmentsBackend,
			org, proj, name, version string, decrypt bool,
		) ([]byte, string, int, error) {
			return eb.GetEnvironment(ctx, org, proj, name, version, decrypt)
		}
	}

	oldContent, _, _, err := getFn(ctx, envBackend, orgName, envProject, envName, revisionArg, false)
	if err != nil {
		if isHTTPNotFound(err) {
			return fmt.Errorf("revision %s not found for environment %s", revisionArg, baseRef)
		}
		return fmt.Errorf("reading revision %s: %w", revisionArg, err)
	}

	_, currentEtag, _, err := getFn(ctx, envBackend, orgName, envProject, envName, "", false)
	if err != nil {
		return fmt.Errorf("reading current environment state: %w", err)
	}

	updateFn := cmd.updateEnvironmentWithProject
	if updateFn == nil {
		updateFn = func(
			ctx context.Context, eb backend.EnvironmentsBackend,
			org, proj, name string, yaml []byte, etag string,
		) error {
			diags, err := eb.UpdateEnvironmentWithProject(ctx, org, proj, name, yaml, etag)
			if err != nil {
				return err
			}
			if len(diags) > 0 {
				return fmt.Errorf("ESC environment %s/%s validation failed:\n%s", proj, name, formatEnvDiags(diags))
			}
			return nil
		}
	}

	if err := updateFn(ctx, envBackend, orgName, envProject, envName, oldContent, currentEtag); err != nil {
		if isHTTPConflict(err) {
			return fmt.Errorf("the environment was modified concurrently; please retry: %w", err)
		}
		return fmt.Errorf("restoring revision %s: %w", revisionArg, err)
	}

	fmt.Fprintf(cmd.stdout, "Restored environment %s to revision %s (created as new revision).\n", baseRef, revisionArg)
	return nil
}
