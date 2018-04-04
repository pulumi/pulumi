// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/term"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// copied from: https://github.com/docker/cli/blob/master/cli/command/out.go
// replace with usage of that library when we can figure out hte right version story

type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal
func (s *commonStream) RestoreTerminal() {
	if s.state != nil {
		term.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal sets the boolean used for isTerminal
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

type outStream struct {
	commonStream
	out io.Writer
}

func (o *outStream) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal
func (o *outStream) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !o.commonStream.isTerminal {
		return nil
	}
	o.commonStream.state, err = term.SetRawTerminalOutput(o.commonStream.fd)
	return err
}

// GetTtySize returns the height and width in characters of the tty
func (o *outStream) GetTtySize() (uint, uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOutStream returns a new OutStream object from a Writer
func newOutStream(out io.Writer) *outStream {
	fd, isTerminal := term.GetFdInfo(out)
	return &outStream{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, out: out}
}

func writeDistributionProgress(outStream io.Writer, progressChan <-chan progress.Progress) {
	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(outStream, false)

	for prog := range progressChan {
		// fmt.Printf("Received progress")
		progressOutput.WriteProgress(prog)
	}
}

type ProgressAndEllipses struct {
	progress.Progress
	Ellipses int
}

// DisplayEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func DisplayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	_, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, 1 /*timesPerSecond*/)

	_, stdout, _ := term.StdStreams()
	// _, isTerminal := term.GetFdInfo(stdout)

	pipeReader, pipeWriter := io.Pipe()
	progressChan := make(chan progress.Progress, 100)

	chanOutput := progress.ChanOutput(progressChan)

	go func() {
		writeDistributionProgress(pipeWriter, progressChan)
		pipeWriter.Close()
	}()

	defer func() {
		// spinner.Reset()
		ticker.Stop()
		done <- true
	}()

	var stackURN resource.URN
	seen := make(map[resource.URN]engine.StepEventMetadata)
	topLevelUrnToInProgressObj := make(map[resource.URN]ProgressAndEllipses)
	var summaryEvent *engine.Event
	diagEvents := []engine.Event{}

	processEndSteps := func() {
		// Mark all in progress resources as done.
		for _, v := range topLevelUrnToInProgressObj {
			chanOutput.WriteProgress(progress.Progress{
				ID:     v.ID,
				Action: "Done!",
			})
		}

		// print the summary
		// out := os.Stdout
		if summaryEvent != nil {
			_, msg := RenderEvent(*summaryEvent, seen, debug, opts)
			if msg != "" {
				chanOutput.WriteProgress(progress.Progress{Message: " "})
				chanOutput.WriteProgress(progress.Progress{Message: msg})
			}
		}

		if len(diagEvents) > 0 {
			chanOutput.WriteProgress(progress.Progress{Message: " "})
		}

		// Print all diagnostics at the end
		for _, v := range diagEvents {
			// out = os.Stdout
			// if v.Severity == diag.Error || v.Severity == diag.Warning {
			// 	out = os.Stderr
			// }

			_, msg := RenderEvent(v, seen, debug, opts)
			if msg != "" {
				chanOutput.WriteProgress(progress.Progress{Message: msg})
			}
		}

		// no more progress events from this point on.
		close(progressChan)
	}

	var makeID func(urn resource.URN) string
	makeID = func(urn resource.URN) string {
		if urn == "" {
			return "global"
		}

		return string(urn.Type()) + "(\"" + string(urn.Name()) + "\")"
	}

	var mapToTopLevelUrn func(urn resource.URN) resource.URN
	mapToTopLevelUrn = func(urn resource.URN) resource.URN {
		if urn == "" || urn == stackURN {
			return stackURN
		}

		v, ok := seen[urn]
		if !ok {
			return stackURN
		}

		parent := v.Res.Parent
		if parent == "" || parent == stackURN {
			return urn
		}

		return mapToTopLevelUrn(parent)
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				for _, v := range topLevelUrnToInProgressObj {
					ellipses := strings.Repeat(".", (v.Ellipses%3)+1) + "  "

					chanOutput.WriteProgress(progress.Progress{
						ID:     v.ID,
						Action: v.Action + ellipses,
					})
				}

				for k, v := range topLevelUrnToInProgressObj {
					topLevelUrnToInProgressObj[k] =
						ProgressAndEllipses{Progress: v.Progress, Ellipses: v.Ellipses + 1}
				}

			// 	spinner.Tick()
			case event := <-events:
				// spinner.Reset()

				// out := os.Stdout
				// if event.Type == engine.DiagEvent {
				// 	payload := event.Payload.(engine.DiagEventPayload)
				// 	if payload.Severity == diag.Error || payload.Severity == diag.Warning {
				// 		out = os.Stderr
				// 	}
				// }

				// msg := RenderEvent(event, seen, debug, opts)
				// if msg != "" && out != nil {
				// 	// fprintIgnoreError(out, event.Type+": ")
				// 	fprintIgnoreError(out, msg)

				// if msg != "" && out != nil {
				// 	// fprintIgnoreError(out, event.Type+": ")
				// 	fprintIgnoreError(out, msg)
				// }
				if event.Type == "" || event.Type == engine.CancelEvent {
					processEndSteps()
					return
				}
				// }

				eventUrn, msg := RenderEvent(event, seen, debug, opts)
				msg = strings.TrimSpace(msg)
				if msg != "" {
					switch event.Type {
					case engine.PreludeEvent: // , engine.StdoutColorEvent:
						chanOutput.WriteProgress(progress.Progress{Message: " "})
						chanOutput.WriteProgress(progress.Progress{Message: msg})
						// fprintIgnoreError(os.Stdout, msg)
						continue
					case engine.SummaryEvent:
						summaryEvent = &event
						continue
					}

					// case engine.ResourceOutputsEvent:
					// 	return RenderResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts)
					// case engine.ResourcePreEvent:
					// 	return RenderResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts)
					// case engine.DiagEvent:
					// }

					if isRootURN(eventUrn) {
						stackURN = eventUrn
					}

					topLevelUrn := mapToTopLevelUrn(eventUrn)
					id := makeID(topLevelUrn)
					prog := progress.Progress{ID: id, Action: msg}
					chanOutput.WriteProgress(prog)

					if event.Type == engine.DiagEvent {
						// also record this diagnostic so we print it at the end.
						diagEvents = append(diagEvents, event)
					} else if event.Type == engine.ResourceOutputsEvent {
						if eventUrn == topLevelUrn {
							// resource finished.  take it out of the in-progress group so that we don't
							// continually update the ellipses for it.
							delete(topLevelUrnToInProgressObj, topLevelUrn)
						}
					} else {
						// mark the latest progress message we made for this resource.
						topLevelUrnToInProgressObj[topLevelUrn] =
							ProgressAndEllipses{Progress: prog, Ellipses: 0}
					}
				}
			}
		}
	}()

	jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
}

