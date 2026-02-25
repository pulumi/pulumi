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

type templateInfoJSON struct {
	Name        string            `json:"name"`
	Publisher   string            `json:"publisher"`
	Source      string            `json:"source"`
	DisplayName string            `json:"displayName"`
	Language    string            `json:"language"`
	Visibility  string            `json:"visibility"`
	Description *string           `json:"description,omitempty"`
	RepoSlug    *string           `json:"repoSlug,omitempty"`
	UpdatedAt   string            `json:"updatedAt"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func newTemplateInfoCmd() *cobra.Command {
	var jsonOut bool
	var publisher string

	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show information about a published template",
		Long: "Show information about a template published to the Private Registry.\n" +
			"\n" +
			"The template can be specified by:\n" +
			"  - Full reference from 'pulumi template ls': publisher/name\n" +
			"  - Just the template name (may require --publisher for disambiguation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			templateArg := args[0]

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

			var matches []apitype.TemplateMetadata
			for tmpl, err := range registry.ListTemplates(ctx, nil) {
				if err != nil {
					return fmt.Errorf("failed to list templates: %w", err)
				}

				if publisher != "" && tmpl.Publisher != publisher {
					continue
				}

				publisherSlashName := tmpl.Publisher + "/" + tmpl.Name
				if tmpl.Name == templateArg ||
					publisherSlashName == templateArg ||
					strings.HasSuffix(tmpl.Name, "/"+templateArg) {
					matches = append(matches, tmpl)
				}
			}

			if len(matches) == 0 {
				if publisher != "" {
					return fmt.Errorf("template %q not found for publisher %q", templateArg, publisher)
				}
				return fmt.Errorf("template %q not found", templateArg)
			}

			if len(matches) > 1 {
				var names []string
				for _, m := range matches {
					names = append(names, fmt.Sprintf("  %s/%s", m.Publisher, m.Name))
				}
				return fmt.Errorf("template %q is ambiguous, matches:\n%s\n\nUse --publisher to disambiguate",
					templateArg, strings.Join(names, "\n"))
			}

			tmpl := matches[0]

			if jsonOut {
				return formatTemplateInfoJSON(cmd, tmpl)
			}
			return formatTemplateInfoConsole(cmd, tmpl)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name", Usage: "Template name or publisher/name reference"},
		},
		Required: 1,
	})

	cmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.Flags().StringVarP(
		&publisher, "publisher", "p", "", "Filter by publisher (for disambiguation)")

	return cmd
}

func formatTemplateInfoJSON(cmd *cobra.Command, tmpl apitype.TemplateMetadata) error {
	output := templateInfoJSON{
		Name:        tmpl.Name,
		Publisher:   tmpl.Publisher,
		Source:      tmpl.Source,
		DisplayName: tmpl.DisplayName,
		Language:    tmpl.Language,
		Visibility:  tmpl.Visibility.String(),
		Description: tmpl.Description,
		RepoSlug:    tmpl.RepoSlug,
		UpdatedAt:   tmpl.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Metadata:    tmpl.Metadata,
	}

	return ui.FprintJSON(cmd.OutOrStdout(), output)
}

func formatTemplateInfoConsole(cmd *cobra.Command, tmpl apitype.TemplateMetadata) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", tmpl.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Publisher: %s\n", tmpl.Publisher)
	if tmpl.DisplayName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Display Name: %s\n", tmpl.DisplayName)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Language: %s\n", tmpl.Language)
	if tmpl.Description != nil && *tmpl.Description != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", *tmpl.Description)
	}
	if tmpl.RepoSlug != nil && *tmpl.RepoSlug != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\n", *tmpl.RepoSlug)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", tmpl.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return nil
}
