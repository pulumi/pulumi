// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/guptarohit/asciigraph"
	"github.com/spf13/cobra"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newMetricsCmd() *cobra.Command {
	var stackID string
	var since string
	var resource string
	var jsonOut bool

	metricsCmd := &cobra.Command{
		Use:   "metrics",
		Short: "[PREVIEW] Show aggregated resource metrics for a stack",
		Long: "[PREVIEW] Show aggregated resource metrics for a stack\n" +
			"\n" +
			"This command aggregates metrics associated with the resources in a stack from the corresponding\n" +
			"provider. For example, for AWS resources, the `pulumi metrics` command will query\n" +
			"CloudWatch for metrics relevant to resources in a stack.\n",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stackID, false, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return fmt.Errorf("getting secrets manager: %w", err)
			}

			cfg, err := getStackConfiguration(s, sm)
			if err != nil {
				return fmt.Errorf("getting stack configuration: %w", err)
			}

			proj, root, err := readProject()
			if err != nil {
				return fmt.Errorf("reading project: %w", err)
			}

			untypedDeployment, err := s.ExportDeployment(context.Background())
			if err != nil {
				return fmt.Errorf("exporting deployment: %w", err)
			}

			snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment, stack.DefaultSecretsProvider)
			if err != nil {
				return fmt.Errorf("deserializing deployment: %w", err)
			}

			info := &logsInfo{
				root:    root,
				project: proj,
				target: &deploy.Target{
					Name:      s.Ref().Name(),
					Config:    cfg.Config,
					Decrypter: cfg.Decrypter,
					Snapshot:  snapshot,
				},
			}
			sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: opts.Color})
			providers, err := engine.LoadProviders(info, engine.ProvidersOptions{
				Diag:       sink,
				StatusDiag: sink,
			})
			if err != nil {
				return err
			}
			tree := operations.NewResourceTree(snapshot.Resources)
			ops := tree.OperationsProvider(providers)

			startTime, err := parseSince(since, time.Now())
			if err != nil {
				return fmt.Errorf("failed to parse argument to '--since' as duration or timestamp: %w", err)
			}

			resourceFilter := resource

			if !jsonOut {
				fmt.Printf(
					opts.Color.Colorize(colors.BrightMagenta+"Collecting metrics for stack %s since %s.\n\n"+colors.Reset),
					s.Ref().String(),
					startTime.Format(timeFormat),
				)
			}

			var entries []*metrics.ResourceMetrics
			var token interface{}
			for {
				batch, nextToken, err := ops.GetMetrics(operations.MetricsQuery{
					StartTime:         startTime,
					ResourceFilter:    resourceFilter,
					ContinuationToken: token,
				})
				if err != nil {
					return fmt.Errorf("failed to get metrics: %w", err)
				}
				entries = append(entries, batch...)
				if nextToken == nil {
					break
				}
				token = nextToken
			}

			for _, r := range entries {
				titled := false
				for _, sm := range r.ScopeMetrics {
					for _, m := range sm.Metrics {
						var points []*metrics.NumberDataPoint
						switch d := m.Data.(type) {
						case *metrics.Metric_Gauge:
							points = d.Gauge.DataPoints
						case *metrics.Metric_Sum:
							points = d.Sum.DataPoints
						}
						if len(points) == 0 {
							continue
						}

						if !titled {
							urn, id := operations.UnpackResource(r.Resource)

							fmt.Printf(
								opts.Color.Colorize(colors.BrightMagenta+"# %s %s (%s)\n\n"+colors.Reset),
								urn.Type(),
								urn.Name(),
								id,
							)

							titled = true
						}

						series := make([]float64, len(points))
						for i, p := range points {
							switch v := p.Value.(type) {
							case *metrics.NumberDataPoint_AsDouble:
								series[i] = v.AsDouble
							case *metrics.NumberDataPoint_AsInt:
								series[i] = float64(v.AsInt)
							}
						}
						plot := asciigraph.Plot(series, asciigraph.Caption(m.Name), asciigraph.Width(40), asciigraph.Height(4))
						fmt.Printf("%s\n\n", plot)
					}
				}
			}

			return nil
		}),
	}

	metricsCmd.PersistentFlags().StringVarP(
		&stackID, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	metricsCmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	metricsCmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	metricsCmd.PersistentFlags().StringVar(
		&since, "since", "1h",
		"Only return logs newer than a relative duration ('5s', '2m', '3h') or absolute timestamp.  "+
			"Defaults to returning the last 1 hour of logs.")
	metricsCmd.PersistentFlags().StringVarP(
		&resource, "resource", "r", "",
		"Only return logs for the requested resource ('name', 'type::name' or full URN).  Defaults to returning all logs.")

	return metricsCmd
}
