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

	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/termutil"
)

// ShowDiffEvents displays the engine events with the diff view.
func ShowDiffEvents(op string, action apitype.UpdateKind,
	eventsC <-chan events.Event, done chan<- bool, opts Options) {

	prefix := fmt.Sprintf("%s%s...", termutil.EmojiOr("✨ ", "@ "), op)

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var spinner termutil.Spinner
	var ticker *time.Ticker
	if stdout == os.Stdout && stderr == os.Stderr && opts.IsInteractive {
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

	seen := make(map[resource.URN]events.StepEventMetadata)

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-eventsC:
			spinner.Reset()

			out := stdout
			if event.Type == events.DiagEvent {
				payload := event.Payload().(events.DiagEventPayload)
				if payload.Severity == apitype.SeverityError || payload.Severity == apitype.SeverityWarning {
					out = stderr
				}
			}

			msg := RenderDiffEvent(action, event, seen, opts)
			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}

			if event.Type == events.CancelEvent {
				return
			}
		}
	}
}

func RenderDiffEvent(action apitype.UpdateKind, event events.Event,
	seen map[resource.URN]events.StepEventMetadata, opts Options) string {

	switch event.Type {
	case events.CancelEvent:
		return ""

		// Currently, prelude, summary, and stdout events are printed the same for both the diff and
		// progress displays.
	case events.PreludeEvent:
		return renderPreludeEvent(event.Payload().(events.PreludeEventPayload), opts)
	case events.SummaryEvent:
		const wroteDiagnosticHeader = false
		return renderSummaryEvent(action, event.Payload().(events.SummaryEventPayload), wroteDiagnosticHeader, opts)
	case events.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload().(events.StdoutEventPayload), opts)

		// Resource operations have very specific displays for either diff or progress displays.
		// These functions should not be directly used by the progress display without validating
		// that the display is appropriate for both.
	case events.ResourceOperationFailed:
		return renderDiffResourceOperationFailedEvent(event.Payload().(events.ResourceOperationFailedPayload), opts)
	case events.ResourceOutputsEvent:
		return renderDiffResourceOutputsEvent(event.Payload().(events.ResourceOutputsEventPayload), seen, opts)
	case events.ResourcePreEvent:
		return renderDiffResourcePreEvent(event.Payload().(events.ResourcePreEventPayload), seen, opts)
	case events.DiagEvent:
		return renderDiffDiagEvent(event.Payload().(events.DiagEventPayload), opts)
	case events.PolicyViolationEvent:
		return renderDiffPolicyViolationEvent(event.Payload().(events.PolicyViolationEventPayload), opts)

	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderDiffDiagEvent(payload events.DiagEventPayload, opts Options) string {
	if payload.Severity == apitype.SeverityDebug && !opts.Debug {
		return ""
	}
	return opts.Color.Colorize(payload.Prefix + payload.Message)
}

func renderDiffPolicyViolationEvent(payload events.PolicyViolationEventPayload, opts Options) string {
	return opts.Color.Colorize(payload.Prefix + payload.Message)
}

func renderStdoutColorEvent(payload events.StdoutEventPayload, opts Options) string {
	return opts.Color.Colorize(payload.Message)
}

func renderSummaryEvent(action apitype.UpdateKind, event events.SummaryEventPayload,
	wroteDiagnosticHeader bool, opts Options) string {

	changes := event.ResourceChanges

	out := &bytes.Buffer{}

	// If this is a failed preview, we only render the Policy Packs that ran. This is because rendering the summary
	// for a failed preview may be surprising/misleading, as it does not describe the totality of the proposed changes
	// (as the preview may have aborted when the error occurred).
	if event.IsPreview && wroteDiagnosticHeader {
		renderPolicyPacks(out, event.PolicyPacks, opts)
		return out.String()
	}
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%sResources:%s\n", colors.SpecHeadline, colors.Reset)))

	var planTo string
	if event.IsPreview {
		planTo = "to "
	}

	var changeKindCount = 0
	var changeCount = 0
	var sameCount = changes[events.OpSame]

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range events.StepOps {
		// Ignore anything that didn't change, or is related to 'reads'.  'reads' are just an
		// indication of the operations we were performing, and are not indicative of any sort of
		// change to the system.
		if op != events.OpSame &&
			op != events.OpRead &&
			op != events.OpReadDiscard &&
			op != events.OpReadReplacement {

			if c := changes[op]; c > 0 {
				opDescription := string(op)
				if !event.IsPreview {
					opDescription = op.PastTense()
				}

				// Increment the change count by the number of changes associated with this step kind
				changeCount += c

				// Increment the number of kinds of changes by one
				changeKindCount++

				// Print a summary of the changes of this kind
				fprintIgnoreError(out, opts.Color.Colorize(
					fmt.Sprintf("    %s%d %s%s%s\n", op.Prefix(true /*done*/), c, planTo, opDescription, colors.Reset)))
			}
		}
	}

	summaryPieces := []string{}
	if changeKindCount >= 2 {
		// Only if we made multiple types of changes do we need to print out the total number of
		// changes.  i.e. we don't need "10 changes" and "+ 10 to create".  We can just say "+ 10 to create"
		summaryPieces = append(summaryPieces, fmt.Sprintf("%s%d %s%s",
			colors.Bold, changeCount, english.PluralWord(changeCount, "change", ""), colors.Reset))
	}

	if sameCount != 0 {
		summaryPieces = append(summaryPieces, fmt.Sprintf("%d unchanged", sameCount))
	}

	if len(summaryPieces) > 0 {
		fprintfIgnoreError(out, "    ")

		for i, piece := range summaryPieces {
			if i > 0 {
				fprintfIgnoreError(out, ". ")
			}

			out.WriteString(opts.Color.Colorize(piece))
		}

		fprintfIgnoreError(out, "\n")
	}

	// Print policy packs loaded. Data is rendered as a table of {policy-pack-name, version}.
	renderPolicyPacks(out, event.PolicyPacks, opts)

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		// Round up to the nearest second.  It's not useful to spit out time with 9 digits of
		// precision.
		roundedSeconds := int64(math.Ceil(event.Duration.Seconds()))
		roundedDuration := time.Duration(roundedSeconds) * time.Second

		fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("\n%sDuration:%s %s\n",
			colors.SpecHeadline, colors.Reset, roundedDuration)))
	}

	return out.String()
}

