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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newLoginCmd() *cobra.Command {
	var cloudURL string
	var defaultOrg string
	var localMode bool

	cmd := &cobra.Command{
		Use:   placeholder.Use,
		Short: placeholder.Short,
		Long:  placeholder.Long,
		Args:  cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOptions := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// If a <cloud> was specified as an argument, use it.
			if len(args) > 0 {
				if cloudURL != "" {
					return errors.New("only one of --cloud-url or argument URL may be specified, not both")
				}
				cloudURL = args[0]
			}

			// For local mode, store state by default in the user's home directory.
			if localMode {
				if cloudURL != "" {
					return errors.New("a URL may not be specified when --local mode is enabled")
				}
				cloudURL = filestate.FilePathPrefix + "~"
			}

			// If we're on Windows, and this is a local login path, then allow the user to provide
			// backslashes as path separators.  We will normalize them here to forward slashes as that's
			// what the gocloud blob system requires.
			if strings.HasPrefix(cloudURL, filestate.FilePathPrefix) && os.PathSeparator != '/' {
				cloudURL = filepath.ToSlash(cloudURL)
			}

			if cloudURL == "" {
				var err error
				cloudURL, err = workspace.GetCurrentCloudURL()
				if err != nil {
					return fmt.Errorf("could not determine current cloud: %w", err)
				}
			} else if url := strings.TrimPrefix(strings.TrimPrefix(
				cloudURL, "https://"), "http://"); strings.HasPrefix(url, "app.pulumi.com/") ||
				strings.HasPrefix(url, "pulumi.com") {
				return fmt.Errorf("%s is not a valid self-hosted backend, "+
					"use `pulumi login` without arguments to log into the Pulumi service backend", cloudURL)
			} else {
				// Ensure we have the correct cloudurl type before logging in
				if err := validateCloudBackendType(cloudURL); err != nil {
					return err
				}
			}

			var be backend.Backend
			var err error
			if filestate.IsFileStateBackendURL(cloudURL) {
				be, err = filestate.Login(cmdutil.Diag(), cloudURL)
				if defaultOrg != "" {
					return fmt.Errorf("unable to set default org for this type of backend")
				}
			} else {
				be, err = httpstate.Login(commandContext(), cmdutil.Diag(), cloudURL, displayOptions)
				// if the user has specified a default org to associate with the backend
				if defaultOrg != "" {
					cloudURL, err := workspace.GetCurrentCloudURL()
					if err != nil {
						return err
					}
					if err := workspace.SetBackendConfigDefaultOrg(cloudURL, defaultOrg); err != nil {
						return err
					}
				}
			}
			if err != nil {
				return fmt.Errorf("problem logging in: %w", err)
			}

			if currentUser, _, err := be.CurrentUser(); err == nil {
				fmt.Printf("Logged in to %s as %s (%s)\n", be.Name(), currentUser, be.URL())
			} else {
				fmt.Printf("Logged in to %s (%s)\n", be.Name(), be.URL())
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log in to")
	cmd.PersistentFlags().StringVar(&defaultOrg, "default-org", "", "A default org to associate with the login. "+
		"Please note, currently, only the managed and self-hosted backends support organizations")
	cmd.PersistentFlags().BoolVarP(&localMode, "local", "l", false, "Use Pulumi in local-only mode")

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
