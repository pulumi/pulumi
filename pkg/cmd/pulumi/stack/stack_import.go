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

package stack

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newStackImportCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager, sp secrets.Provider) *cobra.Command {
	var force bool
	var file string
	var stackName string
	cmd := &cobra.Command{
		Use:   "import",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Import a deployment from standard in into an existing stack",
		Long: "Import a deployment from standard in into an existing stack.\n" +
			"\n" +
			"A deployment that was exported from a stack using `pulumi stack export` and\n" +
			"hand-edited to correct inconsistencies due to failed updates, manual changes\n" +
			"to cloud resources, etc. can be reimported to the stack using this command.\n" +
			"The updated deployment will be read from standard in.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and import a deployment.
			s, err := RequireStack(
				ctx,
				cmdutil.Diag(),
				ws,
				lm,
				stackName,
				LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}

			// Read from stdin or a specified file
			reader := cmd.InOrStdin()
			if file != "" {
				reader, err = os.Open(file)
				if err != nil {
					return fmt.Errorf("could not open file: %w", err)
				}
			}

			// Read the checkpoint from stdin.  We decode this into a json.RawMessage so as not to lose any fields
			// sent by the server that the client CLI does not recognize (enabling round-tripping).
			var deployment apitype.UntypedDeployment
			if err = json.NewDecoder(reader).Decode(&deployment); err != nil {
				return err
			}

			// We do, however, now want to unmarshal the json.RawMessage into a real, typed deployment.  We do this so
			// we can check that the deployment doesn't contain resources from a stack other than the selected one. This
			// catches errors wherein someone imports the wrong stack's deployment (which can seriously hork things).
			snapshot, err := stack.DeserializeUntypedDeployment(ctx, &deployment, sp)
			if err != nil {
				return stack.FormatDeploymentDeserializationError(err, s.Ref().Name().String())
			}

			// If this snapshot is using the service secret manager but for a _different_ stack, we need to
			// reconfigure it for the target stack we're importing into.
			if snapshot.SecretsManager.Type() == service.Type {
				// Pass a dummy ProjectStack here since DefaultSecretManger will want to write to it to say no
				// encryption is in use, but for import purposes we don't care about that.
				ps := workspace.ProjectStack{}
				sm, err := s.DefaultSecretManager(&ps)
				if err != nil {
					return fmt.Errorf("could not create service secrets manager for stack %q: %w",
						s.Ref().String(), err)
				}
				contract.Assertf(
					ps.EncryptedKey == "" && ps.EncryptionSalt == "" && ps.SecretsProvider == "",
					"expected ProjectStack to remain unmodified")
				snapshot.SecretsManager = sm
			}

			if err := SaveSnapshot(ctx, s, snapshot, force); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Import complete.")
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Force the import to occur, even if apparent errors are discovered beforehand (not recommended)")
	cmd.PersistentFlags().StringVarP(
		&file, "file", "", "", "A filename to read stack input from")

	return cmd
}
