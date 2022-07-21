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

	"github.com/pulumi/pulumi/pkg/v3/util/type/event"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ShowQueryEvents displays query events on the CLI.
func ShowQueryEvents(op string, events <-chan event.Event,
	done chan<- bool, opts Options) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op)

	var spinner cmdutil.Spinner
	var ticker *time.Ticker

	if opts.IsInteractive {
		spinner, ticker = cmdutil.NewSpinnerAndTicker(prefix, nil, opts.Color, 8 /*timesPerSecond*/)
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
		case e := <-events:
			spinner.Reset()

			out := os.Stdout
			if e.Type == event.DiagEvent {
				payload := e.Payload().(event.DiagEventPayload)
				if payload.Severity == diag.Error || payload.Severity == diag.Warning {
					out = os.Stderr
				}
			}

			msg := renderQueryEvent(e, opts)
			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}

			if e.Type == event.CancelEvent {
				return
			}
		}
	}
}

func renderQueryEvent(e event.Event, opts Options) string {
	switch e.Type {
	case event.CancelEvent:
		return ""

	case event.StdoutColorEvent:
		return renderStdoutColorEvent(e.Payload().(event.StdoutEventPayload), opts)

	// Includes stdout of the query process.
	case event.DiagEvent:
		return renderQueryDiagEvent(e.Payload().(event.DiagEventPayload), opts)

	case event.PreludeEvent, event.SummaryEvent, event.ResourceOperationFailed,
		event.ResourceOutputsEvent, event.ResourcePreEvent:

		contract.Failf("query mode does not support resource operations")
		return ""

	default:
		contract.Failf("unknown event type '%s'", e.Type)
		return ""
	}
}

func renderQueryDiagEvent(payload event.DiagEventPayload, opts Options) string {
	// Ignore debug messages unless we're in debug mode.
	if payload.Severity == diag.Debug && !opts.Debug {
		return ""
	}

	// Ignore error messages reported through diag events -- these are reported as errors later.
	if payload.Severity == diag.Infoerr {
		return ""
	}

	// For stdout messages, trim ONLY the last newline character.
	if payload.Severity == diag.Info {
		payload.Message = cmdutil.RemoveTrailingNewline(payload.Message)
	}

	return opts.Color.Colorize(payload.Prefix + payload.Message)
}
