// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/docker/docker/pkg/term"
	"golang.org/x/crypto/ssh/terminal"
)

// Progress describes a message we want to show in the display.  There are two types of messages,
// simple 'Messages' which just get printed out as a single uninterpreted line, and 'Actions' which
// are placed and updated in the progress-grid based on their ID.  Messages do not need an ID, while
// Actions must have an ID.
type Progress struct {
	ID      string
	Message string
	Action  string
}

func makeMessageProgress(message string) Progress {
	return Progress{Message: message}
}

func makeActionProgress(id string, action string) Progress {
	contract.Assertf(id != "", "id must be non empty for action %s", action)
	contract.Assertf(action != "", "action must be non empty")

	return Progress{ID: id, Action: action}
}

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

type ProgressDisplay struct {
	opts           backend.DisplayOptions
	progressOutput chan<- Progress

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
	summaryEventPayload *engine.SummaryEventPayload

	// Any system events we've received.  They will be printed at the bottom of all the status rows
	systemEventPayloads []engine.StdoutEventPayload

	// Used to record the order that rows are created in.  That way, when we present in a tree, we
	// can keep things ordered so they will not jump around.
	displayTime int

	// What tick we're currently on.  Used to determine the number of ellipses to concat to
	// a status message to help indicate that things are still working.
	currentTick int

	headerRow    Row
	resourceRows []ResourceRow

	// A mapping from each resource URN we are told about to its current status.
	eventUrnToResourceRow map[resource.URN]ResourceRow

	// Remember if we're a terminal or not.  In a terminal we get a little bit fancier.
	// For example, we'll go back and update previous status messages to make sure things
	// align.  We don't need to do that in non-terminal situations.
	isTerminal bool

	// The width of the terminal.  Used so we can trim resource messages that are too long.
	terminalWidth int

	// If all progress messages are done and we can print out the final display.
	Done bool

	// The column that the suffix should be added to
	suffixColumn int

	// the list of suffixes to rotate through
	suffixesArray []string

	// Maps used so we can generate short IDs for resource urns.
	urnToID map[resource.URN]string

	// Cache of colorized to uncolorized text.  We go between the two a lot, so caching helps
	// prevent lots of recomputation
	colorizedToUncolorized map[string]string

	// Cache of lines we've already printed.  We don't print a progress message again if it hasn't
	// changed between the last time we printed and now.
	printedProgressCache map[string]Progress
}

var (
	// simple regex to take our names like "aws:function:Function" and convert to
	// "aws:Function"
	typeNameRegex = regexp.MustCompile("^(.*):(.*)/(.*):(.*)$")
)

