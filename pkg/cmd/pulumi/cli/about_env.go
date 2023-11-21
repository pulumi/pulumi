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

// A small library for creating consistent and documented environmental variable accesses.
//
// Public environmental variables should be declared as a module level variable.

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func newAboutEnvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "An overview of the environmental variables used by pulumi",
		Args:  cmdutil.NoArgs,
		// Since most variables won't be included here, we hide the command. We will
		// unhide once most existing variables are using the new env var framework and
		// show up here.
		Hidden: !env.Experimental.Value(),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			table := cmdutil.Table{
				Headers: []string{"Variable", "Description", "Value"},
			}
			var foundError bool
			for _, v := range declared.Variables() {
				foundError = foundError || emitEnvVarDiag(v)
				table.Rows = append(table.Rows, cmdutil.TableRow{
					Columns: []string{v.Name(), v.Description, v.Value.String()},
				})
			}
			printTable(table, nil)
			if foundError {
				return errors.New("invalid environmental variables found")
			}
			return nil
		}),
	}
}

func emitEnvVarDiag(val declared.Var) bool {
	err := val.Value.Validate()
	if err.Error != nil {
		cmdutil.Diag().Errorf(&diag.Diag{
			Message: fmt.Sprintf("%s: %v", val.Name(), err.Error),
		})
	}
	if err.Warning != nil {
		cmdutil.Diag().Warningf(&diag.Diag{
			Message: fmt.Sprintf("%s: %v", val.Name(), err.Warning),
		})
	}
	return err.Error != nil
}
