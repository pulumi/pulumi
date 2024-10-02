// Copyright 2016-2019, Pulumi Corporation.
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
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// We use RFC 5424 timestamps with millisecond precision for displaying time stamps on watch
// entries. Go does not pre-define a format string for this format, though it is similar to
// time.RFC3339Nano.
//
// See https://tools.ietf.org/html/rfc5424#section-6.2.3.
const timeFormat = "15:04:05.000"

// ShowWatchEvents renders incoming engine events for display in Watch Mode.
func ShowWatchEvents(op string, events <-chan engine.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()
	for e := range events {
		// In the event of cancelation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		case engine.CancelEvent:
			// Pacify linter.  This event is handled earlier
			continue
		// Events occurring early:
		case engine.PreludeEvent, engine.SummaryEvent, engine.StdoutColorEvent,
			engine.PolicyLoadEvent, engine.PolicyRemediationEvent:
			// Ignore it
			continue
		case engine.PolicyViolationEvent:
			// At this point in time, we don't handle policy events as part of pulumi watch
			continue
		case engine.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload().(engine.DiagEventPayload)
			resourceName := ""
			if p.URN != "" {
				resourceName = p.URN.Name()
			}
			WatchPrefixPrintf(time.Now(), resourceName,
				"%s", renderDiffDiagEvent(p, opts))
		case engine.StartDebuggingEvent:
			continue
		case engine.ResourcePreEvent:
			p := e.Payload().(engine.ResourcePreEventPayload)
			if shouldShow(p.Metadata, opts) {
				WatchPrefixPrintf(time.Now(), p.Metadata.URN.Name(),
					"%s %s\n", p.Metadata.Op, p.Metadata.URN.Type())
			}
		case engine.ResourceOutputsEvent:
			p := e.Payload().(engine.ResourceOutputsEventPayload)
			if shouldShow(p.Metadata, opts) {
				WatchPrefixPrintf(time.Now(), p.Metadata.URN.Name(),
					"done %s %s\n", p.Metadata.Op, p.Metadata.URN.Type())
			}
		case engine.ResourceOperationFailed:
			p := e.Payload().(engine.ResourceOperationFailedPayload)
			if shouldShow(p.Metadata, opts) {
				WatchPrefixPrintf(time.Now(), p.Metadata.URN.Name(),
					"failed %s %s\n", p.Metadata.Op, p.Metadata.URN.Type())
			}
		case engine.ProgressEvent:
			// Progress events are ephemeral and should be skipped.
			continue
		default:
			contract.Failf("unknown event type '%s'", e.Type)
		}
	}
}

// Watch output is written from multiple concurrent goroutines.  For now we synchronize Printfs to
// the watch output stream as a simple way to avoid garbled output.
var watchPrintfMutex sync.Mutex

// WatchPrefixPrintf wraps fmt.Printf with a watch mode prefixer that adds a timestamp and
// resource metadata.
func WatchPrefixPrintf(t time.Time, resourceName string, format string, a ...interface{}) {
	watchPrintfMutex.Lock()
	defer watchPrintfMutex.Unlock()
	prefix := fmt.Sprintf("%12.12s[%20.20s] ", t.Format(timeFormat), resourceName)
	out := &prefixer{os.Stdout, []byte(prefix)}
	_, err := fmt.Fprintf(out, format, a...)
	contract.IgnoreError(err)
}

type prefixer struct {
	writer io.Writer
	prefix []byte
}

var _ io.Writer = (*prefixer)(nil)

func (prefixer *prefixer) Write(p []byte) (int, error) {
	n := 0
	lines := bytes.SplitAfter(p, []byte{'\n'})
	for _, line := range lines {
		// If p ends with a newline, we may see an "" as the last element of lines, which we will skip.
		if len(line) == 0 {
			continue
		}
		_, err := prefixer.writer.Write(prefixer.prefix)
		if err != nil {
			return n, err
		}
		m, err := prefixer.writer.Write(line)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
