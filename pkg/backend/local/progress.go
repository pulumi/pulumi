// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
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

	// The progress message to print.
	Message string

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
func getEventUrn(event engine.Event) resource.URN {
	if event.Type == engine.ResourcePreEvent {
		payload := event.Payload.(engine.ResourcePreEventPayload)
		return payload.Metadata.URN
	} else if event.Type == engine.ResourceOutputsEvent {
		payload := event.Payload.(engine.ResourceOutputsEventPayload)
		return payload.Metadata.URN
	} else if event.Type == engine.DiagEvent {
		payload := event.Payload.(engine.DiagEventPayload)
		return payload.URN
	}

	return ""
}

func writeProgress(chanOutput progress.Output, progress progress.Progress) {
	err := chanOutput.WriteProgress(progress)
	if err != nil {
		contract.IgnoreError(err)
	}
}

var (
	// We want to present a trim name to users for any URN.  These maps, and the helper functions
	// below are used for that.
	urnToID = make(map[resource.URN]string)
	idToUrn = make(map[string]resource.URN)
)

func makeIDWorker(urn resource.URN, suffix int) string {
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

func makeID(urn resource.URN) string {
	if id, has := urnToID[urn]; !has {
		for i := 0; ; i++ {
			id = makeIDWorker(urn, i)

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

// DisplayProgressEvents displays the engine events with docker's progress view.
func DisplayProgressEvents(
	action string, events <-chan engine.Event, done chan<- bool,
	debug bool, opts backend.DisplayOptions) {

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
	seen := make(map[resource.URN]engine.StepEventMetadata)

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

	pipeReader, pipeWriter := io.Pipe()
	progressChan := make(chan progress.Progress, 100)

	chanOutput := progress.ChanOutput(progressChan)

	go func() {
		// docker helper that reads progress messages in from progressChan and converts them
		// into special formatted messages that are written to pipe-writer.
		writeDistributionProgress(pipeWriter, progressChan)

		// Once we've written everything to the pipe, we're done with it can let it go.
		err := pipeWriter.Close()
		contract.IgnoreError(err)

		ticker.Stop()

		// let our caller know we're done.
		done <- true
	}()

	ellipses := []string{"", ".", "..", "..."}
	createInProgressMessage := func(status Status) string {
		msg := status.Message

		// if there are any diagnostics for this resource, add information about the
		// last diagnostic to the status message.
		if len(status.DiagEvents) > 0 {
			diagMsg := renderProgressEvent(
				status.DiagEvents[len(status.DiagEvents)-1], seen, debug, opts, isPreview)

			if diagMsg != "" {
				msg += ". " + diagMsg
			}
		}

		// Add an changing ellipses to help convey that progress is happening.
		msg += ellipses[(status.Tick+currentTick)%len(ellipses)]

		return msg
	}

	createDoneMessage := func(status Status, isPreview bool) string {
		if status.Step.Op == "" {
			contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
		}

		// Colorize the information about the resource operation, and add a summary
		// of all the diagnostics we heard about it.

		msg := colors.ColorizeText(
			getMetadataSummary(status.Step, opts, isPreview, true /*isComplete*/))

		debugEvents := 0
		infoEvents := 0
		errorEvents := 0
		warningEvents := 0
		for _, ev := range status.DiagEvents {
			payload := ev.Payload.(engine.DiagEventPayload)

			switch payload.Severity {
			case diag.Debug:
				debugEvents++
			case diag.Info:
				infoEvents++
			case diag.Infoerr:
				errorEvents++
			case diag.Warning:
				warningEvents++
			case diag.Error:
				errorEvents++
			}
		}

		if debugEvents > 0 {
			msg += fmt.Sprintf(", %v debug message(s)", debugEvents)
		}

		if infoEvents > 0 {
			msg += fmt.Sprintf(", %v info message(s)", infoEvents)
		}

		if warningEvents > 0 {
			msg += fmt.Sprintf(", %v warning(s)", warningEvents)
		}

		if errorEvents > 0 {
			msg += fmt.Sprintf(", %v error(s)", errorEvents)
		}

		return msg
	}

	writeAction := func(id string, msg string) {
		extraWhitespace := 0

		// In the terminal we try to align the status messages for each resource.
		// do not bother with this in the non-terminal case.
		if isTerminal {
			extraWhitespace = maxIDLength - len(id)
			contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxIDLength, id)
		}

		writeProgress(chanOutput, progress.Progress{
			ID:     id,
			Action: strings.Repeat(" ", extraWhitespace) + msg,
		})
	}

	printStatusForTopLevelResource := func(status Status) {
		if !status.Done {
			writeAction(status.ID, createInProgressMessage(status))
		} else {
			writeAction(status.ID, createDoneMessage(status, isPreview))
		}
	}

	printStatusForTopLevelResources := func(includeDone bool) {
		for _, v := range eventUrnToStatus {
			if v.Done && !includeDone {
				continue
			}

			printStatusForTopLevelResource(v)
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
				printStatusForTopLevelResource(v)
			}
		}

		// Print all diagnostics at the end.  We only need to do this if we were summarizing.
		// Otherwise, this would have been seen while we were receiving the events.

		for _, status := range eventUrnToStatus {
			if len(status.DiagEvents) > 0 {
				wroteHeader := false
				for _, v := range status.DiagEvents {
					// out = os.Stdout
					// if v.Severity == diag.Error || v.Severity == diag.Warning {
					// 	out = os.Stderr
					// }

					msg := renderProgressEvent(v, seen, debug, opts, isPreview)
					if msg != "" {
						if !wroteHeader {
							wroteHeader = true
							writeProgress(chanOutput, progress.Progress{Message: " "})
							writeProgress(chanOutput, progress.Progress{ID: status.ID, Message: "Diagnostics"})
						}

						writeProgress(chanOutput, progress.Progress{Message: "  " + msg})
					}
				}
			}
		}

		// print the summary
		if summaryEvent != nil {
			msg := renderProgressEvent(*summaryEvent, seen, debug, opts, isPreview)
			if msg != "" {
				writeProgress(chanOutput, progress.Progress{Message: " "})
				writeProgress(chanOutput, progress.Progress{Message: msg})
			}
		}

		// no more progress events from this point on.  By closing the progress channel, this will
		// cause writeDistributionProgress to finish.  This, in turn, will close the pipeWriter.
		// This will then cause DisplayJSONMessagesToStream to finish once it processes the last
		// message is receives from pipeReader, causing DisplayEvents to finally complete.
		close(progressChan)
	}

	// Main processing loop.  The purpose of this func is to read in events from the engine
	// and translate them into Status objects and progress messages to be presented to the
	// command line.
	go func() {
		for {
			select {
			case <-ticker.C:
				// Got a tick.  Update all the in-progress resources.
				currentTick++
				printStatusForTopLevelResources(false /*includeDone:*/)

			case event := <-events:
				if event.Type == "" || event.Type == engine.CancelEvent {
					// Engine finished sending events.  Do all the final processing and return
					// from this local func.  This will print out things like full diagnostic
					// events, as well as the summary event from the engine.
					processEndSteps()
					return
				}

				eventUrn := getEventUrn(event)
				if isRootURN(eventUrn) {
					stackUrn = eventUrn
				}

				// First just make a string out of the event.  If we get nothing back this isn't an
				// interesting event and we can just skip it.
				msg := renderProgressEvent(event, seen, debug, opts, isPreview)
				if msg == "" {
					continue
				}

				switch event.Type {
				case engine.PreludeEvent:
					// A prelude event can just be printed out directly to the console.
					// Note: we should probably make sure we don't get any prelude events
					// once we start hearing about actual resource events.

					isPreview = event.Payload.(engine.PreludeEventPayload).IsPreview
					writeProgress(chanOutput, progress.Progress{Message: " "})
					writeProgress(chanOutput, progress.Progress{Message: msg})
					continue
				case engine.SummaryEvent:
					// keep track of hte summar event so that we can display it after all other
					// resource-related events we receive.
					summaryEvent = &event
					continue
				}

				// At this point, all events should relate to resources.

				if eventUrn == "" {
					// if the event doesn't have any URN associated with it, just associate
					// it with the stack.
					eventUrn = stackUrn
				}

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
					status.Message = msg
					status.Step = event.Payload.(engine.ResourcePreEventPayload).Metadata
					if status.Step.Op == "" {
						contract.Failf("Got empty op for %s %s", event.Type, msg)
					}
				} else if event.Type == engine.ResourceOutputsEvent {
					// transition the status to done.
					status.Done = true
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
					printStatusForTopLevelResources(true /*includeDone*/)
				} else {
					printStatusForTopLevelResource(status)
				}
			}
		}
	}()

	// Call into Docker to actually suck the progress messages out of pipeReader and display
	// them to the console.
	err := jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
	if err != nil {
		contract.IgnoreError(err)
	}
}

func renderProgressEvent(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions, isPreview bool) string {

	msg := renderProgressEventWorker(event, seen, debug, opts, isPreview)
	return strings.TrimSpace(msg)
}

func renderProgressEventWorker(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions, isPreview bool) string {

	switch event.Type {
	case engine.CancelEvent:
		return ""
	case engine.PreludeEvent:
		return renderPreludeEvent(event.Payload.(engine.PreludeEventPayload), opts)
	case engine.SummaryEvent:
		return renderSummaryEvent(event.Payload.(engine.SummaryEventPayload), opts)
	case engine.ResourceOperationFailed:
		return renderResourceOperationFailedEvent(event.Payload.(engine.ResourceOperationFailedPayload), opts)
	case engine.ResourceOutputsEvent:
		return renderProgressResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts, isPreview)
	case engine.ResourcePreEvent:
		return renderProgressResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts, isPreview)
	case engine.StdoutColorEvent:
		return ""
	case engine.DiagEvent:
		return renderProgressDiagEvent(event.Payload.(engine.DiagEventPayload), debug, opts)
	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

func renderProgressDiagEvent(
	payload engine.DiagEventPayload, debug bool, opts backend.DisplayOptions) string {
	if payload.Severity == diag.Debug && !debug {
		// If this was a debug diagnostic and we're not displaying debug diagnostics,
		// then just return empty.  our callers will then filter out this message.
		return ""
	}

	return opts.Color.Colorize(payload.Message)
}

func getMetadataSummary(
	metadata engine.StepEventMetadata, opts backend.DisplayOptions,
	isPreview bool, isComplete bool) string {

	out := &bytes.Buffer{}
	summary := getMetadataSummaryWorker(metadata, isPreview, isComplete)

	fprintIgnoreError(out, opts.Color.Colorize(summary))
	fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))

	return out.String()
}

