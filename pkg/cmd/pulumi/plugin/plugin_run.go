// Copyright 2016, Pulumi Corporation.
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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPluginRunCmd() *cobra.Command {
	var kind string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a command on a plugin binary",
		Long: "[EXPERIMENTAL] Run a command on a plugin binary.\n" +
			"\n" +
			"Directly executes a plugin binary, if VERSION is not specified " +
			"the latest installed plugin will be used.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !apitype.IsPluginKind(kind) {
				return fmt.Errorf("unrecognized plugin kind: %s", kind)
			}
			kind := apitype.PluginKind(kind)

			source := args[0]
			var pluginPath string
			if plugin.IsLocalPluginPath(ctx, source) {
				pluginPath = source
			} else {
				// TODO: Add support for --server and --checksums.
				pluginSpec, err := workspace.NewPluginDescriptor(ctx, args[0], kind, nil, "", nil)
				if err != nil {
					return err
				}

				if !tokens.IsName(pluginSpec.Name) {
					return fmt.Errorf("invalid plugin name %q", pluginSpec.Name)
				}

				source = fmt.Sprintf("%s %s", pluginSpec.Kind, pluginSpec.Name)
				if pluginSpec.Version != nil {
					source = fmt.Sprintf("%s@%s", source, pluginSpec.Version)
				}

				d := diag.DefaultSink(
					cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{Color: cmdutil.GetGlobalColorization()})

				pluginPath, err = workspace.GetPluginPath(ctx, d, pluginSpec, nil)
				if err != nil {
					// Try to install the plugin, unless auto plugin installs are turned off.
					var me *workspace.MissingError
					if !errors.As(err, &me) || env.DisableAutomaticPluginAcquisition.Value() {
						// Not a MissingError, return the original error.
						return fmt.Errorf("could not get plugin path: %w", err)
					}

					log := func(sev diag.Severity, msg string) {
						d.Logf(sev, diag.RawMessage("", msg))
					}

					_, err = pkgWorkspace.InstallPlugin(ctx, pluginSpec, log, schema.NewLoaderServerFromContext)
					if err != nil {
						return err
					}

					pluginPath, err = workspace.GetPluginPath(ctx, d, pluginSpec, nil)
					if err != nil {
						return fmt.Errorf("could not get plugin path: %w", err)
					}
				}
			}

			pluginArgs := args[1:]

			reg := cmdCmd.NewDefaultRegistry(ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, nil, env.Global())
			pluginHost, err := pkghost.New(context.WithoutCancel(ctx), nil, nil, nil, pkgWorkspace.EnsureLanguageInstalled,
				schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(reg))
			if err != nil {
				return fmt.Errorf("could not create plugin host: %w", err)
			}
			// host is owned here, closed after the context
			defer contract.IgnoreClose(pluginHost)
			pctx, err := plugin.NewContext(ctx, nil, nil, pluginHost, nil, ".", nil, false, nil)
			if err != nil {
				return fmt.Errorf("could not create plugin context: %w", err)
			}
			defer pctx.Close()

			// Always create RPC server for tool plugins
			grpcServer, err := createPluginRPCServer(ctx, pctx)
			if err != nil {
				return fmt.Errorf("create RPC server: %w", err)
			}
			defer grpcServer.Close()

			pluginEnv := env.NewEnv(env.MapStore{"PULUMI_RPC_TARGET": grpcServer.Addr()})

			plugin, err := plugin.ExecPlugin(pctx, pluginPath, source, kind, pluginArgs, "", pluginEnv, false)
			if err != nil {
				return fmt.Errorf("could not execute plugin %s (%s): %w", source, pluginPath, err)
			}

			// Copy the plugin's stdout and stderr to the current process's stdout and stderr.
			// For stdin, we start a copy goroutine but don't wait for it since it will block
			// indefinitely if there's no stdin input.

			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
			var wg sync.WaitGroup
			wg.Add(2) // Only wait for stdout and stderr, not stdin
			go func() {
				defer wg.Done()
				_, err := io.Copy(stdout, plugin.Stdout)
				if err != nil {
					fmt.Fprintf(stderr, "error reading plugin stdout: %v\n", err)
				}
			}()
			go func() {
				defer wg.Done()
				_, err := io.Copy(stderr, plugin.Stderr)
				if err != nil {
					fmt.Fprintf(stderr, "error reading plugin stderr: %v\n", err)
				}
			}()
			// Copy stdin in a separate goroutine but don't wait for it.
			// It will either complete if stdin has data, or be interrupted when plugin.Stdin is closed.
			go func() {
				_, _ = io.Copy(plugin.Stdin, os.Stdin)
				// Don't report error since stdin pipe closure is expected
			}()

			// Wait for the plugin and IO to finish.
			code, err := plugin.Wait(ctx)
			wg.Wait()

			if err != nil {
				return fmt.Errorf("plugin %s exited with error: %w", source, err)
			}
			if code != 0 {
				return pluginErrorCode{source, code}
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name-or-path", Usage: "path|name[@version]"},
			{Name: "args"},
		},
		Required: 1,
		Variadic: true,
	})

	cmd.PersistentFlags().StringVar(&kind,
		"kind", "tool", "The plugin kind")

	return cmd
}

var _ cmdCmd.CustomExitCodeError = pluginErrorCode{}

type pluginErrorCode struct {
	plugin string
	code   int
}

func (pec pluginErrorCode) CustomExitCode() int {
	return pec.code
}

func (pec pluginErrorCode) Error() string {
	return fmt.Sprintf("plugin %s exited with non-zero error code %d", pec.plugin, pec.code)
}

func (pec pluginErrorCode) Unwrap() error { return result.BailError(pec) }
