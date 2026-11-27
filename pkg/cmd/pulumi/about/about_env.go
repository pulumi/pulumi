// Copyright 2016, Pulumi Corporation.
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

package about

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

// envVarAbout is the JSON representation of a declared environment variable.
type envVarAbout struct {
	Name        string `json:"name"`
	Alternative string `json:"alternative,omitempty"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

type aboutEnvRenderFunc func(w io.Writer, vars []declared.Var) error

func newAboutEnvCmd() *cobra.Command {
	output := outputflag.OutputFlag[aboutEnvRenderFunc]{
		RenderForTerminal: func(w io.Writer, vars []declared.Var) error {
			table := cmdutil.Table{
				Headers: []string{"Variable", "Description", "Value"},
			}
			for _, v := range vars {
				table.Rows = append(table.Rows, cmdutil.TableRow{
					Columns: []string{v.Name(), v.Description, v.Value.String()},
				})
			}
			ui.FprintTable(w, table, nil)
			return nil
		},
		RenderJSON: func(w io.Writer, vars []declared.Var) error {
			out := make([]envVarAbout, 0, len(vars))
			for _, v := range vars {
				out = append(out, envVarAbout{
					Name:        v.Name(),
					Alternative: v.Alternative(),
					Description: v.Description,
					Value:       v.Value.String(),
				})
			}
			return ui.FprintJSON(w, out)
		},
	}

	cmd := &cobra.Command{
		Use:   "env",
		Short: "An overview of the environmental variables used by pulumi",
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := declared.Variables()
			foundError := reportInvalidEnvVars(cmdutil.Diag(), vars)
			if err := output.Get()(cmd.OutOrStdout(), vars); err != nil {
				return err
			}
			if foundError {
				return errors.New("invalid environmental variables found")
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	// The parent `about` command declares a persistent --output/--json flag for its own summary.
	// Declaring local flags with the same names here shadows them, so `env` renders its own output.
	outputflag.VarWithJSONAlias(cmd, cmd.Flags(), &output)

	return cmd
}

// reportInvalidEnvVars emits a diagnostic for every variable with an invalid value,
// and reports whether any of them was an error.
func reportInvalidEnvVars(sink diag.Sink, vars []declared.Var) bool {
	var foundError bool
	for _, v := range vars {
		if emitEnvVarDiag(sink, v) {
			foundError = true
		}
	}
	return foundError
}

func emitEnvVarDiag(sink diag.Sink, val declared.Var) bool {
	err := val.Value.Validate()
	if err.Error != nil {
		sink.Errorf(&diag.Diag{
			Message: fmt.Sprintf("%s: %v", val.Name(), err.Error),
		})
	}
	if err.Warning != nil {
		sink.Warningf(&diag.Diag{
			Message: fmt.Sprintf("%s: %v", val.Name(), err.Warning),
		})
	}
	return err.Error != nil
}
