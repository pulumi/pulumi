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

package config

import (
	"context"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type stackEnvironmentsRenderFunc func(w io.Writer, imports []string) error

func newConfigEnvListCmd(parent *configEnvCmd) *cobra.Command {
	output := outputflag.OutputFlag[stackEnvironmentsRenderFunc]{
		RenderForTerminal: formatStackEnvironmentsConsole,
		RenderJSON:        formatStackEnvironmentsJSON,
	}

	impl := configEnvLsCmd{parent: parent, output: &output}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lists imported environments.",
		Long:    "Lists the environments imported into a stack's configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	outputflag.VarWithJSONAlias(cmd, cmd.Flags(), &output)

	return cmd
}

type configEnvLsCmd struct {
	parent *configEnvCmd

	output *outputflag.OutputFlag[stackEnvironmentsRenderFunc]
}

func (cmd *configEnvLsCmd) run(ctx context.Context, _ []string) error {
	return cmd.parent.listStackEnvironments(ctx, cmd.output.Get())
}

func formatStackEnvironmentsConsole(w io.Writer, imports []string) error {
	if len(imports) == 0 {
		ui.Fprintf(w, "This stack configuration has no environments listed. "+
			"Try adding one with `pulumi config env add <projectName>/<envName>`.\n")
		return nil
	}

	rows := []cmdutil.TableRow{}
	for _, imp := range imports {
		rows = append(rows, cmdutil.TableRow{Columns: []string{imp}})
	}

	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"ENVIRONMENTS"},
		Rows:    rows,
	}, nil)
	return nil
}

func formatStackEnvironmentsJSON(w io.Writer, imports []string) error {
	if len(imports) == 0 {
		ui.Fprintf(w, "[]\n")
		return nil
	}
	return ui.FprintJSON(w, imports)
}
