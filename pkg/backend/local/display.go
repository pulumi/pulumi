// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
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

// displayEvents reads events from the `events` channel until it is closed, displaying each event as it comes in.
// Once all events have been read from the channel and displayed, it closes the `done` channel so the caller can
// await all the events being written.
func displayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {
	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil)

	defer func() {
		spinner.Reset()
		ticker.Stop()
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-events:
			var out io.Writer
			var msg string
			spinner.Reset()
			switch event.Type {
			case engine.CancelEvent:
				return
			case engine.PreludeEvent:
				DisplayPreludeEvent(os.Stdout, event.Payload.(engine.PreludeEventPayload), opts)
			case engine.SummaryEvent:
				DisplaySummaryEvent(os.Stdout, event.Payload.(engine.SummaryEventPayload), opts)
			case engine.ResourceOperationFailed:
				DisplayResourceOperationFailedEvent(os.Stdout,
					event.Payload.(engine.ResourceOperationFailedPayload), opts)
			case engine.ResourceOutputsEvent:
				DisplayResourceOutputsEvent(os.Stdout, event.Payload.(engine.ResourceOutputsEventPayload), opts)
			case engine.ResourcePreEvent:
				DisplayResourcePreEvent(os.Stdout, event.Payload.(engine.ResourcePreEventPayload), opts)
			case engine.StdoutColorEvent:
				payload := event.Payload.(engine.StdoutEventPayload)
				out = os.Stdout
				msg = opts.Color.Colorize(payload.Message)
			case engine.DiagEvent:
				payload := event.Payload.(engine.DiagEventPayload)
				if payload.Severity == diag.Error || payload.Severity == diag.Warning {
					out = os.Stderr
				} else if payload.Severity != diag.Debug || debug {
					out = os.Stdout
				}
				msg = opts.Color.Colorize(payload.Message)
			default:
				contract.Failf("unknown event type '%s'", event.Type)
			}

			if msg != "" && out != nil {
				fprintIgnoreError(out, msg)
			}
		}
	}
}

func DisplaySummaryEvent(out io.Writer, event engine.SummaryEventPayload, opts backend.DisplayOptions) {
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
}

func DisplayPreludeEvent(out io.Writer, event engine.PreludeEventPayload, opts backend.DisplayOptions) {
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
}

func DisplayResourceOperationFailedEvent(out io.Writer,
	event engine.ResourceOperationFailedPayload, opts backend.DisplayOptions) {

	// It's not actually useful or interesting to print out any details about
	// the resource state here, because we always assume that the resource state
	// is unknown if an error occurs.
	//
	// In the future, once we get more fine-grained error messages from providers,
	// we can provide useful diagnostics here.
}

func DisplayResourcePreEvent(out io.Writer, event engine.ResourcePreEventPayload, opts backend.DisplayOptions) {
	if shouldShow(event.Metadata, opts) || isRootStack(event.Metadata) {
		fprintIgnoreError(out, opts.Color.Colorize(event.Summary))

		if !opts.Summary {
			fprintIgnoreError(out, opts.Color.Colorize(event.Details))
		}

		fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
	}
}

func DisplayResourceOutputsEvent(out io.Writer, event engine.ResourceOutputsEventPayload, opts backend.DisplayOptions) {
	if (shouldShow(event.Metadata, opts) || isRootStack(event.Metadata)) && !opts.Summary {
		fprintIgnoreError(out, opts.Color.Colorize(event.Text))
	}
}

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step engine.StepEventMetdata) bool {
	return step.URN.Type() == resource.RootStackType
}

// shouldShow returns true if a step should show in the output.
func shouldShow(step engine.StepEventMetdata, opts backend.DisplayOptions) bool {
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
