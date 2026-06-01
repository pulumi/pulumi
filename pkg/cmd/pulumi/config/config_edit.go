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
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdEnv "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/env"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// configEditCmd implements the hidden `pulumi config edit` command, delegating to `pulumi env edit`
// for the linked environment. Only supported for remote-config (ESC-backed) stacks.
type configEditCmd struct {
	ws          pkgWorkspace.Context
	stackRef    *string
	configFile  *string
	editorFlag  string
	showSecrets bool

	// Injectable so tests do not invoke the real ESC editor.
	runEnvEdit func(ctx context.Context, args []string) error
}

func newConfigEditCmd(ws pkgWorkspace.Context, stackRef *string, configFile *string) *cobra.Command {
	impl := &configEditCmd{ws: ws, stackRef: stackRef, configFile: configFile}

	cmd := &cobra.Command{
		Use:    "edit",
		Short:  "Edit a remote config stack's configuration in an editor",
		Hidden: true,
		Long: "Open a remote config stack's linked ESC environment definition in an editor and save it on " +
			"exit.\n\n" +
			"This delegates to `pulumi env edit`: it downloads the definition, opens it in $VISUAL/$EDITOR " +
			"(or --editor), and uploads the result under the environment's etag. Secrets are shown as " +
			"ciphertext unless --show-secrets is passed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if impl.runEnvEdit == nil {
				impl.runEnvEdit = runEnvEditCmd
			}
			return impl.run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&impl.editorFlag, "editor", "",
		"The editor command to use. Overrides $VISUAL and $EDITOR")
	cmd.Flags().BoolVar(&impl.showSecrets, "show-secrets", false,
		"Show secret values in plaintext rather than ciphertext")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

func (cmd *configEditCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	if _, _, err := cmd.ws.ReadProject(); err != nil {
		return err
	}

	stack, err := cmdStack.RequireStack(
		ctx, cmdutil.Diag(), cmd.ws, cmdBackend.DefaultLoginManager,
		*cmd.stackRef, cmdStack.SetCurrent, opts, *cmd.configFile)
	if err != nil {
		return err
	}

	if !configStoreIsRemote(stack, *cmd.configFile) {
		return errors.New("`pulumi config edit` is only supported for remote config stacks")
	}
	return cmd.editRemote(ctx, stack)
}

func (cmd *configEditCmd) editRemote(ctx context.Context, stack backend.Stack) error {
	if err := rejectIfPinned(stack, *cmd.configFile); err != nil {
		return err
	}

	ref := stack.ConfigLocation().EscEnv
	if ref == nil {
		return errors.New("stack is configured for remote config but has no linked environment")
	}
	// `env edit` always targets the latest revision and rejects a versioned ref, so drop any pin.
	envRef, _, _ := strings.Cut(*ref, "@")

	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}
	// env edit accepts [<org>/][<project>/]<env>; qualify with the stack's org so it cannot fall back
	// to the user's default org.
	args := []string{"edit", orgNamer.OrgName() + "/" + envRef}
	if cmd.editorFlag != "" {
		args = append(args, "--editor", cmd.editorFlag)
	}
	if cmd.showSecrets {
		args = append(args, "--show-secrets")
	}

	// env edit rediscovers its backend from PULUMI_BACKEND_URL or the current login and never reads the
	// project's backend.url, so pin it to the backend this stack actually resolved to. Otherwise a
	// project pinned to a different cloud could edit a same-named environment in the ambient one.
	if urler, ok := stack.Backend().(interface{ CloudURL() string }); ok {
		defer bindBackendURL(urler.CloudURL())()
	}
	return cmd.runEnvEdit(ctx, args)
}

func runEnvEditCmd(ctx context.Context, args []string) error {
	return newEnvEditRoot(args).ExecuteContext(ctx)
}

// cobra always executes from the root, so args are set on the root with the env command's name
// prepended; setting them on the subcommand alone is ignored and the process's real os.Args
// (`config edit`) would leak in and fail as an unknown esc command.
func newEnvEditRoot(args []string) *cobra.Command {
	envCmd := cmdEnv.NewEnvCmd()
	root := envCmd.Root()
	if root != envCmd {
		args = append([]string{envCmd.Name()}, args...)
	}
	root.SetArgs(args)
	return root
}

// esc's GetCurrentCloudURL consults PULUMI_BACKEND_URL before the current login, so setting it makes
// esc deterministically target url; the returned func restores the prior value.
func bindBackendURL(url string) func() {
	const key = "PULUMI_BACKEND_URL"
	prev, had := os.LookupEnv(key)
	_ = os.Setenv(key, url)
	return func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}
