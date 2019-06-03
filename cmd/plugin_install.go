// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/blang/semver"
	"github.com/cheggaaa/pb"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newPluginInstallCmd() *cobra.Command {
	var serverURL string
	var cloudURL string
	var exact bool
	var file string
	var reinstall bool
	var verbose bool

	var cmd = &cobra.Command{
		Use:   "install [KIND NAME VERSION]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Install one or more plugins",
		Long: "Install one or more plugins.\n" +
			"\n" +
			"This command is used manually install plugins required by your program.  It may\n" +
			"be run either with a specific KIND, NAME, and VERSION, or by omitting these and\n" +
			"letting Pulumi compute the set of plugins that may be required by the current\n" +
			"project.  VERSION cannot be a range: it must be a specific number.\n" +
			"\n" +
			"If you let Pulumi compute the set to download, it is conservative and may end up\n" +
			"downloading more plugins than is strictly necessary.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			if serverURL != "" && cloudURL != "" {
				return errors.New("only one of server and cloud-url may be specified")
			}

			if cloudURL != "" {
				cmdutil.Diag().Warningf(diag.Message("", "cloud-url is deprecated, please pass '--server "+
					"%s/releases/plugins' instead."), cloudURL)

				serverURL = cloudURL + "/releases/plugins"
			}

			// Note we don't presently set this as the default value for `--server` so we can play games like the above
			// where we want to ensure at most one of `--server` or `--cloud-url` is set.
			if serverURL == "" {
				serverURL = "https://api.pulumi.com/releases/plugins"
			}

			// Parse the kind, name, and version, if specified.
			var installs []workspace.PluginInfo
			if len(args) > 0 {
				if !workspace.IsPluginKind(args[0]) {
					return errors.Errorf("unrecognized plugin kind: %s", args[0])
				} else if len(args) < 2 {
					return errors.New("missing plugin name argument")
				} else if len(args) < 3 {
					return errors.New("missing plugin version argument")
				}
				version, err := semver.ParseTolerant(args[2])
				if err != nil {
					return errors.Wrap(err, "invalid plugin semver")
				}
				installs = append(installs, workspace.PluginInfo{
					Kind:      workspace.PluginKind(args[0]),
					Name:      args[1],
					Version:   &version,
					ServerURL: serverURL,
				})
			} else {
				if file != "" {
					return errors.New("--file (-f) is only valid if a specific package is being installed")
				}

				// If a specific plugin wasn't given, compute the set of plugins the current project needs.
				plugins, err := getProjectPlugins()
				if err != nil {
					return err
				}
				for _, plugin := range plugins {
					// Skip language plugins; by definition, we already have one installed.
					// TODO[pulumi/pulumi#956]: eventually we will want to honor and install these in the usual way.
					if plugin.Kind != workspace.LanguagePlugin {
						installs = append(installs, plugin)
					}
				}
			}

			// Now for each kind, name, version pair, download it from the release website, and install it.
			for _, install := range installs {
				label := fmt.Sprintf("[%s plugin %s]", install.Kind, install)
				cmdutil.Diag().Infoerrf(
					diag.Message("", "%s installing"), label)

				// If the plugin already exists, don't download it unless --reinstall was passed.  Note that
				// by default we accept plugins with >= constraints, unless --exact was passed which requires ==.
				if !reinstall {
					if exact {
						if workspace.HasPlugin(install) {
							if verbose {
								cmdutil.Diag().Infoerrf(
									diag.Message("", "%s skipping install (existing == match)"), label)
							}
							continue
						}
					} else {
						if has, _ := workspace.HasPluginGTE(install); has {
							if verbose {
								cmdutil.Diag().Infoerrf(
									diag.Message("", "%s skipping install (existing >= match)"), label)
							}
							continue
						}
					}
				}

				// If we got here, actually try to do the download.
				var source string
				var tarball io.ReadCloser
				var err error
				if file == "" {
					if verbose {
						cmdutil.Diag().Infoerrf(
							diag.Message("", "%s downloading from %s"), label, install.ServerURL)
					}
					var size int64
					if tarball, size, err = install.Download(); err != nil {
						return errors.Wrapf(err, "%s downloading from %s", label, install.ServerURL)
					}
					// If we know the length of the download, show a progress bar.
					if size != -1 {
						bar := pb.New(int(size))
						tarball = newBarProxyReadCloser(bar, tarball)
						bar.Prefix(displayOpts.Color.Colorize(colors.SpecUnimportant + "Downloading plugin: "))
						bar.Postfix(displayOpts.Color.Colorize(colors.Reset))
						bar.SetMaxWidth(80)
						bar.SetUnits(pb.U_BYTES)
						bar.Start()
					}
				} else {
					source = file
					if verbose {
						cmdutil.Diag().Infoerrf(
							diag.Message("", "%s opening tarball from %s"), label, file)
					}
					if tarball, err = os.Open(file); err != nil {
						return errors.Wrapf(err, "opening file %s", source)
					}
				}
				if verbose {
					cmdutil.Diag().Infoerrf(
						diag.Message("", "%s installing tarball ..."), label)
				}
				if err = install.Install(tarball); err != nil {
					return errors.Wrapf(err, "installing %s from %s", label, source)
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(&serverURL,
		"server", "", "A URL to download plugins from")
	cmd.PersistentFlags().StringVarP(&cloudURL,
		"cloud-url", "c", "", "A cloud URL to download releases from")
	cmd.PersistentFlags().BoolVar(&exact,
		"exact", false, "Force installation of an exact version match (usually >= is accepted)")
	cmd.PersistentFlags().StringVarP(&file,
		"file", "f", "", "Install a plugin from a tarball file, instead of downloading it")
	cmd.PersistentFlags().BoolVar(&reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")
	cmd.PersistentFlags().BoolVar(&verbose,
		"verbose", false, "Print detailed information about the installation steps")

	// We are moving away from supporting this option, for now we mark it hidden.
	contract.AssertNoError(cmd.PersistentFlags().MarkHidden("cloud-url"))

	return cmd
}

// barCloser is an implementation of io.Closer that finishes a progress bar upon Close() as well as closing its
// underlying readCloser.
type barCloser struct {
	bar        *pb.ProgressBar
	readCloser io.ReadCloser
}

func (bc *barCloser) Read(dest []byte) (int, error) {
	return bc.readCloser.Read(dest)
}

func (bc *barCloser) Close() error {
	bc.bar.Finish()
	return bc.readCloser.Close()
}

func newBarProxyReadCloser(bar *pb.ProgressBar, r io.Reader) io.ReadCloser {
	return &barCloser{
		bar:        bar,
		readCloser: bar.NewProxyReader(r),
	}
}
