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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
)

// configEnvRollbackCmd rolls a remote-config stack's linked ESC environment back to a prior revision
// by delegating to `pulumi env version rollback`, which rewrites the revision's contents as a new
// revision (history preserved). Local stacks are not supported.
type configEnvRollbackCmd struct {
	parent *configEnvCmd

	// runEnv runs `pulumi env <args>`. Injectable so tests do not invoke the real ESC backend.
	runEnv func(ctx context.Context, args []string) error
}

func newConfigEnvRollbackCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvRollbackCmd{parent: parent}

	cmd := &cobra.Command{
		Use:    "rollback <revision>",
		Short:  "Roll back a remote-config stack's configuration to a prior revision",
		Hidden: true,
		Long: "Roll back a stack that stores its configuration remotely to the contents of a prior revision\n" +
			"of its backing ESC environment.\n\n" +
			"This delegates to `pulumi env version rollback`: it reads the given revision and writes its\n" +
			"contents as a new revision, so the history is preserved rather than rewritten.",
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			parent.stdout = cmd.OutOrStdout()
			if impl.runEnv == nil {
				impl.runEnv = runEnvCmd
			}
			return impl.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "revision"}},
		Required:  1,
	})

	return cmd
}

func (cmd *configEnvRollbackCmd) run(ctx context.Context, revision string) error {
	opts := display.Options{Color: cmd.parent.color}

	if _, _, err := cmd.parent.ws.ReadProject(); err != nil {
		return err
	}

	stack, err := cmd.parent.requireStack(
		ctx,
		cmd.parent.diags,
		cmd.parent.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.parent.stackRef,
		cmdStack.SetCurrent,
		opts,
		*cmd.parent.configFile,
	)
	if err != nil {
		return err
	}

	if !configStoreIsRemote(stack, *cmd.parent.configFile) {
		return errors.New("`pulumi config env rollback` is only supported for remote config stacks")
	}
	if err := rejectIfPinned(stack, *cmd.parent.configFile); err != nil {
		return err
	}

	ref := stack.ConfigLocation().EscEnv
	if ref == nil || *ref == "" {
		return errors.New("stack is configured for remote config but has no linked environment")
	}
	envRef := stripEnvVersion(*ref)

	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}

	// Qualify with the stack's org so `env version rollback` cannot fall back to the user's default org.
	// The rollback is last-writer-wins (no etag guard), but every revision is retained so a concurrent
	// overwrite is recoverable by rolling back again.
	args := []string{"version", "rollback", orgNamer.OrgName() + "/" + envRef + "@" + revision}

	// env rediscovers its backend from PULUMI_BACKEND_URL or the current login and never reads the
	// project's backend.url, so pin it to the backend this stack actually resolved to. Otherwise a
	// project pinned to a different cloud could roll back a same-named environment in the ambient one.
	if urler, ok := stack.Backend().(interface{ CloudURL() string }); ok {
		defer bindBackendURL(urler.CloudURL())()
	}
	return cmd.runEnv(ctx, args)
}
