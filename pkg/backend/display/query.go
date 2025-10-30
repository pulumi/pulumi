package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// ShowQueryEvents displays query events on the CLI.
func ShowQueryEvents(op string, events <-chan engine.Event, done chan<- bool, opts Options) {
	display.ShowQueryEvents(op, events, done, opts)
}

