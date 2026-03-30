// Copyright 2026, Pulumi Corporation.
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

package registry

import (
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/spf13/cobra"
)

type functionDetailJSON struct {
	Token              string         `json:"token"`
	Description        string         `json:"description,omitempty"`
	DeprecationMessage string         `json:"deprecationMessage,omitempty"`
	Inputs             []propertyJSON `json:"inputs,omitempty"`
	Outputs            []propertyJSON `json:"outputs,omitempty"`
}

func newRegistryFunctionGetCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string

	cmd := &cobra.Command{
		Use:   "get <type-token>",
		Short: "Get detailed info about a function",
		Long: `Get detailed information about a specific function (data source / invoke),
including its input parameters and output properties.

The type token should be in the format <package>:<module>:<function>,
for example: aws:ec2:getAmi.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			token := args[0]
			packageName, err := parsePackageFromToken(token)
			if err != nil {
				return err
			}

			var version *semver.Version
			if versionStr != "" {
				v, err := semver.Parse(versionStr)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", versionStr, err)
				}
				version = &v
			}

			spec, err := loadSchemaForPackage(ctx, reg, packageName, version)
			if err != nil {
				return err
			}

			resolvedToken, fn, err := findFunction(spec, token)
			if err != nil {
				return err
			}

			if jsonOut {
				return formatFunctionDetailJSON(spec, resolvedToken, fn)
			}
			return formatFunctionDetailConsole(spec, resolvedToken, fn)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "type-token", Usage: "<package>:<module>:<function>"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")

	return cmd
}

// findFunction looks up a function in the schema by token, trying exact match then simplified match.
func findFunction(spec *schema.PackageSpec, token string) (string, schema.FunctionSpec, error) {
	if fn, ok := spec.Functions[token]; ok {
		return token, fn, nil
	}

	for fullToken, fn := range spec.Functions {
		simplified, err := schemarender.SimplifyModuleName("function", fullToken)
		if err != nil {
			continue
		}
		if simplified == token {
			return fullToken, fn, nil
		}
	}

	return "", schema.FunctionSpec{}, fmt.Errorf("function %q not found", token)
}

func formatFunctionDetailJSON(spec *schema.PackageSpec, token string, fn schema.FunctionSpec) error {
	var inputs []propertyJSON
	if fn.Inputs != nil {
		var err error
		inputs, err = propertiesToJSON(spec, fn.Inputs.Properties, fn.Inputs.Required)
		if err != nil {
			return err
		}
	}

	var outputs []propertyJSON
	returnType := fn.ReturnType
	if returnType == nil && fn.Outputs != nil {
		returnType = &schema.ReturnTypeSpec{ObjectTypeSpec: fn.Outputs}
	}
	if returnType != nil && returnType.ObjectTypeSpec != nil {
		var err error
		outputs, err = propertiesToJSON(spec, returnType.ObjectTypeSpec.Properties, returnType.ObjectTypeSpec.Required)
		if err != nil {
			return err
		}
	}

	detail := functionDetailJSON{
		Token:              token,
		Description:        fn.Description,
		DeprecationMessage: fn.DeprecationMessage,
		Inputs:             inputs,
		Outputs:            outputs,
	}
	return ui.PrintJSON(detail)
}

func formatFunctionDetailConsole(
	spec *schema.PackageSpec, token string, fn schema.FunctionSpec, inline ...bool,
) error {
	var md strings.Builder

	fmt.Fprintf(&md, "# %s\n\n", token)

	if fn.Description != "" {
		desc := descriptionBeforeExamples(fn.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}

		code := firstExampleCode(fn.Description, "typescript")
		if code != "" {
			fmt.Fprintf(&md, "## Example Usage\n\n```typescript\n%s\n```\n\n", code)
		}
	}

	if fn.Inputs != nil && len(fn.Inputs.Properties) > 0 {
		fmt.Fprintf(&md, "## Inputs\n\n")
		fmt.Fprintf(&md, "| Name | Type | Required | Description |\n")
		fmt.Fprintf(&md, "|------|------|----------|-------------|\n")
		for _, name := range maputil.SortedKeys(fn.Inputs.Properties) {
			prop := fn.Inputs.Properties[name]
			typ, err := schemarender.GetType(spec, prop.TypeSpec)
			if err != nil {
				return err
			}
			requiredStr := ""
			if slices.Contains(fn.Inputs.Required, name) {
				requiredStr = "yes"
			}
			desc := truncateDesc(prop.Description, 50)
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, requiredStr, desc)
		}
		fmt.Fprintf(&md, "\n")
	}

	returnType := fn.ReturnType
	if returnType == nil && fn.Outputs != nil {
		returnType = &schema.ReturnTypeSpec{ObjectTypeSpec: fn.Outputs}
	}
	if returnType != nil {
		if returnType.ObjectTypeSpec != nil {
			obj := returnType.ObjectTypeSpec
			if len(obj.Properties) > 0 {
				fmt.Fprintf(&md, "## Outputs\n\n")
				fmt.Fprintf(&md, "| Name | Type | Always | Description |\n")
				fmt.Fprintf(&md, "|------|------|--------|-------------|\n")
				for _, name := range maputil.SortedKeys(obj.Properties) {
					prop := obj.Properties[name]
					typ, err := schemarender.GetType(spec, prop.TypeSpec)
					if err != nil {
						return err
					}
					alwaysStr := ""
					if slices.Contains(obj.Required, name) {
						alwaysStr = "yes"
					}
					desc := truncateDesc(prop.Description, 50)
					fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, alwaysStr, desc)
				}
				fmt.Fprintf(&md, "\n")
			}
		} else if returnType.TypeSpec != nil {
			typ, err := schemarender.GetType(spec, *returnType.TypeSpec)
			if err != nil {
				return err
			}
			fmt.Fprintf(&md, "## Outputs\n\n")
			fmt.Fprintf(&md, "Returns: `%s`\n\n", typ)
		}
	}

	if len(inline) > 0 && inline[0] {
		return ui.RenderMarkdownInline(md.String())
	}
	return ui.RenderMarkdown(md.String())
}
