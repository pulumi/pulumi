// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// displayEvents reads events from the `events` channel until it is closed, displaying each event as it comes in.
// Once all events have been read from the channel and displayed, it closes the `done` channel so the caller can
// await all the events being written.
func displayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {
	prefix := fmt.Sprintf("✨ %s...", action)
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
