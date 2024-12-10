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
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
	"github.com/pgavlin/fx"
	"github.com/pulumi/appdash"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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

func findGRPCPayloads(t *appdash.Trace) []string {
	return fx.ToSlice(fx.FMap(fx.IterSlice(t.Annotations), func(a appdash.Annotation) (string, bool) {
		if a.Key == "Msg" {
			var msg map[string]interface{}
			if err := json.Unmarshal(a.Value, &msg); err == nil {
				if req, ok := msg["gRPC request"]; ok {
					s, _ := req.(string)
					return s, true
				}
				if resp, ok := msg["gRPC response"]; ok {
					s, _ := resp.(string)
					return s, true
				}
			}
		}
		return "", false
	}))
}

var (
	urnFieldRe      = regexp.MustCompile(`urn:"([^"]+)"`)
	tokFieldRe      = regexp.MustCompile(`tok:"([^"]+)"`)
	depFieldRe      = regexp.MustCompile(`dependencies:"([^"]+)"`)
	parentFieldRe   = regexp.MustCompile(`parent:"([^"]+)"`)
	providerFieldRe = regexp.MustCompile(`provider:"([^"]+)"`)
)

func extractURN(t *appdash.Trace) resource.URN {
	urns := fx.ToSlice(fx.FMap(fx.IterSlice(findGRPCPayloads(t)), func(payload string) (resource.URN, bool) {
		matches := urnFieldRe.FindStringSubmatch(payload)
		if len(matches) == 0 {
			return "", false
		}
		urn := resource.URN(matches[1])
		if !urn.IsValid() {
			return "", false
		}
		return urn, true
	}))
	if len(urns) == 0 {
		return ""
	}
	return urns[0]
}

func extractResourceType(t *appdash.Trace) string {
	urn := extractURN(t)
	if !urn.IsValid() {
		return ""
	}
	return string(urn.Type())
}

