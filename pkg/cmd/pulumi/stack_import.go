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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Fetch the current stack and import a deployment.
			s, err := requireStack(ctx, ws, DefaultLoginManager, stackName, stackLoadOnly, opts)
			if err != nil {
				return err
			}

			// Read from stdin or a specified file
			reader := os.Stdin
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
			snapshot, err := stack.DeserializeUntypedDeployment(ctx, &deployment, stack.DefaultSecretsProvider)
			if err != nil {
				return checkDeploymentVersionError(err, s.Ref().Name().String())
			}
			if err := saveSnapshot(ctx, s, snapshot, force); err != nil {
				return err
			}
			fmt.Printf("Import complete.\n")
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

func saveSnapshot(ctx context.Context, s backend.Stack, snapshot *deploy.Snapshot, force bool) error {
	stackName := s.Ref().Name()
	var result error
	for _, res := range snapshot.Resources {
		if res.URN.Stack() != stackName.Q() {
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
	if !backend.DisableIntegrityChecking {
		if err := snapshot.VerifyIntegrity(); err != nil {
			msg := fmt.Sprintf("state file contains errors: %v", err)
			if force {
				cmdutil.Diag().Warningf(diag.Message("", msg))
			} else {
				result = multierror.Append(result, errors.New(msg))
			}
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
	sdp, err := stack.SerializeDeployment(ctx, snapshot, false /* showSecrets */)
	if err != nil {
		return fmt.Errorf("constructing deployment for upload: %w", err)
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
	if err = s.ImportDeployment(ctx, &dep); err != nil {
		return fmt.Errorf("could not import deployment: %w", err)
	}
	return nil
}
