// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPluginRunCmd() *cobra.Command {
	var kind string

	cmd := &cobra.Command{
		Use:    "run NAME[@VERSION] [ARGS]",
		Args:   cmdutil.MinimumNArgs(1),
		Hidden: !env.Dev.Value(),
		Short:  "Run a command on a plugin binary",
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
				pluginSpec, err := workspace.NewPluginSpec(ctx, source, kind, nil, "", nil)
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

				d := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: cmdutil.GetGlobalColorization()})

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

					_, err = pkgWorkspace.InstallPlugin(ctx, pluginSpec, log)
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

			pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil)
			if err != nil {
				return fmt.Errorf("could not create plugin context: %w", err)
			}

			plugin, err := plugin.ExecPlugin(pctx, pluginPath, source, kind, pluginArgs, "", nil, false)
			if err != nil {
				return fmt.Errorf("could not execute plugin %s (%s): %w", source, pluginPath, err)
			}

			// Copy the plugin's stdout and stderr to the current process's stdout and stderr, and stdin to the
			// plugin's stdin.

			var wg sync.WaitGroup
			wg.Add(3)
			go func() {
				defer wg.Done()
				_, err := io.Copy(os.Stdout, plugin.Stdout)
				if err != nil && !errors.Is(err, io.EOF) {
					fmt.Fprintf(os.Stderr, "error reading plugin stdout: %v\n", err)
				}
			}()
			go func() {
				defer wg.Done()
				_, err := io.Copy(os.Stderr, plugin.Stderr)
				if err != nil && !errors.Is(err, io.EOF) {
					fmt.Fprintf(os.Stderr, "error reading plugin stderr: %v\n", err)
				}
			}()
			go func() {
				defer wg.Done()
				// Source based plugins don't support stdin yet.
				if plugin.Stdin == nil {
					b := make([]byte, 1024)
					n, err := os.Stdin.Read(b)
					if err != nil && !errors.Is(err, io.EOF) {
						fmt.Fprintf(os.Stderr, "error reading plugin stdin: %v\n", err)
					}
					if n > 0 {
						fmt.Fprintf(os.Stderr, "plugin stdin not supported for source based plugins\n")
					}
				} else {
					_, err := io.Copy(plugin.Stdin, os.Stdin)
					if err != nil && !errors.Is(err, io.EOF) {
						fmt.Fprintf(os.Stderr, "error copying plugin stdin: %v\n", err)
					}
				}
			}()

			// Wait for the plugin to finish.
			code, err := plugin.Wait()
			if err != nil {
				return fmt.Errorf("plugin %s exited with error: %w", source, err)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&kind,
		"kind", "tool", "The plugin kind")

	return cmd
}
