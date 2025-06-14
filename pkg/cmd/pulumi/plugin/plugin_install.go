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
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/util"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPluginInstallCmd() *cobra.Command {
	var picmd pluginInstallCmd
	cmd := &cobra.Command{
		Use:   "install [KIND NAME [VERSION]]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Install one or more plugins",
		Long: "Install one or more plugins.\n" +
			"\n" +
			"This command is used to manually install plugins required by your program. It\n" +
			"may be run with a specific KIND, NAME, and optionally, VERSION, or by omitting\n" +
			"these arguments and letting Pulumi compute the set of plugins required by the\n" +
			"current project. When Pulumi computes the download set automatically, it may\n" +
			"download more plugins than are strictly necessary.\n" +
			"\n" +
			"If VERSION is specified, it cannot be a range; it must be a specific number.\n" +
			"If VERSION is unspecified, Pulumi will attempt to look up the latest version of\n" +
			"the plugin, though the result is not guaranteed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return picmd.Run(ctx, args)
		},
	}

	cmd.PersistentFlags().StringVar(&picmd.serverURL,
		"server", "", "A URL to download plugins from")
	cmd.PersistentFlags().BoolVar(&picmd.exact,
		"exact", false, "Force installation of an exact version match (usually >= is accepted)")
	cmd.PersistentFlags().StringVarP(&picmd.file,
		"file", "f", "", "Install a plugin from a binary, folder or tarball, instead of downloading it")
	cmd.PersistentFlags().BoolVar(&picmd.reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")
	cmd.PersistentFlags().StringVar(&picmd.checksum,
		"checksum", "", "The expected SHA256 checksum for the plugin archive")

	return cmd
}

type pluginInstallCmd struct {
	serverURL string
	exact     bool
	file      string
	reinstall bool
	checksum  string

	diag     diag.Sink
	env      env.Env
	color    colors.Colorization
	registry registry.Registry

	pluginGetLatestVersion func(
		workspace.PluginSpec, context.Context,
	) (*semver.Version, error) // == workspace.PluginSpec.GetLatestVersion

	installPluginSpec func(
		ctx context.Context, label string,
		install workspace.PluginSpec, file string,
		sink diag.Sink, color colors.Colorization, reinstall bool,
	) error // == installPluginSpec
}

