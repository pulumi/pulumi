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

// Status helps us keep track for a resource as it is worked on by the engine.
type Status struct {
	// The simple short ID we have generated for the resource to present it to the user.
	// Usually similar to the form: aws.Function("name")
	ID string

	// The change that the engine wants apply to that resource.
	Step engine.StepEventMetadata

	// The tick we were on when we created this status.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	Tick int

	// If the engine finished processing this resources.
	Done bool

	// If we failed this operation for any reason.
	Failed bool

	// All the diagnostic events we've heard about this resource.  We'll print the last
	// diagnostic in the status region while a resource is in progress.  At the end we'll
	// print out all diagnostics for a resource.
	DiagEvents []engine.Event
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

// Returns the worst diagnostic we've seen, along with counts of all the diagnostic kinds.  Used to
// produce a diagnostic string to go along with any resource if it has had any issues.
func getDiagnosticInformation(status Status) (
	worstDiag *engine.Event, errorEvents, warningEvents, infoEvents, debugEvents int) {

	errors := 0
	warnings := 0
	infos := 0
	debugs := 0

	var lastError, lastWarning, lastInfo, lastDebug *engine.Event

	for _, ev := range status.DiagEvents {
		payload := ev.Payload.(engine.DiagEventPayload)

		switch payload.Severity {
		case diag.Error:
			errors++
			lastError = &ev
		case diag.Warning:
			warnings++
			lastWarning = &ev
		case diag.Info:
			infos++
			lastInfo = &ev
		case diag.Debug:
			debugs++
			lastDebug = &ev
		}
	}

	if lastError != nil {
		worstDiag = lastError
	} else if lastWarning != nil {
		worstDiag = lastWarning
	} else if lastInfo != nil {
		worstDiag = lastInfo
	} else {
		worstDiag = lastDebug
	}

	return worstDiag, errors, warnings, infos, debugs
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
	maxIDLength int

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
func (display *ProgressDisplay) getMessagePadding(status Status) string {
	extraWhitespace := 0

	// In the terminal we try to align the status messages for each resource.
	// do not bother with this in the non-terminal case.
	if display.isTerminal {
		id := status.ID
		extraWhitespace = display.maxIDLength - len(id)
		contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", display.maxIDLength, id)
	}

	return strings.Repeat(" ", extraWhitespace)
}

// Gets the fully padded message to be shown.  The message will always include the ID of the
// status, then some amount of optional padding, then some amount of msgWithColors, then the
// suffix.  Importantly, if there isn't enough room to display all of that on the terminal, then
// the msg will be truncated to try to make it fit.
func (display *ProgressDisplay) getPaddedMessage(status Status, msgWithColors string, suffix string) string {
	id := status.ID
	padding := display.getMessagePadding(status)

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
		maxMsgLength := display.terminalWidth - len(id) - len(":") - len(padding) - len(suffix) - 1
		if maxMsgLength < 0 {
			maxMsgLength = 0
		}

		msgWithColors = colors.TrimColorizedString(msgWithColors, maxMsgLength)
	}

	return padding + msgWithColors + suffix
}

// Gets the single line summary to show for a resource.  This will include the current state of
// the resource (i.e. "Creating", "Replaced", "Failed", etc.) as well as relevant diagnostic
// information if there is any.
func (display *ProgressDisplay) getUnpaddedStatusSummary(status Status) string {
	if status.Step.Op == "" {
		contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
	}

	worstDiag, errors, warnings, infos, debugs := getDiagnosticInformation(status)

	failed := status.Failed || errors > 0
	msg := display.getMetadataSummary(status.Step, status.Done, failed)

	if errors > 0 {
		msg += fmt.Sprintf(", %v error(s)", errors)
	}

	if warnings > 0 {
		msg += fmt.Sprintf(", %v warning(s)", warnings)
	}

	if infos > 0 {
		msg += fmt.Sprintf(", %v info message(s)", infos)
	}

	if debugs > 0 {
		msg += fmt.Sprintf(", %v debug message(s)", debugs)
	}

	if worstDiag != nil {
		diagMsg := display.renderProgressDiagEvent(*worstDiag)
		if diagMsg != "" {
			msg += ". " + diagMsg
		}
	}

	return msg
}

var ellipsesArray = []string{"", ".", "..", "..."}

func (display *ProgressDisplay) refreshSingleStatusMessage(status Status) {
	unpaddedMsg := display.getUnpaddedStatusSummary(status)
	suffix := ""

	if !status.Done {
		suffix = ellipsesArray[(status.Tick+display.currentTick)%len(ellipsesArray)]
	}

	msg := display.getPaddedMessage(status, unpaddedMsg, suffix)

	display.colorizeAndWriteProgress(progress.Progress{
		ID:     status.ID,
		Action: msg,
	})
}

func (display *ProgressDisplay) refreshAllStatusMessages(includeDone bool) {
	for _, v := range display.eventUrnToStatus {
		if v.Done && !includeDone {
			continue
		}

		display.refreshSingleStatusMessage(v)
	}
}

// Performs all the work at the end once we've heard about the last message from the engine.
// Specifically, this will update the status messages for any resources, and will also then
// print out all final diagnostics. and finally will print out the summary.
func (display *ProgressDisplay) processEndSteps() {
	// Mark all in progress resources as done.
	for k, v := range display.eventUrnToStatus {
		if !v.Done {
			v.Done = true
			display.eventUrnToStatus[k] = v
			display.refreshSingleStatusMessage(v)
		}
	}

	// Print all diagnostics we've seen.

	for _, status := range display.eventUrnToStatus {
		if len(status.DiagEvents) > 0 {
			wroteHeader := false
			for _, v := range status.DiagEvents {
				msg := display.renderProgressDiagEvent(v)
				if msg != "" {
					if !wroteHeader {
						wroteHeader = true
						display.colorizeAndWriteProgress(progress.Progress{Message: " "})
						display.colorizeAndWriteProgress(progress.Progress{ID: status.ID, Message: "Diagnostics"})
					}

					display.colorizeAndWriteProgress(progress.Progress{Message: "  " + msg})
				}
			}
		}
	}

	// print the summary
	if display.summaryEvent != nil {
		msg := renderSummaryEvent(display.summaryEvent.Payload.(engine.SummaryEventPayload), display.opts)
		display.colorizeAndWriteProgress(progress.Progress{Message: msg})
	}
}

func (display *ProgressDisplay) processTick() {
	// Got a tick.  Update all the in-progress resources.
	display.currentTick++

	currentTerminalWidth, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
	if currentTerminalWidth != display.terminalWidth {
		// terminal width changed.  Update our output.
		display.terminalWidth = currentTerminalWidth
		display.refreshAllStatusMessages(true /*includeDone*/)
	} else {
		display.refreshAllStatusMessages(false /*includeDone*/)
	}
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
		display.colorizeAndWriteProgress(progress.Progress{
			Message: renderPreludeEvent(payload, display.opts),
		})
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

	refreshAllStatuses := false
	status, has := display.eventUrnToStatus[eventUrn]
	if !has {
		// first time we're hearing about this resource.  Create an initial nearly-empty
		// status for it, assigning it a nice short ID.
		status = Status{Tick: display.currentTick}
		status.Step.Op = deploy.OpSame
		status.ID = display.makeID(eventUrn)

		if display.isTerminal {
			// in the terminal we want to align the status portions of messages. If we
			// heard about a resource with a longer id, go and update all in-flight and
			// finished resources so that their statuses get aligned.
			if len(status.ID) > display.maxIDLength {
				display.maxIDLength = len(status.ID)
				refreshAllStatuses = true
			}
		}
	}

	if event.Type == engine.ResourcePreEvent {
		status.Step = event.Payload.(engine.ResourcePreEventPayload).Metadata
		if status.Step.Op == "" {
			contract.Failf("Got empty op for %s", event.Type)
		}
	} else if event.Type == engine.ResourceOutputsEvent {
		// transition the status to done.
		if !isRootURN(eventUrn) {
			status.Done = true
		}
	} else if event.Type == engine.ResourceOperationFailed {
		status.Done = true
		status.Failed = true
	} else if event.Type == engine.DiagEvent {
		// also record this diagnostic so we print it at the end.
		status.DiagEvents = append(status.DiagEvents, event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	// Ensure that this updated status is recorded.
	display.eventUrnToStatus[eventUrn] = status

	// refresh the progress information for this resource.  (or update all resources if
	// we need to realign everything)
	if refreshAllStatuses {
		display.refreshAllStatusMessages(true /*includeDone*/)
	} else {
		display.refreshSingleStatusMessage(status)
	}
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
	return payload.Message
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
			writeString(out, ". Changes:")

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

	if isRootStack(step) {
		if failed {
			return makeError("Failed")
		}

		return "Completed"
	}

	if display.isPreview && !isRootStack(step) {
		return display.getStepInProgressDescription(step)
	}

	op := step.Op

	getDescription := func() string {
		if failed {
			switch op {
			case deploy.OpSame:
				return "Failed"
			case deploy.OpCreate, deploy.OpCreateReplacement:
				return "Creating failed"
			case deploy.OpUpdate:
				return "Updating failed"
			case deploy.OpDelete, deploy.OpDeleteReplaced:
				return "Deleting failed"
			case deploy.OpReplace:
				return "Replacing failed"
			}
		} else {
			switch op {
			case deploy.OpSame:
				return "Unchanged"
			case deploy.OpCreate:
				return "Created"
			case deploy.OpUpdate:
				return "Updated"
			case deploy.OpDelete:
				return "Deleted"
			case deploy.OpReplace:
				return "Replaced"
			case deploy.OpCreateReplacement:
				return "Created for replacement"
			case deploy.OpDeleteReplaced:
				return "Deleted for replacement"
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

func (display *ProgressDisplay) getStepInProgressDescription(step engine.StepEventMetadata) string {
	if isRootStack(step) {
		return "Running"
	}

	op := step.Op

	getDescription := func() string {
		if display.isPreview {
			switch op {
			case deploy.OpSame:
				return "Would not change"
			case deploy.OpCreate:
				return "Would create"
			case deploy.OpUpdate:
				return "Would update"
			case deploy.OpDelete:
				return "Would delete"
			case deploy.OpReplace:
				return "Would replace"
			case deploy.OpCreateReplacement:
				return "Would creating for replacement"
			case deploy.OpDeleteReplaced:
				return "Would delete for replacement"
			}
		} else {
			switch op {
			case deploy.OpSame:
				return "Unchanged"
			case deploy.OpCreate:
				return "Creating"
			case deploy.OpUpdate:
				return "Updating"
			case deploy.OpDelete:
				return "Deleting"
			case deploy.OpReplace:
				return "Replacing"
			case deploy.OpCreateReplacement:
				return "Creating for replacement"
			case deploy.OpDeleteReplaced:
				return "Deleting for replacement"
			}
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
