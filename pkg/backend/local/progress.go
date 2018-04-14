// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

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
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"golang.org/x/crypto/ssh/terminal"
)

type DiagInfo struct {
	ErrorCount, WarningCount, InfoCount, DebugCount int

	// The last event of each severity kind.  We'll print out the most significant of these next
	// to a resource while it is in progress.
	LastError, LastWarning, LastInfoError, LastInfo, LastDebug *engine.Event

	// All the diagnostic events we've heard about this resource.  We'll print the last
	// diagnostic in the status region while a resource is in progress.  At the end we'll
	// print out all diagnostics for a resource.
	DiagEvents []engine.Event
}

type Status interface {
	// The simple short ID we have generated for the resource to present it to the user.
	// Usually similar to the form: aws.Function("name")
	ID() string

	// The change that the engine wants apply to that resource.
	Step() engine.StepEventMetadata
	SetStep(step engine.StepEventMetadata)

	// The tick we were on when we created this status.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	Tick() int

	Done() bool
	SetDone()

	Failed() bool
	SetFailed()

	DiagInfo() *DiagInfo
	RecordDiagEvent(diagEvent engine.Event)

	Columns() []string
}

// Status helps us keep track for a resource as it is worked on by the engine.
type statusData struct {
	_Display *ProgressDisplay

	// The simple short ID we have generated for the resource to present it to the user.
	// Usually similar to the form: aws.Function("name")
	_ID string

	// The change that the engine wants apply to that resource.
	_Step engine.StepEventMetadata

	// The tick we were on when we created this status.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	_Tick int

	// If the engine finished processing this resources.
	_Done bool

	// If we failed this operation for any reason.
	_Failed bool

	_DiagInfo *DiagInfo

	_Columns []string
}

func (data *statusData) ID() string {
	return data._ID
}

func (data *statusData) Step() engine.StepEventMetadata {
	return data._Step
}

func (data *statusData) SetStep(step engine.StepEventMetadata) {
	data._Step = step
	data.ClearCachedData()
}

func (data *statusData) Tick() int {
	return data._Tick
}

func (data *statusData) Done() bool {
	return data._Done
}

func (data *statusData) SetDone() {
	data._Done = true
	data.ClearCachedData()
}

func (data *statusData) Failed() bool {
	return data._Failed
}

func (data *statusData) SetFailed() {
	data._Failed = true
	data.ClearCachedData()
}

func (data *statusData) DiagInfo() *DiagInfo {
	return data._DiagInfo
}

func (data *statusData) RecordDiagEvent(diagEvent engine.Event) {
	combineDiagnosticInfo(data._DiagInfo, diagEvent)
	data.ClearCachedData()
}

func (data *statusData) ClearCachedData() {
	data._Columns = []string{}
}

func (data *statusData) Columns() []string {
	if len(data._Columns) == 0 {
		columns := make([]string, 2)

		columns[0] = data._ID
		columns[1] = data._Display.getUnpaddedStatusSummary(data)

		data._Columns = columns
	}

	return data._Columns
}

var (
	// simple regex to take our names like "aws:function:Function" and convert to
	// "aws:Function"
	typeNameRegex = regexp.MustCompile("^(.*):(.*):(.*)$")
)

func simplifyTypeName(typ tokens.Type) string {
	typeString := string(typ)
	return typeNameRegex.ReplaceAllString(typeString, "$1:$3")
}

