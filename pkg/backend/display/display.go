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

package display

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/channel"
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

// printPermalinkNonInteractive prints an update's permalink prefaced with `View Live: `.
// This message is printed in non-interactive scenarios.
// In order to maintain backwards compatibility with older versions of the Automation API,
// the message is not changed for non-interactive scenarios.
func printPermalinkNonInteractive(out io.Writer, opts Options, permalink string) {
	printPermalink(out, opts, "View Live", permalink)
}

// printPermalinkInteractive prints an update's permalink prefaced with `View in Browser (Ctrl+O): `.
// This is printed in interactive scenarios that use the tree renderer.
func printPermalinkInteractive(term terminal.Terminal, opts Options, permalink string) {
	printPermalink(term, opts, "View in Browser (Ctrl+O)", permalink)
}

func printPermalink(out io.Writer, opts Options, message, permalink string) {
	if !opts.SuppressPermalink && permalink != "" {
		// Print a URL at the beginning of the update pointing to the Pulumi Service.
		headline := colors.SpecHeadline + message + ": " + colors.Underline + colors.BrightBlue + permalink +
			colors.Reset + "\n\n"
		fmt.Fprint(out, opts.Color.Colorize(headline))
	}
}

// stampEvents stamps each event with a sequence number and a timestamp, this is to ensure consistent
// timestamps between the events written to event log files and the events displayed in the terminal.
func stampEvents(events <-chan engine.Event) <-chan engine.StampedEvent {
	stampedEvents := make(chan engine.StampedEvent)
	go func() {
		var sequence int
		for e := range events {
			timestamp := int(time.Now().Unix())
			stampedEvents <- engine.StampedEvent{Event: e, Sequence: sequence, Timestamp: timestamp}
			sequence++
		}
		close(stampedEvents)
	}()
	return stampedEvents
}

// ShowEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func ShowEvents(
	op string, action apitype.UpdateKind, stack tokens.StackName, proj tokens.PackageName,
	permalink string, events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool,
) {
	// As we show events we need to stamp them with a sequence number and a timestamp, this is so we get
	// consistent timestamps written in both startEventLogger and ShowJSONEvents. If we feed these two
	// separate functions the raw `engine.Event` channel to do their own timestamping we can end up with
	// timestamp differences.
	stampedEvents := stampEvents(events)

	if opts.EventLogPath != "" {
		stampedEvents, done = startEventLogger(stampedEvents, done, opts)
	}

	// Need to filter the engine events here to exclude any internal events.
	stampedEvents = channel.FilterRead(stampedEvents, func(e engine.StampedEvent) bool {
		return !e.Event.Internal()
	})

	streamPreview := cmdutil.IsTruthy(os.Getenv("PULUMI_ENABLE_STREAMING_JSON_PREVIEW"))

	// If we're in non-preview JSON display mode we need to show the stamped events, for anything else we need
	// to show the raw events. We work out here if we're transforming the stamped events back to raw events so
	// that we can consistently use `rawEvents` in both `ShowPreviewDigest` and the non-json display modes. We
	// can't always create rawEvents because if we're in ShowJSONEvents we'll have two consumers competing for
	// the stamped events.
	var rawEvents chan engine.Event
	if !opts.JSONDisplay || (isPreview && !streamPreview) {
		rawEvents = make(chan engine.Event)
		go func() {
			for e := range stampedEvents {
				rawEvents <- e.Event
			}
			close(rawEvents)
		}()
	}

	if opts.JSONDisplay {
		if isPreview && !streamPreview {
			ShowPreviewDigest(rawEvents, done, opts)
		} else {
			ShowJSONEvents(stampedEvents, done, opts)
		}
		return
	}

	if opts.Type != DisplayProgress {
		printPermalinkNonInteractive(os.Stdout, opts, permalink)
	}

	switch opts.Type {
	case DisplayDiff:
		ShowDiffEvents(op, rawEvents, done, opts)
	case DisplayProgress:
		ShowProgressEvents(op, action, stack, proj, permalink, rawEvents, done, opts, isPreview)
	case DisplayQuery:
		contract.Failf("DisplayQuery can only be used in query mode, which should be invoked " +
			"directly instead of through ShowEvents")
	case DisplayWatch:
		ShowWatchEvents(op, rawEvents, done, opts)
	default:
		contract.Failf("Unknown display type %d", opts.Type)
	}
}

func logJSONEvent(encoder *json.Encoder, event engine.StampedEvent, opts Options) error {
	apiEvent, err := ConvertEngineEvent(event.Event, false /* showSecrets */)
	if err != nil {
		return err
	}

	apiEvent.Sequence = event.Sequence
	apiEvent.Timestamp = event.Timestamp
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

func startEventLogger(
	events <-chan engine.StampedEvent, done chan<- bool, opts Options,
) (<-chan engine.StampedEvent, chan<- bool) {
	// Before moving further, attempt to open the log file.
	//
	// Try setting O_APPEND to see if that helps with the malformed reads we've been seeing in automation api:
	// https://github.com/pulumi/pulumi/issues/6768
	logFile, err := os.OpenFile(opts.EventLogPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0o666)
	if err != nil {
		logging.V(7).Infof("could not create event log: %v", err)
		return events, done
	}

	outEvents, outDone := make(chan engine.StampedEvent), make(chan bool)
	go func() {
		defer close(done)
		defer func() {
			contract.IgnoreError(logFile.Close())
		}()

		encoder := json.NewEncoder(logFile)
		encoder.SetEscapeHTML(false)
		for e := range events {
			if err = logJSONEvent(encoder, e, opts); err != nil {
				logging.V(7).Infof("failed to log event: %v", err)
			}

			outEvents <- e

			if e.Type == engine.CancelEvent {
				break
			}
		}

		<-outDone
	}()

	return outEvents, outDone
}

type nopSpinner struct{}

func (s *nopSpinner) Tick() {
}

func (s *nopSpinner) Reset() {
}

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step engine.StepEventMetadata) bool {
	return isRootURN(step.URN)
}

func isRootURN(urn resource.URN) bool {
	return urn != "" && urn.QualifiedType() == resource.RootStackType
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

	// For non-logical replacement operations, only show them during progress-style updates (since this is integrated
	// into the resource status update), or if it is requested explicitly (for diffs and JSON outputs).
	if !opts.ShowReplacementSteps {
		if (opts.Type == DisplayDiff || opts.JSONDisplay) && !step.Logical && deploy.IsReplacementStep(step.Op) {
			return false
		}
	}

	// Otherwise, default to showing the operation.
	return true
}

func fprintIgnoreError(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	contract.IgnoreError(err)
}
