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
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigEnvPinCmd(parent *configEnvCmd) *cobra.Command {
	impl := &configEnvPinCmd{parent: parent}

	cmd := &cobra.Command{
		Use:    "pin <version-or-tag>",
		Short:  "Pin a remote-config stack to an environment revision or tag",
		Hidden: true,
		Long: "Pin a stack that stores its configuration remotely to a specific revision or tag of its\n" +
			"backing ESC environment. While pinned, configuration mutations are refused. Pass `latest`\n" +
			"to unpin and track the most recent revision again.",
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			parent.stdout = cmd.OutOrStdout()
			return impl.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "version-or-tag"}},
		Required:  1,
	})

	return cmd
}

type configEnvPinCmd struct {
	parent *configEnvCmd

	// getEnvironment is injectable so tests can validate a version without a real backend.
	getEnvironment func(
		ctx context.Context, org, project, env, version string, decrypt bool,
	) ([]byte, string, int, error)
}

func (cmd *configEnvPinCmd) run(ctx context.Context, version string) error {
	opts := display.Options{Color: cmd.parent.color}

	project, _, err := cmd.parent.ws.ReadProject()
	if err != nil {
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

	loc := stack.ConfigLocation()
	// An explicit --config-file selects a local file; pinning operates only on the remote environment.
	if !configStoreIsRemote(stack, *cmd.parent.configFile) {
		return errors.New("this stack does not use remote configuration; there is nothing to pin")
	}
	if loc.EscEnv == nil || *loc.EscEnv == "" {
		return errors.New("this stack does not reference a backing environment")
	}
	baseRef := stripEnvVersion(*loc.EscEnv)

	if strings.EqualFold(version, "latest") {
		if err := cmd.link(ctx, stack, project, baseRef); err != nil {
			return err
		}
		fmt.Fprintln(cmd.parent.stdout, "Unpinned configuration.")
		return nil
	}

	envBackend, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}
	envProject, envName, err := splitEnvRef(baseRef)
	if err != nil {
		return err
	}

	getEnv := cmd.getEnvironment
	if getEnv == nil {
		getEnv = envBackend.GetEnvironment
	}
	if _, _, _, err := getEnv(ctx, orgNamer.OrgName(), envProject, envName, version, false); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("version %q not found for environment %s", version, baseRef)
		}
		return fmt.Errorf("validating version %q: %w", version, err)
	}

	if err := cmd.link(ctx, stack, project, baseRef+"@"+version); err != nil {
		return err
	}

	kind := "tag"
	if _, err := strconv.Atoi(version); err == nil {
		kind = "revision"
	}
	fmt.Fprintf(cmd.parent.stdout, "Pinned configuration to %s %s.\n", kind, version)
	return nil
}

// link repoints the stack's remote configuration to ref via SaveRemoteConfig, which requires a
// project stack with a nil Config and exactly one environment import that it adopts as the new link.
func (cmd *configEnvPinCmd) link(
	ctx context.Context, stack backend.Stack, project *workspace.Project, ref string,
) error {
	// SaveRemoteConfig replaces the config link wholesale, so carry over the current secrets-provider
	// metadata; re-linking with empty metadata would clear it and break decryption for a passphrase/KMS
	// stack. Read it from the remote stack config (empty config-file) — a local --config-file would
	// carry the wrong provider metadata into the remote link.
	current, err := cmd.parent.loadProjectStack(ctx, cmd.parent.diags, project, stack, "")
	if err != nil {
		return fmt.Errorf("reading current stack configuration: %w", err)
	}

	ps := &workspace.ProjectStack{
		Environment: workspace.NewEnvironment([]string{ref}),
		Config:      nil,
	}
	if current != nil {
		ps.SecretsProvider = current.SecretsProvider
		ps.EncryptedKey = current.EncryptedKey
		ps.EncryptionSalt = current.EncryptionSalt
	}
	if err := stack.SaveRemoteConfig(ctx, ps); err != nil {
		return fmt.Errorf("updating remote configuration link: %w", err)
	}
	return nil
}
