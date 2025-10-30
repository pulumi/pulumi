package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// Progress describes a message we want to show in the display.  There are two types of messages,
// simple 'Messages' which just get printed out as a single uninterpreted line, and 'Actions' which
// are placed and updated in the progress-grid based on their ID.  Messages do not need an ID, while
// Actions must have an ID.
type Progress = display.Progress

// ShowProgressOutput displays a progress stream from `in` to `out`, `isInteractive` describes if
// `out` is a terminal. If this is the case, it will print `\n` at the end of each line and move the
// cursor while displaying.
func ShowProgressOutput(in <-chan Progress, out io.Writer, isInteractive bool) {
	display.ShowProgressOutput(in, out, isInteractive)
}

