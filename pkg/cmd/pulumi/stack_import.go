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
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackImportCmd() *cobra.Command {
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and import a deployment.
			s, err := requireStack(stackName, false, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}
			stackName := s.Ref().Name()

			// Read from stdin or a specified file
			reader := os.Stdin
			if file != "" {
				reader, err = os.Open(file)
				if err != nil {
					return errors.Wrap(err, "could not open file")
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
			snapshot, err := stack.DeserializeUntypedDeployment(&deployment, stack.DefaultSecretsProvider)
			if err != nil {
				return checkDeploymentVersionError(err, stackName.String())
			}
			var result error
			for _, res := range snapshot.Resources {
				if res.URN.Stack() != stackName {
					msg := fmt.Sprintf("resource '%s' is from a different stack (%s != %s)",
						res.URN, res.URN.Stack(), stackName)
					if force {
						// If --force was passed, just issue a warning and proceed anyway.
						// Note: we could associate this diagnostic with the resource URN
						// we have.  However, this sort of message seems to be better as
						// something associated with the stack as a whole.
						cmdutil.Diag().Warningf(diag.Message("" /*urn*/, msg))
					} else {
						// Otherwise, gather up an error so that we can quit before doing damage.
						result = multierror.Append(result, errors.New(msg))
					}
				}
			}
			// Validate the stack. If --force was passed, issue an error if validation fails. Otherwise, issue a warning.
			if err := snapshot.VerifyIntegrity(); err != nil {
				msg := fmt.Sprintf("state file contains errors: %v", err)
				if force {
					cmdutil.Diag().Warningf(diag.Message("", msg))
				} else {
					result = multierror.Append(result, errors.New(msg))
				}
			}
			if result != nil {
				return multierror.Append(result,
					errors.New("importing this file could be dangerous; rerun with --force to proceed anyway"))
			}

			// Explicitly clear-out any pending operations.
			if snapshot.PendingOperations != nil {
				for _, op := range snapshot.PendingOperations {
					msg := fmt.Sprintf(
						"removing pending operation '%s' on '%s' from snapshot", op.Type, op.Resource.URN)
					cmdutil.Diag().Warningf(diag.Message(op.Resource.URN, msg))
				}

				snapshot.PendingOperations = nil
			}
			sdp, err := stack.SerializeDeployment(snapshot, snapshot.SecretsManager, false /* showSecrets */)
			if err != nil {
				return errors.Wrap(err, "constructing deployment for upload")
			}

			bytes, err := json.Marshal(sdp)
			if err != nil {
				return err
			}

			dep := apitype.UntypedDeployment{
				Version:    apitype.DeploymentSchemaVersionCurrent,
				Deployment: bytes,
			}

			// Now perform the deployment.
			if err = s.ImportDeployment(commandContext(), &dep); err != nil {
				return errors.Wrap(err, "could not import deployment")
			}
			fmt.Printf("Import successful.\n")
			return nil
		}),
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
