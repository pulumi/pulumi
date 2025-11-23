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

func NewShowCmd(ws workspace.Context, sn string) *cobra.Command {
	var stackName string
	var name string
	var pOpts printOptions
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show resources in the stack",
		Long: "Show resources in the stack" + "\n" +
			"This Command shows resources with their properties in the stack.\n" +
			"By default resources of the current stack will be shown, to view\n" +
			"in other stack --stack can be passed. Resources can be filtered by\n" +
			"their name using --name.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdOut := cmd.OutOrStdout()
			ctx := cmd.Context()
			snk := cmdutil.Diag()

			var stckName string
			if sn != "" {
				stckName = sn
			} else {
				stckName = stackName
			}
			s, err := stack.RequireStack(ctx, snk, ws, backend.DefaultLoginManager, stckName, stack.OfferNew, display.Options{})
			if err != nil {
				return err
			}

			ss, err := s.Snapshot(ctx, secrets.DefaultProvider)
			if err != nil {
				return err
			}

			resources := ss.Resources
			for _, res := range resources {
				if strings.Contains(res.URN.Name(), name) {
					cmdOut.Write([]byte(renderResourceState(res, pOpts)))
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

func renderResourceState(rs *resource.State, pOpts printOptions) string {
	var resStateString string

	resStateString += "\n"
	name := colors.Always.Colorize(colors.Bold+"Name: "+colors.Reset) + rs.URN.Name()
	resStateString += name + "\n"
	urn := colors.Always.Colorize(colors.Bold+"URN: "+colors.Reset) + string(rs.URN)
	resStateString += urn + "\n"
	properties := colors.Always.Colorize(colors.Bold + "Properties: " + colors.Reset)
	resStateString += properties

	resourcePropertiesString := ""
	if pOpts.keysOnly {
		for k := range rs.Outputs {
			if strings.HasPrefix(string(k), "__") {
				continue
			}
			resourcePropertiesString += " " + string(k) + ","
		}
		resourcePropertiesString = strings.TrimSuffix(resourcePropertiesString, ",")
		resourcePropertiesString += "\n"
	} else {
		resourcePropertiesString += "\n"
		for k, v := range rs.Outputs {
			if strings.HasPrefix(string(k), "__") {
				continue
			}
			resourcePropertiesString += "    " + string(k) + ": " + renderPropertyVal(v, "    ") + "\n"
		}
	}
	resStateString += resourcePropertiesString
	return resStateString
}

// render resource properties , properties can be nested Arrays or maps
// we recursively render property values.

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
		var res string
		for _, v := range rsp.ArrayValue() {
			newIdent := currIdent + "    "
			if v.IsObject() {
				return renderPropertyVal(v, newIdent)
			}
			if v.IsArray() {
				return renderPropertyVal(v, newIdent)
			}
			res += "\n" + newIdent + "- " + trimBrackets(v.String())
		}
		return res
	}

	return trimBrackets(rsp.String())
}

func trimBrackets(propertyVal string) string {
	var trimmedPropertyStr string
	trimmedPropertyStr = strings.TrimPrefix(propertyVal, "{")
	trimmedPropertyStr = strings.TrimSuffix(trimmedPropertyStr, "}")
	return trimmedPropertyStr
}