func simplifyTypeName(typ tokens.Type) string {
	typeString := string(typ)
	return typeNameRegex.ReplaceAllString(typeString, "$1:$2:$4")
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
func (display *ProgressDisplay) colorizeAndWriteProgress(progress Progress) {
	if progress.Message != "" {
		progress.Message = display.opts.Color.Colorize(progress.Message)
	}

	if progress.Action != "" {
		progress.Action = display.opts.Color.Colorize(progress.Action)
	}

	if progress.ID != "" {
		// don't repeat the same output if there is no difference between the last time we
		// printed it and now.
		lastProgress, has := display.printedProgressCache[progress.ID]
		if has && lastProgress.Message == progress.Message && lastProgress.Action == progress.Action {
			return
		}

		display.printedProgressCache[progress.ID] = progress
	}

	display.progressOutput <- progress
}

func (display *ProgressDisplay) writeSimpleMessage(msg string) {
	display.colorizeAndWriteProgress(makeMessageProgress(msg))
}

func (display *ProgressDisplay) writeBlankLine() {
	display.writeSimpleMessage(" ")
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

	// The channel we push progress messages into, and which DisplayProgressToStream pulls
	// from to display to the console.
	progressOutput := make(chan Progress)

	display := &ProgressDisplay{
		opts:                   opts,
		progressOutput:         progressOutput,
		eventUrnToResourceRow:  make(map[resource.URN]ResourceRow),
		suffixColumn:           int(statusColumn),
		suffixesArray:          []string{"", ".", "..", "..."},
		urnToID:                make(map[resource.URN]string),
		colorizedToUncolorized: make(map[string]string),
		printedProgressCache:   make(map[string]Progress),
		displayTime:            1,
	}

	// display.writeSimpleMessage(fmt.Sprintf("Max suffix length %v", display.maxSuffixLength))

	_, stdout, _ := term.StdStreams()
	_, isTerminal := term.GetFdInfo(stdout)

	terminalWidth, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	contract.IgnoreError(err)

	display.isTerminal = isTerminal
	display.terminalWidth = terminalWidth

	go func() {
		display.processEvents(ticker, events)

		// no more progress events from this point on.  By closing the pipe, this will then cause
		// DisplayJSONMessagesToStream to finish once it processes the last message is receives from
		// pipeReader, causing DisplayEvents to finally complete.
		close(progressOutput)
	}()

	DisplayProgressToStream(progressOutput, stdout, isTerminal)

	ticker.Stop()

	// let our caller know we're done.
	done <- true
}

// Gets the padding necessary to prepend to a message in order to keep it aligned in the
// terminal.
func (display *ProgressDisplay) getMessagePadding(
	uncolorizedColumns []string, columnIndex int, maxColumnLengths []int) string {

	extraWhitespace := 1

	// In the terminal we try to align the status messages for each resource.
	// do not bother with this in the non-terminal case.
	if columnIndex >= 0 && display.isTerminal {
		column := uncolorizedColumns[columnIndex]
		maxLength := maxColumnLengths[columnIndex]

		extraWhitespace = maxLength - utf8.RuneCountInString(column)
		contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxLength, column)

		// Place two spaces between all columns (except after the first column).  The first
		// column already has a ": " so it doesn't need the extra space.
		if columnIndex >= 0 {
			extraWhitespace += 2
		}
	}

	return strings.Repeat(" ", extraWhitespace)
}

// Gets the fully padded message to be shown.  The message will always include the ID of the
// status, then some amount of optional padding, then some amount of msgWithColors, then the
// suffix.  Importantly, if there isn't enough room to display all of that on the terminal, then
// the msg will be truncated to try to make it fit.
func (display *ProgressDisplay) getPaddedMessage(
	colorizedColumns, uncolorizedColumns []string, maxColumnLengths []int) string {

	colorizedMessage := ""

	for i := 0; i < len(colorizedColumns); i++ {
		padding := display.getMessagePadding(uncolorizedColumns, i-1, maxColumnLengths)
		colorizedMessage += padding + colorizedColumns[i]
	}

	if display.isTerminal {
		// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
		// msgWithColors having the color code information embedded with it.  So we need to get
		// the right substring of it, assuming that embedded colors are just markup and do not
		// actually contribute to the length
		maxMsgLength := display.terminalWidth - 1
		if maxMsgLength < 0 {
			maxMsgLength = 0
		}

		colorizedMessage = colors.TrimColorizedString(colorizedMessage, maxMsgLength)
	}

	return colorizedMessage
}

func (display *ProgressDisplay) uncolorizeString(v string) string {
	uncolorized, has := display.colorizedToUncolorized[v]
	if !has {
		uncolorized = colors.Never.Colorize(v)
		display.colorizedToUncolorized[v] = uncolorized
	}

	return uncolorized
}

func (display *ProgressDisplay) uncolorizeColumns(columns []string) []string {
	uncolorizedColumns := make([]string, len(columns))

	for i, v := range columns {
		uncolorizedColumns[i] = display.uncolorizeString(v)
	}

	return uncolorizedColumns
}

func (display *ProgressDisplay) refreshSingleRow(id string, row Row, maxColumnLengths []int) {
	colorizedColumns := row.ColorizedColumns()
	colorizedColumns[display.suffixColumn] += row.ColorizedSuffix()
	display.refreshColumns(id, colorizedColumns, maxColumnLengths)
}

func (display *ProgressDisplay) refreshColumns(
	id string, colorizedColumns []string, maxColumnLengths []int) {

	uncolorizedColumns := display.uncolorizeColumns(colorizedColumns)

	msg := display.getPaddedMessage(colorizedColumns, uncolorizedColumns, maxColumnLengths)

	if display.isTerminal {
		display.colorizeAndWriteProgress(makeActionProgress(id, msg))
	} else {
		display.writeSimpleMessage(msg)
	}
}

