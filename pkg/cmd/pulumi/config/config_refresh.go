// Copyright 2024, Pulumi Corporation.
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

package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigRefreshCmd(stk *string) *cobra.Command {
	var force bool
	refreshCmd := &cobra.Command{
		Use:   "refresh",
		Short: "Update the local configuration based on the most recent deployment of the stack",
		Args:  cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Ensure the stack exists.
			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stk,
				cmdStack.LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}

			c, err := backend.GetLatestConfiguration(ctx, s)
			if err != nil {
				return err
			}

			configPath, err := cmdStack.GetProjectStackPath(s)
			if err != nil {
				return err
			}

			ps, err := workspace.LoadProjectStack(project, configPath)
			if err != nil {
				return err
			}

			ps.Config = c
			// Also restore the secrets provider from state
			untypedDeployment, err := s.ExportDeployment(ctx)
			if err != nil {
				return fmt.Errorf("getting deployment: %w", err)
			}
			deployment, err := stack.UnmarshalUntypedDeployment(ctx, untypedDeployment)
			if err != nil {
				return fmt.Errorf("unmarshaling deployment: %w", err)
			}
			if deployment.SecretsProviders != nil {
				// TODO: It would be really nice if the format of secrets state in the config file matched
				// what we kept in the statefile. That would go well with the pluginification of secret
				// providers as well, but for now just switch on the secret provider type and ask it to fill in
				// the config file for us.
				if deployment.SecretsProviders.Type == passphrase.Type {
					err = passphrase.EditProjectStack(ps, deployment.SecretsProviders.State)
				} else if deployment.SecretsProviders.Type == cloud.Type {
					err = cloud.EditProjectStack(ps, deployment.SecretsProviders.State)
				} else {
					// Anything else assume we can just clear all the secret bits
					ps.EncryptionSalt = ""
					ps.SecretsProvider = ""
					ps.EncryptedKey = ""
				}

				if err != nil {
					return err
				}
			}

			// If the configuration file doesn't exist, or force has been passed, save it in place.
			if _, err = os.Stat(configPath); os.IsNotExist(err) || force {
				return ps.Save(configPath)
			}

			// Otherwise we'll create a backup, let's figure out what name to use by adding ".bak" over and over
			// until we get to a name not in use.
			backupFile := configPath + ".bak"
			for {
				_, err = os.Stat(backupFile)
				if os.IsNotExist(err) {
					if err = os.Rename(configPath, backupFile); err != nil {
						return fmt.Errorf("backing up existing configuration file: %w", err)
					}

					fmt.Printf("backed up existing configuration file to %s\n", backupFile)
					break
				} else if err != nil {
					return fmt.Errorf("backing up existing configuration file: %w", err)
				}

				backupFile = backupFile + ".bak"
			}

			err = ps.Save(configPath)
			if err == nil {
				fmt.Printf("refreshed configuration for stack '%s'\n", s.Ref().Name())
			}
			return err
		}),
	}
	refreshCmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false, "Overwrite configuration file, if it exists, without creating a backup")

	return refreshCmd
}
