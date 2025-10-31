package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// ShowDiffEvents displays the engine events with the diff view.
func ShowDiffEvents(op string, events <-chan engine.Event, done chan<- bool, opts Options) {
	display.ShowDiffEvents(op, events, done, opts)
}

func RenderDiffEvent(event engine.Event, resourcesErrored int, seen map[resource.URN]engine.StepEventMetadata, opts Options) string {
	return display.RenderDiffEvent(event, resourcesErrored, seen, opts)
}

// CreateDiff renders a view of the given events, enforcing an order of rendering that is consistent
// with the diff view.
func CreateDiff(events []engine.Event, displayOpts Options) (string, error) {
	return display.CreateDiff(events, displayOpts)
}

