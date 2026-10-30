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

package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// NewAuthCmd returns the `pulumi auth` command group.
func NewAuthCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Check cloud provider credentials",
		Long: "Check cloud provider credentials.\n" +
			"\n" +
			"Use the auth family of commands to verify that the credentials Pulumi will use\n" +
			"for a cloud provider are valid, before running `pulumi up`.",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newAuthCheckCmd(ws, lm))

	return cmd
}

func newAuthCheckCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var stackName string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check that a provider's credentials are valid",
		Long: "Check that a provider's credentials are valid.\n" +
			"\n" +
			"This command configures the given provider the same way `pulumi up` would and\n" +
			"reports whether it accepts the configuration and credentials:\n" +
			"\n" +
			"    $ pulumi auth check aws\n" +
			"\n" +
			"If the provider is not installed, the latest version is installed first.\n" +
			"\n" +
			"Inside a project with a selected stack (or with `--stack`), the stack's configuration\n" +
			"is used, including ESC environments and secrets. Otherwise only the environment and\n" +
			"ambient credentials, such as the AWS shared config files, are used.\n" +
			"\n" +
			"Only providers that declare support for credential checks in their schema can be\n" +
			"checked.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}
			sink := cmdutil.Diag()

			// This must happen before the provider launches, because opening the stack's ESC
			// environment sets the environment variables that the provider inherits.
			target, err := loadCheckTarget(ctx, sink, ws, lm, stackName, opts)
			if err != nil {
				return err
			}

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			registry := cmdCmd.NewDefaultRegistry(ctx, lm, ws, nil, sink, env.Global())
			pluginHost, err := pkghost.New(context.WithoutCancel(ctx), sink, sink, nil,
				pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(registry))
			if err != nil {
				return err
			}
			// host is owned here, closed after the context
			defer contract.IgnoreClose(pluginHost)
			pctx, err := plugin.NewContext(ctx, sink, sink, pluginHost, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

			prov, _, err := packages.ProviderFromSource(ws, pctx, args[0], registry, env.Global(), 0, "")
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(prov)

			return checkCredentials(ctx, cmd.OutOrStdout(), prov, target)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "provider"}},
		Required:  1,
	})

	cmd.Flags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack whose configuration to use. Defaults to the current stack, if any")

	return cmd
}

// loadCheckTarget returns a target holding the configuration of the named stack, or of the current
// stack if stackName is empty. Without a project, or without a selected stack, it returns nil so
// that only ambient credentials are checked.
func loadCheckTarget(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	stackName string, opts display.Options,
) (*deploy.Target, error) {
	proj, _, err := ws.ReadProject("")
	if errors.Is(err, workspace.ErrProjectNotFound) {
		if stackName != "" {
			return nil, errors.New("--stack can only be used inside a Pulumi project")
		}
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var s backend.Stack
	if stackName != "" {
		s, err = cmdStack.RequireStack(ctx, sink, ws, lm, stackName, cmdStack.LoadOnly, opts, "")
	} else {
		s, err = currentStack(ctx, ws, lm, proj)
	}
	if err != nil || s == nil {
		return nil, err
	}

	sc, sm, err := cmdConfig.GetStackConfiguration(
		ctx, sink, cmdStack.NewStackSecretsManagerLoaderFromEnv(), s, proj, "", nil)
	if err != nil {
		return nil, fmt.Errorf("loading configuration for stack %s: %w", s.Ref(), err)
	}
	return stackTarget(ctx, s.Ref().Name(), proj, sc, sm)
}

// stackTarget merges ESC and project configuration into the stack's configuration, as `pulumi up`
// does. Unlike `pulumi up` it doesn't validate the result: only the provider's values matter here.
func stackTarget(
	ctx context.Context, name tokens.StackName, proj *workspace.Project, sc backend.StackConfiguration, sm secrets.Manager,
) (*deploy.Target, error) {
	if err := pkgWorkspace.ApplyProjectConfig(
		ctx, name.String(), proj, sc.Environment, sc.Config, sm.Encrypter(), sm.Decrypter(),
	); err != nil {
		return nil, fmt.Errorf("applying configuration for stack %s: %w", name, err)
	}
	return &deploy.Target{Name: name, Config: sc.Config, Decrypter: sc.Decrypter}, nil
}

// currentStack returns the currently selected stack, or nil if none is selected. Unlike other
// stack commands it never prompts: if the user is not logged in, there is no current stack.
func currentStack(
	ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, proj *workspace.Project,
) (backend.Stack, error) {
	b, err := cmdBackend.NonInteractiveCurrentBackend(ctx, ws, lm, proj)
	if err != nil || b == nil {
		return nil, err
	}
	return state.CurrentStack(ctx, ws, b)
}

// checkCredentials runs the credentials check against prov and reports the outcome. It returns an
// error if the provider does not support the check or rejects the configuration or credentials.
func checkCredentials(ctx context.Context, stdout io.Writer, prov plugin.Provider, target *deploy.Target) error {
	if target != nil {
		fmt.Fprintf(stdout, "Using configuration from stack %s\n", target.Name)
	} else {
		fmt.Fprintln(stdout, "No stack selected; using credentials from the environment")
	}

	info, err := credentialcheck.Check(ctx, prov, plugin.GetSchemaRequest{}, target)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%sYour %s credentials are valid\n", cmdutil.EmojiOr("✅ ", ""), info.DisplayName)
	return nil
}
