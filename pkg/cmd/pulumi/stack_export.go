// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type StackExportConfig struct {
	PulumiConfig

	File        string
	Stack       string
	Version     string
	ShowSecrets bool
}

func newStackExportCmd() *cobra.Command {
	var config StackExportConfig

	cmd := &cobra.Command{
		Use:   "export",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Export a stack's deployment to standard out",
		Long: "Export a stack's deployment to standard out.\n" +
			"\n" +
			"The deployment can then be hand-edited and used to update the stack via\n" +
			"`pulumi stack import`. This process may be used to correct inconsistencies\n" +
			"in a stack's state due to failed deployments, manual changes to cloud\n" +
			"resources, etc.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and export its deployment
			s, err := requireStack(ctx, config.Stack, stackLoadOnly, opts)
			if err != nil {
				return err
			}

			var deployment *apitype.UntypedDeployment
			// Export the latest version of the checkpoint by default. Otherwise, we require that
			// the backend/stack implements the ability the export previous checkpoints.
			if config.Version == "" {
				deployment, err = s.ExportDeployment(ctx)
				if err != nil {
					return err
				}
			} else {
				// Check that the stack and its backend supports the ability to do this.
				be := s.Backend()
				specificExpBE, ok := be.(backend.SpecificDeploymentExporter)
				if !ok {
					return fmt.Errorf("the current backend (%s) does not provide the ability to export previous deployments",
						be.Name())
				}

				deployment, err = specificExpBE.ExportDeploymentForVersion(ctx, s, config.Version)
				if err != nil {
					return err
				}
			}

			// Read from stdin or a specified file.
			writer := os.Stdout
			if config.File != "" {
				writer, err = os.Create(config.File)
				if err != nil {
					return fmt.Errorf("could not open file: %w", err)
				}
			}

			if config.ShowSecrets {
				// log show secrets event
				snap, err := stack.DeserializeUntypedDeployment(ctx, deployment, stack.DefaultSecretsProvider)
				if err != nil {
					return checkDeploymentVersionError(err, config.Stack)
				}

				serializedDeployment, err := stack.SerializeDeployment(ctx, snap, true)
				if err != nil {
					return err
				}

				data, err := json.Marshal(serializedDeployment)
				if err != nil {
					return err
				}

				deployment = &apitype.UntypedDeployment{
					Version:    3,
					Deployment: data,
				}

				log3rdPartySecretsProviderDecryptionEvent(ctx, s, "", "pulumi stack export")
			}

			// Write the deployment.
			enc := json.NewEncoder(writer)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "    ")

			if err = enc.Encode(deployment); err != nil {
				return fmt.Errorf("could not export deployment: %w", err)
			}

			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&config.Stack, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVarP(
		&config.File, "file", "", "", "A filename to write stack output to")
	cmd.PersistentFlags().StringVarP(
		&config.Version, "version", "", "", "Previous stack version to export. (If unset, will export the latest.)")
	cmd.Flags().BoolVarP(
		&config.ShowSecrets, "show-secrets", "", false, "Emit secrets in plaintext in exported stack. Defaults to `false`")
	return cmd
}
