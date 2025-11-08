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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func ShowCmd() *cobra.Command {
	var stackName string
	var name string
	var keysOnly bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show resources in the stack",
		Long: "Show resources in the stack" + "\n" +
			"This Command shows resources with their properties in the stack.\n" +
			"By default resources of the current stack will be shown, to view\n" +
			"in other stack --stack can be passed. Resources can be filtered by\n" +
			"their name using --name.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := workspace.Instance
			ctx := cmd.Context()
			snk := cmdutil.Diag()

			s, err := stack.RequireStack(ctx, snk, ws, backend.DefaultLoginManager, stackName, stack.OfferNew, display.Options{})
			if err != nil {
				return err
			}

			ss, err := s.Snapshot(ctx, secrets.DefaultProvider)
			if err != nil {
				return err
			}
			printResourceState := func(rs *resource.State) {
				fmt.Println()

				name := colors.Always.Colorize(colors.Bold+"Name: "+colors.Reset) + rs.URN.Name()
				fmt.Println(name)
				urn := colors.Always.Colorize(colors.Bold+"URN: "+colors.Reset) + string(rs.URN)
				fmt.Println(urn)
				properties := colors.Always.Colorize(colors.Bold + "Properties: " + colors.Reset)
				fmt.Printf("%s", properties)

				resourcePropertiesString := ""
				if keysOnly {
					for k := range rs.Outputs {
						if strings.HasPrefix(string(k), "__") {
							continue
						}
						resourcePropertiesString += " " + string(k) + ","
					}
					resourcePropertiesString = strings.TrimSuffix(resourcePropertiesString, ",")
				} else {
					resourcePropertiesString += "\n"
					for k, v := range rs.Outputs {
						if strings.HasPrefix(string(k), "__") {
							continue
						}
						resourcePropertiesString += "    " + string(k) + ": " + renderPropertyVal(v, "    ") + "\n"
					}
				}
				fmt.Println(resourcePropertiesString)
				fmt.Println()
			}

			resources := ss.Resources
			for _, res := range resources {
				if strings.Contains(res.URN.Name(), name) {
					printResourceState(res)
				}
			}

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "the stack for which resources will be shown")
	cmd.PersistentFlags().StringVar(&name, "name", "", "filter resources by name")
	cmd.PersistentFlags().BoolVar(&keysOnly, "keys-only", false, "only show property keys")

	return cmd
}

// render resource properties , properties can be nested Arrays
func renderPropertyVal(rsp resource.PropertyValue, currIdent string) string {
	if rsp.IsObject() {
		newIdent := currIdent + "    "
		var res string
		objMap := rsp.ObjectValue()
		for k, v := range objMap {
			res += "\n" + newIdent + string(k) + ": " + renderPropertyVal(v, newIdent)
		}
		return res
	}
	if rsp.IsArray() {
		res := "\n" + currIdent
		for _, v := range rsp.ArrayValue() {
			newIdent := currIdent + "    "
			if v.IsObject() {
				return renderPropertyVal(v, newIdent)
			}
			if v.IsArray() {
				return renderPropertyVal(v, newIdent)
			}
			res += "- " + TrimBrackets(v.String())
		}
		return res
	}

	return TrimBrackets(rsp.String())
}

// remove brackets from property strings
func TrimBrackets(propertyVal string) string {
	var trimmedPropertyStr string
	trimmedPropertyStr = strings.TrimPrefix(propertyVal, "{")
	trimmedPropertyStr = strings.TrimSuffix(trimmedPropertyStr, "}")
	return trimmedPropertyStr
}
