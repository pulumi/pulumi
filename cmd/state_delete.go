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

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/edit"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var force bool // Force deletion of protected resources

func newStateDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a resource from a stack's state",
		Long: `Deletes a resource from a stack's state
		
This command deletes a resource from a stack's state, as long as it is safe to do so. Resources can't be deleted if
there exist other resources that depend on it or are parented to it. Protected resources will not be deleted unless
it is specifically requested using the --force flag.`,
		Args: cmdutil.ExactArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			urn := resource.URN(args[0])
			err := runStateEdit(urn, doDeletion)
			if err != nil {
				switch e := err.(type) {
				case edit.ResourceHasDependenciesError:
					message := "This resource can't be safely deleted because the following resources depend on it:\n"
					for _, dependentResource := range e.Dependencies {
						depUrn := dependentResource.URN
						message += fmt.Sprintf(" * %-15q (%s)\n", depUrn.Name(), depUrn)
					}

					message += "\nDelete those resources first before deleting this one."
					return errors.New(message)
				case edit.ResourceProtectedError:
					return errors.New(
						"This resource can't be safely deleted because it is protected. " +
							"Re-run this command with --force to force deletion")
				default:
					return err
				}
			}
			fmt.Println("Resource deleted successfully")
			return nil
		}),
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force deletion of protected resources")
	return cmd
}

// doDeletion implements edit.OperationFunc and deletes a resource from the snapshot. If the `force` flag is present,
// doDeletion will unprotect the resource before deleting it.
func doDeletion(snap *deploy.Snapshot, res *resource.State) error {
	if !force {
		return edit.DeleteResource(snap, res)
	}

	if res.Protect {
		cmdutil.Diag().Warningf(diag.RawMessage("" /*urn*/, "deleting protected resource due to presence of --force"))
		res.Protect = false
	}

	return edit.DeleteResource(snap, res)
}
