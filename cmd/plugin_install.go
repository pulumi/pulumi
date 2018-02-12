// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"io"
	"os"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newPluginInstallCmd() *cobra.Command {
	var cloudURL string
	var file string
	var cmd = &cobra.Command{
		Use:   "install [KIND NAME VERSION]",
		Args:  cmdutil.MaximumNArgs(3),
		Short: "Install one or more plugins",
		Long: "Install one or more plugins.\n" +
			"\n" +
			"By default, Pulumi will download plugins as needed during program execution.\n" +
			"If you prefer, you may use the install command to manually install plugins:\n" +
			"either by running it with a specific KIND, NAME, and VERSION, or by omitting\n" +
			"these and letting Pulumi compute the set of plugins that may be required by\n" +
			"the current project.  VERSION cannot be a range: it must be a specific number.\n" +
			"\n" +
			"Note that in this latter mode, Pulumi is conservative and may download more\n" +
			"than is strictly required.  To only download the precise list of what a project\n" +
			"needs, simply run Pulumi in its default mode of downloading them on demand: it\n" +
			"will download precisely what it needs.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
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
					Kind:    workspace.PluginKind(args[0]),
					Name:    args[1],
					Version: &version,
				})
			} else if file == "" {
				return errors.New("--file (-f) is only valid if a specific package is being installed")
			}

			// If a specific plugin wasn't given, compute the set of plugins the current project needs.
			// TODO[pulumi/home#11]: before calling this work item complete, we need to implement this.

			// Target the cloud URL for downloads.
			var releases cloud.Backend
			if len(installs) > 0 && file != "" {
				releases = cloud.New(cmdutil.Diag(), cloud.ValueOrDefaultURL(cloudURL))
			}

			// Now for each kind, name, version pair, download it from the release website, and install it.
			for _, install := range installs {
				var source string
				var tarball io.ReadCloser
				var err error
				if file == "" {
					source = releases.CloudURL()
					if tarball, err = releases.DownloadPlugin(install, true); err != nil {
						return errors.Wrapf(err, "downloading %s from %s", install.String(), source)
					}
				} else {
					source = file
					if tarball, err = os.Open(file); err != nil {
						return errors.Wrapf(err, "opening file %s", source)
					}
				}
				if err = install.Install(tarball); err != nil {
					return errors.Wrapf(err, "installing %s from %s", install.String(), source)
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL,
		"cloud-url", "c", "", "A cloud URL to download releases from")
	cmd.PersistentFlags().StringVarP(&file,
		"file", "f", "", "Install a plugin from a tarball file, instead of downloading it")

	return cmd
}
