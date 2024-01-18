// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newInstallCmd() *cobra.Command {
	var reinstall bool

	cmd := &cobra.Command{
		Use:   "install",
		Args:  cmdutil.NoArgs,
		Short: "Install packages and plugins for the current program",
		Long: "Install packages and plugins for the current program.\n" +
			"\n" +
			"This command is used to manually install packages and plugins required by your program.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Load the project
			proj, root, err := readProject()
			if err != nil {
				return err
			}

			span := opentracing.SpanFromContext(ctx)
			projinfo := &engine.Projinfo{Proj: proj, Root: root}
			pwd, main, pctx, err := engine.ProjectInfoContext(
				projinfo,
				nil,
				cmdutil.Diag(),
				cmdutil.Diag(),
				false,
				span,
				nil,
			)
			if err != nil {
				return err
			}

			defer pctx.Close()

			// First make sure the language plugin is present.  We need this to load the required resource plugins.
			// TODO: we need to think about how best to version this.  For now, it always picks the latest.
			runtime := proj.Runtime
			lang, err := pctx.Host.LanguageRuntime(pctx.Root, pctx.Pwd, runtime.Name(), runtime.Options())
			if err != nil {
				return fmt.Errorf("load language plugin %s: %w", runtime.Name(), err)
			}

			if err = lang.InstallDependencies(pwd, main); err != nil {
				return fmt.Errorf("installing dependencies: %w", err)
			}

			// Compute the set of plugins the current project needs.
			installs, err := lang.GetRequiredPlugins(plugin.ProgInfo{
				Pwd:     pwd,
				Program: main,
			})
			if err != nil {
				return err
			}

			// Now for each kind, name, version pair, download it from the release website, and install it.
			for _, install := range installs {
				// PluginSpec.String() just returns the name and version, we want the kind too.
				label := fmt.Sprintf("%s plugin %s", install.Kind, install)

				// If the plugin already exists, don't download it unless --reinstall was passed.
				if !reinstall {
					if install.Version != nil {
						if workspace.HasPlugin(install) {
							logging.V(1).Infof("%s skipping install (existing == match)", label)
							continue
						}
					} else {
						if has, _ := workspace.HasPluginGTE(install); has {
							logging.V(1).Infof("%s skipping install (existing >= match)", label)
							continue
						}
					}
				}

				pctx.Diag.Infoerrf(diag.Message("", "%s installing"), label)

				// If we got here, actually try to do the download.
				withProgress := func(stream io.ReadCloser, size int64) io.ReadCloser {
					return workspace.ReadCloserProgressBar(stream, size, "Downloading plugin", displayOpts.Color)
				}
				retry := func(err error, attempt int, limit int, delay time.Duration) {
					pctx.Diag.Warningf(
						diag.Message("", "Error downloading plugin: %s\nWill retry in %v [%d/%d]"), err, delay, attempt, limit)
				}

				r, err := workspace.DownloadToFile(install, withProgress, retry)
				if err != nil {
					return fmt.Errorf("%s downloading from %s: %w", label, install.PluginDownloadURL, err)
				}
				defer func() {
					err := os.Remove(r.Name())
					if err != nil {
						pctx.Diag.Warningf(
							diag.Message("", "Error removing temporary file %s: %s"), r.Name(), err)
					}
				}()

				payload := workspace.TarPlugin(r)

				logging.V(1).Infof("%s installing tarball ...", label)
				if err = install.InstallWithContext(ctx, payload, reinstall); err != nil {
					return fmt.Errorf("installing %s: %w", label, err)
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(&reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")

	return cmd
}
