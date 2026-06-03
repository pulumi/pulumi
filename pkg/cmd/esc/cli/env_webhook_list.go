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

package cli

import (
	"errors"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvWebhookListCmd(env *envCommand) *cobra.Command {
	var count int
	var output string

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name>",
		Aliases: []string{"ls"},
		Short:   "List environment webhooks.",
		Long: "[EXPERIMENTAL] List environment webhooks\n" +
			"\n" +
			"This command lists the webhooks attached to the given environment.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the list command does not accept versions")
			}
			if count < 0 {
				return errors.New("--count must be non-negative")
			}

			hooks, err := env.esc.client.ListEnvironmentWebhooks(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			if count > 0 && len(hooks) > count {
				hooks = hooks[:count]
			}

			if format == outputJSON {
				out := struct {
					Webhooks []webhookSummaryJSON `json:"webhooks"`
				}{Webhooks: make([]webhookSummaryJSON, 0, len(hooks))}
				for _, h := range hooks {
					out.Webhooks = append(out.Webhooks, webhookSummaryJSON{
						Name:        h.Name,
						DisplayName: h.DisplayName,
						PayloadURL:  h.PayloadURL,
						Active:      h.Active,
						Format:      h.Format,
					})
				}
				return writeJSON(env.esc.stdout, out)
			}

			printWebhooks(env.esc.stdout, hooks)
			return nil
		},
	}

	cmd.Flags().IntVar(&count, "count", 0, "The maximum number of webhooks to return (all if unset)")
	addOutputFlag(cmd, &output)

	return cmd
}

// webhookSummaryJSON is the slim per-row projection emitted by `env webhook list`.
// Mirrors the columns shown by printWebhooks.
type webhookSummaryJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	PayloadURL  string `json:"payloadUrl"`
	Active      bool   `json:"active"`
	Format      string `json:"format,omitempty"`
}

func printWebhooks(stdout io.Writer, hooks []client.EnvironmentWebhook) {
	if len(hooks) == 0 {
		return
	}
	t := newTable(stdout)
	t.AppendHeader(table.Row{"NAME", "DISPLAY NAME", "URL", "ACTIVE", "FORMAT"})
	for _, h := range hooks {
		format := h.Format
		if format == "" {
			format = "-"
		}
		t.AppendRow(table.Row{h.Name, h.DisplayName, h.PayloadURL, h.Active, format})
	}
	t.Render()
}
