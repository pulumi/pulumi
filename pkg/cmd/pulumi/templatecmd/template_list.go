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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/dustin/go-humanize"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type templateListArgs struct {
	jsonOut bool
}

type templateListCmd struct {
	stdout io.Writer
}

func newTemplateListCmd() *cobra.Command {
	var args templateListArgs

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List published templates in the Private Registry",
		Long: "List published templates in the Private Registry.\n\n" +
			"This command lists all templates that have been published to the Private Registry\n" +
			"for the current organization.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			tplListCmd := templateListCmd{stdout: cmd.OutOrStdout()}
			return tplListCmd.Run(ctx, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().BoolVarP(
		&args.jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

func (tplCmd *templateListCmd) Run(ctx context.Context, args templateListArgs) error {
	if tplCmd.stdout == nil {
		tplCmd.stdout = os.Stdout
	}

	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return fmt.Errorf("failed to determine current project: %w", err)
	}

	b, err := cmdBackend.CurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
		display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		return err
	}

	registry, err := b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("backend does not support Private Registry operations: %w", err)
	}

	templates, err := collectTemplates(ctx, registry)
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Publisher != templates[j].Publisher {
			return templates[i].Publisher < templates[j].Publisher
		}
		return templates[i].Name < templates[j].Name
	})

	if args.jsonOut {
		return formatTemplatesJSON(tplCmd.stdout, templates)
	}
	return formatTemplatesConsole(tplCmd.stdout, templates)
}

func collectTemplates(ctx context.Context, registry backend.CloudRegistry) ([]apitype.TemplateMetadata, error) {
	var templates []apitype.TemplateMetadata
	for tmpl, err := range registry.ListTemplates(ctx, nil) {
		if err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

type templateSummaryJSON struct {
	Name        string  `json:"name"`
	Publisher   string  `json:"publisher"`
	DisplayName string  `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	Language    string  `json:"language,omitempty"`
	Source      string  `json:"source"`
	Visibility  string  `json:"visibility"`
	LastUpdated string  `json:"lastUpdated"`
}

func formatTemplatesJSON(stdout io.Writer, templates []apitype.TemplateMetadata) error {
	output := make([]templateSummaryJSON, len(templates))
	for i, tmpl := range templates {
		output[i] = templateSummaryJSON{
			Name:        tmpl.Name,
			Publisher:   tmpl.Publisher,
			DisplayName: tmpl.DisplayName,
			Description: tmpl.Description,
			Language:    tmpl.Language,
			Source:      tmpl.Source,
			Visibility:  tmpl.Visibility.String(),
			LastUpdated: tmpl.UpdatedAt.UTC().Format("2006-01-02 15:04:05"),
		}
	}
	return ui.FprintJSON(stdout, output)
}

func formatTemplatesConsole(stdout io.Writer, templates []apitype.TemplateMetadata) error {
	if len(templates) == 0 {
		fmt.Fprintln(stdout, "No templates found.")
		return nil
	}

	headers := []string{"NAME", "PUBLISHER", "LANGUAGE", "VISIBILITY", "LAST UPDATED"}

	rows := make([]cmdutil.TableRow, len(templates))
	for i, tmpl := range templates {
		rows[i] = cmdutil.TableRow{
			Columns: []string{
				tmpl.Name,
				tmpl.Publisher,
				tmpl.Language,
				tmpl.Visibility.String(),
				humanize.Time(tmpl.UpdatedAt),
			},
		}
	}

	ui.FprintTable(stdout, cmdutil.Table{
		Headers: headers,
		Rows:    rows,
	}, nil)

	return nil
}
