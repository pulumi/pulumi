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
			switch event.Type {
			case engine.CancelEvent:
				return
			case engine.PreludeEvent:
				displayPreludeEvent(os.Stdout, event.Payload.(engine.PreludeEventPayload), opts)
			case engine.SummaryEvent:
				displaySummaryEvent(os.Stdout, event.Payload.(engine.SummaryEventPayload), opts)
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
				spinner.Reset()
				_, err := fmt.Fprint(out, msg)
				contract.IgnoreError(err)
			}
		}
	}
}

// nolint: gas
func displaySummaryEvent(out io.Writer, event engine.SummaryEventPayload, opts backend.DisplayOptions) {
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

	fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf("%vinfo%v: %v %v %v\n",
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
				fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf("    %v%v %v %v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, opDescription, colors.Reset)))
			}
		}
	}
	if c := changes[deploy.OpSame]; c > 0 {
		fmt.Fprintf(out, "      %v %v unchanged\n", c, plural("resource", c))
	}

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		if changeCount > 0 {
			fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf("%vUpdate duration: %v%v\n",
				colors.SpecUnimportant, event.Duration, colors.Reset)))
		}

		if event.MaybeCorrupt {
			fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf(
				"%vA catastrophic error occurred; resources states may be unknown%v\n",
				colors.SpecAttention, colors.Reset)))
		}
	}
}

// nolint: gas
func displayPreludeEvent(out io.Writer, event engine.PreludeEventPayload, opts backend.DisplayOptions) {
	if opts.ShowConfig {
		fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset)))

		var keys []string
		for key := range event.Config {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(out, "    %v: %v\n", key, event.Config[key])
		}
	}

	action := "Previewing"
	if !event.IsPreview {
		action = "Performing"
	}

	fmt.Fprint(out, opts.Color.Colorize(fmt.Sprintf("%v%v changes:%v\n", colors.SpecUnimportant, action, colors.Reset)))
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}