// Ensure our stored dimension info is up to date.
func (display *ProgressDisplay) updateTerminalWidth() {
	// don't do any refreshing if we're not in a terminal
	if display.isTerminal {
		currentTerminalWidth, _, err := terminal.GetSize(int(os.Stdout.Fd()))
		contract.IgnoreError(err)

		if currentTerminalWidth != display.terminalWidth {
			display.terminalWidth = currentTerminalWidth

			// also clear our display cache as we want to reprint all lines.
			display.printedProgressCache = make(map[string]Progress)
		}
	}
}

type treeNode struct {
	row Row

	colorizedColumns []string
	colorizedSuffix  string

	childNodes []*treeNode
}

func (display *ProgressDisplay) getOrCreateTreeNode(
	result *[]*treeNode, urn resource.URN, row ResourceRow, urnToTreeNode map[resource.URN]*treeNode) *treeNode {

	node, has := urnToTreeNode[urn]
	if has {
		return node
	}

	node = &treeNode{
		row:              row,
		colorizedColumns: row.ColorizedColumns(),
		colorizedSuffix:  row.ColorizedSuffix(),
	}

	urnToTreeNode[urn] = node

	if urn != "" && urn != display.stackUrn {
		var parentURN resource.URN

		res := row.Step().Res
		if res != nil {
			parentURN = res.Parent
		}

		parentRow, hasParentRow := display.eventUrnToResourceRow[parentURN]
		if !hasParentRow {
			// if we couldn't find a parent for this yet, parent this to the stack if we
			// have an entry for that.  Otherwise, we'll just make this a top level item.
			parentURN = display.stackUrn
			parentRow, hasParentRow = display.eventUrnToResourceRow[parentURN]
		}

		if hasParentRow {
			parentNode := display.getOrCreateTreeNode(result, parentURN, parentRow, urnToTreeNode)
			parentNode.childNodes = append(parentNode.childNodes, node)
			return node
		}
	}

	*result = append(*result, node)
	return node
}

func (display *ProgressDisplay) generateTreeNodes() []*treeNode {
	result := []*treeNode{}

	result = append(result, &treeNode{
		row:              display.headerRow,
		colorizedColumns: display.headerRow.ColorizedColumns(),
	})

	urnToTreeNode := make(map[resource.URN]*treeNode)
	for urn, row := range display.eventUrnToResourceRow {
		display.getOrCreateTreeNode(&result, urn, row, urnToTreeNode)
	}

	return result
}

func (display *ProgressDisplay) addIndentations(treeNodes []*treeNode, isRoot bool, indentation string) {

	childIndentation := indentation + "│  "
	lastChildIndentation := indentation + "   "

	for i, node := range treeNodes {
		isLast := i == len(treeNodes)-1

		prefix := indentation

		var nestedIndentation string
		if !isRoot {
			if isLast {
				prefix += "└─ "
				nestedIndentation = lastChildIndentation
			} else {
				prefix += "├─ "
				nestedIndentation = childIndentation
			}
		}

		node.colorizedColumns[typeColumn] = prefix + node.colorizedColumns[typeColumn]
		display.addIndentations(node.childNodes, false /*isRoot*/, nestedIndentation)
	}
}

func (display *ProgressDisplay) convertNodesToRows(
	nodes []*treeNode, maxSuffixLength int, rows *[][]string, maxColumnLengths *[]int) {

	for _, node := range nodes {
		if len(*maxColumnLengths) == 0 {
			*maxColumnLengths = make([]int, len(node.colorizedColumns))
		}

		colorizedColumns := make([]string, len(node.colorizedColumns))
		uncolorisedColumns := display.uncolorizeColumns(node.colorizedColumns)

		for i, colorizedColumn := range node.colorizedColumns {
			columnWidth := utf8.RuneCountInString(uncolorisedColumns[i])

			if i == display.suffixColumn {
				columnWidth += maxSuffixLength
				colorizedColumns[i] = colorizedColumn + node.colorizedSuffix
			} else {
				colorizedColumns[i] = colorizedColumn
			}

			if columnWidth > (*maxColumnLengths)[i] {
				(*maxColumnLengths)[i] = columnWidth
			}
		}

		*rows = append(*rows, colorizedColumns)

		display.convertNodesToRows(node.childNodes, maxSuffixLength, rows, maxColumnLengths)
	}
}

type sortable []*treeNode

