package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// ShowWatchEvents renders incoming engine events for display in Watch Mode.
func ShowWatchEvents(op string, permalink string, events <-chan engine.Event, done chan<- bool, opts Options) {
	display.ShowWatchEvents(op, permalink, events, done, opts)
}

// WatchPrefixPrintf wraps fmt.Printf with a watch mode prefixer that adds a timestamp and
// resource metadata.
func WatchPrefixPrintf(t time.Time, colorization colors.Colorization, resourceName string, format string, a ...any) {
	display.WatchPrefixPrintf(t, colorization, resourceName, format, a...)
}

