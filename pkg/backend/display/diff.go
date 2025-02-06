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
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize/english"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ShowDiffEvents displays the engine events with the diff view.
func ShowDiffEvents(op string, events <-chan engine.Event, done chan<- bool, opts Options) {
	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op)

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var spinner cmdutil.Spinner
	var ticker *time.Ticker
	if stdout == os.Stdout && stderr == os.Stderr {
		spinner, ticker = cmdutil.NewSpinnerAndTicker(prefix, nil, opts.Color, 8 /*timesPerSecond*/, opts.SuppressProgress)
	} else {
		spinner = &nopSpinner{}
		ticker = time.NewTicker(math.MaxInt64)
	}

	defer func() {
		spinner.Reset()
		ticker.Stop()
		close(done)
	}()

	seen := make(map[resource.URN]engine.StepEventMetadata)

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-events:
			spinner.Reset()

			out := stdout
			if event.Type == engine.DiagEvent {
				payload := event.Payload().(engine.DiagEventPayload)
				if payload.Severity == diag.Error || payload.Severity == diag.Warning {
					out = stderr
				}
			}

			msg := RenderDiffEvent(event, seen, opts)
			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}

			if event.Type == engine.CancelEvent {
				return
			}
		}
	}
}

func RenderDiffEvent(event engine.Event, seen map[resource.URN]engine.StepEventMetadata, opts Options) string {
	switch event.Type {
	case engine.CancelEvent:
		return ""
	case engine.PolicyLoadEvent:
		return ""
	case engine.StartDebuggingEvent:
		return ""
	case engine.ProgressEvent:
		return ""

		// Currently, prelude, summary, and stdout events are printed the same for both the diff and
		// progress displays.
	case engine.PreludeEvent:
		return renderPreludeEvent(event.Payload().(engine.PreludeEventPayload), opts)
	case engine.SummaryEvent:
		return renderSummaryEvent(event.Payload().(engine.SummaryEventPayload), true, opts)
	case engine.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload().(engine.StdoutEventPayload), opts)

		// Resource operations have very specific displays for either diff or progress displays.
		// These functions should not be directly used by the progress display without validating
		// that the display is appropriate for both.
	case engine.ResourceOperationFailed:
		return renderDiffResourceOperationFailedEvent(event.Payload().(engine.ResourceOperationFailedPayload), opts)
	case engine.ResourceOutputsEvent:
		return renderDiffResourceOutputsEvent(event.Payload().(engine.ResourceOutputsEventPayload), seen, opts)
	case engine.ResourcePreEvent:
		return renderDiffResourcePreEvent(event.Payload().(engine.ResourcePreEventPayload), seen, opts)
	case engine.DiagEvent:
		return renderDiffDiagEvent(event.Payload().(engine.DiagEventPayload), opts)
	case engine.PolicyRemediationEvent:
		return renderDiffPolicyRemediationEvent(event.Payload().(engine.PolicyRemediationEventPayload), "", true, opts)
	case engine.PolicyViolationEvent:
		return renderDiffPolicyViolationEvent(event.Payload().(engine.PolicyViolationEventPayload), "", "", opts)

	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderDiffDiagEvent(payload engine.DiagEventPayload, opts Options) string {
	if payload.Severity == diag.Debug && !opts.Debug {
		return ""
	}
	return opts.Color.Colorize(payload.Prefix + payload.Message)
}

func renderDiffPolicyRemediationEvent(payload engine.PolicyRemediationEventPayload,
	prefix string, detailed bool, opts Options,
) string {
	// Diff the before/after state. If there is no diff, we show nothing.
	diff := payload.Before.Diff(payload.After)
	if diff == nil {
		return ""
	}

	// Print the individual remediation's name and target resource type/name.
	remediationLine := fmt.Sprintf("%s[remediate]  %s%s  (%s: %s)",
		colors.SpecInfo, payload.PolicyName, colors.Reset, payload.ResourceURN.Type(), payload.ResourceURN.Name())

	// If there is already a prefix string requested, use it, otherwise fall back to a default.
	if prefix == "" {
		remediationLine = fmt.Sprintf("    %s%s@v%s %s%s",
			colors.SpecInfo, payload.PolicyPackName, payload.PolicyPackVersion, colors.Reset, remediationLine)
	} else {
		remediationLine = fmt.Sprintf("%s%s", prefix, remediationLine)
	}

	// Render the event's diff; if a detailed diff is requested, a full object diff is emitted, otherwise
	// a short diff summary similar to what is show for an update row is emitted.
	if detailed {
		var b bytes.Buffer
		PrintObjectDiff(&b, *diff, nil,
			false /*planning*/, 2, true /*summary*/, true /*truncateOutput*/, false /*debug*/)
		remediationLine = fmt.Sprintf("%s\n%s", remediationLine, b.String())
	} else {
		var b bytes.Buffer
		writeShortDiff(&b, diff, nil)
		remediationLine = fmt.Sprintf("%s [%s]", remediationLine, b.String())
	}

	return opts.Color.Colorize(remediationLine + "\n")
}

func renderDiffPolicyViolationEvent(payload engine.PolicyViolationEventPayload,
	prefix string, linePrefix string, opts Options,
) string {
	// Colorize mandatory and warning violations differently.
	c := colors.SpecWarning
	if payload.EnforcementLevel == apitype.Mandatory {
		c = colors.SpecError
	}

	// Print the individual policy's name and target resource type/name.
	policyLine := fmt.Sprintf("%s[%s]  %s%s  (%s: %s)",
		c, payload.EnforcementLevel, payload.PolicyName, colors.Reset,
		payload.ResourceURN.Type(), payload.ResourceURN.Name())

	// If there is already a prefix string requested, use it, otherwise fall back to a default.
	if prefix == "" {
		policyLine = fmt.Sprintf("    %s%s@v%s %s%s",
			colors.SpecInfo, payload.PolicyPackName, payload.PolicyPackVersion, colors.Reset, policyLine)
	} else {
		policyLine = fmt.Sprintf("%s%s", prefix, policyLine)
	}

	// If there is a line prefix, separate the heading and lines with a newline.
	if linePrefix != "" {
		policyLine += "\n"
	}

	// The message may span multiple lines, so we massage it so it will be indented properly.
	message := strings.TrimSuffix(payload.Message, "\n")
	message = strings.ReplaceAll(message, "\n", "\n"+linePrefix)
	policyLine = fmt.Sprintf("%s%s%s", policyLine, linePrefix, message)
	return opts.Color.Colorize(policyLine + "\n")
}

func renderStdoutColorEvent(payload engine.StdoutEventPayload, opts Options) string {
	return opts.Color.Colorize(payload.Message)
}

func renderSummaryEvent(event engine.SummaryEventPayload, diffStyleSummary bool, opts Options) string {
	changes := event.ResourceChanges

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%sResources:%s\n", colors.SpecHeadline, colors.Reset)))

	var planTo string
	if event.IsPreview {
		planTo = "to "
	}

	changeKindCount := 0
	changeCount := 0
	sameCount := changes[deploy.OpSame]

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range deploy.StepOps {
		// Ignore anything that didn't change, or is related to 'reads'.  'reads' are just an
		// indication of the operations we were performing, and are not indicative of any sort of
		// change to the system.
		if op != deploy.OpSame && op != deploy.OpRead && op != deploy.OpReadDiscard && op != deploy.OpReadReplacement {
			if c := changes[op]; c > 0 {
				opDescription := string(op)
				if !event.IsPreview {
					opDescription = deploy.PastTense(op)
				}

				// Increment the change count by the number of changes associated with this step kind
				changeCount += c

				// Increment the number of kinds of changes by one
				changeKindCount++

				// Print a summary of the changes of this kind
				fprintIgnoreError(out, opts.Color.Colorize(
					fmt.Sprintf("    %s%d %s%s%s\n", deploy.Prefix(op, true /*done*/), c, planTo, opDescription, colors.Reset)))
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
		fprintIgnoreError(out, "    ")

		for i, piece := range summaryPieces {
			if i > 0 {
				fprintIgnoreError(out, ". ")
			}

			out.WriteString(opts.Color.Colorize(piece))
		}

		fprintIgnoreError(out, "\n")
	}

	if diffStyleSummary {
		// Print policy packs loaded. Data is rendered as a table of {policy-pack-name, version}.
		// This is only shown during the diff view, because in the progress view we have a nicer
		// summarization and grouping of all violations and remediations that have occurred. The
		// diff view renders events incrementally as we go, so it cannot do this.
		renderPolicyPacks(out, event.PolicyPacks, opts)
	}

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

func renderPreludeEvent(event engine.PreludeEventPayload, opts Options) string {
	// Only if we have been instructed to show configuration values will we print anything during the prelude.
	if !opts.ShowConfig {
		return ""
	}

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%sConfiguration:%s\n", colors.SpecUnimportant, colors.Reset)))

	keys := slice.Prealloc[string](len(event.Config))
	for key := range event.Config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, err := fmt.Fprintf(out, "    %v: %v\n", key, event.Config[key])
		contract.IgnoreError(err)
	}

	return out.String()
}

func renderDiffResourceOperationFailedEvent(
	payload engine.ResourceOperationFailedPayload, opts Options,
) string {
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
	metadata engine.StepEventMetadata,
	planning, debug, refresh bool,
	seen map[resource.URN]engine.StepEventMetadata,
	opts Options,
) {
	indent := getIndent(metadata, seen)
	summary := getResourcePropertiesSummary(metadata, indent)

	var details string
	// An OpSame might have a diff due to metadata changes (e.g. protect) but we should never print a property diff,
	// even if the properties appear to have changed. See https://github.com/pulumi/pulumi/issues/15944 for context.
	if metadata.Op != deploy.OpSame {
		if metadata.DetailedDiff != nil {
			var buf bytes.Buffer
			if diff := engine.TranslateDetailedDiff(&metadata, refresh); diff != nil {
				PrintObjectDiff(&buf, *diff, nil /*include*/, planning, indent+1, opts.SummaryDiff, opts.TruncateOutput, debug)
			} else {
				PrintObject(
					&buf, metadata.Old.Inputs, planning, indent+1, deploy.OpSame, true /*prefix*/, opts.TruncateOutput, debug)
			}
			details = buf.String()
		} else {
			details = getResourcePropertiesDetails(
				metadata, indent, planning, opts.SummaryDiff, opts.TruncateOutput, debug)
		}
	}
	fprintIgnoreError(out, opts.Color.Colorize(summary))
	fprintIgnoreError(out, opts.Color.Colorize(details))
	fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
}

func renderDiffResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts Options,
) string {
	seen[payload.Metadata.URN] = payload.Metadata
	if payload.Metadata.Op == deploy.OpRefresh || payload.Metadata.Op == deploy.OpImport {
		return ""
	}

	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		renderDiff(out, payload.Metadata, payload.Planning, payload.Debug, false /* refresh */, seen, opts)
	}
	return out.String()
}

func renderDiffResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts Options,
) string {
	out := &bytes.Buffer{}
	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		refresh := false // are these outputs from a refresh?
		if m, has := seen[payload.Metadata.URN]; has && m.Op == deploy.OpRefresh {
			refresh = true
		}

		// There are two cases where we want to display a diff at the point of a
		// resource output event:
		//
		// * Imports, where we now have information from the provider about the
		//   resource being imported.
		// * Refreshes, where similarly we might have updated information about the
		//   resource from the provider.
		//
		// Note that refresh step result operations will be OpUpdates (something
		// changed in the provider), OpSames (nothing changed in the provider), or
		// OpDeletes (the resource was deleted in the provider). We only want to
		// display a diff in the OpUpdate case. In the OpSame case, there is no diff
		// (otherwise the operation would have been OpUpdate), and in the OpDelete
		// case, we will already be indicating a deletion and it doesn't make sense
		// to display a diff that shows that we are deleting everything.
		if payload.Metadata.Op == deploy.OpImport || (refresh && payload.Metadata.Op == deploy.OpUpdate) {
			renderDiff(out, payload.Metadata, payload.Planning, payload.Debug, refresh, seen, opts)
			return out.String()
		}

		indent := getIndent(payload.Metadata, seen)

		if refresh {
			// We would not have rendered the summary yet in this case, so do it now.
			summary := getResourcePropertiesSummary(payload.Metadata, indent)
			fprintIgnoreError(out, opts.Color.Colorize(summary))
		}

		if !opts.SuppressOutputs {
			// We want to hide same outputs if we're doing a read and the user didn't ask to see
			// things that are the same.
			text := getResourceOutputsPropertiesString(
				payload.Metadata, indent+1, payload.Planning,
				payload.Debug, refresh, opts.ShowSameResources)
			if text != "" {
				header := fmt.Sprintf("%v%v--outputs:--%v\n",
					deploy.Color(payload.Metadata.Op), getIndentationString(indent+1, payload.Metadata.Op, false), colors.Reset)
				fprintIgnoreError(out, opts.Color.Colorize(header))
				fprintIgnoreError(out, opts.Color.Colorize(text))
			}
		}
	}
	return out.String()
}

