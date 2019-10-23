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

package display

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// We use RFC 5424 timestamps with millisecond precision for displaying time stamps on log entries. Go does not
// pre-define a format string for this format, though it is similar to time.RFC3339Nano.
//
// See https://tools.ietf.org/html/rfc5424#section-6.2.3.
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

func ShowWatchEvents(op string, action apitype.UpdateKind, events <-chan engine.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()
	for e := range events {
		// In the event of cancelation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		// Events ocurring early:
		case engine.PreludeEvent, engine.SummaryEvent, engine.StdoutColorEvent:
			// Ignore it
			continue
		case engine.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload.(engine.DiagEventPayload)
			if p.Ephemeral || p.Severity == diag.Debug {
				continue
			}
			if p.URN != "" {
				s := renderDiffDiagEvent(p, opts)
				stdout := newPrefixer(os.Stdout, fmt.Sprintf("%30.30s[%30.30s] ", time.Now().Format(timeFormat), p.URN.Name()))
				fprintIgnoreError(stdout, s)
			}

		case engine.ResourcePreEvent:
			p := e.Payload.(engine.ResourcePreEventPayload)
			if shouldShow(p.Metadata, opts) {
				s := fmt.Sprintf("%s %s", p.Metadata.Op, p.Metadata.URN.Type())
				s = fmt.Sprintf("%30.30s[%30.30s] %v\n", time.Now().Format(timeFormat), p.Metadata.URN.Name(), s)
				fprintIgnoreError(os.Stdout, s)
			}
		case engine.ResourceOutputsEvent:
			p := e.Payload.(engine.ResourceOutputsEventPayload)
			if shouldShow(p.Metadata, opts) {
				s := fmt.Sprintf("done %s %s", p.Metadata.Op, p.Metadata.URN.Type())
				s = fmt.Sprintf("%30.30s[%30.30s] %v\n", time.Now().Format(timeFormat), p.Metadata.URN.Name(), s)
				fprintIgnoreError(os.Stdout, s)
			}
		case engine.ResourceOperationFailed:
			p := e.Payload.(engine.ResourceOperationFailedPayload)
			if shouldShow(p.Metadata, opts) {
				s := fmt.Sprintf("failed %s %s", p.Metadata.Op, p.Metadata.URN.Type())
				s = fmt.Sprintf("%30.30s[%30.30s] %v\n", time.Now().Format(timeFormat), p.Metadata.URN.Name(), s)
				fprintIgnoreError(os.Stdout, s)
			}
		default:
			contract.Failf("unknown event type '%s'", e.Type)
		}
	}
}

type prefixer struct {
	writer io.Writer
	prefix []byte
}

// newPrefixer wraps an io.Writer, prepending a fixed prefix after each \n emitting on the wrapped writer
func newPrefixer(writer io.Writer, prefix string) *prefixer {
	return &prefixer{writer, []byte(prefix)}
}

var _ io.Writer = (*prefixer)(nil)

func (prefixer *prefixer) Write(p []byte) (int, error) {
	n := 0
	lines := bytes.SplitAfter(p, []byte{'\n'})
	for _, line := range lines {
		if len(line) > 0 {
			_, err := prefixer.writer.Write(prefixer.prefix)
			if err != nil {
				return n, err
			}
		}
		m, err := prefixer.writer.Write(line)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
