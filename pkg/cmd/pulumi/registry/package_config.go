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

type configVarJSON struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Description        string `json:"description,omitempty"`
	Required           bool   `json:"required,omitempty"`
	Default            any    `json:"default,omitempty"`
	Secret             bool   `json:"secret,omitempty"`
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

func newRegistryPackageConfigCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string

	cmd := &cobra.Command{
		Use:   "config <package>",
		Short: "Show configuration variables for a package",
		Long: `Show the configuration variables accepted by a package's provider.

These are the settings you configure with 'pulumi config set <package>:<key> <value>'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			var version *semver.Version
			if versionStr != "" {
				v, err := semver.Parse(versionStr)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", versionStr, err)
				}
				version = &v
			}

			spec, err := loadSchemaForPackage(ctx, reg, args[0], version)
			if err != nil {
				return err
			}

			if spec.Config.Variables == nil || len(spec.Config.Variables) == 0 {
				fmt.Println("No configuration variables")
				return nil
			}

			if jsonOut {
				return formatConfigJSON(spec)
			}
			return formatConfigConsole(spec)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")

	return cmd
}

func formatConfigJSON(spec *schema.PackageSpec) error {
	var items []configVarJSON
	for _, name := range maputil.SortedKeys(spec.Config.Variables) {
		prop := spec.Config.Variables[name]
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			typ = "unknown"
		}
		items = append(items, configVarJSON{
			Name:               name,
			Type:               typ,
			Description:        schemarender.SummaryFromDescription(prop.Description),
			Required:           slices.Contains(spec.Config.Required, name),
			Default:            prop.Default,
			Secret:             prop.Secret,
			DeprecationMessage: schemarender.CleanDescription(prop.DeprecationMessage),
		})
	}
	return ui.PrintJSON(items)
}

func formatConfigConsole(spec *schema.PackageSpec) error {
	displayName := spec.DisplayName
	if displayName == "" {
		displayName = spec.Name
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# %s Provider Configuration\n\n", displayName)
	fmt.Fprintf(&md, "Set configuration with: `pulumi config set %s:<key> <value>`\n\n", spec.Name)

	for _, name := range maputil.SortedKeys(spec.Config.Variables) {
		prop := spec.Config.Variables[name]

		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			typ = "unknown"
		}

		// Name and type on one line, with badges.
		badges := ""
		if slices.Contains(spec.Config.Required, name) {
			badges += " **required**"
		}
		if prop.Secret {
			badges += " **secret**"
		}
		fmt.Fprintf(&md, "### `%s` (`%s`)%s\n\n", name, typ, badges)

		desc := schemarender.SummaryFromDescription(prop.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}
	}

	return ui.RenderMarkdown(md.String()) // uses pager so content persists until dismissed
}

// escapePipe replaces literal pipe characters with an escaped form so they
// do not break markdown table formatting.
func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
