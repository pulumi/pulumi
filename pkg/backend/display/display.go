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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// ShowEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func ShowEvents(
	op string, action apitype.UpdateKind, stack tokens.QName, proj tokens.PackageName,
	events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool) {

	if opts.EventLogPath != "" {
		events, done = startEventLogger(events, done, opts)
	}

	streamPreview := cmdutil.IsTruthy(os.Getenv("PULUMI_ENABLE_STREAMING_JSON_PREVIEW"))

	if opts.JSONDisplay {
		if isPreview && !streamPreview {
			ShowPreviewDigest(events, done, opts)
		} else {
			ShowJSONEvents(events, done, opts)
		}
		return
	}

	switch opts.Type {
	case DisplayDiff:
		ShowDiffEvents(op, action, events, done, opts)
	case DisplayProgress:
		ShowProgressEvents(op, action, stack, proj, events, done, opts, isPreview)
	case DisplayQuery:
		contract.Failf("DisplayQuery can only be used in query mode, which should be invoked " +
			"directly instead of through ShowEvents")
	case DisplayWatch:
		ShowWatchEvents(op, action, events, done, opts)
	default:
		contract.Failf("Unknown display type %d", opts.Type)
	}
}

func logJSONEvent(encoder *json.Encoder, event engine.Event, opts Options, seq int) error {
	apiEvent, err := ConvertEngineEvent(event)
	if err != nil {
		return err
	}

	apiEvent.Sequence = seq
	apiEvent.Timestamp = int(time.Now().Unix())
	// If opts.Color == "never" (i.e. NO_COLOR is specified or --color=never), clean up the color directives
	// from the emitted events.
	if opts.Color == colors.Never {
		switch {
		case apiEvent.DiagnosticEvent != nil:
			apiEvent.DiagnosticEvent.Message = colors.Never.Colorize(apiEvent.DiagnosticEvent.Message)
			apiEvent.DiagnosticEvent.Prefix = colors.Never.Colorize(apiEvent.DiagnosticEvent.Prefix)
			apiEvent.DiagnosticEvent.Color = string(colors.Never)
		case apiEvent.StdoutEvent != nil:
			apiEvent.StdoutEvent.Message = colors.Never.Colorize(apiEvent.StdoutEvent.Message)
			apiEvent.StdoutEvent.Color = string(colors.Never)
		case apiEvent.PolicyEvent != nil:
			apiEvent.PolicyEvent.Message = colors.Never.Colorize(apiEvent.PolicyEvent.Message)
			apiEvent.PolicyEvent.Color = string(colors.Never)
		}
	}

	return encoder.Encode(apiEvent)
}

func startEventLogger(events <-chan engine.Event, done chan<- bool, opts Options) (<-chan engine.Event, chan<- bool) {
	// Before moving further, attempt to open the log file.
	logFile, err := os.Create(opts.EventLogPath)
	if err != nil {
		logging.V(7).Infof("could not create event log: %v", err)
		return events, done
	}

	outEvents, outDone := make(chan engine.Event), make(chan bool)
	go func() {
		defer close(done)
		defer func() {
			contract.IgnoreError(logFile.Close())
		}()

		sequence := 0
		encoder := json.NewEncoder(logFile)
		encoder.SetEscapeHTML(false)
		for e := range events {
			if err = logJSONEvent(encoder, e, opts, sequence); err != nil {
				logging.V(7).Infof("failed to log event: %v", err)
			}
			sequence++

			outEvents <- e

			if e.Type == engine.CancelEvent {
				break
			}
		}

		<-outDone
	}()

	return outEvents, outDone
}

type nopSpinner struct {
}

func (s *nopSpinner) Tick() {
}

func (s *nopSpinner) Reset() {
}

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step engine.StepEventMetadata) bool {
	return isRootURN(step.URN)
}

func isRootURN(urn resource.URN) bool {
	return urn != "" && urn.Type() == resource.RootStackType
}

// shouldShow returns true if a step should show in the output.
func shouldShow(step engine.StepEventMetadata, opts Options) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if step.Op == deploy.OpSame {
		// If the op is the same, it is possible that the resource's metadata changed.  In that case, still show it.
		if step.Old.Protect != step.New.Protect {
			return true
		}
		return opts.ShowSameResources
	}

	// For logical replacement operations, only show them during progress-style updates (since this is integrated
	// into the resource status update), or if it is requested explicitly (for diffs and JSON outputs).
	if (opts.Type == DisplayDiff || opts.JSONDisplay) && !step.Logical && !opts.ShowReplacementSteps {
		return false
	}

	// Otherwise, default to showing the operation.
	return true
}

func fprintfIgnoreError(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.IgnoreError(err)
}

func fprintIgnoreError(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	contract.IgnoreError(err)
}
