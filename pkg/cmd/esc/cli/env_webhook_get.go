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
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvWebhookGetCmd(env *envCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Short: "Get an environment webhook.",
		Long: "[EXPERIMENTAL] Get an environment webhook\n" +
			"\n" +
			"This command prints the named webhook attached to the given environment.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the get command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			w, err := env.esc.client.GetEnvironmentWebhook(ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			if format == outputJSON {
				return writeJSON(env.esc.stdout, webhookJSON(*w))
			}

			printWebhook(env.esc.stdout, *w)
			return nil
		},
	}

	addOutputFlag(cmd, &output)

	return cmd
}

// printWebhook renders a single webhook as a key/value block.
func printWebhook(stdout io.Writer, w client.EnvironmentWebhook) {
	fmt.Fprintf(stdout, "Name: %s\n", w.Name)
	fmt.Fprintf(stdout, "Display name: %s\n", w.DisplayName)
	fmt.Fprintf(stdout, "URL: %s\n", w.PayloadURL)
	fmt.Fprintf(stdout, "Active: %t\n", w.Active)
	format := w.Format
	if format == "" {
		format = "-"
	}
	fmt.Fprintf(stdout, "Format: %s\n", format)
	events := "-"
	if len(w.Filters) > 0 {
		events = strings.Join(w.Filters, ", ")
	}
	fmt.Fprintf(stdout, "Events: %s\n", events)
	groups := "-"
	if len(w.Groups) > 0 {
		groups = strings.Join(w.Groups, ", ")
	}
	fmt.Fprintf(stdout, "Event groups: %s\n", groups)
	fmt.Fprintf(stdout, "Has secret: %t\n", w.HasSecret)
}

// webhookDetailJSON is the slim webhook projection emitted by `env webhook get`.
// Mirrors the fields shown by printWebhook; identity fields (organization /
// project / env / stack) and secret material are omitted on purpose — for the
// full API response use `pulumi api`.
type webhookDetailJSON struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	PayloadURL  string   `json:"payloadUrl"`
	Active      bool     `json:"active"`
	Format      string   `json:"format,omitempty"`
	Events      []string `json:"events,omitempty"`
	EventGroups []string `json:"eventGroups,omitempty"`
	HasSecret   bool     `json:"hasSecret"`
}

func webhookJSON(w client.EnvironmentWebhook) webhookDetailJSON {
	return webhookDetailJSON{
		Name:        w.Name,
		DisplayName: w.DisplayName,
		PayloadURL:  w.PayloadURL,
		Active:      w.Active,
		Format:      w.Format,
		Events:      w.Filters,
		EventGroups: w.Groups,
		HasSecret:   w.HasSecret,
	}
}
