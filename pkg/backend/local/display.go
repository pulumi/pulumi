// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
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
	"github.com/pulumi/pulumi/pkg/tokens"
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

type Status struct {
	ID   string
	Step engine.StepEventMetadata
	Tick int
	Done bool

	Action     string
	DiagEvents []engine.Event
}

var (
	typeNameRegex = regexp.MustCompile("^(.*):(.*):(.*)$")
)

func simplifyTypeName(typ tokens.Type) string {
	typeString := string(typ)
	return typeNameRegex.ReplaceAllString(typeString, "$1:$3")
}

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

	contract.Failf("Unhandled event type '%s'", event.Type)
	return ""
}

// DisplayEvents reads events from the `events` channel until it is closed, displaying each event as
// it comes in. Once all events have been read from the channel and displayed, it closes the `done`
// channel so the caller can await all the events being written.
func DisplayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	_, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, 1 /*timesPerSecond*/)

	_, stdout, _ := term.StdStreams()
	_, isTerminal := term.GetFdInfo(stdout)

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

	isPreview := false
	summarize := opts.Summary

	var stackUrn resource.URN
	seen := make(map[resource.URN]engine.StepEventMetadata)
	var summaryEvent *engine.Event
	maxIDLength := 0
	currentTick := 0

	topLevelResourceToStatus := make(map[resource.URN]Status)

	createInProgressMessage := func(status Status) string {
		msg := status.Action
		if len(status.DiagEvents) > 0 {
			diagMsg := RenderEvent(
				status.DiagEvents[len(status.DiagEvents)-1], seen, debug, opts, isPreview)

			if diagMsg != "" {
				msg += ". " + diagMsg
			}
		}

		ellipses := strings.Repeat(".", (status.Tick+currentTick)%3) + "  "
		msg += ellipses

		return msg
	}

	createDoneMessage := func(status Status, isPreview bool) string {
		if status.Step.Op == "" {
			contract.Failf("Finishing a resource we never heard about: '%s'", status.ID)
		}

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

	printStatusForTopLevelResource := func(status Status) {
		writeAction := func(msg string) {
			extraWhitespace := 0

			// In the terminal we try to align the status messages for each resource.
			// do not bother with this in the non-terminal case.
			if isTerminal {
				extraWhitespace = maxIDLength - len(status.ID)
				contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxIDLength, status.ID)
			}

			chanOutput.WriteProgress(progress.Progress{
				ID:     status.ID,
				Action: strings.Repeat(" ", extraWhitespace) + msg,
			})
		}

		if !status.Done {
			writeAction(createInProgressMessage(status))
		} else {
			writeAction(createDoneMessage(status, isPreview))
		}
	}

	printStatusForTopLevelResources := func(includeDone bool) {
		for _, v := range topLevelResourceToStatus {
			if v.Done && !includeDone {
				continue
			}

			printStatusForTopLevelResource(v)
		}
	}

	processEndSteps := func() {
		// Mark all in progress resources as done.
		for k, v := range topLevelResourceToStatus {
			if !v.Done {
				v.Done = true
				topLevelResourceToStatus[k] = v
				printStatusForTopLevelResource(v)
			}
		}

		// print the summary
		if summaryEvent != nil {
			msg := RenderEvent(*summaryEvent, seen, debug, opts, isPreview)
			if msg != "" {
				chanOutput.WriteProgress(progress.Progress{Message: " "})
				chanOutput.WriteProgress(progress.Progress{Message: msg})
			}
		}

		// Print all diagnostics at the end.  We only need to do this if we were summarizing.
		// Otherwise, this would have been seen while we were receiving the events.

		if !summarize {
			for _, status := range topLevelResourceToStatus {
				if len(status.DiagEvents) > 0 {
					chanOutput.WriteProgress(progress.Progress{Message: " "})
					for _, v := range status.DiagEvents {
						// out = os.Stdout
						// if v.Severity == diag.Error || v.Severity == diag.Warning {
						// 	out = os.Stderr
						// }

						msg := RenderEvent(v, seen, debug, opts, isPreview)
						if msg != "" {
							chanOutput.WriteProgress(progress.Progress{Message: msg})
						}
					}
				}
			}
		}

		// no more progress events from this point on.
		close(progressChan)
	}

	urnToID := make(map[resource.URN]string)
	idToUrn := make(map[string]resource.URN)

	makeIDWorker := func(urn resource.URN, suffix int) string {
		// for i := 0
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

	makeID := func(urn resource.URN) string {
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

	var mapToStackUrnOrImmediateStackChildUrn func(urn resource.URN) resource.URN
	mapToStackUrnOrImmediateStackChildUrn = func(urn resource.URN) resource.URN {
		if urn == "" || urn == stackUrn {
			return stackUrn
		}

		v, ok := seen[urn]
		if !ok {
			return stackUrn
		}

		parent := v.Res.Parent
		if parent == "" || parent == stackUrn {
			return urn
		}

		return mapToStackUrnOrImmediateStackChildUrn(parent)
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				currentTick++
				printStatusForTopLevelResources(false /*includeDone:*/)

			case event := <-events:
				if event.Type == "" || event.Type == engine.CancelEvent {
					processEndSteps()
					return
				}

				// First just make a string out of the event.  If we get nothing back this isn't an
				// interesting event and we can just skip it.
				msg := RenderEvent(event, seen, debug, opts, isPreview)
				if msg == "" {
					continue
				}

				switch event.Type {
				case engine.PreludeEvent:
					isPreview = event.Payload.(engine.PreludeEventPayload).IsPreview
					chanOutput.WriteProgress(progress.Progress{Message: " "})
					chanOutput.WriteProgress(progress.Progress{Message: msg})
					continue
				case engine.SummaryEvent:
					// keep track of hte summar event so that we can display it after all other
					// resource-related events we receive.
					summaryEvent = &event
					continue
				}

				eventUrn := getEventUrn(event)
				if isRootURN(eventUrn) {
					stackUrn = eventUrn
				}

				if eventUrn == "" {
					eventUrn = stackUrn
				}

				var topLevelUrn resource.URN
				if summarize {
					// if we're summarizing, then we want to write this message associated
					// either with the stack-urn, or an immediate child of the stack-urn.
					topLevelUrn = mapToStackUrnOrImmediateStackChildUrn(eventUrn)
				} else {
					// otherwise, we print the information out for each resource.
					topLevelUrn = eventUrn
				}

				refreshAllStatuses := false
				status, has := topLevelResourceToStatus[topLevelUrn]
				if !has {
					status = Status{Tick: currentTick}
					status.Step.Op = deploy.OpSame
					status.ID = makeID(topLevelUrn)

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
					status.Action = msg
					status.Step = event.Payload.(engine.ResourcePreEventPayload).Metadata
					if status.Step.Op == "" {
						contract.Failf("Got empty op for %s %s", event.Type, msg)
					}
				} else if event.Type == engine.ResourceOutputsEvent {
					if eventUrn == topLevelUrn {
						status.Done = true
					}
				} else if event.Type == engine.DiagEvent {
					// also record this diagnostic so we print it at the end.
					status.DiagEvents = append(status.DiagEvents, event)
				} else {
					contract.Failf("Unhandled event type '%s'", event.Type)
				}

				topLevelResourceToStatus[topLevelUrn] = status

				if refreshAllStatuses {
					printStatusForTopLevelResources(true /*includeDone*/)
				} else {
					printStatusForTopLevelResource(status)
				}
			}
		}
	}()

	jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
}

