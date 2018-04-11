// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// DisplayEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func DisplayEvents(
	action string, events <-chan engine.Event,
	done chan<- bool, opts backend.DisplayOptions) {

	if opts.DiffDisplay {
		DisplayDiffEvents(action, events, done, opts)
	} else {
		DisplayProgressEvents(action, events, done, opts)
	}
}

// DisplayDiffEvents displays the engine events with the diff view.
func DisplayDiffEvents(action string,
	events <-chan engine.Event, done chan<- bool, opts backend.DisplayOptions) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, 8 /*timesPerSecond*/)

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

func RenderDiffEvent(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata, opts backend.DisplayOptions) string {

	switch event.Type {
	case engine.CancelEvent:
		return ""
	case engine.PreludeEvent:
		return renderPreludeEvent(event.Payload.(engine.PreludeEventPayload), opts)
	case engine.SummaryEvent:
		return renderSummaryEvent(event.Payload.(engine.SummaryEventPayload), opts)
	case engine.ResourceOperationFailed:
		return renderResourceOperationFailedEvent(event.Payload.(engine.ResourceOperationFailedPayload), opts)
	case engine.ResourceOutputsEvent:
		return renderResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts)
	case engine.ResourcePreEvent:
		return renderResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts)
	case engine.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload.(engine.StdoutEventPayload), opts)
	case engine.DiagEvent:
		return renderDiagEvent(event.Payload.(engine.DiagEventPayload), opts)
	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderDiagEvent(payload engine.DiagEventPayload, opts backend.DisplayOptions) string {
	if payload.Severity == diag.Debug && !opts.Debug {
		return ""
	}
	return opts.Color.Colorize(payload.Message)
}

func renderStdoutColorEvent(
	payload engine.StdoutEventPayload, opts backend.DisplayOptions) string {

	return opts.Color.Colorize(payload.Message)
}

func renderSummaryEvent(event engine.SummaryEventPayload, opts backend.DisplayOptions) string {
	changes := event.ResourceChanges

	changeCount := 0
	for op, c := range changes {
		if op != deploy.OpSame {
			changeCount += c
		}
	}
	var kind string
	if event.IsPreview {
		kind = "previewed"
	} else {
		kind = "performed"
	}

	var changesLabel string
	if changeCount == 0 {
		kind = "required"
		changesLabel = "no"
	} else {
		changesLabel = strconv.Itoa(changeCount)
	}

	if changeCount > 0 || changes[deploy.OpSame] > 0 {
		kind += ":"
	}

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vinfo%v: %v %v %v\n",
		colors.SpecInfo, colors.Reset, changesLabel, plural("change", changeCount), kind)))

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
				fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("    %v%v %v %v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, opDescription, colors.Reset)))
			}
		}
	}
	if c := changes[deploy.OpSame]; c > 0 {
		fprintfIgnoreError(out, "      %v %v unchanged\n", c, plural("resource", c))
	}

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		if changeCount > 0 {
			fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vUpdate duration: %v%v\n",
				colors.SpecUnimportant, event.Duration, colors.Reset)))
		}
	}

	return out.String()
}

func renderPreludeEvent(event engine.PreludeEventPayload, opts backend.DisplayOptions) string {
	out := &bytes.Buffer{}

	if opts.ShowConfig {
		fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset)))

		var keys []string
		for key := range event.Config {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fprintfIgnoreError(out, "    %v: %v\n", key, event.Config[key])
		}
	}

	action := "Previewing"
	if !event.IsPreview {
		action = "Performing"
	}

	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%v%v changes:%v\n", colors.SpecUnimportant, action, colors.Reset)))

	return out.String()
}

func renderResourceOperationFailedEvent(
	payload engine.ResourceOperationFailedPayload, opts backend.DisplayOptions) string {


	return ""
}

func renderResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions) string {

	seen[payload.Metadata.URN] = payload.Metadata

	out := &bytes.Buffer{}

	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		indent := engine.GetIndent(payload.Metadata, seen)
		summary := engine.GetResourcePropertiesSummary(payload.Metadata, indent)
		details := engine.GetResourcePropertiesDetails(payload.Metadata, indent, payload.Planning, payload.Debug)

		fprintIgnoreError(out, opts.Color.Colorize(summary))
		fprintIgnoreError(out, opts.Color.Colorize(details))
		fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
	}

	return out.String()
}

func renderResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions) string {

	out := &bytes.Buffer{}

	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		indent := engine.GetIndent(payload.Metadata, seen)
		text := engine.GetResourceOutputsPropertiesString(payload.Metadata, indent+1, payload.Planning, payload.Debug)

		fprintIgnoreError(out, opts.Color.Colorize(text))
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
func shouldShow(step engine.StepEventMetadata, opts backend.DisplayOptions) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if step.Op == deploy.OpSame {
		// If the op is the same, it is possible that the resource's metadata changed.  In that case, still show it.
		if step.Old.Protect != step.New.Protect {
			return true
		}
		return opts.ShowSames
	} else if step.Op == deploy.OpCreateReplacement || step.Op == deploy.OpDeleteReplaced {
		return opts.ShowReplacementSteps
	} else if step.Op == deploy.OpReplace {
		return !opts.ShowReplacementSteps
	}
	return true
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}

func fprintfIgnoreError(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.IgnoreError(err)
}

func fprintIgnoreError(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	contract.IgnoreError(err)
}
