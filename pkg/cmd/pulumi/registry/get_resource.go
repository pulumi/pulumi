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

type resourceDetailJSON struct {
	Token              string         `json:"token"`
	Description        string         `json:"description,omitempty"`
	IsComponent        bool           `json:"isComponent,omitempty"`
	DeprecationMessage string         `json:"deprecationMessage,omitempty"`
	Inputs             []propertyJSON `json:"inputs,omitempty"`
	Outputs            []propertyJSON `json:"outputs,omitempty"`
}

type propertyJSON struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Description        string `json:"description,omitempty"`
	Required           bool   `json:"required,omitempty"`
	Default            any    `json:"default,omitempty"`
	Secret             bool   `json:"secret,omitempty"`
	ReplaceOnChanges   bool   `json:"replaceOnChanges,omitempty"`
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

func newRegistryResourceGetCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string

	cmd := &cobra.Command{
		Use:   "get <type-token>",
		Short: "Get detailed info about a resource type",
		Long: `Get detailed information about a specific resource type, including
its input properties, output properties, and descriptions.

The type token should be in the format <package>:<module>:<resource>,
for example: aws:ec2:Instance.`,
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

			resolvedToken, res, err := findResource(spec, token)
			if err != nil {
				return err
			}

			if jsonOut {
				return formatResourceDetailJSON(spec, resolvedToken, res)
			}
			return formatResourceDetailConsole(spec, resolvedToken, res)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "type-token", Usage: "<package>:<module>:<resource>"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")

	return cmd
}

// parsePackageFromToken extracts the package name from a type token like "aws:ec2:Instance".
func parsePackageFromToken(token string) (string, error) {
	parts := strings.Split(token, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid type token %q: expected format <package>:<module>:<name>", token)
	}
	return parts[0], nil
}

// findResource looks up a resource in the schema by token, trying exact match then simplified match.
func findResource(spec *schema.PackageSpec, token string) (string, schema.ResourceSpec, error) {
	// Try exact match first.
	if res, ok := spec.Resources[token]; ok {
		return token, res, nil
	}

	// Try matching by simplified token (handles "aws:ec2:Instance" matching "aws:ec2/instance:Instance").
	for fullToken, res := range spec.Resources {
		simplified, err := schemarender.SimplifyModuleName("resource", fullToken)
		if err != nil {
			continue
		}
		if simplified == token {
			return fullToken, res, nil
		}
	}

	return "", schema.ResourceSpec{}, fmt.Errorf("resource %q not found", token)
}

func formatResourceDetailJSON(spec *schema.PackageSpec, token string, res schema.ResourceSpec) error {
	inputs, err := propertiesToJSON(spec, res.InputProperties, res.RequiredInputs)
	if err != nil {
		return err
	}
	outputs, err := propertiesToJSON(spec, res.Properties, res.Required)
	if err != nil {
		return err
	}

	detail := resourceDetailJSON{
		Token:              token,
		Description:        res.Description,
		IsComponent:        res.IsComponent,
		DeprecationMessage: res.DeprecationMessage,
		Inputs:             inputs,
		Outputs:            outputs,
	}
	return ui.PrintJSON(detail)
}

func propertiesToJSON(
	spec *schema.PackageSpec,
	props map[string]schema.PropertySpec,
	required []string,
) ([]propertyJSON, error) {
	var result []propertyJSON
	for _, name := range maputil.SortedKeys(props) {
		prop := props[name]
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			return nil, err
		}
		result = append(result, propertyJSON{
			Name:               name,
			Type:               typ,
			Description:        schemarender.SummaryFromDescription(prop.Description),
			Required:           slices.Contains(required, name),
			Default:            prop.Default,
			Secret:             prop.Secret,
			ReplaceOnChanges:   prop.ReplaceOnChanges,
			DeprecationMessage: prop.DeprecationMessage,
		})
	}
	return result, nil
}

func formatResourceDetailConsole(
	spec *schema.PackageSpec, token string, res schema.ResourceSpec, inline ...bool,
) error {
	var md strings.Builder

	fmt.Fprintf(&md, "# %s\n\n", token)

	if res.IsComponent {
		fmt.Fprintf(&md, "**Component resource**\n\n")
	}

	if res.Description != "" {
		// Show only the prose description, not embedded code examples.
		desc := descriptionBeforeExamples(res.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}

		// Show first code example if available (matching the website's "Example Usage" section).
		code := firstExampleCode(res.Description, "typescript")
		if code != "" {
			fmt.Fprintf(&md, "## Example Usage\n\n```typescript\n%s\n```\n\n", code)
		}
	}

	if len(res.InputProperties) > 0 {
		fmt.Fprintf(&md, "## Inputs\n\n")
		fmt.Fprintf(&md, "| Name | Type | Required | Description |\n")
		fmt.Fprintf(&md, "|------|------|----------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.InputProperties) {
			prop := res.InputProperties[name]
			typ, err := schemarender.GetType(spec, prop.TypeSpec)
			if err != nil {
				return err
			}
			requiredStr := ""
			if slices.Contains(res.RequiredInputs, name) {
				requiredStr = "yes"
			}
			desc := truncateDesc(prop.Description, 50)
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, requiredStr, desc)
		}
		fmt.Fprintf(&md, "\n")
	}

	if len(res.Properties) > 0 {
		fmt.Fprintf(&md, "## Outputs\n\n")
		fmt.Fprintf(&md, "| Name | Type | Always | Description |\n")
		fmt.Fprintf(&md, "|------|------|--------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.Properties) {
			prop := res.Properties[name]
			typ, err := schemarender.GetType(spec, prop.TypeSpec)
			if err != nil {
				return err
			}
			alwaysStr := ""
			if slices.Contains(res.Required, name) {
				alwaysStr = "yes"
			}
			desc := truncateDesc(prop.Description, 50)
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, alwaysStr, desc)
		}
		fmt.Fprintf(&md, "\n")
	}

	if len(inline) > 0 && inline[0] {
		return ui.RenderMarkdownInline(md.String())
	}
	return ui.RenderMarkdown(md.String())
}

// escapePipes escapes pipe characters in a string for use in markdown table cells.
func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// truncateDesc truncates a description for use in table cells.
func truncateDesc(s string, maxLen int) string {
	s = escapePipes(schemarender.SummaryFromDescription(s))
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