func RenderEvent(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions, isPreview bool) string {

	msg := renderEventWorker(event, seen, debug, opts, isPreview)
	return strings.TrimSpace(msg)
}

func renderEventWorker(
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
		return renderResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), seen, opts, isPreview)
	case engine.ResourcePreEvent:
		return renderResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), seen, opts, isPreview)
	case engine.StdoutColorEvent:
		return renderStdoutColorEvent(event.Payload.(engine.StdoutEventPayload), opts)
	case engine.DiagEvent:
		return renderDiagEvent(event.Payload.(engine.DiagEventPayload), debug, opts)
	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
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
	payload engine.DiagEventPayload, debug bool, opts backend.DisplayOptions) string {
	if payload.Severity == diag.Debug && !debug {
		return ""
	}

	return opts.Color.Colorize(payload.Message)
	// var msg = "Diag: " + string(payload.URN) + ": "
	// msg += opts.Color.Colorize(payload.Message)

	// return upToFirstNewLine(opts, msg)
}

func renderStdoutColorEvent(
	payload engine.StdoutEventPayload, opts backend.DisplayOptions) string {

	return opts.Color.Colorize(payload.Message)
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

func writeString(b *bytes.Buffer, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}

func write(b *bytes.Buffer, op deploy.StepOp, format string, a ...interface{}) {
	writeString(b, op.Color())
	writeString(b, fmt.Sprintf(format, a...))
	writeString(b, colors.Reset)
}

func renderResourcePreEvent(
	payload engine.ResourcePreEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions,
	isPreview bool) string {

	seen[payload.Metadata.URN] = payload.Metadata

	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		summary := getMetadataSummary(payload.Metadata, opts, isPreview, false /*isComplete*/)
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

		return summary
	} else {
		return ""
	}

	// return upToFirstNewLine(opts, out.String())
}

func renderResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload,
	seen map[resource.URN]engine.StepEventMetadata,
	opts backend.DisplayOptions,
	isPreview bool) string {

	// out := &bytes.Buffer{}

	if shouldShow(payload.Metadata, opts) {
		// indent := engine.GetIndent(payload.Metadata, seen)
		// text := engine.GetResourceOutputsPropertiesString(payload.Metadata, payload.Planning, payload.Debug)
		// if opts.Summary {
		// 	// if we're summarizing, then our info is being added to a parent node.
		// 	// so we want to print out our child info
		// 	summary := getMetadataSummary(payload.Metadata, opts)
		// 	return payload.Metadata.URN, summary + " - Done!"
		// } else {
		return getMetadataSummary(payload.Metadata, opts, isPreview, true /*isComplete*/)
		// }

		// fprintIgnoreError(out, opts.Color.Colorize(text))
	}

	return ""
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
