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

package templatecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type templateListRenderFunc func(w io.Writer, templates []apitype.TemplateMetadata) error

type templateListArgs struct {
	name         string
	org          string
	search       string
	renderOutput templateListRenderFunc
}

type templateListCmd struct {
	registryFactory func(ctx context.Context) registry.Registry
}

// newTemplateListCmd builds the `pulumi template list` command. registryFactory
// produces the [registry.Registry] used to fetch templates; pass nil to use the
// default factory derived from the current backend and login state.
func newTemplateListCmd(
	registryFactory func(ctx context.Context) registry.Registry,
) *cobra.Command {
	if registryFactory == nil {
		registryFactory = defaultTemplateRegistryFactory
	}

	listCmd := &templateListCmd{registryFactory: registryFactory}
	var args templateListArgs
	output := outputflag.OutputFlag[templateListRenderFunc]{
		RenderForTerminal: renderTemplatesTable,
		RenderJSON:        renderTemplatesJSON,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List registry-backed templates",
		Long: "[EXPERIMENTAL] List templates from the Pulumi Cloud registry.\n" +
			"\n" +
			"No authentication is required; without a Pulumi Cloud session, only publicly\n" +
			"visible templates are returned.\n" +
			"\n" +
			"Results are paginated by the server; the command follows continuation tokens\n" +
			"internally so all matching templates are streamed in a single invocation.",
		Example: "  # List every visible registry template.\n" +
			"  pulumi template list\n\n" +
			"  # Filter by template name.\n" +
			"  pulumi template list --name aws-quickstart\n\n" +
			"  # Filter to templates owned by a specific organization.\n" +
			"  pulumi template list --org myorg\n\n" +
			"  # Free-text search across name, display name, description, metadata, and runtime.\n" +
			"  pulumi template list --search serverless\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi template list --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			args.renderOutput = output.Get()
			return listCmd.Run(cmd.Context(), cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.name, "name", "",
		"Filter to templates whose name matches the given value")
	cmd.Flags().StringVar(&args.org, "org", "",
		"Filter to templates owned by the given organization")
	cmd.Flags().StringVar(&args.search, "search", "",
		"Free-text search across name, display name, description, metadata values, and runtime")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

// Run executes the list operation. ctx and out are decoupled from cobra so the
// function is straightforward to drive from tests.
func (c *templateListCmd) Run(ctx context.Context, out io.Writer, args templateListArgs) error {
	reg := c.registryFactory(ctx)

	// Pagination is handled inside the registry iterator; we collect into a slice
	// so both renderers can take a stable, deterministic view.
	templates := []apitype.TemplateMetadata{}
	for tmpl, err := range reg.ListTemplates(ctx, registry.ListTemplatesOptions{
		Name:   args.name,
		Org:    args.org,
		Search: args.search,
	}) {
		if err != nil {
			return fmt.Errorf("listing templates: %w", err)
		}
		templates = append(templates, tmpl)
	}

	return args.renderOutput(out, templates)
}

func renderTemplatesTable(w io.Writer, templates []apitype.TemplateMetadata) error {
	if len(templates) == 0 {
		fmt.Fprintln(w, "No templates found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	// StyleLight upper-cases headers by default; mirror existing list commands
	// (e.g. `pulumi org search`) which preserve title case.
	t.Style().Format.Header = text.FormatDefault
	t.AppendHeader(table.Row{"Name", "Publisher", "Source", "Language", "Visibility", "Updated"})
	for _, tmpl := range templates {
		updated := ""
		if !tmpl.UpdatedAt.IsZero() {
			updated = tmpl.UpdatedAt.UTC().Format("2006-01-02")
		}
		t.AppendRow(table.Row{
			tmpl.Name,
			tmpl.Publisher,
			tmpl.Source,
			tmpl.Language,
			tmpl.Visibility,
			updated,
		})
	}
	t.Render()
	return nil
}

// templateJSONEntry wraps apitype.TemplateMetadata for JSON output, shadowing
// the embedded UpdatedAt with a pointer so the zero time is omitted via
// `omitempty`. The Pulumi Cloud service emits "0001-01-01T00:00:00Z" for
// templates that don't have a real updated timestamp; surfacing that to
// scripting consumers is noise.
type templateJSONEntry struct {
	apitype.TemplateMetadata
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

func renderTemplatesJSON(w io.Writer, templates []apitype.TemplateMetadata) error {
	entries := make([]templateJSONEntry, 0, len(templates))
	for _, tmpl := range templates {
		entry := templateJSONEntry{TemplateMetadata: tmpl}
		if !tmpl.UpdatedAt.IsZero() {
			u := tmpl.UpdatedAt
			entry.UpdatedAt = &u
		}
		entries = append(entries, entry)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Templates []templateJSONEntry `json:"templates"`
	}{Templates: entries})
}

func defaultTemplateRegistryFactory(ctx context.Context) registry.Registry {
	return cmdCmd.NewDefaultRegistry(
		ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance,
		nil /* project */, cmdutil.Diag(), env.Global(),
	)
}