// CreateDiff renders a view of the given events, enforcing an order of rendering that is consistent
// with the diff view.
func CreateDiff(events []engine.Event, displayOpts Options) (string, error) {
	buff := &bytes.Buffer{}

	seen := make(map[resource.URN]engine.StepEventMetadata)
	displayOpts.SummaryDiff = true

	if displayOpts.Color == "" {
		// ColorizeWithMaxWidth panics if the colorization mode is not recognized, so we enforce it here.
		return "", errors.New("color must be specified")
	}

	remediationEventsDiff := make([]string, 0)
	for _, e := range events {
		if e.Type == engine.SummaryEvent {
			continue
		}

		msg := RenderDiffEvent(e, seen, displayOpts)
		if msg == "" {
			continue
		}

		// Keep track of remediation events separately, since we print them after the ordinary resource
		// diff information.
		if e.Type == engine.PolicyRemediationEvent {
			remediationEventsDiff = append(remediationEventsDiff, msg)
			continue
		}

		_, err := buff.WriteString(msg)
		contract.IgnoreError(err)
	}

	// Print policy remediations last.
	if len(remediationEventsDiff) > 0 {
		_, err := buff.WriteString(displayOpts.Color.Colorize(
			fmt.Sprintf("\n%s  Policy Remediations:%s\n", colors.SpecHeadline, colors.Reset)))
		contract.IgnoreError(err)
		for _, msg := range remediationEventsDiff {
			_, err := buff.WriteString(msg)
			contract.IgnoreError(err)
		}
	}

	return strings.TrimSpace(buff.String()), nil
}
