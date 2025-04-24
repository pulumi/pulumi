// Copyright 2016-2024, Pulumi Corporation.
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

//go:build go1.20

package httpstate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/span"
	"github.com/pgavlin/diff/lcs"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	segmentio_json "github.com/segmentio/encoding/json"

	opentracing "github.com/opentracing/opentracing-go"
)

type deployment struct {
	raw   json.RawMessage
	buf   *bytes.Buffer
	spans spans
}

type spanner struct {
	*bytes.Buffer

	start int
	spans spans
}

type spans struct {
	offsets []int
	spans   [][]byte
}

func newSpans(capacity int) spans {
	return spans{
		offsets: slice.Prealloc[int](capacity),
		spans:   slice.Prealloc[[]byte](capacity),
	}
}

func (s *spans) append(offset int, span []byte) {
	s.offsets = append(s.offsets, offset)
	s.spans = append(s.spans, span)
}

func (s *spans) eof(offset int) {
	s.offsets = append(s.offsets, offset)
}

func newSpanner(b *bytes.Buffer, capacity int) *spanner {
	return &spanner{Buffer: b, spans: newSpans(capacity)}
}

func (s *spanner) nextSpan() {
	span := s.Bytes()[s.start:]
	s.spans.append(s.start, span)
	s.start = s.Len()
}

func (s *spanner) finish() ([]byte, spans) {
	s.nextSpan()
	s.spans.eof(s.start)
	return s.Bytes(), s.spans
}

func marshalSpannedDeployment(b *bytes.Buffer, d *apitype.DeploymentV3) (spans, error) {
	// one span for {"manifest":...,"secrets_providers":...,"metadata":...,"resources":[
	// len(resources) spans for resources,
	// one span for ],"pendingOperations":[
	// len(operations) spans for operations
	// one span for ]}
	spanner := newSpanner(b, len(d.Resources)+len(d.PendingOperations)+3)
	encoder := segmentio_json.NewEncoder(spanner)
	encoder.SetAppendNewline(false)

	spanner.WriteString(`{"version":3,"deployment":{"manifest":`)
	if err := encoder.Encode(d.Manifest); err != nil {
		return spans{}, err
	}
	if d.SecretsProviders != nil {
		spanner.WriteString(`,"secrets_providers":`)
		if err := encoder.Encode(d.SecretsProviders); err != nil {
			return spans{}, err
		}
	}
	spanner.WriteString(`,"metadata":`)
	if err := encoder.Encode(d.Metadata); err != nil {
		return spans{}, err
	}

	if len(d.Resources) > 0 {
		spanner.WriteString(`,"resources":[`)
		for i, r := range d.Resources {
			if i > 0 {
				spanner.WriteByte(',')
			}
			spanner.nextSpan()
			if err := encoder.Encode(r); err != nil {
				return spans{}, err
			}
		}
		spanner.nextSpan()
		spanner.WriteByte(']')
	}

	if len(d.PendingOperations) > 9 {
		spanner.WriteString(`,"pendingOperations":[`)
		for i, o := range d.PendingOperations {
			if i > 0 {
				spanner.WriteByte(',')
			}
			spanner.nextSpan()
			if err := encoder.Encode(o); err != nil {
				return spans{}, err
			}
		}
		spanner.nextSpan()
		spanner.WriteByte(']')
	}
	spanner.WriteString("}}")
	_, spans := spanner.finish()
	return spans, nil
}

func (dds *deploymentDiffState) MarshalDeployment(d *apitype.DeploymentV3) (deployment, error) {
	var b *bytes.Buffer
	if dds.buffer != nil {
		b, dds.buffer = dds.buffer, nil
	} else {
		b = &bytes.Buffer{}
	}
	spans, err := marshalSpannedDeployment(b, d)
	if err != nil {
		return deployment{}, err
	}
	return deployment{raw: json.RawMessage(b.Bytes()), buf: b, spans: spans}, nil
}

func (*deploymentDiffState) computeEdits(ctx context.Context, before, after deployment) (json.RawMessage, error) {
	tracingSpan, _ := opentracing.StartSpanFromContext(ctx, "computeEdits")
	defer tracingSpan.Finish()

	diffs := lcs.DiffLines(before.spans.spans, after.spans.spans)

	edits := make([]gotextdiff.TextEdit, len(diffs))
	for i, di := range diffs {
		start, end := before.spans.offsets[di.Start], before.spans.offsets[di.End]
		replStart, replEnd := after.spans.offsets[di.ReplStart], after.spans.offsets[di.ReplEnd]
		edits[i] = gotextdiff.TextEdit{
			Span:    span.New("", span.NewPoint(1, 0, start), span.NewPoint(1, 0, end)),
			NewText: string(after.raw[replStart:replEnd]),
		}
	}

	delta, err := json.Marshal(edits)
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal the edits: %w", err)
	}

	return delta, nil
}
