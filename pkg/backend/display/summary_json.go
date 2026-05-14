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

package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// SummaryJSON is a one-line JSON summary of a stack operation, intended for
// programmatic / LLM consumers when `--output json` is set.
//
// The shape is intentionally narrower than `apitype.SummaryEvent`: it only
// surfaces the fields that make sense as a final summary of the run (no
// `isPreview`, no `policyPacks`, no `maybeCorrupt`).
type SummaryJSON struct {
	// Result is the high-level outcome of the operation.
	Result apitype.OperationResult `json:"result"`
	// Duration is how long the operation took.
	Duration time.Duration `json:"duration"`
	// Summary is the count per operation kind (create, update, etc).
	Summary display.ResourceChanges `json:"summary,omitempty"`
}

// summaryJSONFromEvent extracts the summary JSON shape from a SummaryEventPayload.
func summaryJSONFromEvent(p engine.SummaryEventPayload) SummaryJSON {
	return SummaryJSON{
		Result:   p.Result,
		Duration: p.Duration,
		Summary:  p.ResourceChanges,
	}
}

// writeSummaryJSON encodes a SummaryJSON to w as a single line.
func writeSummaryJSON(w io.Writer, s SummaryJSON) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	// json.Encoder always appends a trailing newline; that's the line break we want.
	_, err := io.Copy(w, &buf)
	return err
}

// tapSummaryJSON returns a copy of the input channel that, in addition to
// forwarding every event, watches for a SummaryEvent and writes its summary
// to stdout as a single-line JSON object.
//
// The tap is only attached when Options.SummaryJSON is set; the rest of the
// display pipeline is otherwise unaffected.
func tapSummaryJSON(in <-chan engine.Event, opts Options) <-chan engine.Event {
	out := make(chan engine.Event)
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	go func() {
		defer close(out)
		for e := range in {
			if e.Type == engine.SummaryEvent {
				if payload, ok := e.Payload().(engine.SummaryEventPayload); ok {
					if err := writeSummaryJSON(stdout, summaryJSONFromEvent(payload)); err != nil {
						fmt.Fprintf(stderr, "warning: failed to write summary JSON: %v\n", err)
					}
				}
			}
			out <- e
			if e.Type == engine.CancelEvent {
				return
			}
		}
	}()
	return out
}
