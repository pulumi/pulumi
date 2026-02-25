// Copyright 2025, Pulumi Corporation.
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

package templatecmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type templateListJSON struct {
	Name        string  `json:"name"`
	Publisher   string  `json:"publisher"`
	Source      string  `json:"source"`
	Language    string  `json:"language"`
	Visibility  string  `json:"visibility"`
	Description *string `json:"description,omitempty"`
	UpdatedAt   string  `json:"updatedAt"`
}

func newTemplateLsCmd() *cobra.Command {
	var jsonOut bool
	var publisher string
	var name string

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List published templates",
		Long: "List templates published to the Private Registry.\n" +
			"\n" +
			"This command lists all templates accessible to the current user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			ws := pkgWorkspace.Instance
			currentProject, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, currentProject, displayOpts)
			if err != nil {
				return err
			}

			registry, err := b.GetCloudRegistry()
			if err != nil {
				return fmt.Errorf("backend does not support Private Registry operations: %w", err)
			}

			var nameFilter *string
			if name != "" {
				nameFilter = &name
			}

			var templates []apitype.TemplateMetadata
			for tmpl, err := range registry.ListTemplates(ctx, nameFilter) {
				if err != nil {
					return fmt.Errorf("failed to list templates: %w", err)
				}

				if publisher != "" && tmpl.Publisher != publisher {
					continue
				}

				templates = append(templates, tmpl)
			}

			if len(templates) == 0 {
				if jsonOut {
					return ui.FprintJSON(cmd.OutOrStdout(), []templateListJSON{})
				}

				if publisher != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No templates found for publisher %q\n", publisher)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No templates found")
				}
				return nil
			}

			if jsonOut {
				return formatTemplatesJSON(cmd, templates)
			}
			return formatTemplatesConsole(cmd, templates)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.Flags().StringVarP(
		&publisher, "publisher", "p", "", "Filter templates by publisher")
	cmd.Flags().StringVarP(
		&name, "name", "n", "", "Filter templates by name")

	return cmd
}

func formatTemplatesJSON(cmd *cobra.Command, templates []apitype.TemplateMetadata) error {
	output := make([]templateListJSON, len(templates))
	for i, tmpl := range templates {
		output[i] = templateListJSON{
			Name:        tmpl.Name,
			Publisher:   tmpl.Publisher,
			Source:      tmpl.Source,
			Language:    tmpl.Language,
			Visibility:  tmpl.Visibility.String(),
			Description: tmpl.Description,
			UpdatedAt:   tmpl.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return ui.FprintJSON(cmd.OutOrStdout(), output)
}

func formatTemplatesConsole(cmd *cobra.Command, templates []apitype.TemplateMetadata) error {
	rows := make([]cmdutil.TableRow, len(templates))
	for i, tmpl := range templates {
		shortName := tmpl.Name
		if idx := strings.LastIndex(tmpl.Name, "/"); idx >= 0 {
			shortName = tmpl.Name[idx+1:]
		}

		rows[i] = cmdutil.TableRow{
			Columns: []string{shortName, tmpl.Publisher, tmpl.Language},
		}
	}

	ui.FprintTable(cmd.OutOrStdout(), cmdutil.Table{
		Headers: []string{"NAME", "PUBLISHER", "LANGUAGE"},
		Rows:    rows,
	}, nil)

	return nil
}
