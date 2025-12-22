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

package show

import (
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type ShowCmdOpts struct {
	Lm backend.LoginManager
	Sp secrets.Provider
	Ws workspace.Context
}

func NewShowCmd(cmdOpts ShowCmdOpts) *cobra.Command {
	var stackName string
	var name string
	var pOpts printOptions

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show resources in the stack",
		Long: "Show resources in the stack" + "\n" +
			"This command shows resources and their properties in a stack.\n" +
			"By default resources of the current stack will be shown, to view\n" +
			"in other stacks `--stack` can be passed. Resources can be filtered by\n" +
			"their name using `--name`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdOut := cmd.OutOrStdout()
			ctx := cmd.Context()
			snk := cmdutil.Diag()
			lm := cmdOpts.Lm
			sp := cmdOpts.Sp
			ws := cmdOpts.Ws
			s, err := stack.RequireStack(ctx, snk, ws, lm, stackName, stack.OfferNew, display.Options{})
			if err != nil {
				return err
			}

			ss, err := s.Snapshot(ctx, sp)
			if err != nil {
				return err
			}

			resources := ss.Resources
			for _, res := range resources {
				if strings.Contains(res.URN.Name(), name) {
					printResourceState(res, pOpts, cmdOut)
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "the stack for which resources will be shown")
	cmd.PersistentFlags().StringVar(&name, "name", "", "filter resources by name")
	cmd.PersistentFlags().BoolVar(&pOpts.keysOnly, "keys-only", false, "only show property keys")

	return cmd
}

type printOptions struct {
	keysOnly bool
}

func printResourceState(rs *resource.State, popts printOptions, outputDest io.Writer) {
	name := colors.Always.Colorize(colors.Bold+"Name: "+colors.Reset) + rs.URN.Name()
	fmt.Fprintln(outputDest, name)

	urn := colors.Always.Colorize(colors.Bold+"URN: "+colors.Reset) + string(rs.URN)
	fmt.Fprintln(outputDest, urn)

	properties := colors.Always.Colorize(colors.Bold + "Properties: " + colors.Reset)
	fmt.Fprintln(outputDest, properties)
	if popts.keysOnly {
		propertiesStr := ""
		keys := resource.PropertyMap.StableKeys(rs.Outputs)
		for _, k := range keys {
			if strings.HasPrefix(string(k), "__") {
				continue
			}
			propertiesStr += string(k) + ", "
		}
		strings.TrimSuffix(propertiesStr, ", ")
		propertiesStr += "\n\n"
		fmt.Fprint(outputDest, propertiesStr)
		return
	}
	keys := resource.PropertyMap.StableKeys(rs.Outputs)
	for _, v := range keys {
		propKeyStr := string(v)

		if strings.HasPrefix(propKeyStr, "__") {
			continue
		}
		fmt.Fprintln(outputDest, propKeyStr+":")
		fmt.Fprintln(outputDest, display.PropertyValueToString(rs.Outputs[v], false, false))
	}
}