// getEventUrn returns the resource URN associated with an event, or the empty URN if this is not an
// event that has a URN.  If this is also a 'step' event, then this will return the step metadata as
// well.
func getEventUrnAndMetadata(event engine.Event) (resource.URN, *engine.StepEventMetadata) {
	if event.Type == engine.ResourcePreEvent {
		payload := event.Payload.(engine.ResourcePreEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	} else if event.Type == engine.ResourceOutputsEvent {
		payload := event.Payload.(engine.ResourceOutputsEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	} else if event.Type == engine.ResourceOperationFailed {
		payload := event.Payload.(engine.ResourceOperationFailedPayload)
		return payload.Metadata.URN, &payload.Metadata
	} else if event.Type == engine.DiagEvent {
		return event.Payload.(engine.DiagEventPayload).URN, nil
	}

	return "", nil
}

// Converts the colorization tags in a progress message and then actually writes the progress
// message to the output stream.  This should be the only place in this file where we actually
// process colorization tags.
func (display *ProgressDisplay) colorizeAndWriteProgress(progress progress.Progress) {
	if progress.Message != "" {
		progress.Message = display.opts.Color.Colorize(progress.Message)
	}

	if progress.Action != "" {
		progress.Action = display.opts.Color.Colorize(progress.Action)
	}

	err := display.progressOutput.WriteProgress(progress)
	contract.IgnoreError(err)
}

func (display *ProgressDisplay) writeSimpleMessage(msg string) {
	display.colorizeAndWriteProgress(progress.Progress{Message: msg})
}

func (display *ProgressDisplay) writeBlankLine() {
	display.writeSimpleMessage(" ")
}

// Returns the worst diagnostic we've seen.  Used to produce a diagnostic string to go along with
// any resource if it has had any issues.
func getWorstDiagnostic(status Status) *engine.Event {
	diagInfo := status.DiagInfo()
	if diagInfo.LastError != nil {
		return diagInfo.LastError
	}

	if diagInfo.LastWarning != nil {
		return diagInfo.LastWarning
	}

	if diagInfo.LastInfoError != nil {
		return diagInfo.LastInfoError
	}

	if diagInfo.LastInfo != nil {
		return diagInfo.LastInfo
	}

	return diagInfo.LastDebug
}

type ProgressDisplay struct {
	opts           backend.DisplayOptions
	progressOutput progress.Output

	// Whether or not we're previewing.  We don't know what we are actually doing until
	// we get the initial 'prelude' event.
	//
	// this flag is only used to adjust how we describe what's going on to the user.
	// i.e. if we're previewing we say things like "Would update" instead of "Updating".
	isPreview bool

	// The urn of the stack.
	stackUrn resource.URN

	// The summary event from the engine.  If we get this, we'll print this after all
	// normal resource events are heard.  That way we don't interfere with all the progress
	// messages we're outputting for them.
	summaryEvent *engine.Event

	// The length of the largest ID we've seen.  We use this so we can align status messages per
	// resource.  i.e. status messages for shorter IDs will get passed with spaces so that
	// everything aligns.
	maxColumnLengths []int

	// What tick we're currently on.  Used to determine the number of ellipses to concat to
	// a status message to help indicate that things are still working.
	currentTick int

	// A mapping from each resource URN we are told about to its current status.
	eventUrnToStatus map[resource.URN]Status

	// Remember if we're a terminal or not.  In a terminal we get a little bit fancier.
	// For example, we'll go back and update previous status messages to make sure things
	// align.  We don't need to do that in non-terminal situations.
	isTerminal bool

	// The width of the terminal.  Used so we can trim resource messages that are too long.
	terminalWidth int

	// Maps used so we can generate short IDs for resource urns.
	urnToID map[resource.URN]string
	idToUrn map[string]resource.URN

	// If all progress messages are done and we can print out the final display.
	Done bool
}

// DisplayProgressEvents displays the engine events with docker's progress view.
func DisplayProgressEvents(
	action string, events <-chan engine.Event,
	done chan<- bool, opts backend.DisplayOptions) {

	// Create a ticker that will update all our status messages once a second.  Any
	// in-flight resources will get a varying .  ..  ... ticker appended to them to
	// let the user know what is still being worked on.
	_, ticker := cmdutil.NewSpinnerAndTicker(
		fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action),
		nil, 1 /*timesPerSecond*/)

	// The streams we used to connect to docker's progress system.  We push progress messages into
	// progressOutput.  It pushes messages into pipeWriter.  Those are then read below in
	// DisplayJSONMessagesToStream.
	pipeReader, pipeWriter := io.Pipe()
	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(pipeWriter, false)

	display := &ProgressDisplay{
		opts:             opts,
		progressOutput:   progressOutput,
		eventUrnToStatus: make(map[resource.URN]Status),
		urnToID:          make(map[resource.URN]string),
		idToUrn:          make(map[string]resource.URN),
	}

	display.initializeTermInfo()

	go func() {
		display.processEvents(ticker, events)

		// no more progress events from this point on.  By closing the pipe, this will then cause
		// DisplayJSONMessagesToStream to finish once it processes the last message is receives from
		// pipeReader, causing DisplayEvents to finally complete.
		err := pipeWriter.Close()
		contract.IgnoreError(err)
	}()

	// Call into Docker to actually suck the progress messages out of pipeReader and display
	// them to the console.
	_, stdout, _ := term.StdStreams()
	err := jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
	contract.IgnoreError(err)

	ticker.Stop()

	// let our caller know we're done.
	done <- true
}

func (display *ProgressDisplay) initializeTermInfo() {
	_, stdout, _ := term.StdStreams()
	_, isTerminal := term.GetFdInfo(stdout)

	terminalWidth, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	contract.IgnoreError(err)

	display.isTerminal = isTerminal
	display.terminalWidth = terminalWidth
}

func (display *ProgressDisplay) makeID(urn resource.URN) string {
	makeSingleID := func(suffix int) string {
		var id string
		if urn == "" {
			id = "global"
		} else {
			id = simplifyTypeName(urn.Type()) + "(\"" + string(urn.Name()) + "\")"
		}

		if suffix > 0 {
			id += fmt.Sprintf("-%v", suffix)
		}

		return id
	}

	if id, has := display.urnToID[urn]; !has {
		for i := 0; ; i++ {
			id = makeSingleID(i)

			if _, has = display.idToUrn[id]; !has {
				display.urnToID[urn] = id
				display.idToUrn[id] = urn

				return id
			}
		}
	} else {
		return id
	}
}

// Gets the padding necessary to prepend to a message in order to keep it aligned in the
// terminal.
func (display *ProgressDisplay) getMessagePadding(columns []string, columnIndex int) string {
	extraWhitespace := 1

	// In the terminal we try to align the status messages for each resource.
	// do not bother with this in the non-terminal case.
	if display.isTerminal {
		column := columns[columnIndex]
		maxIDLength := display.maxColumnLengths[columnIndex]
		extraWhitespace = maxIDLength - len(column)
		contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxIDLength, column)
	}

	return strings.Repeat(" ", extraWhitespace)
}