func RenderEvent(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions) (resource.URN, string) {

	urn, msg := renderEventWorker(event, seen, debug, opts)
	return urn, strings.TrimSpace(msg)
}

func renderEventWorker(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions) (resource.URN, string) {

	switch event.Type {
	case engine.CancelEvent:
		return "", ""
	case engine.PreludeEvent:
		return "", renderPreludeEvent(event.Payload.(engine.PreludeEventPayload), opts)
	case engine.SummaryEvent:
		return "", renderSummaryEvent(event.Payload.(engine.SummaryEventPayload), opts)
	case engine.ResourceOperationFailed:
		return "", renderResourceOperationFailedEvent(event.Payload.(engine.ResourceOperationFailedPayload), opts)
	case engine.ResourceOutputsEvent:
		return renderResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts)
	case engine.ResourcePreEvent:
		return renderResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts)
	case engine.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload.(engine.StdoutEventPayload), opts)
	case engine.DiagEvent:
		return renderDiagEvent(event.Payload.(engine.DiagEventPayload), debug, opts)
	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return "", ""
	}
}

// func upToFirstNewLine(opts backend.DisplayOptions, msg string) string {
// 	if msg == "" {
// 		return msg
// 	}

// 	var newLineIndex = strings.Index(msg, "\n")
// 	if newLineIndex >= 0 {
// 		msg = msg[0:newLineIndex]
// 	}

// 	if len(msg) > 180 {
// 		msg = msg[0:180]
// 	}

// 	msg = msg + opts.Color.Colorize(colors.Reset) + "\n"
// 	return msg
// }

func renderDiagEvent(
	payload engine.DiagEventPayload, debug bool, opts backend.DisplayOptions) (resource.URN, string) {
	if payload.Severity == diag.Debug && !debug {
		return "", ""
	}

	return payload.URN, opts.Color.Colorize(payload.Message)
	// var msg = "Diag: " + string(payload.URN) + ": "
	// msg += opts.Color.Colorize(payload.Message)

	// return upToFirstNewLine(opts, msg)
}

func renderStdoutColorEvent(
	payload engine.StdoutEventPayload, opts backend.DisplayOptions) (resource.URN, string) {

	return "", opts.Color.Colorize(payload.Message)
}

