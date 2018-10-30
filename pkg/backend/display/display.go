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
	"math"
	"os"
	"sort"
	"time"

	"github.com/dustin/go-humanize/english"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// ShowEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func ShowEvents(
	op string, action apitype.UpdateKind, stack tokens.QName, proj tokens.PackageName,
	events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool) {

	if opts.DiffDisplay {
		ShowDiffEvents(op, action, events, done, opts)
	} else {
		ShowProgressEvents(op, action, stack, proj, events, done, opts, isPreview)
	}
}

type nopSpinner struct {
}

func (s *nopSpinner) Tick() {
}

func (s *nopSpinner) Reset() {
}

// ShowDiffEvents displays the engine events with the diff view.
func ShowDiffEvents(op string, action apitype.UpdateKind,
	events <-chan engine.Event, done chan<- bool, opts Options) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op)

	var spinner cmdutil.Spinner
	var ticker *time.Ticker

	if opts.IsInteractive {
		spinner, ticker = cmdutil.NewSpinnerAndTicker(prefix, nil, 8 /*timesPerSecond*/)
	} else {
		spinner = &nopSpinner{}
		ticker = time.NewTicker(math.MaxInt64)
	}

	defer func() {
		spinner.Reset()
		ticker.Stop()
		done <- true
	}()

	seen := make(map[resource.URN]engine.StepEventMetadata)

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-events:
			spinner.Reset()

			out := os.Stdout
			if event.Type == engine.DiagEvent {
				payload := event.Payload.(engine.DiagEventPayload)
				if payload.Severity == diag.Error || payload.Severity == diag.Warning {
					out = os.Stderr
				}
			}

			msg := RenderDiffEvent(action, event, seen, opts)
			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}

			if event.Type == engine.CancelEvent {
				return
			}
		}
	}
}

func RenderDiffEvent(action apitype.UpdateKind, event engine.Event,
	seen map[resource.URN]engine.StepEventMetadata, opts Options) string {

	switch event.Type {
	case engine.CancelEvent:
		return ""

		// Currently, prelude, summar, and stdout events are printed the same for both the diff and
		// progress displays.
	case engine.PreludeEvent:
		return renderPreludeEvent(event.Payload.(engine.PreludeEventPayload), opts)
	case engine.SummaryEvent:
		return renderSummaryEvent(action, event.Payload.(engine.SummaryEventPayload), opts)
	case engine.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload.(engine.StdoutEventPayload), opts)

		// Resource operations have very specific displays for either diff or progress displays.
		// These functions should not be directly used by the progress display without validating
		// that the display is appropriate for both.
	case engine.ResourceOperationFailed:
		return renderDiffResourceOperationFailedEvent(event.Payload.(engine.ResourceOperationFailedPayload), opts)
	case engine.ResourceOutputsEvent:
		return renderDiffResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts)
	case engine.ResourcePreEvent:
		return renderDiffResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts)
	case engine.DiagEvent:
		return renderDiffDiagEvent(event.Payload.(engine.DiagEventPayload), opts)

	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderDiffDiagEvent(payload engine.DiagEventPayload, opts Options) string {
	if payload.Severity == diag.Debug && !opts.Debug {
		return ""
	}
	return opts.Color.Colorize(payload.Message)
}

func renderStdoutColorEvent(payload engine.StdoutEventPayload, opts Options) string {
	return opts.Color.Colorize(payload.Message)
}

func renderSummaryEvent(action apitype.UpdateKind, event engine.SummaryEventPayload, opts Options) string {
	changes := event.ResourceChanges

	changeCount := 0
	for op, c := range changes {
		if op != deploy.OpSame {
			changeCount += c
		}
	}

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%sResources:%s\n", colors.SpecHeadline, colors.Reset)))
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("    %s%d %s%s\n",
			colors.Bold, changeCount, english.PluralWord(changeCount, "change", ""), colors.Reset)))

	var planTo string
	if event.IsPreview {
		planTo = "to "
	}

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range deploy.StepOps {
		if op != deploy.OpSame {
			if c := changes[op]; c > 0 {
				opDescription := string(op)
				if !event.IsPreview {
					opDescription = op.PastTense()
				}
				fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("    %s%d %s%s%s\n",
					op.Prefix(), c, planTo, opDescription, colors.Reset)))
			}
		}
	}

	if c := changes[deploy.OpSame]; c > 0 {
		fprintfIgnoreError(out, "    %d unchanged\n", c)
	}

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("\n%sDuration: %s%s\n",
			colors.SpecHeadline, colors.Reset, event.Duration)))
	}

	return out.String()
}

func renderPreludeEvent(event engine.PreludeEventPayload, opts Options) string {
	// Only if we have been instructed to show configuration values will we print anything during the prelude.
	if !opts.ShowConfig {
		return ""
	}

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%sConfiguration:%s\n", colors.SpecUnimportant, colors.Reset)))

	var keys []string
	for key := range event.Config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fprintfIgnoreError(out, "    %v: %v\n", key, event.Config[key])
	}

	return out.String()
}

func renderDiffResourceOperationFailedEvent(
	payload engine.ResourceOperationFailedPayload, opts Options) string {

	// It's not actually useful or interesting to print out any details about
	// the resource state here, because we always assume that the resource state
	// is unknown if an error occurs.
	//
	// In the future, once we get more fine-grained error messages from providers,
	// we can provide useful diagnostics here.

	return ""
}

func renderDiffResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts Options) string {

	seen[payload.Metadata.URN] = payload.Metadata
	if payload.Metadata.Op == deploy.OpRefresh {
		return ""
	}

	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		indent := engine.GetIndent(payload.Metadata, seen)
		summary := engine.GetResourcePropertiesSummary(payload.Metadata, indent)
		details := engine.GetResourcePropertiesDetails(
			payload.Metadata, indent, payload.Planning, opts.SummaryDiff, payload.Debug)

		fprintIgnoreError(out, opts.Color.Colorize(summary))
		fprintIgnoreError(out, opts.Color.Colorize(details))
		fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
	}
	return out.String()
}

func renderDiffResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts Options) string {

	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		indent := engine.GetIndent(payload.Metadata, seen)

		refresh := false // are these outputs from a refresh?
		if m, has := seen[payload.Metadata.URN]; has && m.Op == deploy.OpRefresh {
			refresh = true
			summary := engine.GetResourcePropertiesSummary(payload.Metadata, indent)
			fprintIgnoreError(out, opts.Color.Colorize(summary))
		}

		if !opts.SuppressOutputs {
			if text := engine.GetResourceOutputsPropertiesString(
				payload.Metadata, indent+1, payload.Planning, payload.Debug, refresh); text != "" {
				fprintfIgnoreError(out, "%v%v--outputs:--%v\n",
					payload.Metadata.Op.Color(), engine.GetIndentationString(indent+1), colors.Reset)
				fprintIgnoreError(out, opts.Color.Colorize(text))
			}
		}
	}
	return out.String()
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
