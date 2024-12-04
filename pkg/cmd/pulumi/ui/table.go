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

package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func PrintTable(table cmdutil.Table, opts *cmdutil.TableRenderOptions) {
	FprintTable(os.Stdout, table, opts)
}

func FprintTable(out io.Writer, table cmdutil.Table, opts *cmdutil.TableRenderOptions) {
	fmt.Fprint(out, renderTable(table, opts))
}

func renderTable(table cmdutil.Table, opts *cmdutil.TableRenderOptions) string {
	if opts == nil {
		opts = &cmdutil.TableRenderOptions{}
	}
	if len(opts.HeaderStyle) == 0 {
		style := make([]colors.Color, len(table.Headers))
		for i := range style {
			style[i] = colors.SpecHeadline
		}
		opts.HeaderStyle = style
	}
	if opts.Color == "" {
		opts.Color = cmdutil.GetGlobalColorization()
	}
	return table.Render(opts)
}