func renderSummaryEvent(event engine.SummaryEventPayload, opts backend.DisplayOptions) string {
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

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vinfo%v: %v %v %v\n",
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
				fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("    %v%v %v %v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, opDescription, colors.Reset)))
			}
		}
	}
	if c := changes[deploy.OpSame]; c > 0 {
		fprintfIgnoreError(out, "      %v %v unchanged\n", c, plural("resource", c))
	}

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		if changeCount > 0 {
			fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vUpdate duration: %v%v\n",
				colors.SpecUnimportant, event.Duration, colors.Reset)))
		}
	}

	return out.String()
}

func renderPreludeEvent(event engine.PreludeEventPayload, opts backend.DisplayOptions) string {
	out := &bytes.Buffer{}

	if opts.ShowConfig {
		fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset)))

		var keys []string
		for key := range event.Config {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fprintfIgnoreError(out, "    %v: %v\n", key, event.Config[key])
		}
	}

	action := "Previewing"
	if !event.IsPreview {
		action = "Performing"
	}

	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%v%v changes:%v\n", colors.SpecUnimportant, action, colors.Reset)))

	return out.String()
}

func renderResourceOperationFailedEvent(
	payload engine.ResourceOperationFailedPayload, opts backend.DisplayOptions) string {

	// It's not actually useful or interesting to print out any details about
	// the resource state here, because we always assume that the resource state
	// is unknown if an error occurs.
	//
	// In the future, once we get more fine-grained error messages from providers,
	// we can provide useful diagnostics here.

	return ""
}

func getMetadataSummary(metadata engine.StepEventMetadata, opts backend.DisplayOptions) string {
	out := &bytes.Buffer{}
	summary := engine.GetResourcePropertiesSummary(metadata, 0)
	// details := engine.GetResourcePropertiesDetails(payload.Metadata, indent, payload.Planning, payload.Debug)

	// fprintIgnoreError(out, "Pre: ")
	// fprintIgnoreError(out, payload.Metadata.URN)
	// fprintIgnoreError(out, ": ")
	fprintIgnoreError(out, opts.Color.Colorize(summary))

	// if !opts.Summary {
	// 	fprintIgnoreError(out, opts.Color.Colorize(details))
	// }

	fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))

	return out.String()
}

func renderResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions) (resource.URN, string) {

	seen[payload.Metadata.URN] = payload.Metadata

	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		summary := getMetadataSummary(payload.Metadata, opts)
		// out := &bytes.Buffer{}

		// // indent := engine.GetIndent(payload.Metadata, seen)
		// summary := engine.GetResourcePropertiesSummary(payload.Metadata, 0)
		// // details := engine.GetResourcePropertiesDetails(payload.Metadata, indent, payload.Planning, payload.Debug)

		// // fprintIgnoreError(out, "Pre: ")
		// // fprintIgnoreError(out, payload.Metadata.URN)
		// // fprintIgnoreError(out, ": ")
		// fprintIgnoreError(out, opts.Color.Colorize(summary))

		// // if !opts.Summary {
		// // 	fprintIgnoreError(out, opts.Color.Colorize(details))
		// // }

		// fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))

		// return payload.Metadata.URN, out.String()

		return payload.Metadata.URN, summary
	} else {
		return payload.Metadata.URN, ""
	}

	// return upToFirstNewLine(opts, out.String())
}

func renderResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions) (resource.URN, string) {

	// out := &bytes.Buffer{}

	if shouldShow(payload.Metadata, opts) && !opts.Summary {
		// indent := engine.GetIndent(payload.Metadata, seen)
		// text := engine.GetResourceOutputsPropertiesString(payload.Metadata, payload.Planning, payload.Debug)
		summary := getMetadataSummary(payload.Metadata, opts)
		return payload.Metadata.URN, summary + " - Done!"

		// fprintIgnoreError(out, opts.Color.Colorize(text))
	}

	return payload.Metadata.URN, ""
}

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step engine.StepEventMetadata) bool {
	return isRootURN(step.URN)
}

func isRootURN(urn resource.URN) bool {
	return urn != "" && urn.Type() == resource.RootStackType
}

// shouldShow returns true if a step should show in the output.
func shouldShow(step engine.StepEventMetadata, opts backend.DisplayOptions) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if step.Op == deploy.OpSame {
		// If the op is the same, it is possible that the resource's metadata changed.  In that case, still show it.
		if step.Old.Protect != step.New.Protect {
			return true
		}
		return opts.ShowSames
	} else if step.Op == deploy.OpCreateReplacement || step.Op == deploy.OpDeleteReplaced {
		return opts.ShowReplacementSteps
	} else if step.Op == deploy.OpReplace {
		return !opts.ShowReplacementSteps
	}
	return true
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}

func fprintfIgnoreError(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.IgnoreError(err)
}

func fprintIgnoreError(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	contract.IgnoreError(err)
}