// Gets the fully padded message to be shown.  The message will always include the ID of the
// status, then some amount of optional padding, then some amount of msgWithColors, then the
// suffix.  Importantly, if there isn't enough room to display all of that on the terminal, then
// the msg will be truncated to try to make it fit.
func (display *ProgressDisplay) getPaddedMessage(columns []string, suffix string) string {

	msgWithColors := ""
	for i := 1; i < len(columns); i++ {
		padding := display.getMessagePadding(columns, i-1)
		column := padding + columns[i]
		msgWithColors += column
	}

	// In the terminal, only include the first line of the message
	if display.isTerminal {
		newLineIndex := strings.Index(msgWithColors, "\n")
		if newLineIndex >= 0 {
			msgWithColors = msgWithColors[0:newLineIndex]
		}

		// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
		// msgWithColors having the color code information embedded with it.  So we need to get
		// the right substring of it, assuming that embedded colors are just markup and do not
		// actually contribute to the length
		id := columns[0]
		maxMsgLength := display.terminalWidth - len(id) - len(":") - len(suffix) - 1
		if maxMsgLength < 0 {
			maxMsgLength = 0
		}

		msgWithColors = colors.TrimColorizedString(msgWithColors, maxMsgLength)
	}

	return msgWithColors + suffix
}

