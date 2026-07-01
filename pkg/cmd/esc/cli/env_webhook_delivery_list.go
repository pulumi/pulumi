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

package cli

import (
	"errors"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

func newEnvWebhookDeliveryListCmd(env *envCommand) *cobra.Command {
	var (
		utc    bool
		count  int
		output string
	)

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Aliases: []string{"ls"},
		Short:   "List environment webhook deliveries.",
		Long: "[EXPERIMENTAL] List environment webhook deliveries\n" +
			"\n" +
			"This command lists the deliveries recorded for the named webhook.\n",
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
				return errors.New("the list command does not accept versions")
			}
			if count < 0 {
				return errors.New("--count must be non-negative")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			deliveries, err := env.esc.client.ListEnvironmentWebhookDeliveries(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			if count > 0 && len(deliveries) > count {
				deliveries = deliveries[:count]
			}

			if format == outputJSON {
				out := struct {
					Deliveries []webhookDeliveryJSON `json:"deliveries"`
				}{Deliveries: make([]webhookDeliveryJSON, 0, len(deliveries))}
				for _, d := range deliveries {
					out.Deliveries = append(out.Deliveries, webhookDeliveryJSON{
						ID:           d.ID,
						Kind:         d.Kind,
						Timestamp:    utcFlag(utc).time(time.Unix(d.Timestamp, 0)).Format(time.RFC3339),
						ResponseCode: d.ResponseCode,
						Duration:     d.Duration,
					})
				}
				return writeJSON(env.esc.stdout, out)
			}

			printWebhookDeliveries(env.esc.stdout, deliveries, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "Display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "The maximum number of deliveries to return (all if unset)")
	addOutputFlag(cmd, &output)

	return cmd
}

// webhookDeliveryJSON is the slim per-delivery projection emitted by JSON
// output. Mirrors the columns shown by printWebhookDeliveries; the bulky
// payload / request-headers / response-headers / response-body fields are
// omitted (use `pulumi api` for the full record).
type webhookDeliveryJSON struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Timestamp    string `json:"timestamp"`
	ResponseCode int64  `json:"responseCode"`
	Duration     int64  `json:"duration"`
}

func printWebhookDeliveries(stdout io.Writer, ds []client.EnvironmentWebhookDelivery, utc utcFlag) {
	if len(ds) == 0 {
		return
	}
	t := newTable(stdout)
	t.AppendHeader(table.Row{"ID", "KIND", "TIMESTAMP", "RESPONSE", "DURATION (ms)"})
	for _, d := range ds {
		ts := time.Unix(d.Timestamp, 0)
		t.AppendRow(table.Row{
			d.ID,
			d.Kind,
			utc.time(ts).Format(time.RFC3339),
			d.ResponseCode,
			d.Duration,
		})
	}
	t.Render()
}