func (cmd *pluginInstallCmd) Run(ctx context.Context, args []string) error {
	if cmd.env == nil {
		cmd.env = env.Global()
	}
	if cmd.diag == nil {
		cmd.diag = cmdutil.Diag()
	}
	if cmd.color == "" {
		cmd.color = cmdutil.GetGlobalColorization()
	}
	if cmd.pluginGetLatestVersion == nil {
		cmd.pluginGetLatestVersion = (workspace.PluginSpec).GetLatestVersion
	}
	if cmd.installPluginSpec == nil {
		cmd.installPluginSpec = installPluginSpec
	}
	if cmd.registry == nil {
		cmd.registry = registry.NewOnDemandRegistry(func() (registry.Registry, error) {
			b, err := cmdBackend.NonInteractiveCurrentBackend(
				ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, nil,
			)
			if err == nil && b != nil {
				return b.GetReadOnlyPackageRegistry(), nil
			}
			if b == nil || errors.Is(err, backenderr.ErrLoginRequired) {
				return unauthenticatedregistry.New(cmd.diag, cmd.env), nil
			}
			return nil, fmt.Errorf("could not get registry backend: %w", err)
		})
	}

	// Parse the kind, name, and version, if specified.
	var installs []workspace.PluginSpec
	if len(args) > 0 {
		if !apitype.IsPluginKind(args[0]) {
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
		if len(args) < 3 && cmd.file != "" {
			return errors.New("missing plugin version argument, this is required if installing from a file")
		}

		var checksums map[string][]byte
		if cmd.checksum != "" {
			checksumBytes, err := hex.DecodeString(cmd.checksum)
			if err != nil {
				return fmt.Errorf("--checksum was not a valid hex string: %w", err)
			}
			checksums = map[string][]byte{
				runtime.GOOS + "-" + runtime.GOARCH: checksumBytes,
			}
		}

		pluginSpec, err := workspace.NewPluginSpec(ctx,
			args[1], apitype.PluginKind(args[0]), version, cmd.serverURL, checksums)
		if err != nil {
			return err
		}

		// Bundled plugins are generally not installable with this command. They are expected to be
		// distributed with Pulumi itself. But we turn this check off if PULUMI_DEV is set so we can
		// test installing plugins that are being moved to their own distribution (such as when we move
		// pulumi-nodejs).
		if !cmd.env.GetBool(env.Dev) && workspace.IsPluginBundled(pluginSpec.Kind, pluginSpec.Name) {
			return fmt.Errorf(
				"the %v %v plugin is bundled with Pulumi, and cannot be directly installed"+
					" with this command. If you need to reinstall this plugin, reinstall"+
					" Pulumi via your package manager or install script.",
				pluginSpec.Name,
				pluginSpec.Kind,
			)
		}

		// Try and set known plugin download URLs
		if urlSet := util.SetKnownPluginDownloadURL(&pluginSpec); urlSet {
			cmd.diag.Infof(
				diag.Message("", "Plugin download URL set to %s"), pluginSpec.PluginDownloadURL)
		}

		if pluginSpec.Kind == apitype.ResourcePlugin && // The registry only supports resource plugins
			!cmd.env.GetBool(env.DisableRegistryResolve) &&
			pluginSpec.PluginDownloadURL == "" { // Don't override explicit pluginDownloadURLs
			pkgMetadata, err := registry.ResolvePackageFromName(ctx, cmd.registry, pluginSpec.Name, pluginSpec.Version)
			if err == nil {
				pluginSpec.Name = pkgMetadata.Name
				pluginSpec.PluginDownloadURL = pkgMetadata.PluginDownloadURL
			} else if errors.Is(err, registry.ErrNotFound) &&
				registry.PulumiPublishedBeforeRegistry(pluginSpec.Name) {
				// If the error is [registry.ErrNotFound], then this could
				// be a Pulumi plugin that isn't in the registry.
				//
				// We just ignore the failure in the hope that the
				// download against GitHub will succeed.
			} else {
				for _, suggested := range registry.GetSuggestedPackages(err) {
					cmd.diag.Infof(diag.Message("", "%s/%s/%s@%s is a similar package"),
						suggested.Source, suggested.Publisher, suggested.Name,
						suggested.Version,
					)
				}
				return fmt.Errorf("Unable to resolve package from name: %w", err)
			}
		}

		// If we don't have a version try to look one up
		if version == nil && pluginSpec.Version == nil {
			latestVersion, err := cmd.pluginGetLatestVersion(pluginSpec, ctx)
			if err != nil {
				return err
			}
			pluginSpec.Version = latestVersion
		}

		installs = append(installs, pluginSpec)
	} else {
		if cmd.file != "" {
			return errors.New("--file (-f) is only valid if a specific package is being installed")
		}
		if cmd.checksum != "" {
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
			if plugin.Kind != apitype.LanguagePlugin {
				installs = append(installs, plugin)
			}
		}
	}

	// Now for each kind, name, version pair, download it from the release website, and install it.
	for _, install := range installs {
		label := fmt.Sprintf("[%s plugin %s]", install.Kind, install)

		// If the plugin already exists, don't download it unless --reinstall was passed.  Note that
		// by default we accept plugins with >= constraints, unless --exact was passed which requires ==.
		if !cmd.reinstall {
			if cmd.exact {
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

		cmd.diag.Infoerrf(diag.Message("", "%s installing"), label)
		err := cmd.installPluginSpec(ctx, label, install, cmd.file, cmd.diag, cmd.color, cmd.reinstall)
		if err != nil {
			return err
		}
	}

	return nil
}

func installPluginSpec(
	ctx context.Context, label string,
	install workspace.PluginSpec, file string,
	sink diag.Sink, color colors.Colorization, reinstall bool,
) error {
	// If we got here, actually try to do the download.
	var source string
	var payload workspace.PluginContent
	var err error
	if file == "" {
		withProgress := func(stream io.ReadCloser, size int64) io.ReadCloser {
			return workspace.ReadCloserProgressBar(stream, size, "Downloading plugin", color)
		}
		retry := func(err error, attempt int, limit int, delay time.Duration) {
			sink.Warningf(
				diag.Message("", "Error downloading plugin: %s\nWill retry in %v [%d/%d]"), err, delay, attempt, limit)
		}

		r, err := workspace.DownloadToFile(ctx, install, withProgress, retry)
		if err != nil {
			return fmt.Errorf("%s downloading from %s: %w", label, install.PluginDownloadURL, err)
		}
		defer func() { contract.IgnoreError(os.Remove(r.Name())) }()

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
	return nil
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
		// Windows doesn't have executable bits to check
		if runtime.GOOS != "windows" && (stat.Mode()&0o100) == 0 {
			return nil, fmt.Errorf("%s is not executable", source)
		}
		return workspace.SingleFilePlugin(f, spec), nil
	}
	return workspace.TarPlugin(f), nil
}
