// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
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
// event that has a URN.
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

func colorizeAndWriteProgress(opts backend.DisplayOptions, output progress.Output, progress progress.Progress) {
	if progress.Message != "" {
		progress.Message = opts.Color.Colorize(progress.Message)
	}

	if progress.Action != "" {
		progress.Action = opts.Color.Colorize(progress.Action)
	}

	output.WriteProgress(progress)
}

func getDiagnosticInformation(status Status) (
	worstDiag *engine.Event, errorEvents, warningEvents, infoEvents, debugEvents int) {

	errorEvents = 0
	warningEvents = 0
	infoEvents = 0
	debugEvents = 0

	var lastError, lastInfoError, lastWarning, lastInfo, lastDebug *engine.Event

	for _, ev := range status.DiagEvents {
		payload := ev.Payload.(engine.DiagEventPayload)

		switch payload.Severity {
		case diag.Error:
			errorEvents++
			lastError = &ev
		case diag.Infoerr:
			errorEvents++
			lastInfoError = &ev
		case diag.Warning:
			warningEvents++
			lastWarning = &ev
		case diag.Info:
			infoEvents++
			lastInfo = &ev
		case diag.Debug:
			debugEvents++
			lastDebug = &ev
		}
	}

	if lastError != nil {
		worstDiag = lastError
		return
	}

	if lastInfoError != nil {
		worstDiag = lastInfoError
		return
	}

	if lastWarning != nil {
		worstDiag = lastWarning
		return
	}

	if lastInfo != nil {
		worstDiag = lastInfo
		return
	}

	worstDiag = lastDebug
	return
}