func getMetadataSummaryWorker(step engine.StepEventMetadata, isPreview bool, isComplete bool) string {
	var b bytes.Buffer

	// Next, print the resource type (since it is easy on the eyes and can be quickly identified).
	if isComplete {
		writeString(&b, getStepCompleteDescription(step.Op, isPreview))
	} else {
		writeString(&b, getStepDescription(step.Op, isPreview))
	}
	writeString(&b, colors.Reset)

	if step.Old != nil && step.New != nil && step.Old.Inputs != nil && step.New.Inputs != nil {
		diff := step.Old.Inputs.Diff(step.New.Inputs)

		if diff != nil {
			writeString(&b, ". Changes:")

			updates := make(resource.PropertyMap)
			for k := range diff.Updates {
				updates[k] = resource.PropertyValue{}
			}

			writePropertyKeys(&b, diff.Adds, deploy.OpCreate)
			writePropertyKeys(&b, diff.Deletes, deploy.OpDelete)
			writePropertyKeys(&b, updates, deploy.OpReplace)
		}
	}

	return b.String()
}

func getStepCompleteDescription(op deploy.StepOp, isPreview bool) string {
	if isPreview {
		return getStepDescription(op, isPreview)
	}

	return op.Prefix() + getStepCompleteDescriptionNoColor(op) + colors.Reset
}

func getStepDescription(op deploy.StepOp, isPreview bool) string {
	return op.Prefix() + getStepDescriptionNoColor(op, isPreview) + colors.Reset
}

func getStepDescriptionNoColor(op deploy.StepOp, isPreview bool) string {
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

func getStepCompleteDescriptionNoColor(op deploy.StepOp) string {
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

	contract.Failf("Unrecognized resource step op: %v", op)
	return ""
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

func renderProgressResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions,
	isPreview bool) string {

	seen[payload.Metadata.URN] = payload.Metadata

	if shouldShow(payload.Metadata, opts) {
		return getMetadataSummary(payload.Metadata, opts, isPreview, false /*isComplete*/)
	}

	return ""
}

func renderProgressResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions,
	isPreview bool) string {

	if shouldShow(payload.Metadata, opts) {
		return getMetadataSummary(payload.Metadata, opts, isPreview, true /*isComplete*/)
	}

	return ""
}

func writeString(b *bytes.Buffer, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}