func renderPolicyPacks(out io.Writer, policyPacks map[string]string, opts Options) {
	if len(policyPacks) == 0 {
		return
	}
	fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("\n%sPolicy Packs run:%s\n",
		colors.SpecHeadline, colors.Reset)))

	// Calculate column width for the `name` column
	const nameColHeader = "Name"
	maxNameLen := len(nameColHeader)
	for pp := range policyPacks {
		if l := len(pp); l > maxNameLen {
			maxNameLen = l
		}
	}

	// Print the column headers and the policy packs.
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("    %s%s%s\n",
			columnHeader(nameColHeader), messagePadding(nameColHeader, maxNameLen, 2),
			columnHeader("Version"))))
	for pp, ver := range policyPacks {
		fprintIgnoreError(out, opts.Color.Colorize(
			fmt.Sprintf("    %s%s%s\n", pp, messagePadding(pp, maxNameLen, 2), ver)))
	}
}

func renderPreludeEvent(event events.PreludeEventPayload, opts Options) string {
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
	payload events.ResourceOperationFailedPayload, opts Options) string {

	// It's not actually useful or interesting to print out any details about
	// the resource state here, because we always assume that the resource state
	// is unknown if an error occurs.
	//
	// In the future, once we get more fine-grained error messages from providers,
	// we can provide useful diagnostics here.

	return ""
}

func renderDiff(
	out io.Writer,
	metadata events.StepEventMetadata,
	planning, debug bool,
	seen map[resource.URN]events.StepEventMetadata,
	opts Options) {

	indent := events.GetIndent(metadata, seen)
	summary := events.GetResourcePropertiesSummary(metadata, indent)

	var details string
	if metadata.DetailedDiff != nil {
		var buf bytes.Buffer
		if diff := translateDetailedDiff(metadata); diff != nil {
			events.PrintObjectDiff(&buf, *diff, nil /*include*/, planning, indent+1, opts.SummaryDiff, debug)
		} else {
			events.PrintObject(
				&buf, metadata.Old.Inputs, planning, indent+1, events.OpSame, true /*prefix*/, debug)
		}
		details = buf.String()
	} else {
		details = events.GetResourcePropertiesDetails(
			metadata, indent, planning, opts.SummaryDiff, debug)
	}

	fprintIgnoreError(out, opts.Color.Colorize(summary))
	fprintIgnoreError(out, opts.Color.Colorize(details))
	fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
}

func renderDiffResourcePreEvent(
	payload events.ResourcePreEventPayload,
	seen map[resource.URN]events.StepEventMetadata,
	opts Options) string {

	seen[payload.Metadata.URN] = payload.Metadata
	if payload.Metadata.Op == events.OpRefresh || payload.Metadata.Op == events.OpImport {
		return ""
	}

	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		renderDiff(out, payload.Metadata, payload.Planning, payload.Debug, seen, opts)
	}
	return out.String()
}

func renderDiffResourceOutputsEvent(
	payload events.ResourceOutputsEventPayload,
	seen map[resource.URN]events.StepEventMetadata,
	opts Options) string {

	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		// If this is the output step for an import, we actually want to display the diff at this point.
		if payload.Metadata.Op == events.OpImport {
			renderDiff(out, payload.Metadata, payload.Planning, payload.Debug, seen, opts)
			return out.String()
		}

		indent := events.GetIndent(payload.Metadata, seen)

		refresh := false // are these outputs from a refresh?
		if m, has := seen[payload.Metadata.URN]; has && m.Op == events.OpRefresh {
			refresh = true
			summary := events.GetResourcePropertiesSummary(payload.Metadata, indent)
			fprintIgnoreError(out, opts.Color.Colorize(summary))
		}

		if !opts.SuppressOutputs {
			// We want to hide same outputs if we're doing a read and the user didn't ask to see
			// things that are the same.
			text := events.GetResourceOutputsPropertiesString(
				payload.Metadata, indent+1, payload.Planning,
				payload.Debug, refresh, opts.ShowSameResources)
			if text != "" {
				header := fmt.Sprintf("%v%v--outputs:--%v\n",
					payload.Metadata.Op.Color(), events.GetIndentationString(indent+1), colors.Reset)
				fprintfIgnoreError(out, opts.Color.Colorize(header))
				fprintIgnoreError(out, opts.Color.Colorize(text))
			}
		}
	}
	return out.String()
}