// Gets the single line summary to show for a resource.  This will include the current state of
// the resource (i.e. "Creating", "Replaced", "Failed", etc.) as well as relevant diagnostic
// information if there is any.
func (display *ProgressDisplay) getUnpaddedStatusSummary(status Status) string {
	if status.Step().Op == "" {
		contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
	}

	worstDiag := getWorstDiagnostic(status)

	diagInfo := status.DiagInfo()
	failed := status.Failed() || diagInfo.ErrorCount > 0
	msg := display.getMetadataSummary(status.Step(), status.Done(), failed)

	if diagInfo.ErrorCount == 1 {
		msg += ", 1 error"
	} else if diagInfo.ErrorCount > 1 {
		msg += fmt.Sprintf(", %v errors", diagInfo.ErrorCount)
	}

	if diagInfo.WarningCount == 1 {
		msg += ", 1 warning"
	} else if diagInfo.WarningCount > 1 {
		msg += fmt.Sprintf(", %v warnings", diagInfo.WarningCount)
	}

	if diagInfo.InfoCount == 1 {
		msg += ", 1 info message"
	} else if diagInfo.InfoCount > 1 {
		msg += fmt.Sprintf(", %v info messages", diagInfo.InfoCount)
	}

	if diagInfo.DebugCount == 1 {
		msg += ", 1 debug message"
	} else if diagInfo.ErrorCount > 1 {
		msg += fmt.Sprintf(", %v debug messages", diagInfo.DebugCount)
	}

	// If we're not totally done, also print out the worst diagnostic next to the status message.
	// This is helpful for long running tasks to know what's going on.  However, once done, we print
	// the diagnostics at the bottom, so we don't need to show this.
	if worstDiag != nil && !display.Done {
		diagMsg := display.renderProgressDiagEvent(*worstDiag)
		if diagMsg != "" {
			msg += ". " + diagMsg
		}
	}

	return msg
}

var ellipsesArray = []string{"", ".", "..", "..."}

func (display *ProgressDisplay) refreshSingleStatusMessage(status Status) {
	columns := status.Columns()

	// unpaddedMsg := display.getUnpaddedStatusSummary(status)
	suffix := ""

	if !status.Done() {
		suffix = ellipsesArray[(status.Tick()+display.currentTick)%len(ellipsesArray)]
	}

	msg := display.getPaddedMessage(columns, suffix)

	display.colorizeAndWriteProgress(progress.Progress{
		ID:     columns[0],
		Action: msg,
	})
}

// Ensure our stored dimension info is up to date.  Returns 'true' if the stored dimension info is
// updated.
func (display *ProgressDisplay) updateDimensions() bool {
	updated := false

	// don't do any refreshing if we're not in a terminal
	if display.isTerminal {
		currentTerminalWidth, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
		if currentTerminalWidth != display.terminalWidth {
			// terminal width changed.  Refresh everything
			display.terminalWidth = currentTerminalWidth
			updated = true
		}

		for _, status := range display.eventUrnToStatus {
			columns := status.Columns()

			if len(display.maxColumnLengths) == 0 {
				display.maxColumnLengths = make([]int, len(columns))
			}

			for i, column := range columns {
				if len(column) > display.maxColumnLengths[i] {
					display.maxColumnLengths[i] = len(column)
					updated = true
				}
			}
		}
	}

	return updated
}

func (display *ProgressDisplay) refreshAllIfInTerminal() {
	if display.isTerminal {
		// make sure our stored dimension info is up to date
		display.updateDimensions()

		for _, v := range display.eventUrnToStatus {
			display.refreshSingleStatusMessage(v)
		}
	}
}

