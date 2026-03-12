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
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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

func newConfigEnvPinCmd(ws pkgWorkspace.Context, stackRef *string) *cobra.Command {
	impl := &configPinCmd{
		ws:       ws,
		stackRef: stackRef,
		stdout:   os.Stdout,
	}

	cmd := &cobra.Command{
		Use:   "pin <version-or-tag>",
		Short: "Pin the stack's config to a specific revision or tag",
		Long: `Pin the stack's configuration to a specific ESC environment revision or tag.

Use a number to pin to a revision, or a string to pin to a tag.
Use "latest" to unpin and return to tracking the latest revision.

This command is only meaningful for remote config stacks. For local stacks
it is a no-op with an informational message.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return impl.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "version-or-tag"},
		},
		Required: 1,
	})

	return cmd
}

type configPinCmd struct {
	ws       pkgWorkspace.Context
	stackRef *string
	stdout   io.Writer

	// loadRemoteConfig is overridable for testing.
	loadRemoteConfig func(
		ctx context.Context, stack backend.Stack, project *workspace.Project,
	) (*workspace.ProjectStack, error)
	// saveRemoteConfig is overridable for testing.
	saveRemoteConfig func(
		ctx context.Context, stack backend.Stack, ps *workspace.ProjectStack,
	) error
	// getEnvironment is overridable for testing.
	getEnvironment func(
		ctx context.Context, envBackend backend.EnvironmentsBackend,
		org, project, name, version string,
	) error
}

func (cmd *configPinCmd) run(ctx context.Context, versionArg string) error {
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
		fmt.Fprintln(cmd.stdout, "Config pinning is only applicable to remote config stacks; this stack uses local config.")
		return nil
	}

	project, _, err := cmd.ws.ReadProject()
	if err != nil {
		return err
	}

	return cmd.pinStack(ctx, stack, project, *loc.EscEnv, versionArg)
}

func (cmd *configPinCmd) pinStack(
	ctx context.Context,
	stack backend.Stack,
	project *workspace.Project,
	escEnv string,
	versionArg string,
) error {
	baseRef := stripEscEnvVersion(escEnv)

	if strings.EqualFold(versionArg, "latest") {
		return cmd.updateEnvRef(ctx, stack, project, baseRef, "Unpinned config to track the latest revision.")
	}

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

	validateFn := cmd.getEnvironment
	if validateFn == nil {
		validateFn = func(ctx context.Context, eb backend.EnvironmentsBackend, org, proj, name, version string) error {
			_, _, _, err := eb.GetEnvironment(ctx, org, proj, name, version, false)
			return err
		}
	}

	if err := validateFn(ctx, envBackend, orgName, envProject, envName, versionArg); err != nil {
		if isHTTPNotFound(err) {
			return fmt.Errorf("version %q not found for environment %s", versionArg, baseRef)
		}
		return fmt.Errorf("validating version %q: %w", versionArg, err)
	}

	pinnedRef := baseRef + "@" + versionArg
	msg := fmt.Sprintf("Pinned config to version %s.", versionArg)
	if _, err := strconv.Atoi(versionArg); err != nil {
		msg = fmt.Sprintf("Pinned config to tag %q.", versionArg)
	}

	return cmd.updateEnvRef(ctx, stack, project, pinnedRef, msg)
}

func (cmd *configPinCmd) updateEnvRef(
	ctx context.Context,
	stack backend.Stack,
	project *workspace.Project,
	newEnvRef string,
	successMsg string,
) error {
	loadFn := cmd.loadRemoteConfig
	if loadFn == nil {
		loadFn = func(ctx context.Context, s backend.Stack, p *workspace.Project) (*workspace.ProjectStack, error) {
			return s.LoadRemoteConfig(ctx, p)
		}
	}

	ps, err := loadFn(ctx, stack, project)
	if err != nil {
		return fmt.Errorf("loading remote config: %w", err)
	}
	if ps == nil {
		ps = &workspace.ProjectStack{}
	}

	ps.Environment = workspace.NewEnvironment([]string{newEnvRef})

	saveFn := cmd.saveRemoteConfig
	if saveFn == nil {
		saveFn = func(ctx context.Context, s backend.Stack, ps *workspace.ProjectStack) error {
			return s.SaveRemoteConfig(ctx, ps)
		}
	}

	if err := saveFn(ctx, stack, ps); err != nil {
		return fmt.Errorf("updating stack config: %w", err)
	}

	fmt.Fprintln(cmd.stdout, successMsg)
	return nil
}

// stripEscEnvVersion removes the @version suffix from an ESC environment reference.
// "myproject/dev@3" → "myproject/dev", "myproject/dev" → "myproject/dev".
func stripEscEnvVersion(ref string) string {
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		return ref[:i]
	}
	return ref
}

// escEnvVersion returns the version suffix from an ESC environment reference,
// or empty string if unpinned. "myproject/dev@3" → "3", "myproject/dev" → "".
func escEnvVersion(ref string) string {
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		return ref[i+1:]
	}
	return ""
}

// isStackPinned returns true if the stack's ESC environment reference includes a version pin.
func isStackPinned(stack backend.Stack) bool {
	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return false
	}
	return escEnvVersion(*loc.EscEnv) != ""
}

// rejectIfPinned returns an error if the stack is pinned to a specific version.
// Callers should check this before creating a ConfigEditor for mutation operations.
func rejectIfPinned(stack backend.Stack) error {
	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return nil
	}
	version := escEnvVersion(*loc.EscEnv)
	if version == "" {
		return nil
	}
	return fmt.Errorf(
		"config is pinned to version %s; run `pulumi config env pin latest` to unpin before making changes",
		version)
}