func (sortable sortable) Len() int {
	return len(sortable)
}

func (sortable sortable) Less(i, j int) bool {
	return sortable[i].row.FirstDisplayTime() < sortable[j].row.FirstDisplayTime()
}

func (sortable sortable) Swap(i, j int) {
	sortable[i], sortable[j] = sortable[j], sortable[i]
}

func sortNodes(nodes []*treeNode) {
	sort.Sort(sortable(nodes))

	for _, node := range nodes {
		childNodes := node.childNodes
		sortNodes(childNodes)
		node.childNodes = childNodes
	}
}

func (display *ProgressDisplay) filterOutUnnecessaryNodesAndSetDisplayTimes(nodes []*treeNode) []*treeNode {
	result := []*treeNode{}

	for _, node := range nodes {
		node.childNodes = display.filterOutUnnecessaryNodesAndSetDisplayTimes(node.childNodes)

		if node.row.HideRowIfUnnecessary() && len(node.childNodes) == 0 {
			continue
		}

		display.displayTime++
		node.row.SetFirstDisplayTime(display.displayTime)
		result = append(result, node)
	}

	return result
}

func (display *ProgressDisplay) refreshAllRowsIfInTerminal() {
	if display.isTerminal && display.headerRow != nil {
		// make sure our stored dimension info is up to date
		display.updateTerminalWidth()

		rootNodes := display.generateTreeNodes()
		rootNodes = display.filterOutUnnecessaryNodesAndSetDisplayTimes(rootNodes)
		sortNodes(rootNodes)
		display.addIndentations(rootNodes, true /*isRoot*/, "")

		maxSuffixLength := 0
		for _, v := range display.suffixesArray {
			runeCount := utf8.RuneCountInString(v)
			if runeCount > maxSuffixLength {
				maxSuffixLength = runeCount
			}
		}

		var rows [][]string
		var maxColumnLengths []int
		display.convertNodesToRows(rootNodes, maxSuffixLength, &rows, &maxColumnLengths)

		for i, row := range rows {
			var id string
			if i == 0 {
				id = "#"
			} else {
				id = fmt.Sprintf("%v", i)
			}

			display.refreshColumns(id, row, maxColumnLengths)
		}

		systemID := len(rows)

		printedHeader := false
		for _, payload := range display.systemEventPayloads {
			msg := payload.Color.Colorize(payload.Message)
			lines := splitIntoDisplayableLines(msg)

			if len(lines) == 0 {
				continue
			}

			if !printedHeader {
				printedHeader = true
				display.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID), " "))
				systemID++

				display.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID),
					colors.Yellow+"System Messages"+colors.Reset))
				systemID++
			}

			for _, line := range lines {
				display.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID), fmt.Sprintf("  %s", line)))
				systemID++
			}
		}
	}
}

