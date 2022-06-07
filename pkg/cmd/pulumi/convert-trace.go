// Copyright 2016-2021, Pulumi Corporation.
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
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
	"github.com/spf13/cobra"
	"sourcegraph.com/sourcegraph/appdash"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// each root is sampled at a particular granularity. the nested traces are converted into a callstack at each
// sampling point.

func traceRoots(querier appdash.Queryer) ([]*appdash.Trace, error) {
	traces, err := querier.Traces(appdash.TracesOpts{})
	if err != nil {
		return nil, err
	}

	var roots []*appdash.Trace
	for _, t := range traces {
		if t.ID.Parent == 0 {
			roots = append(roots, t)
		}
	}

	return roots, nil
}

type traceSample struct {
	when  time.Duration
	where []string
}

func (s *traceSample) key() string {
	return strings.Join(s.where, "<")
}

func findGRPCPayload(t *appdash.Trace) string {
	for _, a := range t.Annotations {
		if a.Key == "Msg" {
			var msg map[string]interface{}
			if err := json.Unmarshal(a.Value, &msg); err == nil {
				if req, ok := msg["gRPC request"]; ok {
					s, _ := req.(string)
					return s
				}
			}
		}
	}
	return ""
}

var urnFieldRe = regexp.MustCompile(`urn:"([^"]+)"`)
var tokFieldRe = regexp.MustCompile(`tok:"([^"]+)"`)

func extractResourceType(t *appdash.Trace) string {
	grpcPayload := findGRPCPayload(t)
	matches := urnFieldRe.FindStringSubmatch(grpcPayload)
	if len(matches) == 0 {
		return ""
	}
	urn := resource.URN(matches[1])
	if !urn.IsValid() {
		return ""
	}
	return string(urn.Type())
}

func extractFunctionToken(t *appdash.Trace) string {
	grpcPayload := findGRPCPayload(t)
	matches := tokFieldRe.FindStringSubmatch(grpcPayload)
	if len(matches) == 0 {
		return ""
	}
	return matches[1]
}

func getDecorator(t *appdash.Trace) string {
	for _, a := range t.Annotations {
		if a.Key == "pulumi-decorator" && len(a.Value) != 0 {
			return string(a.Value)
		}
	}

	switch t.Name() {
	case "/pulumirpc.ResourceMonitor/Invoke", "/pulumirpc.ResourceProvider/Invoke":
		return extractFunctionToken(t)
	case "/pulumirpc.ResourceMonitor/ReadResource", "/pulumirpc.ResourceMonitor/RegisterResource",
		"/pulumirpc.ResourceProvider/Check", "/pulumirpc.ResourceProvider/CheckConfig",
		"/pulumirpc.ResourceProvider/Diff", "/pulumirpc.ResourceProvider/DiffConfig",
		"/pulumirpc.ResourceProvider/Create", "/pulumirpc.ResourceProvider/Update",
		"/pulumirpc.ResourceProvider/Delete":
		return extractResourceType(t)
	default:
		return ""
	}
}

func convertTrace(root *appdash.Trace, start time.Time, quantum time.Duration) ([]traceSample, error) {
	timespanEvent, err := root.TimespanEvent()
	if err != nil {
		return nil, err
	}

	// find the trace name
	name := root.Name()
	if name == "" {
		name = "span-" + root.ID.String()
	}
	if decorator := getDecorator(root); decorator != "" {
		name += "(" + decorator + ")"
	}

	// convert each subspan
	var samples []traceSample
	for _, sub := range root.Sub {
		subSamples, err := convertTrace(sub, start, quantum)
		if err != nil {
			return nil, err
		}
		samples = append(samples, subSamples...)
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].when < samples[j].when
	})

	// determine where to start sampling
	delta := timespanEvent.Start().Sub(start)
	if delta < 0 {
		delta = 0
	}
	when := delta / quantum * quantum
	if delta%quantum != 0 {
		when += quantum
	}

	// quantize the trace
	n := int(timespanEvent.End().Sub(start.Add(when)) / quantum)
	if n == 0 {
		n = 1
	}

	result := make([]traceSample, n)
	for i := 0; i < n; i, when = i+1, when+quantum {
		if len(samples) == 0 || when < samples[0].when {
			result[i] = traceSample{when: when, where: []string{name}}
			continue
		}

		s := samples[0]
		s.where = append(s.where, name)
		result[i] = s
		samples = samples[1:]
	}

	return result, nil
}

func newConvertTraceCmd() *cobra.Command {
	var quantum time.Duration
	var cmd = &cobra.Command{
		Use:   "convert-trace [trace-file]",
		Short: "Convert a trace from the Pulumi CLI to Google's pprof format",
		Long: "Convert a trace from the Pulumi CLI to Google's pprof format.\n" +
			"\n" +
			"This command is used to convert execution traces collected by a prior\n" +
			"invocation of the Pulumi CLI from their native format to Google's\n" +
			"pprof format. The converted trace is written to stdout, and can be\n" +
			"inspected using `go tool pprof`.",
		Args:   cmdutil.ExactArgs(1),
		Hidden: !hasDebugCommands(),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			store := appdash.NewMemoryStore()
			if err := readTrace(args[0], store); err != nil {
				return err
			}

			roots, err := traceRoots(store)
			if err != nil {
				return err
			}

			var start time.Time
			for _, t := range roots {
				timespanEvent, err := t.TimespanEvent()
				if err != nil {
					return err
				}
				if start.IsZero() || timespanEvent.Start().Before(start) {
					start = timespanEvent.Start()
				}
			}

			locations, locationTable := []*profile.Location{}, map[string]*profile.Location{}
			functions, functionTable := []*profile.Function{}, map[string]*profile.Function{}
			samples, sampleTable := []*profile.Sample{}, map[string]*profile.Sample{}
			for _, t := range roots {
				rootSamples, err := convertTrace(t, start, quantum)
				if err != nil {
					return err
				}
				for _, s := range rootSamples {
					k := s.key()

					if sample, ok := sampleTable[k]; ok {
						sample.Value[0]++
						sample.Value[1] += int64(quantum)
						continue
					}

					sampleLocations := make([]*profile.Location, len(s.where))
					for i, w := range s.where {
						if l, ok := locationTable[w]; ok {
							sampleLocations[i] = l
							continue
						}

						f, ok := functionTable[w]
						if !ok {
							f = &profile.Function{ID: uint64(len(functions)) + 1, Name: w, SystemName: w}
							functions, functionTable[w] = append(functions, f), f
						}

						l := &profile.Location{ID: uint64(len(locations)) + 1, Line: []profile.Line{{Function: f}}}
						locations, locationTable[w] = append(locations, l), l

						sampleLocations[i] = l
					}

					sample := &profile.Sample{
						Location: sampleLocations,
						Value:    []int64{1, int64(quantum)},
					}

					samples, sampleTable[k] = append(samples, sample), sample
				}
			}

			p := profile.Profile{
				SampleType: []*profile.ValueType{
					{
						Type: "samples",
						Unit: "count",
					},
					{
						Type: "time",
						Unit: "nanoseconds",
					},
				},
				Sample:   samples,
				Location: locations,
				Function: functions,
			}

			return p.Write(os.Stdout)
		}),
	}

	cmd.Flags().DurationVarP(&quantum, "granularity", "g", 500*time.Millisecond, "the sample granularity")

	return cmd
}
