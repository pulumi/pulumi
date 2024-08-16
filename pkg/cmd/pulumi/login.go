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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type LoginArgs struct {
	CloudURL   string `args:"cloud-url" argsShort:"c" argsUsage:"A cloud URL to log in to"`
	DefaultOrg string `args:"default-org" argsUsage:"A default org to associate with the login. Please note, currently, only the managed and self-hosted backends support organizations"`
	LocalMode  bool   `args:"local" argsShort:"l" argsUsage:"Use Pulumi in local-only mode"`
	Insecure   bool   `argsUsage:"Allow insecure server connections when using SSL"`
}

func newLoginCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [<url>]",
		Short: "Log in to the Pulumi Cloud",
		Long: "Log in to the Pulumi Cloud.\n" +
			"\n" +
			"The Pulumi Cloud manages your stack's state reliably. Simply run\n" +
			"\n" +
			"    $ pulumi login\n" +
			"\n" +
			"and this command will prompt you for an access token, including a way to launch your web browser to\n" +
			"easily obtain one. You can script by using `PULUMI_ACCESS_TOKEN` environment variable.\n" +
			"\n" +
			"By default, this will log in to the managed Pulumi Cloud backend.\n" +
			"If you prefer to log in to a self-hosted Pulumi Cloud backend, specify a URL. For example, run\n" +
			"\n" +
			"    $ pulumi login https://api.pulumi.acmecorp.com\n" +
			"\n" +
			"to log in to a self-hosted Pulumi Cloud running at the api.pulumi.acmecorp.com domain.\n" +
			"\n" +
			"For `https://` URLs, the CLI will speak REST to a Pulumi Cloud that manages state and concurrency control.\n" +
			"You can specify a default org to use when logging into the Pulumi Cloud backend or a " +
			"self-hosted Pulumi Cloud.\n" +
			"\n" +
			"[PREVIEW] If you prefer to operate Pulumi independently of a Pulumi Cloud, and entirely local to your computer,\n" +
			"pass `file://<path>`, where `<path>` will be where state checkpoints will be stored. For instance,\n" +
			"\n" +
			"    $ pulumi login file://~\n" +
			"\n" +
			"will store your state information on your computer underneath `~/.pulumi`. It is then up to you to\n" +
			"manage this state, including backing it up, using it in a team environment, and so on.\n" +
			"\n" +
			"As a shortcut, you may pass --local to use your home directory (this is an alias for `file://~`):\n" +
			"\n" +
			"    $ pulumi login --local\n" +
			"\n" +
			"[PREVIEW] Additionally, you may leverage supported object storage backends from one of the cloud providers " +
			"to manage the state independent of the Pulumi Cloud. For instance,\n" +
			"\n" +
			"AWS S3:\n" +
			"\n" +
			"    $ pulumi login s3://my-pulumi-state-bucket\n" +
			"\n" +
			"GCP GCS:\n" +
			"\n" +
			"    $ pulumi login gs://my-pulumi-state-bucket\n" +
			"\n" +
			"Azure Blob:\n" +
			"\n" +
			"    $ pulumi login azblob://my-pulumi-state-bucket\n",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			args := UnmarshalArgs[LoginArgs](v, cmd)

			ctx := cmd.Context()
			displayOptions := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// If a <cloud> was specified as an argument, use it.
			if len(cliArgs) > 0 {
				if args.CloudURL != "" {
					return errors.New("only one of --cloud-url or argument URL may be specified, not both")
				}
				args.CloudURL = cliArgs[0]
			}

			// For local mode, store state by default in the user's home directory.
			if args.LocalMode {
				if args.CloudURL != "" {
					return errors.New("a URL may not be specified when --local mode is enabled")
				}
				args.CloudURL = diy.FilePathPrefix + "~"
			}

			// If we're on Windows, and this is a local login path, then allow the user to provide
			// backslashes as path separators.  We will normalize them here to forward slashes as that's
			// what the gocloud blob system requires.
			if strings.HasPrefix(args.CloudURL, diy.FilePathPrefix) && os.PathSeparator != '/' {
				args.CloudURL = filepath.ToSlash(args.CloudURL)
			}

			// Try to read the current project
			project, _, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			if args.CloudURL == "" {
				var err error
				args.CloudURL, err = workspace.GetCurrentCloudURL(project)
				if err != nil {
					return fmt.Errorf("could not determine current cloud: %w", err)
				}
			} else if url := strings.TrimPrefix(strings.TrimPrefix(
				args.CloudURL, "https://"), "http://"); strings.HasPrefix(url, "app.pulumi.com/") ||
				strings.HasPrefix(url, "pulumi.com") {
				return fmt.Errorf("%s is not a valid self-hosted backend, "+
					"use `pulumi login` without arguments to log into the Pulumi Cloud backend", args.CloudURL)
			} else {
				// Ensure we have the correct cloudurl type before logging in
				if err := validateCloudBackendType(args.CloudURL); err != nil {
					return err
				}
			}

			var be backend.Backend
			if diy.IsDIYBackendURL(args.CloudURL) {
				be, err = diy.Login(ctx, cmdutil.Diag(), args.CloudURL, project)
				if args.DefaultOrg != "" {
					return errors.New("unable to set default org for this type of backend")
				}
			} else {
				be, err = loginToCloud(ctx, args.CloudURL, project, args.Insecure, displayOptions)
				// if the user has specified a default org to associate with the backend
				if args.DefaultOrg != "" {
					cloudURL, err := workspace.GetCurrentCloudURL(project)
					if err != nil {
						return err
					}
					if err := workspace.SetBackendConfigDefaultOrg(cloudURL, args.DefaultOrg); err != nil {
						return err
					}
				}
			}
			if err != nil {
				return fmt.Errorf("problem logging in: %w", err)
			}

			if currentUser, _, _, err := be.CurrentUser(); err == nil {
				// TODO should we print the token information here? (via team MyTeam token MyToken)
				fmt.Printf("Logged in to %s as %s (%s)\n", be.Name(), currentUser, be.URL())
			} else {
				fmt.Printf("Logged in to %s (%s)\n", be.Name(), be.URL())
			}

			return nil
		}),
	}

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[LoginArgs](v, cmd)

	return cmd
}

func validateCloudBackendType(typ string) error {
	kind := strings.SplitN(typ, ":", 2)[0]
	supportedKinds := []string{"azblob", "gs", "s3", "file", "https", "http"}
	for _, supportedKind := range supportedKinds {
		if kind == supportedKind {
			return nil
		}
	}
	return fmt.Errorf("unknown backend cloudUrl format '%s' (supported Url formats are: "+
		"azblob://, gs://, s3://, file://, https:// and http://)",
		kind)
}
