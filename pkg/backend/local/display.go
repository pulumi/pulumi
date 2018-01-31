// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
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
