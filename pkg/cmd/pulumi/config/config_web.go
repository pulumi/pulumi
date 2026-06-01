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
	"io"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// consoleURLProvider is implemented by Pulumi Cloud backends. CloudConsoleURL joins the given path
// segments onto the console base URL; it never embeds tokens or credentials.
type consoleURLProvider interface {
	CloudConsoleURL(paths ...string) string
}

// configEnvConsoleCmd implements the hidden `pulumi config env console` command, opening the linked
// ESC environment in the Pulumi Cloud console. It is only supported for remote (ESC-backed) stacks.
type configEnvConsoleCmd struct {
	ws         pkgWorkspace.Context
	stackRef   *string
	configFile *string

	// Injectable so tests do not launch a browser.
	openBrowser func(url string) error

	stdout io.Writer
}

func newConfigEnvConsoleCmd(ws pkgWorkspace.Context, stackRef *string, configFile *string) *cobra.Command {
	impl := &configEnvConsoleCmd{ws: ws, stackRef: stackRef, configFile: configFile}

	cmd := &cobra.Command{
		Use:    "console",
		Short:  "Open the stack's ESC environment in the Pulumi Cloud console",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			impl.stdout = cmd.OutOrStdout()
			if impl.openBrowser == nil {
				impl.openBrowser = browser.OpenURL
			}
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

func (cmd *configEnvConsoleCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	if _, _, err := cmd.ws.ReadProject(); err != nil {
		return err
	}

	stack, err := cmdStack.RequireStack(
		ctx, cmdutil.Diag(), cmd.ws, cmdBackend.DefaultLoginManager,
		*cmd.stackRef, cmdStack.LoadOnly, opts, *cmd.configFile)
	if err != nil {
		return err
	}
	return cmd.runWithStack(stack, *cmd.configFile)
}

func (cmd *configEnvConsoleCmd) runWithStack(stack backend.Stack, configFile string) error {
	// An explicit --config-file selects the local store even on a linked stack, matching the rest of
	// the config commands, so the remote console is unavailable in that case.
	if !configStoreIsRemote(stack, configFile) {
		return errors.New("config env console is only supported for remote config stacks")
	}
	loc := stack.ConfigLocation()
	if loc.EscEnv == nil {
		return errors.New("stack is configured for remote config but has no linked environment")
	}
	envProject, envName, err := splitEnvRef(*loc.EscEnv)
	if err != nil {
		return err
	}

	provider, ok := stack.Backend().(consoleURLProvider)
	if !ok {
		return errors.New("config env console requires a Pulumi Cloud backend")
	}
	orgNamer, ok := stack.(interface{ OrgName() string })
	if !ok {
		return errors.New("internal error: stack does not provide an organization name")
	}

	url := provider.CloudConsoleURL(orgNamer.OrgName(), "esc", envProject, envName)
	if url == "" {
		return errors.New("the backend did not provide a console URL for this environment")
	}

	ui.Fprintf(cmd.stdout, "Opening %s ...\n", url)
	if err := cmd.openBrowser(url); err != nil {
		ui.Fprintf(cmd.stdout, "unable to open the browser automatically; open the URL above manually.\n")
	}
	return nil
}
