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
	"fmt"
	"math"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/termutil"
)

// ShowQueryEvents displays query events on the CLI.
func ShowQueryEvents(op string, eventsC <-chan events.Event,
	done chan<- bool, opts Options) {

	prefix := fmt.Sprintf("%s%s...", termutil.EmojiOr("✨ ", "@ "), op)

	var spinner termutil.Spinner
	var ticker *time.Ticker

	if opts.IsInteractive {
		spinner, ticker = termutil.NewSpinnerAndTicker(prefix, nil, 8 /*timesPerSecond*/)
	} else {
		spinner = &nopSpinner{}
		ticker = time.NewTicker(math.MaxInt64)
	}

	defer func() {
		spinner.Reset()
		ticker.Stop()
		close(done)
	}()

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-eventsC:
			spinner.Reset()

			out := os.Stdout
			if event.Type == events.DiagEvent {
				payload := event.Payload().(events.DiagEventPayload)
				if payload.Severity == apitype.SeverityError || payload.Severity == apitype.SeverityWarning {
					out = os.Stderr
				}
			}

			msg := renderQueryEvent(event, opts)
			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}

			if event.Type == events.CancelEvent {
				return
			}
		}
	}
}

func renderQueryEvent(event events.Event, opts Options) string {
	switch event.Type {
	case events.CancelEvent:
		return ""

	case events.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload().(events.StdoutEventPayload), opts)

	// Includes stdout of the query process.
	case events.DiagEvent:
		return renderQueryDiagEvent(event.Payload().(events.DiagEventPayload), opts)

	case events.PreludeEvent, events.SummaryEvent, events.ResourceOperationFailed,
		events.ResourceOutputsEvent, events.ResourcePreEvent:

		contract.Failf("query mode does not support resource operations")
		return ""

	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderQueryDiagEvent(payload events.DiagEventPayload, opts Options) string {
	// Ignore debug messages unless we're in debug mode.
	if payload.Severity == apitype.SeverityDebug && !opts.Debug {
		return ""
	}

	// Ignore error messages reported through diag events -- these are reported as errors later.
	if payload.Severity == apitype.SeverityInfoerr {
		return ""
	}

	// For stdout messages, trim ONLY the last newline character.
	if payload.Severity == apitype.SeverityInfo {
		payload.Message = termutil.RemoveTrailingNewline(payload.Message)
	}

	return opts.Color.Colorize(payload.Prefix + payload.Message)
}
