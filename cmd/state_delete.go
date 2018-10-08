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

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/edit"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

func newStateDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a resource from a stack's state",
		Long: `Deletes a resource from a stack's state
		
This command deletes a resource from a stack's state, as long as it is safe to do so. Resources can't be deleted if
there exist other resources that depend on it or are parented to it.`,
		Args: cmdutil.ExactArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			urn := resource.URN(args[0])
			err := runStateEdit(urn, edit.DeleteResource)
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
				default:
					return err
				}
			}
			fmt.Println("Resource deleted successfully")
			return nil
		}),
	}

	return cmd
}
