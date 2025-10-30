package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// ShowEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func ShowEvents(op string, action apitype.UpdateKind, stack tokens.StackName, proj tokens.PackageName, permalink string, events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool) {
	display.ShowEvents(op, action, stack, proj, permalink, events, done, opts, isPreview)
}