func extractFunctionToken(t *appdash.Trace) string {
	tokens := fx.ToSlice(fx.FMap(fx.IterSlice(findGRPCPayloads(t)), func(payload string) (string, bool) {
		matches := tokFieldRe.FindStringSubmatch(payload)
		if len(matches) == 0 {
			return "", false
		}
		return matches[1], true
	}))
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

func extractDependencies(t *appdash.Trace) []string {
	deps := fx.ToSlice(fx.FMap(fx.IterSlice(findGRPCPayloads(t)), func(payload string) ([]string, bool) {
		matches := depFieldRe.FindAllStringSubmatch(payload, -1)
		if len(matches) == 0 {
			return nil, false
		}
		return fx.ToSlice(fx.Map(fx.IterSlice(matches), func(matches []string) string {
			return matches[1]
		})), true
	}))
	if len(deps) == 0 {
		return nil
	}
	return deps[0]
}

func extractParent(t *appdash.Trace) string {
	parents := fx.ToSlice(fx.FMap(fx.IterSlice(findGRPCPayloads(t)), func(payload string) (string, bool) {
		matches := parentFieldRe.FindStringSubmatch(payload)
		if len(matches) == 0 {
			return "", false
		}
		return matches[1], true
	}))
	if len(parents) == 0 {
		return ""
	}
	return parents[0]
}

func extractProvider(t *appdash.Trace) string {
	providers := fx.ToSlice(fx.FMap(fx.IterSlice(findGRPCPayloads(t)), func(payload string) (string, bool) {
		matches := providerFieldRe.FindStringSubmatch(payload)
		if len(matches) == 0 {
			return "", false
		}
		return matches[1], true
	}))
	if len(providers) == 0 {
		return ""
	}
	return providers[0]
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

func convertTraceToSamples(root *appdash.Trace, start time.Time, quantum time.Duration) ([]traceSample, error) {
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
		subSamples, err := convertTraceToSamples(sub, start, quantum)
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

	// We are multiplying a duration by a duration here, which can be buggy (e.g. consider that time.Second * time.Second
	// is *not* one second, and that in general multiplying durations is not well-defined). However, in this case it's
	// fine since we are computing the ratio of the delta to the quantum, so we shouldn't be changing scale accidentally.
	//
	//nolint:durationcheck
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

func convertTraceToPprof(quantum time.Duration, querier appdash.Queryer) error {
	roots, err := traceRoots(querier)
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
		rootSamples, err := convertTraceToSamples(t, start, quantum)
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
}

type otelSpan struct {
	sdktrace.ReadOnlySpan

	urn string

	criticalDependency string
	criticalPathLength *time.Duration

	name       string
	context    trace.SpanContext
	parent     trace.SpanContext
	start      time.Time
	end        time.Time
	links      []sdktrace.Link
	attributes []attribute.KeyValue
	resource   *sdkresource.Resource

	children []*otelSpan
}

// Name returns the name of the span.
func (s *otelSpan) Name() string {
	return s.name
}

// SpanContext returns the unique SpanContext that identifies the span.
func (s *otelSpan) SpanContext() trace.SpanContext {
	return s.context
}

// Parent returns the unique SpanContext that identifies the parent of the
// span if one exists. If the span has no parent the returned SpanContext
// will be invalid.
func (s *otelSpan) Parent() trace.SpanContext {
	return s.parent
}

// SpanKind returns the role the span plays in a Trace.
func (s *otelSpan) SpanKind() trace.SpanKind {
	return trace.SpanKindInternal
}

// StartTime returns the time the span started recording.
func (s *otelSpan) StartTime() time.Time {
	return s.start
}

// EndTime returns the time the span stopped recording. It will be zero if
// the span has not ended.
func (s *otelSpan) EndTime() time.Time {
	return s.end
}

// Attributes returns the defining attributes of the span.
// The order of the returned attributes is not guaranteed to be stable across invocations.
func (s *otelSpan) Attributes() []attribute.KeyValue {
	return s.attributes
}

// Links returns all the links the span has to other spans.
func (s *otelSpan) Links() []sdktrace.Link {
	return s.links
}

// Events returns all the events that occurred within in the spans
// lifetime.
func (s *otelSpan) Events() []sdktrace.Event {
	return nil
}

// Status returns the spans status.
func (s *otelSpan) Status() sdktrace.Status {
	return sdktrace.Status{Code: codes.Ok}
}

// InstrumentationLibrary returns information about the instrumentation
// library that created the span.
func (s *otelSpan) InstrumentationLibrary() instrumentation.Library {
	return s.InstrumentationScope()
}

// InstrumentationScope returns information about the instrumentation
// scope that created the span.
func (s *otelSpan) InstrumentationScope() instrumentation.Scope {
	return instrumentation.Scope{Name: "pulumi-convert"}
}

// Resource returns information about the entity that produced the span.
func (s *otelSpan) Resource() *sdkresource.Resource {
	return s.resource
}

// DroppedAttributes returns the number of attributes dropped by the span
// due to limits being reached.
func (s *otelSpan) DroppedAttributes() int {
	return 0
}

// DroppedLinks returns the number of links dropped by the span due to
// limits being reached.
func (s *otelSpan) DroppedLinks() int {
	return 0
}

// DroppedEvents returns the number of events dropped by the span due to
// limits being reached.
func (s *otelSpan) DroppedEvents() int {
	return 0
}

// ChildSpanCount returns the count of spans that consider the span a
// direct parent.
func (s *otelSpan) ChildSpanCount() int {
	return len(s.children)
}

type otelTrace struct {
	id       trace.TraceID
	resource *sdkresource.Resource
	spans    []sdktrace.ReadOnlySpan

	registerResourceSpans map[string]sdktrace.ReadOnlySpan

	ignoreLogSpans bool
}

func (t *otelTrace) criticalPath(span *otelSpan) (string, time.Duration) {
	if span.criticalPathLength != nil {
		return span.criticalDependency, *span.criticalPathLength
	}

	type edge struct {
		urn    string
		length time.Duration
	}
	edges := fx.ToSlice(fx.Map(fx.IterSlice(span.Links()), func(l sdktrace.Link) edge {
		spanID := l.SpanContext.SpanID()
		span := t.spans[binary.BigEndian.Uint64(spanID[:])-1].(*otelSpan)
		_, duration := t.criticalPath(span)
		return edge{urn: span.urn, length: duration + span.EndTime().Sub(span.StartTime())}
	}))
	var maximum edge
	for _, e := range edges {
		if e.length > maximum.length {
			maximum = e
		}
	}
	span.criticalDependency = maximum.urn
	span.criticalPathLength = &maximum.length
	return maximum.urn, maximum.length
}

// Extract the start and end times from a trace
//
// If the trace doesn't have a TimespanEvent, check the sub spans and infer the
// start and end times from them
func extractTimespan(root *appdash.Trace) (time.Time, time.Time, error) {
	timespanEvent, err := root.TimespanEvent()
	if err == nil {
		return timespanEvent.Start(), timespanEvent.End(), nil
	}

	var start, end time.Time
	// Try and extract timespan from sub spans
	for _, sub := range root.Sub {
		if subTimeSpan, err := sub.TimespanEvent(); err == nil {
			subStart := subTimeSpan.Start()
			subEnd := subTimeSpan.End()
			if start.IsZero() {
				start = subStart
				end = subEnd
			} else {
				if subStart.UnixNano() < start.UnixNano() {
					start = subStart
				}
				if subEnd.UnixNano() > end.UnixNano() {
					end = subEnd
				}
			}
		}
	}

	if start.IsZero() {
		return start, end, errors.New("time span event not found")
	}

	return start, end, nil
}

func (t *otelTrace) newSpan(root *appdash.Trace, parent *otelSpan) error {
	start, end, err := extractTimespan(root)
	if err != nil {
		return err
	}

	// find the trace name
	name := root.Name()
	if name == "" {
		name = "span-" + root.ID.String()
	}
	if decorator := getDecorator(root); decorator != "" {
		name += "(" + decorator + ")"
	}

	if name == "/pulumirpc.Engine/Log" && t.ignoreLogSpans {
		return nil
	}

	isRegisterResource := root.Name() == "/pulumirpc.ResourceMonitor/RegisterResource"

	urn := extractURN(root)
	parentURN := extractParent(root)
	provider := extractProvider(root)
	deps := extractDependencies(root)
	attributes := fx.ToSlice(fx.Map(fx.IterSlice(root.Annotations), func(a appdash.Annotation) attribute.KeyValue {
		return attribute.String(a.Key, string(a.Value))
	}))
	if urn != "" {
		attributes = append(attributes, attribute.String("urn", string(urn)))
	}
	if parentURN != "" {
		attributes = append(attributes, attribute.String("parent", parentURN))
	}
	if provider != "" {
		attributes = append(attributes, attribute.String("provider", provider))
	}
	attributes = append(attributes, attribute.StringSlice("dependencies", deps))

	links := fx.ToSlice(fx.FMap(fx.IterSlice(deps), func(dep string) (sdktrace.Link, bool) {
		reg, ok := t.registerResourceSpans[dep]
		if !ok {
			return sdktrace.Link{}, false
		}
		return sdktrace.Link{
			SpanContext: reg.SpanContext(),
			Attributes:  []attribute.KeyValue{attribute.String("pulumi", "dependency")},
		}, true
	}))
	if reg, ok := t.registerResourceSpans[parentURN]; ok {
		links = append(links, sdktrace.Link{
			SpanContext: reg.SpanContext(),
			Attributes:  []attribute.KeyValue{attribute.String("pulumi", "parent")},
		})
	}
	if ref, err := providers.ParseReference(provider); err == nil {
		if reg, ok := t.registerResourceSpans[string(ref.URN())]; ok {
			links = append(links, sdktrace.Link{
				SpanContext: reg.SpanContext(),
				Attributes:  []attribute.KeyValue{attribute.String("pulumi", "provider")},
			})
		}
	}

	// create the parent span
	this := &otelSpan{
		urn:  string(urn),
		name: name,
		context: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: t.id,
			SpanID:  t.getNextSpanID(),
		}),
		start:      start,
		end:        end,
		resource:   t.resource,
		attributes: attributes,
		links:      links,
	}
	if parent != nil {
		this.parent = parent.context
		parent.children = append(parent.children, this)
	}
	if isRegisterResource && urn != "" {
		t.registerResourceSpans[string(urn)] = this
	}
	t.spans = append(t.spans, this)

	// do a little bit of analysis on the path
	criticalDependency, criticalPathLength := t.criticalPath(this)
	if criticalPathLength != 0 {
		this.attributes = append(this.attributes, attribute.String("criticalDependency", criticalDependency))
		this.attributes = append(this.attributes, attribute.String("criticalPathLength", criticalPathLength.String()))
	}

	// convert each subspan
	for _, sub := range root.Sub {
		if err = t.newSpan(sub, this); err != nil {
			return err
		}
	}

	return nil
}

func (t *otelTrace) getNextSpanID() trace.SpanID {
	var id trace.SpanID
	binary.BigEndian.PutUint64(id[:], uint64(len(t.spans))+1)
	return id
}

func exportTraceToOtel(querier appdash.Queryer, ignoreLogSpans bool) error {
	fmt.Printf("converting trace...\n")

	roots, err := traceRoots(querier)
	if err != nil {
		return err
	}

	// Generate a random ID for the trace.
	t := otelTrace{
		resource:              sdkresource.Default(),
		registerResourceSpans: map[string]sdktrace.ReadOnlySpan{},
		ignoreLogSpans:        ignoreLogSpans,
	}
	if _, err = rand.Read(t.id[:]); err != nil {
		return err
	}

	// Conver the trace roots.
	for _, r := range roots {
		if err = t.newSpan(r, nil); err != nil {
			return err
		}
	}

	// Export the results to the collector.
	fmt.Print("dialing collector...\n")
	exporter, err := otlptracegrpc.New(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("exporting spans for trace %v...\n", t.id)
	spans := t.spans
	for len(spans) > 0 {
		batchSize := 1
		if len(spans) < batchSize {
			batchSize = len(spans)
		}
		if err = exporter.ExportSpans(context.Background(), spans[:batchSize]); err != nil {
			return err
		}
		spans = spans[batchSize:]
	}

	fmt.Printf("shutting down...\n")
	return exporter.Shutdown(context.Background())
}

func newConvertTraceCmd() *cobra.Command {
	var otel bool
	var ignoreLogSpans bool
	var quantum time.Duration
	cmd := &cobra.Command{
		Use:   "convert-trace <trace-file>",
		Short: "Convert a trace from the Pulumi CLI to Google's pprof format",
		Long: "Convert a trace from the Pulumi CLI to Google's pprof format.\n" +
			"\n" +
			"This command is used to convert execution traces collected by a prior\n" +
			"invocation of the Pulumi CLI from their native format to Google's\n" +
			"pprof format. The converted trace is written to stdout, and can be\n" +
			"inspected using `go tool pprof`.",
		Args:   cmdutil.ExactArgs(1),
		Hidden: !env.DebugCommands.Value(),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			store := appdash.NewMemoryStore()
			if err := readTrace(args[0], store); err != nil {
				return err
			}
			if otel {
				return exportTraceToOtel(store, ignoreLogSpans)
			}
			return convertTraceToPprof(quantum, store)
		}),
	}

	cmd.Flags().DurationVarP(&quantum, "granularity", "g", 500*time.Millisecond, "the sample granularity")
	cmd.Flags().BoolVar(&otel, "otel", false, "true to export to OpenTelemetry")
	cmd.Flags().BoolVar(&ignoreLogSpans, "ignore-log-spans", true, "true to ignore log spans")

	return cmd
}
