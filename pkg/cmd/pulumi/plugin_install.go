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

package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"

	"github.com/blang/semver"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPluginInstallCmd() *cobra.Command {
	var serverURL string
	var exact bool
	var file string
	var reinstall bool
	var checksum string

	var cmd = &cobra.Command{
		Use:   "install [KIND NAME [VERSION]]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Install one or more plugins",
		Long: "Install one or more plugins.\n" +
			"\n" +
			"This command is used manually install plugins required by your program.  It may\n" +
			"be run either with a specific KIND, NAME, and VERSION, or by omitting these and\n" +
			"letting Pulumi compute the set of plugins that may be required by the current\n" +
			"project. If specified VERSION cannot be a range: it must be a specific number.\n" +
			"\n" +
			"If you let Pulumi compute the set to download, it is conservative and may end up\n" +
			"downloading more plugins than is strictly necessary.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Parse the kind, name, and version, if specified.
			var installs []workspace.PluginSpec
			if len(args) > 0 {
				if !workspace.IsPluginKind(args[0]) {
					return fmt.Errorf("unrecognized plugin kind: %s", args[0])
				} else if len(args) < 2 {
					return errors.New("missing plugin name argument")
				}

				var version *semver.Version
				if len(args) == 3 {
					parsedVersion, err := semver.ParseTolerant(args[2])
					version = &parsedVersion
					if err != nil {
						return fmt.Errorf("invalid plugin semver: %w", err)
					}
				}
				if len(args) < 3 && file != "" {
					return errors.New("missing plugin version argument, this is required if installing from a file")
				}

				var checksums map[string][]byte
				if checksum != "" {
					checksumBytes, err := hex.DecodeString(checksum)
					if err != nil {
						return fmt.Errorf("--checksum was not a valid hex string: %w", err)
					}
					checksums = map[string][]byte{
						runtime.GOOS + "-" + runtime.GOARCH: checksumBytes,
					}
				}

				pluginSpec := workspace.PluginSpec{
					Kind:              workspace.PluginKind(args[0]),
					Name:              args[1],
					Version:           version,
					PluginDownloadURL: serverURL, // If empty, will use default plugin source.
					Checksums:         checksums,
				}

				// If we don't have a version try to look one up
				if version == nil {
					latestVersion, err := pluginSpec.GetLatestVersion()
					if err != nil {
						return err
					}
					pluginSpec.Version = latestVersion
				}

				installs = append(installs, pluginSpec)
			} else {
				if file != "" {
					return errors.New("--file (-f) is only valid if a specific package is being installed")
				}
				if checksum != "" {
					return errors.New("--checksum is only valid if a specific package is being installed")
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

				// If the plugin already exists, don't download it unless --reinstall was passed.  Note that
				// by default we accept plugins with >= constraints, unless --exact was passed which requires ==.
				if !reinstall {
					if exact {
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

				cmdutil.Diag().Infoerrf(
					diag.Message("", "%s installing"), label)

				// If we got here, actually try to do the download.
				var source string
				var payload workspace.PluginContent
				var err error
				if file == "" {
					withProgress := func(stream io.ReadCloser, size int64) io.ReadCloser {
						return workspace.ReadCloserProgressBar(stream, size, "Downloading plugin", displayOpts.Color)
					}
					retry := func(err error, attempt int, limit int, delay time.Duration) {
						cmdutil.Diag().Warningf(
							diag.Message("", "Error downloading plugin: %s\nWill retry in %v [%d/%d]"), err, delay, attempt, limit)
					}

					r, err := workspace.DownloadToFile(install, withProgress, retry)
					if err != nil {
						return fmt.Errorf("%s downloading from %s: %w", label, install.PluginDownloadURL, err)
					}

					payload = workspace.TarPlugin(r)
				} else {
					source = file
					logging.V(1).Infof("%s opening tarball from %s", label, file)
					payload, err = getFilePayload(file, install)
					if err != nil {
						return err
					}
				}
				logging.V(1).Infof("%s installing tarball ...", label)
				if err = install.InstallWithContext(ctx, payload, reinstall); err != nil {
					return fmt.Errorf("installing %s from %s: %w", label, source, err)
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(&serverURL,
		"server", "", "A URL to download plugins from")
	cmd.PersistentFlags().BoolVar(&exact,
		"exact", false, "Force installation of an exact version match (usually >= is accepted)")
	cmd.PersistentFlags().StringVarP(&file,
		"file", "f", "", "Install a plugin from a binary, folder or tarball, instead of downloading it")
	cmd.PersistentFlags().BoolVar(&reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")
	cmd.PersistentFlags().StringVar(&checksum,
		"checksum", "", "The expected SHA256 checksum for the plugin archive")

	return cmd
}

func getFilePayload(file string, spec workspace.PluginSpec) (workspace.PluginContent, error) {
	source := file
	stat, err := os.Stat(file)
	if err != nil {
		return nil, fmt.Errorf("stat on file %s: %w", source, err)
	}

	if stat.IsDir() {
		return workspace.DirPlugin(file), nil
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", source, err)
	}
	compressHeader := make([]byte, 5)
	_, err = f.Read(compressHeader)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", source, err)
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seeking back in file %s: %w", source, err)
	}
	if !encoding.IsCompressed(compressHeader) {
		if (stat.Mode() & 0100) == 0 {
			return nil, fmt.Errorf("%s is not executable", source)
		}
		return workspace.SingleFilePlugin(f, spec), nil
	}
	return workspace.TarPlugin(f), nil
}