// Performs all the work at the end once we've heard about the last message from the engine.
// Specifically, this will update the status messages for any resources, and will also then
// print out all final diagnostics. and finally will print out the summary.
func (display *ProgressDisplay) processEndSteps() {
	display.Done = true

	// Mark all in progress resources as done.
	for _, v := range display.eventUrnToStatus {
		if !v.Done() {
			v.SetDone()
			display.refreshSingleStatusMessage(v)
		}
	}

	// Now refresh everything.  this ensures that we go back and remove things like the diagnostic
	// messages from a status message (since we're going to print them all) below.  Note, this will
	// only do something in a terminal.  This i what we want, because if we're not in a terminal we
	// don't really want to reprint any finished items we've already printed.
	display.refreshAllIfInTerminal()

	// Print all diagnostics we've seen.

	wroteDiagnosticHeader := false

	for _, status := range display.eventUrnToStatus {
		if len(status.DiagInfo().DiagEvents) > 0 {
			wroteResourceHeader := false
			for _, v := range status.DiagInfo().DiagEvents {
				msg := display.renderProgressDiagEvent(v)

				lines := strings.Split(msg, "\n")

				// Trim off any trailing blank lines in the message.
				for len(lines) > 0 {
					lastLine := lines[len(lines)-1]
					if strings.TrimSpace(colors.Never.Colorize(lastLine)) == "" {
						lines = lines[0 : len(lines)-1]
					} else {
						break
					}
				}

				if len(lines) == 0 {
					continue
				}

				if !wroteDiagnosticHeader {
					wroteDiagnosticHeader = true
					display.writeBlankLine()
					display.writeSimpleMessage("Diagnostics:")
				}

				if !wroteResourceHeader {
					wroteResourceHeader = true
					display.writeSimpleMessage("  " + status.ID() + ":")
				}

				for _, line := range lines {
					line = strings.TrimRightFunc(line, unicode.IsSpace)
					display.writeSimpleMessage("    " + line)
				}

				display.writeBlankLine()
			}
		}
	}

	// print the summary
	if display.summaryEvent != nil {
		msg := renderSummaryEvent(display.summaryEvent.Payload.(engine.SummaryEventPayload), display.opts)

		display.writeBlankLine()
		display.writeSimpleMessage(msg)
	}
}

func (display *ProgressDisplay) processTick() {
	// Got a tick.  Update all  resources if we're in a terminal.  If we're not, then this won't do
	// anything.
	display.currentTick++

	display.refreshAllIfInTerminal()
}

