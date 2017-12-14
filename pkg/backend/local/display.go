// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// displayEvents reads events from the `events` channel until it is closed, displaying each event as it comes in.
// Once all events have been read from the channel and displayed, it closes the `done` channel so the caller can
// await all the events being written.
func displayEvents(events <-chan engine.Event, done chan bool, debug bool) {
	defer func() {
		done <- true
	}()

Outer:
	for event := range events {
		switch event.Type {
		case engine.CancelEvent:
			break Outer
		case engine.StdoutColorEvent:
			payload := event.Payload.(engine.StdoutEventPayload)
			fmt.Print(payload.Color.Colorize(payload.Message))
		case engine.DiagEvent:
			payload := event.Payload.(engine.DiagEventPayload)
			var out io.Writer
			out = os.Stdout

			if payload.Severity == diag.Error || payload.Severity == diag.Warning {
				out = os.Stderr
			}
			if payload.Severity == diag.Debug && !debug {
				out = ioutil.Discard
			}
			msg := payload.Message
			msg = payload.Color.Colorize(msg)
			_, fmterr := fmt.Fprint(out, msg)
			contract.IgnoreError(fmterr)
		default:
			contract.Failf("unknown event type '%s'", event.Type)
		}
	}
}
