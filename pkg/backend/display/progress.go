package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// DiagInfo contains the bundle of diagnostic information for a single resource.
type DiagInfo = display.DiagInfo

// ProgressDisplay organizes all the information needed for a dynamically updated "progress" view of an update.
type ProgressDisplay = display.ProgressDisplay

type CaptureProgressEvents = display.CaptureProgressEvents

// ShowProgressEvents displays the engine events with docker's progress view.
func ShowProgressEvents(op string, action apitype.UpdateKind, stack tokens.StackName, proj tokens.PackageName, permalink string, events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool) {
	display.ShowProgressEvents(op, action, stack, proj, permalink, events, done, opts, isPreview)
}

// RenderProgressEvents renders the engine events as if to a terminal, providing a simple interface
// for rendering the progress of an update.
// 
// A "simple" terminal is used which does not render control sequences. The simple terminal's output
// is written to opts.Stdout.
// 
// For consistent output, these settings are enforced:
// 
// 	opts.Color = colors.Never
// 	opts.RenderOnDirty = false
// 	opts.IsInteractive = true
func RenderProgressEvents(op string, action apitype.UpdateKind, stack tokens.StackName, proj tokens.PackageName, permalink string, events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool, width, height int) {
	display.RenderProgressEvents(op, action, stack, proj, permalink, events, done, opts, isPreview, width, height)
}

// NewCaptureProgressEvents renders the provided engine events channel to an internal buffer. It returns a
// CaptureProgressEvents instance that can be used to access both the output and the display instance after processing
// the events. This is useful for detecting whether a failure was detected in the display layer, e.g. used to send the
// output to Copilot if a failure was detected.
func NewCaptureProgressEvents(stack tokens.StackName, proj tokens.PackageName, opts Options, isPreview bool, action apitype.UpdateKind) *CaptureProgressEvents {
	return display.NewCaptureProgressEvents(stack, proj, opts, isPreview, action)
}