// Performs all the work at the end once we've heard about the last message from the engine.
// Specifically, this will update the status messages for any resources, and will also then
// print out all final diagnostics. and finally will print out the summary.
func (display *ProgressDisplay) processEndSteps() {
	display.Done = true

	for _, v := range display.eventUrnToResourceRow {
		// transition everything to the done state.  If we're not in an a terminal and this is a
		// transition, then print out the transition.  Don't bother doing this in a terminal as
		// we're going to refresh everything when we break out of the loop.
		if !v.Done() {
			v.SetDone()

			if !display.isTerminal {
				display.refreshSingleRow("", v, nil)
			}
		} else {
			// Explicitly transition the status so that we clear out any cached data for it.
			v.SetDone()
		}
	}

	// Now refresh everything.  this ensures that we go back and remove things like the diagnostic
	// messages from a status message (since we're going to print them all) below.  Note, this will
	// only do something in a terminal.  This i what we want, because if we're not in a terminal we
	// don't really want to reprint any finished items we've already printed.
	display.refreshAllRowsIfInTerminal()

	// Print all diagnostics we've seen.

	wroteDiagnosticHeader := false

	for _, row := range display.eventUrnToResourceRow {
		events := row.DiagInfo().DiagEvents
		if len(events) > 0 {
			wroteResourceHeader := false
			for _, v := range events {
				msg := display.renderProgressDiagEvent(v)

				lines := splitIntoDisplayableLines(msg)
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
					columns := row.ColorizedColumns()
					display.writeSimpleMessage("  " +
						// columns[idColumn] + ": " +
						columns[typeColumn] + ": " +
						columns[nameColumn])
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
	if display.summaryEventPayload != nil {
		msg := renderSummaryEvent(*display.summaryEventPayload, display.opts)

		if !wroteDiagnosticHeader {
			display.writeBlankLine()
		}

		display.writeSimpleMessage(msg)
	}
}

func splitIntoDisplayableLines(msg string) []string {
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

	return lines
}

func (display *ProgressDisplay) processTick() {
	// Got a tick.  Update all  resources if we're in a terminal.  If we're not, then this won't do
	// anything.
	display.currentTick++

	display.refreshAllRowsIfInTerminal()
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
		payload := event.Payload.(engine.SummaryEventPayload)
		display.summaryEventPayload = &payload
		return
	case engine.DiagEvent:
		msg := display.renderProgressDiagEvent(event)
		if msg == "" {
			return
		}
	case engine.StdoutColorEvent:
		display.handleSystemEvent(event.Payload.(engine.StdoutEventPayload))
		return
	}

	// Don't bother showing certain events (for example, things that are unchanged). However
	// always show the root 'stack' resource so we can indicate that it's still running, and
	// also so we have something to attach unparented diagnostic events to.
	hideRowIfUnnecessary := false
	if metadata != nil && !shouldShow(*metadata, display.opts) && !isRootURN(eventUrn) {
		hideRowIfUnnecessary = true
	}

	// At this point, all events should relate to resources.

	row, has := display.eventUrnToResourceRow[eventUrn]
	if !has {
		// first time we're hearing about this resource.  Create an initial nearly-empty
		// status for it, assigning it a nice short ID.
		row = &resourceRowData{
			display:              display,
			tick:                 display.currentTick,
			diagInfo:             &DiagInfo{},
			step:                 engine.StepEventMetadata{Op: deploy.OpSame},
			hideRowIfUnnecessary: hideRowIfUnnecessary,
		}

		display.eventUrnToResourceRow[eventUrn] = row
		display.ensureHeaderRow()
		display.resourceRows = append(display.resourceRows, row)
	}

	if event.Type == engine.ResourcePreEvent {
		step := event.Payload.(engine.ResourcePreEventPayload).Metadata
		if step.Op == "" {
			contract.Failf("Got empty op for %s", event.Type)
		}

		row.SetStep(step)
	} else if event.Type == engine.ResourceOutputsEvent {
		// transition the status to done.
		if !isRootURN(eventUrn) {
			row.SetDone()
		}
	} else if event.Type == engine.ResourceOperationFailed {
		row.SetDone()
		row.SetFailed()
	} else if event.Type == engine.DiagEvent {
		// also record this diagnostic so we print it at the end.
		row.RecordDiagEvent(event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	if display.isTerminal {
		// if we're in a terminal, then refresh everything so that all our columns line up
		display.refreshAllRowsIfInTerminal()
	} else {
		// otherwise, just print out this single row.
		display.refreshSingleRow("", row, nil)
	}
}

func (display *ProgressDisplay) handleSystemEvent(payload engine.StdoutEventPayload) {
	// Make sure we have a header to display
	display.ensureHeaderRow()

	display.systemEventPayloads = append(display.systemEventPayloads, payload)

	if display.isTerminal {
		// if we're in a terminal, then refresh everything.  The system events will come after
		// all the normal rows
		display.refreshAllRowsIfInTerminal()
	} else {
		// otherwise, in a non-terminal, just print out the actual event.
		display.writeSimpleMessage(renderStdoutColorEvent(payload, display.opts))
	}
}

func (display *ProgressDisplay) ensureHeaderRow() {
	if display.headerRow == nil {
		// about to make our first status message.  make sure we present the header line first.
		display.headerRow = &headerRowData{display: display}
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
	return strings.TrimRightFunc(payload.Message, unicode.IsSpace)
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
		return "no change"
	case deploy.OpCreate:
		return "create"
	case deploy.OpUpdate:
		return "update"
	case deploy.OpDelete:
		return "delete"
	case deploy.OpReplace:
		return "replace"
	case deploy.OpCreateReplacement:
		return "create for replacement"
	case deploy.OpDeleteReplaced:
		return "delete for replacement"
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