func (display *ProgressDisplay) processNormalEvent(event engine.Event) {
	eventUrn, metadata := getEventUrnAndMetadata(event)
	if isRootURN(eventUrn) {
		display.stackUrn = eventUrn
	}

	if eventUrn == "" {
		// if the event doesn't have any URN associated with it, just associate
		// it with the stack.
		eventUrn = display.stackUrn
	}

	switch event.Type {
	case engine.PreludeEvent:
		// A prelude event can just be printed out directly to the console.
		// Note: we should probably make sure we don't get any prelude events
		// once we start hearing about actual resource events.

		payload := event.Payload.(engine.PreludeEventPayload)
		display.isPreview = payload.IsPreview
		display.writeSimpleMessage(renderPreludeEvent(payload, display.opts))
		return
	case engine.SummaryEvent:
		// keep track of the summar event so that we can display it after all other
		// resource-related events we receive.
		display.summaryEvent = &event
		return
	case engine.DiagEvent:
		msg := display.renderProgressDiagEvent(event)
		if msg == "" {
			return
		}
	}

	// Don't bother showing certain events (for example, things that are unchanged). However
	// always show the root 'stack' resource so we can indicate that it's still running, and
	// also so we have something to attach unparented diagnostic events to.
	if metadata != nil && !shouldShow(*metadata, display.opts) && !isRootURN(eventUrn) {
		return
	}

	// At this point, all events should relate to resources.

	status, has := display.eventUrnToStatus[eventUrn]
	if !has {
		// first time we're hearing about this resource.  Create an initial nearly-empty
		// status for it, assigning it a nice short ID.
		status = &statusData{
			_Display:  display,
			_ID:       display.makeID(eventUrn),
			_Tick:     display.currentTick,
			_DiagInfo: &DiagInfo{},
			_Step:     engine.StepEventMetadata{Op: deploy.OpSame},
		}

		display.eventUrnToStatus[eventUrn] = status
	}

	if event.Type == engine.ResourcePreEvent {
		status.SetStep(event.Payload.(engine.ResourcePreEventPayload).Metadata)
		if status.Step().Op == "" {
			contract.Failf("Got empty op for %s", event.Type)
		}
	} else if event.Type == engine.ResourceOutputsEvent {
		// transition the status to done.
		if !isRootURN(eventUrn) {
			status.SetDone()
		}
	} else if event.Type == engine.ResourceOperationFailed {
		status.SetDone()
		status.SetFailed()
	} else if event.Type == engine.DiagEvent {
		// also record this diagnostic so we print it at the end.
		status.RecordDiagEvent(event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	// See if this new status information causes us to have to refresh everything.  Otherwise,
	// just refresh the info for that single status message.
	if display.updateDimensions() {
		contract.Assertf(display.isTerminal, "we should only need to refresh if we're in a terminal")
		display.refreshAllIfInTerminal()
	} else {
		display.refreshSingleStatusMessage(status)
	}
}

func combineDiagnosticInfo(diagInfo *DiagInfo, event engine.Event) {
	payload := event.Payload.(engine.DiagEventPayload)

	switch payload.Severity {
	case diag.Error:
		diagInfo.ErrorCount++
		diagInfo.LastError = &event
	case diag.Warning:
		diagInfo.WarningCount++
		diagInfo.LastWarning = &event
	case diag.Infoerr:
		diagInfo.InfoCount++
		diagInfo.LastInfoError = &event
	case diag.Info:
		diagInfo.InfoCount++
		diagInfo.LastInfo = &event
	case diag.Debug:
		diagInfo.DebugCount++
		diagInfo.LastDebug = &event
	}

	diagInfo.DiagEvents = append(diagInfo.DiagEvents, event)
}

func (display *ProgressDisplay) processEvents(ticker *time.Ticker, events <-chan engine.Event) {
	// Main processing loop.  The purpose of this func is to read in events from the engine
	// and translate them into Status objects and progress messages to be presented to the
	// command line.
	for {
		select {
		case <-ticker.C:
			display.processTick()

		case event := <-events:
			if event.Type == "" || event.Type == engine.CancelEvent {
				// Engine finished sending events.  Do all the final processing and return
				// from this local func.  This will print out things like full diagnostic
				// events, as well as the summary event from the engine.
				display.processEndSteps()
				return
			}

			display.processNormalEvent(event)
		}
	}
}

func (display *ProgressDisplay) renderProgressDiagEvent(event engine.Event) string {
	payload := event.Payload.(engine.DiagEventPayload)

	if payload.Severity == diag.Debug && !display.opts.Debug {
		return ""
	}
	return strings.TrimRightFunc(payload.Message, unicode.IsSpace)
}

func (display *ProgressDisplay) getMetadataSummary(
	step engine.StepEventMetadata, done bool, failed bool) string {

	out := &bytes.Buffer{}

	if done {
		writeString(out, display.getStepDoneDescription(step, failed))
	} else {
		writeString(out, display.getStepInProgressDescription(step))
	}
	writeString(out, colors.Reset)

	if step.Old != nil && step.New != nil && step.Old.Inputs != nil && step.New.Inputs != nil {
		diff := step.Old.Inputs.Diff(step.New.Inputs)

		if diff != nil {
			writeString(out, "  changes:")

			updates := make(resource.PropertyMap)
			for k := range diff.Updates {
				updates[k] = resource.PropertyValue{}
			}

			writePropertyKeys(out, diff.Adds, deploy.OpCreate)
			writePropertyKeys(out, diff.Deletes, deploy.OpDelete)
			writePropertyKeys(out, updates, deploy.OpReplace)
		}
	}

	fprintIgnoreError(out, colors.Reset)

	return out.String()
}

func (display *ProgressDisplay) getStepDoneDescription(step engine.StepEventMetadata, failed bool) string {
	makeError := func(v string) string {
		return colors.SpecError + "**" + v + "**" + colors.Reset
	}

	if display.isPreview {
		// During a preview, when we transition to done, we still just print the same thing we
		// did while running the step.
		return step.Op.Prefix() + getPreviewText(step.Op) + colors.Reset
	}

	// most of the time a stack is unchanged.  in that case we just show it as "running->done"
	if isRootStack(step) && step.Op == deploy.OpSame {
		return "done"
	}

	op := step.Op

	getDescription := func() string {
		if failed {
			switch op {
			case deploy.OpSame:
				return "failed"
			case deploy.OpCreate, deploy.OpCreateReplacement:
				return "creating failed"
			case deploy.OpUpdate:
				return "updating failed"
			case deploy.OpDelete, deploy.OpDeleteReplaced:
				return "deleting failed"
			case deploy.OpReplace:
				return "replacing failed"
			}
		} else {
			switch op {
			case deploy.OpSame:
				return "unchanged"
			case deploy.OpCreate:
				return "created"
			case deploy.OpUpdate:
				return "updated"
			case deploy.OpDelete:
				return "deleted"
			case deploy.OpReplace:
				return "replaced"
			case deploy.OpCreateReplacement:
				return "created for replacement"
			case deploy.OpDeleteReplaced:
				return "deleted for replacement"
			}
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}

	if failed {
		return makeError(getDescription())
	}

	return op.Prefix() + getDescription() + colors.Reset
}

func getPreviewText(op deploy.StepOp) string {
	switch op {
	case deploy.OpSame:
		return "[no change]"
	case deploy.OpCreate:
		return "[create]"
	case deploy.OpUpdate:
		return "[update]"
	case deploy.OpDelete:
		return "[delete]"
	case deploy.OpReplace:
		return "[replace]"
	case deploy.OpCreateReplacement:
		return "[create for replacement]"
	case deploy.OpDeleteReplaced:
		return "[delete for replacement]"
	}

	contract.Failf("Unrecognized resource step op: %v", op)
	return ""
}

func (display *ProgressDisplay) getStepInProgressDescription(step engine.StepEventMetadata) string {
	op := step.Op

	if isRootStack(step) && op == deploy.OpSame {
		// most of the time a stack is unchanged.  in that case we just show it as "running->done".
		// otherwise, we show what is actually happening to it.
		return "running"
	}

	getDescription := func() string {
		if display.isPreview {
			return getPreviewText(op)
		}

		switch op {
		case deploy.OpSame:
			return "unchanged"
		case deploy.OpCreate:
			return "creating"
		case deploy.OpUpdate:
			return "updating"
		case deploy.OpDelete:
			return "deleting"
		case deploy.OpReplace:
			return "replacing"
		case deploy.OpCreateReplacement:
			return "creating for replacement"
		case deploy.OpDeleteReplaced:
			return "deleting for replacement"
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
	return op.Prefix() + getDescription() + colors.Reset
}

func writePropertyKeys(b *bytes.Buffer, propMap resource.PropertyMap, op deploy.StepOp) {
	if len(propMap) > 0 {
		writeString(b, " ")
		writeString(b, op.Prefix())

		index := 0
		for k := range propMap {
			if index != 0 {
				writeString(b, ",")
			}
			writeString(b, string(k))
			index++
		}

		writeString(b, colors.Reset)
	}
}

func writeString(b *bytes.Buffer, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}