// DisplayProgressEvents displays the engine events with docker's progress view.
func DisplayProgressEvents(
	action string, events <-chan engine.Event,
	done chan<- bool, opts backend.DisplayOptions) {

	// Create a ticker that will update all our status messages once a second.  Any
	// in-flight resources will get a varying .  ..  ... ticker appended to them to
	// let the user know what is still being worked on.
	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	_, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, 1 /*timesPerSecond*/)

	// Whether or not we're previewing.  We don't know what we are actually doing until
	// we get the initial 'prelude' event.
	//
	// this flag is only used to adjust how we describe what's going on to the user.
	// i.e. if we're previewing we say things like "Would update" instead of "Updating".
	isPreview := false

	// The urn of the stack.
	var stackUrn resource.URN

	// The summary event from the engine.  If we get this, we'll print this after all
	// normal resource events are heard.  That way we don't interfere with all the progress
	// messages we're outputting for them.
	var summaryEvent *engine.Event

	// The length of the largest ID we've seen.  We use this so we can align status messages per
	// resource.  i.e. status messages for shorter IDs will get passed with spaces so that
	// everything aligns.
	maxIDLength := 0

	// What tick we're currently on.  Used to determine the number of ellipses to concat to
	// a status message to help indicate that things are still working.
	currentTick := 0

	// A mapping from each resource URN we are told about to its current status.
	eventUrnToStatus := make(map[resource.URN]Status)

	// As we receive information for the engine, we will convert them into Status objects
	// that we track.  In turn, every time we update our status (or our ticker fires) we'll
	// update the "progress channel".  This progress chanel is what the Docker cli listens
	// to which it then updates the actual CLI with.
	_, stdout, _ := term.StdStreams()

	// Remember if we're a terminal or not.  In a terminal we get a little bit fancier.
	// For example, we'll go back and update previous status messages to make sure things
	// align.  We don't need to do that in non-terminal situations.
	_, isTerminal := term.GetFdInfo(stdout)

	terminalWidth, _, _ := terminal.GetSize(int(os.Stdout.Fd()))

	pipeReader, pipeWriter := io.Pipe()

	// Channel where we actually push our raw progress messages into.  These will be then
	// be converted by the docker pipeline into the messages printed to the terminal.
	// progressChan := make(chan progress.Progress, 100)

	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(pipeWriter, false)

	// We want to present a trim name to users for any URN.  These maps, and the helper function
	// below are used for that.
	urnToID := make(map[resource.URN]string)
	idToUrn := make(map[string]resource.URN)

	makeID := func(urn resource.URN) string {
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

		if id, has := urnToID[urn]; !has {
			for i := 0; ; i++ {
				id = makeSingleID(i)

				if _, has = idToUrn[id]; !has {
					urnToID[urn] = id
					idToUrn[id] = urn

					return id
				}
			}
		} else {
			return id
		}
	}

	getMessagePadding := func(id string) string {
		extraWhitespace := 0

		// In the terminal we try to align the status messages for each resource.
		// do not bother with this in the non-terminal case.
		if isTerminal {
			extraWhitespace = maxIDLength - len(id)
			contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxIDLength, id)
		}

		return strings.Repeat(" ", extraWhitespace)
	}

	getPaddedMessage := func(status Status, msgWithColors string, suffix string) string {
		id := status.ID
		padding := getMessagePadding(id)

		// In the terminal, only include the first line of the message
		if isTerminal {
			newLineIndex := strings.Index(msgWithColors, "\n")
			if newLineIndex >= 0 {
				msgWithColors = msgWithColors[0:newLineIndex]
			}
		}

		if isTerminal {
			// we don't want to go past the end of the terminal.  Note: this is made complex due to
			// msgWithColors having the color code information embedded with it.  So we need to get
			// the right substring of it, assuming that embedded colors are just markup and do not
			// actually contribute to the length
			maxMsgLength := terminalWidth - len(id) - len(":") - len(padding) - len(suffix) - 1
			if maxMsgLength < 0 {
				maxMsgLength = 0
			}

			msgWithColors = colors.TrimColorizedString(msgWithColors, maxMsgLength)
		}

		return padding + msgWithColors + suffix
	}

	getSummaryAndDiagnosticMessage := func(status Status) string {
		if status.Step.Op == "" {
			contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
		}

		worstDiag, errors, warnings, infos, debugs := getDiagnosticInformation(status)

		failed := status.Failed || errors > 0
		msg := getMetadataSummary(status.Step, opts, isPreview, status.Done, failed)

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
			diagMsg := renderProgressDiagEvent(*worstDiag, opts)
			if diagMsg != "" {
				msg += ". " + diagMsg
			}
		}

		return msg
	}

	ellipsesArray := []string{"", ".", "..", "..."}
	createInProgressMessage := func(status Status) string {
		msg := getSummaryAndDiagnosticMessage(status)
		ellipses := ellipsesArray[(status.Tick+currentTick)%len(ellipsesArray)]

		return getPaddedMessage(status, msg, ellipses)
	}

	createDoneMessage := func(status Status, isPreview bool) string {
		if status.Step.Op == "" {
			contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
		}

		msg := getSummaryAndDiagnosticMessage(status)
		return getPaddedMessage(status, msg, "")
	}

	printStatusMessage := func(status Status) {
		var msg string
		if status.Done {
			msg = createDoneMessage(status, isPreview)
		} else {
			msg = createInProgressMessage(status)
		}

		colorizeAndWriteProgress(opts, progressOutput, progress.Progress{
			ID:     status.ID,
			Action: msg,
		})
	}

	updateAllStatusMessages := func(includeDone bool) {
		for _, v := range eventUrnToStatus {
			if v.Done && !includeDone {
				continue
			}

			printStatusMessage(v)
		}
	}

	// Performs all the work at the end once we've heard about the last message
	// from the engine. Specifically, this will update the status messages for
	// any resources, and will also then print out all final diagnostics. and
	// finally will print out the summary.
	processEndSteps := func() {
		// Mark all in progress resources as done.
		for k, v := range eventUrnToStatus {
			if !v.Done {
				v.Done = true
				eventUrnToStatus[k] = v
				printStatusMessage(v)
			}
		}

		// Print all diagnostics at the end.  We only need to do this if we were summarizing.
		// Otherwise, this would have been seen while we were receiving the events.

		for _, status := range eventUrnToStatus {
			if len(status.DiagEvents) > 0 {
				wroteHeader := false
				for _, v := range status.DiagEvents {
					msg := renderProgressDiagEvent(v, opts)
					if msg != "" {
						if !wroteHeader {
							wroteHeader = true
							colorizeAndWriteProgress(opts, progressOutput, progress.Progress{Message: " "})
							colorizeAndWriteProgress(opts, progressOutput, progress.Progress{ID: status.ID, Message: "Diagnostics"})
						}

						colorizeAndWriteProgress(opts, progressOutput, progress.Progress{Message: "  " + msg})
					}
				}
			}
		}

		// print the summary
		if summaryEvent != nil {
			msg := renderSummaryEvent(summaryEvent.Payload.(engine.SummaryEventPayload), opts)
			if msg != "" {
				colorizeAndWriteProgress(opts, progressOutput, progress.Progress{Message: " "})
				colorizeAndWriteProgress(opts, progressOutput, progress.Progress{Message: msg})
			}
		}

		// no more progress events from this point on.  By closing the pipe, this will then cause
		// DisplayJSONMessagesToStream to finish once it processes the last message is receives from
		// pipeReader, causing DisplayEvents to finally complete.
		err := pipeWriter.Close()
		contract.IgnoreError(err)
	}

	defer func() {
		ticker.Stop()

		// let our caller know we're done.
		done <- true
	}()

	processTick := func() {
		// Got a tick.  Update all the in-progress resources.
		currentTick++

		currentTerminalWidth, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
		if currentTerminalWidth != terminalWidth {
			// terminal width changed.  Update our output.
			terminalWidth = currentTerminalWidth
			updateAllStatusMessages(true /*includeDone*/)
		} else {
			updateAllStatusMessages(false /*includeDone*/)
		}
	}

	processNormalEvent := func(event engine.Event) {
		eventUrn, metadata := getEventUrnAndMetadata(event)
		if isRootURN(eventUrn) {
			stackUrn = eventUrn
		}

		if eventUrn == "" {
			// if the event doesn't have any URN associated with it, just associate
			// it with the stack.
			eventUrn = stackUrn
		}

		switch event.Type {
		case engine.PreludeEvent:
			// A prelude event can just be printed out directly to the console.
			// Note: we should probably make sure we don't get any prelude events
			// once we start hearing about actual resource events.

			payload := event.Payload.(engine.PreludeEventPayload)
			isPreview = payload.IsPreview
			colorizeAndWriteProgress(opts, progressOutput, progress.Progress{
				Message: renderPreludeEvent(payload, opts),
			})
			return
		case engine.SummaryEvent:
			// keep track of the summar event so that we can display it after all other
			// resource-related events we receive.
			summaryEvent = &event
			return
		case engine.DiagEvent:
			msg := renderProgressDiagEvent(event, opts)
			if msg == "" {
				return
			}
		}

		// Don't bother showing certain events (for example, things that are unchanged). However
		// always show the root 'stack' resource so we can indicate that it's still running, and
		// also so we have something to attach unparented diagnostic events to.
		if metadata != nil && !shouldShow(*metadata, opts) && !isRootURN(eventUrn) {
			return
		}

		// At this point, all events should relate to resources.

		refreshAllStatuses := false
		status, has := eventUrnToStatus[eventUrn]
		if !has {
			// first time we're hearing about this resource.  Create an initial nearly-empty
			// status for it, assigning it a nice short ID.
			status = Status{Tick: currentTick}
			status.Step.Op = deploy.OpSame
			status.ID = makeID(eventUrn)

			if isTerminal {
				// in the terminal we want to align the status portions of messages. If we
				// heard about a resource with a longer id, go and update all in-flight and
				// finished resources so that their statuses get aligned.
				if len(status.ID) > maxIDLength {
					maxIDLength = len(status.ID)
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
		eventUrnToStatus[eventUrn] = status

		// refresh the progress information for this resource.  (or update all resources if
		// we need to realign everything)
		if refreshAllStatuses {
			updateAllStatusMessages(true /*includeDone*/)
		} else {
			printStatusMessage(status)
		}
	}

	// Main processing loop.  The purpose of this func is to read in events from the engine
	// and translate them into Status objects and progress messages to be presented to the
	// command line.
	go func() {
		for {
			select {
			case <-ticker.C:
				processTick()

			case event := <-events:
				if event.Type == "" || event.Type == engine.CancelEvent {
					// Engine finished sending events.  Do all the final processing and return
					// from this local func.  This will print out things like full diagnostic
					// events, as well as the summary event from the engine.
					processEndSteps()
					return
				}

				processNormalEvent(event)
			}
		}
	}()

	// Call into Docker to actually suck the progress messages out of pipeReader and display
	// them to the console.
	err := jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
	contract.IgnoreError(err)
}

func renderProgressDiagEvent(event engine.Event, opts backend.DisplayOptions) string {
	payload := event.Payload.(engine.DiagEventPayload)
	if payload.Severity == diag.Debug && !opts.Debug {
		return ""
	}
	return payload.Message
}

func getMetadataSummary(
	step engine.StepEventMetadata, opts backend.DisplayOptions,
	isPreview bool, done bool, failed bool) string {

	out := &bytes.Buffer{}

	if done {
		writeString(out, getStepDoneDescription(step, isPreview, failed))
	} else {
		writeString(out, getStepInProgressDescription(step, isPreview))
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

func getStepDoneDescription(step engine.StepEventMetadata, isPreview bool, failed bool) string {
	makeError := func(v string) string {
		return colors.SpecError + "**" + v + "**" + colors.Reset
	}

	if isRootStack(step) {
		if failed {
			return makeError("Failed")
		}

		return "Completed"
	}

	if isPreview && !isRootStack(step) {
		return getStepInProgressDescription(step, isPreview)
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

func getStepInProgressDescription(step engine.StepEventMetadata, isPreview bool) string {
	if isRootStack(step) {
		return "Running"
	}

	op := step.Op

	getDescription := func() string {
		if isPreview {
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

func renderResourceMetadata(
	metadata engine.StepEventMetadata, seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions, isPreview bool, done bool, failed bool) string {

	seen[metadata.URN] = metadata

	if shouldShow(metadata, opts) {
		return getMetadataSummary(metadata, opts, isPreview, done, failed)
	}

	return ""
}

func writeString(b *bytes.Buffer, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}
