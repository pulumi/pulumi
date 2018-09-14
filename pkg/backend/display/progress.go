// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package display

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

	"github.com/docker/docker/pkg/term"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
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

	// The very last diagnostic event we got for this resource (regardless of severity). We'll print
	// this out in the non-interactive mode whenever we get new events. Importantly, we don't want
	// to print out the most significant diagnostic, as that means a flurry of event swill cause us
	// to keep printing out the most significant diagnostic over and over again.
	LastDiag *engine.DiagEventPayload

	// The last event of each severity kind.  We'll print out the most significant of these (in the
	// tree-view) next to a resource while it is in progress.
	LastError, LastWarning, LastInfoError, LastInfo, LastDebug *engine.DiagEventPayload

	// All the diagnostic events we've heard about this resource.  We'll print the last diagnostic
	// in the status region while a resource is in progress.  At the end we'll print out all
	// diagnostics for a resource.
	//
	// Diagnostic events are bucketed by their associated stream ID (with 0 being the default
	// stream)
	StreamIDToDiagPayloads map[int32][]engine.DiagEventPayload
}

type ProgressDisplay struct {
	opts           Options
	progressOutput chan<- Progress

	// action is the kind of action (preview, update, refresh, etc) being performed.
	action apitype.UpdateKind

	// Whether or not we're previewing.  We don't know what we are actually doing until
	// we get the initial 'prelude' event.
	//
	// this flag is only used to adjust how we describe what's going on to the user.
	// i.e. if we're previewing we say things like "Would update" instead of "Updating".
	isPreview bool

	// The urn of the stack.
	stackUrn resource.URN

	// Whether or not we've seen outputs for the stack yet.
	seenStackOutputs bool

	// The summary event from the engine.  If we get this, we'll print this after all
	// normal resource events are heard.  That way we don't interfere with all the progress
	// messages we're outputting for them.
	summaryEventPayload *engine.SummaryEventPayload

	// Any system events we've received.  They will be printed at the bottom of all the status rows
	systemEventPayloads []engine.StdoutEventPayload

	// Used to record the order that rows are created in.  That way, when we present in a tree, we
	// can keep things ordered so they will not jump around.
	displayOrderCounter int

	// What tick we're currently on.  Used to determine the number of ellipses to concat to
	// a status message to help indicate that things are still working.
	currentTick int

	// A spinner to use to show that we're still doing work even when no output has been
	// printed to the console in a while.
	nonInteractiveSpinner cmdutil.Spinner

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
	done bool

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

	if !display.isTerminal {
		// We're about to display something.  Reset our spinner so that it will go on the next line.
		display.nonInteractiveSpinner.Reset()
	}

	display.progressOutput <- progress
}

func (display *ProgressDisplay) writeSimpleMessage(msg string) {
	display.colorizeAndWriteProgress(makeMessageProgress(msg))
}

func (display *ProgressDisplay) writeBlankLine() {
	display.writeSimpleMessage(" ")
}

// ShowProgressEvents displays the engine events with docker's progress view.
func ShowProgressEvents(
	op string, action apitype.UpdateKind, events <-chan engine.Event, done chan<- bool, opts Options) {

	// Create a ticker that will update all our status messages once a second.  Any
	// in-flight resources will get a varying .  ..  ... ticker appended to them to
	// let the user know what is still being worked on.
	spinner, ticker := cmdutil.NewSpinnerAndTicker(
		fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op),
		nil, 1 /*timesPerSecond*/)

	// The channel we push progress messages into, and which ShowProgressOutput pulls
	// from to display to the console.
	progressOutput := make(chan Progress)

	display := &ProgressDisplay{
		action:                 action,
		opts:                   opts,
		progressOutput:         progressOutput,
		eventUrnToResourceRow:  make(map[resource.URN]ResourceRow),
		suffixColumn:           int(statusColumn),
		suffixesArray:          []string{"", ".", "..", "..."},
		urnToID:                make(map[resource.URN]string),
		colorizedToUncolorized: make(map[string]string),
		printedProgressCache:   make(map[string]Progress),
		displayOrderCounter:    1,
		nonInteractiveSpinner:  spinner,
	}

	// display.writeSimpleMessage(fmt.Sprintf("Max suffix length %v", display.maxSuffixLength))

	_, stdout, _ := term.StdStreams()

	terminalWidth, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	contract.IgnoreError(err)

	display.isTerminal = opts.IsInteractive
	display.terminalWidth = terminalWidth

	go func() {
		display.processEvents(ticker, events)

		// no more progress events from this point on.  By closing the pipe, this will then cause
		// DisplayJSONMessagesToStream to finish once it processes the last message is receives from
		// pipeReader, causing DisplayEvents to finally complete.
		close(progressOutput)
	}()

	ShowProgressOutput(progressOutput, stdout, display.isTerminal)

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

	// if it's the not the root item, attach it as a child node to an appropriate parent item.
	if urn != "" && urn != display.stackUrn {
		var parentURN resource.URN

		res := row.Step().Res
		if res != nil {
			parentURN = res.Parent
		}

		parentRow, hasParentRow := display.eventUrnToResourceRow[parentURN]

		if !hasParentRow {
			// If we haven't heard about this node's parent, then  just parent it to the stack.
			// Note: getting the parent row for the stack-urn will always succeed as we ensure that
			// such a row is always there in ensureHeaderAndStackRows
			parentURN = display.stackUrn
			parentRow = display.eventUrnToResourceRow[parentURN]
		}

		parentNode := display.getOrCreateTreeNode(result, parentURN, parentRow, urnToTreeNode)
		parentNode.childNodes = append(parentNode.childNodes, node)
		return node
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
	return sortable[i].row.DisplayOrderIndex() < sortable[j].row.DisplayOrderIndex()
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

		display.displayOrderCounter++
		node.row.SetDisplayOrderIndex(display.displayOrderCounter)
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
	// Figure out the rows that are currently in progress.
	inProgressRows := []ResourceRow{}

	for _, v := range display.eventUrnToResourceRow {
		if !v.IsDone() {
			inProgressRows = append(inProgressRows, v)
		}
	}

	// transition the display to the 'done' state.  this will transitively cause all
	// rows to become done.
	display.done = true

	// Now print out all those rows that were in progress.  They will now be 'done'
	// since the display was marked 'done'.
	if !display.isTerminal {
		for _, v := range inProgressRows {
			display.refreshSingleRow("", v, nil)
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
		for id, payloads := range row.DiagInfo().StreamIDToDiagPayloads {
			if len(payloads) > 0 {
				if id != 0 {
					// for the non-default stream merge all the messages from the stream into a single
					// message.
					p := display.mergeStreamPayloadsToSinglePayload(payloads)
					payloads = []engine.DiagEventPayload{p}
				}

				wroteResourceHeader := false
				for _, v := range payloads {
					if v.Ephemeral {
						continue
					}

					msg := display.renderProgressDiagEvent(v, true /*includePrefix:*/)

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
	}

	// If we get stack outputs, display them at the end.
	if display.stackUrn != "" && display.seenStackOutputs {
		stackStep := display.eventUrnToResourceRow[display.stackUrn].Step()
		props := engine.GetResourceOutputsPropertiesString(
			stackStep, 1, display.isPreview, display.opts.Debug, false /* refresh */)
		if props != "" {
			if !wroteDiagnosticHeader {
				display.writeBlankLine()
			}

			wroteDiagnosticHeader = true
			display.writeSimpleMessage(props)
		}
	}

	// print the summary
	if display.summaryEventPayload != nil {
		msg := renderSummaryEvent(display.action, *display.summaryEventPayload, display.opts)

		if !wroteDiagnosticHeader {
			display.writeBlankLine()
		}

		display.writeSimpleMessage(msg)
	}
}

func (display *ProgressDisplay) mergeStreamPayloadsToSinglePayload(
	payloads []engine.DiagEventPayload) engine.DiagEventPayload {
	buf := bytes.Buffer{}

	for _, p := range payloads {
		buf.WriteString(display.renderProgressDiagEvent(p, false /*includePrefix:*/))
	}

	firstPayload := payloads[0]
	msg := buf.String()
	return engine.DiagEventPayload{
		URN:       firstPayload.URN,
		Message:   msg,
		Prefix:    firstPayload.Prefix,
		Color:     firstPayload.Color,
		Severity:  firstPayload.Severity,
		StreamID:  firstPayload.StreamID,
		Ephemeral: firstPayload.Ephemeral,
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
	// Got a tick.  Update the progress display if we're in a terminal.  If we're not,
	// print a hearbeat message every 10 seconds after our last output so that the user
	// knows something is going on.  This is also helpful for hosts like jenkins that
	// often timeout a process if output is not seen in a while.
	display.currentTick++

	if display.isTerminal {
		display.refreshAllRowsIfInTerminal()
	} else {
		// Update the spinner to let the user know that that work is still happening.
		display.nonInteractiveSpinner.Tick()
	}
}

func (display *ProgressDisplay) getRowForURN(urn resource.URN, metadata *engine.StepEventMetadata) ResourceRow {
	// If there's already a row for this URN, return it.
	row, has := display.eventUrnToResourceRow[urn]
	if has {
		return row
	}

	// First time we're hearing about this resource. Create an initial nearly-empty status for it.
	step := engine.StepEventMetadata{Op: deploy.OpSame}
	if metadata != nil {
		step = *metadata
	}

	// If this is the first time we're seeing an event for the stack resource, check to see if we've already
	// recorded root events that we want to reassociate with this URN.
	if isRootURN(urn) {
		display.stackUrn = urn

		if row, has = display.eventUrnToResourceRow[""]; has {
			row.SetStep(step)
			display.eventUrnToResourceRow[urn] = row
			delete(display.eventUrnToResourceRow, "")
			return row
		}
	}

	row = &resourceRowData{
		display:              display,
		tick:                 display.currentTick,
		diagInfo:             &DiagInfo{},
		step:                 step,
		hideRowIfUnnecessary: true,
	}

	display.eventUrnToResourceRow[urn] = row

	display.ensureHeaderAndStackRows()
	display.resourceRows = append(display.resourceRows, row)
	return row
}

func (display *ProgressDisplay) processNormalEvent(event engine.Event) {
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
		msg := display.renderProgressDiagEvent(event.Payload.(engine.DiagEventPayload), true /*includePrefix:*/)
		if msg == "" {
			return
		}
	case engine.StdoutColorEvent:
		display.handleSystemEvent(event.Payload.(engine.StdoutEventPayload))
		return
	}

	// At this point, all events should relate to resources.
	eventUrn, metadata := getEventUrnAndMetadata(event)
	if eventUrn == "" {
		// If this event has no URN, associate it with the stack. Note that there may not yet be a stack resource, in
		// which case this is a no-op.
		eventUrn = display.stackUrn
	}
	isRootEvent := eventUrn == display.stackUrn

	row := display.getRowForURN(eventUrn, metadata)

	// Don't bother showing certain events (for example, things that are unchanged). However
	// always show the root 'stack' resource so we can indicate that it's still running, and
	// also so we have something to attach unparented diagnostic events to.
	hideRowIfUnnecessary := metadata != nil && !shouldShow(*metadata, display.opts) && !isRootEvent
	if !hideRowIfUnnecessary {
		row.SetHideRowIfUnnecessary(false)
	}

	if event.Type == engine.ResourcePreEvent {
		step := event.Payload.(engine.ResourcePreEventPayload).Metadata
		row.SetStep(step)
	} else if event.Type == engine.ResourceOutputsEvent {
		isRefresh := display.getStepOp(row.Step()) == deploy.OpRefresh
		step := event.Payload.(engine.ResourceOutputsEventPayload).Metadata

		// Is this the stack outputs event? If so, we'll need to print it out at the end of the plan.
		if step.URN == display.stackUrn {
			display.seenStackOutputs = true
		}

		row.SetStep(step)
		row.AddOutputStep(step)

		// If we're not in a terminal, we may not want to display this row again: if we're displaying a preview or if
		// this step is a no-op for a custom resource, refreshing this row will simply duplicate its earlier output.
		hasMeaningfulOutput := isRefresh ||
			!display.isPreview && (step.Res == nil || step.Res.Custom && step.Op != deploy.OpSame)
		if !display.isTerminal && !hasMeaningfulOutput {
			return
		}
	} else if event.Type == engine.ResourceOperationFailed {
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
	display.ensureHeaderAndStackRows()

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

func (display *ProgressDisplay) ensureHeaderAndStackRows() {
	if display.headerRow == nil {
		// about to make our first status message.  make sure we present the header line first.
		display.headerRow = &headerRowData{display: display}
	}

	// we've added at least one row to the table.  make sure we have a row to designate the
	// stack if we haven't already heard about it yet.  This also ensures that as we build
	// the tree we can always guarantee there's a 'root' to parent anything to.
	_, hasStackRow := display.eventUrnToResourceRow[display.stackUrn]
	if hasStackRow {
		return
	}

	stackRow := &resourceRowData{
		display:              display,
		tick:                 display.currentTick,
		diagInfo:             &DiagInfo{},
		step:                 engine.StepEventMetadata{Op: deploy.OpSame},
		hideRowIfUnnecessary: false,
	}

	display.eventUrnToResourceRow[display.stackUrn] = stackRow
	display.resourceRows = append(display.resourceRows, stackRow)
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

func (display *ProgressDisplay) renderProgressDiagEvent(payload engine.DiagEventPayload, includePrefix bool) string {
	if payload.Severity == diag.Debug && !display.opts.Debug {
		return ""
	}

	msg := payload.Message
	if includePrefix {
		msg = payload.Prefix + msg
	}

	return strings.TrimRightFunc(msg, unicode.IsSpace)
}

func (display *ProgressDisplay) getStepDoneDescription(step engine.StepEventMetadata, failed bool) string {
	makeError := func(v string) string {
		return colors.SpecError + "**" + v + "**" + colors.Reset
	}

	op := display.getStepOp(step)

	if display.isPreview {
		// During a preview, when we transition to done, we'll print out summary text describing the step instead of a
		// past-tense verb describing the step that was performed.
		return op.Color() + display.getPreviewDoneText(step) + colors.Reset
	}

	// most of the time a stack is unchanged.  in that case we just show it as "running->done"
	if isRootStack(step) && op == deploy.OpSame {
		return "done"
	}

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
			case deploy.OpRead, deploy.OpReadReplacement:
				return "reading failed"
			case deploy.OpRefresh:
				return "refreshing failed"
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
				return "created replacement"
			case deploy.OpDeleteReplaced:
				return "deleted original"
			case deploy.OpRead:
				return "read"
			case deploy.OpReadReplacement:
				return "read for replacement"
			case deploy.OpRefresh:
				return "refresh"
			}
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}

	if failed {
		return makeError(getDescription())
	}

	return op.Color() + getDescription() + colors.Reset
}

func (display *ProgressDisplay) getPreviewText(step engine.StepEventMetadata) string {
	switch step.Op {
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
		return "create replacement"
	case deploy.OpDeleteReplaced:
		return "delete original"
	case deploy.OpRead:
		return "read"
	case deploy.OpReadReplacement:
		return "read for replacement"
	case deploy.OpRefresh:
		return "refreshing"
	}

	contract.Failf("Unrecognized resource step op: %v", step.Op)
	return ""
}

// getPreviewDoneText returns a textual representation for this step, suitable for display during a preview once the
// preview has completed.
func (display *ProgressDisplay) getPreviewDoneText(step engine.StepEventMetadata) string {
	switch step.Op {
	case deploy.OpSame:
		return "no change"
	case deploy.OpCreate:
		return "create"
	case deploy.OpUpdate:
		return "update"
	case deploy.OpDelete:
		return "delete"
	case deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced, deploy.OpReadReplacement:
		return "replace"
	case deploy.OpRead:
		return "read"
	case deploy.OpRefresh:
		return "refresh"
	}

	contract.Failf("Unrecognized resource step op: %v", step.Op)
	return ""
}

func (display *ProgressDisplay) getStepOp(step engine.StepEventMetadata) deploy.StepOp {
	op := step.Op

	// We will commonly hear about replacements as an actual series of steps.  i.e. 'create
	// replacement', 'replace', 'delete original'.  During the actual application of these steps we
	// want to see these individual steps.  However, both before we apply all of them, and after
	// they're all done, we want to show this as a single conceptual 'replace'/'replaced' step.
	//
	// Note: in non-interactive mode we can show these all as individual steps.  This only applies
	// to interactive mode, where there is only one line shown per resource, and we want it to be as
	// clear as possible
	if display.isTerminal {
		// During preview, show the steps for replacing as a single 'replace' plan.
		// Once done, show the steps for replacing as a single 'replaced' step.
		// During update, we'll show these individual steps.
		if display.isPreview || display.done {
			if op == deploy.OpCreateReplacement || op == deploy.OpDeleteReplaced {
				return deploy.OpReplace
			}
		}
	}

	return op
}

func (display *ProgressDisplay) getStepOpLabel(step engine.StepEventMetadata) string {
	return display.getStepOp(step).Prefix() + colors.Reset
}

func (display *ProgressDisplay) getStepInProgressDescription(step engine.StepEventMetadata) string {
	op := display.getStepOp(step)

	if isRootStack(step) && op == deploy.OpSame {
		// most of the time a stack is unchanged.  in that case we just show it as "running->done".
		// otherwise, we show what is actually happening to it.
		return "running"
	}

	getDescription := func() string {
		if display.isPreview {
			return display.getPreviewText(step)
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
			return "creating replacement"
		case deploy.OpDeleteReplaced:
			return "deleting original"
		case deploy.OpRead:
			return "reading"
		case deploy.OpReadReplacement:
			return "reading for replacement"
		case deploy.OpRefresh:
			return "refreshing"
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
	return op.Color() + getDescription() + colors.Reset
}

func writePropertyKeys(b *bytes.Buffer, propMap resource.PropertyMap, op deploy.StepOp) {
	if len(propMap) > 0 {
		writeString(b, " ")
		writeString(b, op.Prefix())

		keys := make([]string, 0, len(propMap))
		for k := range propMap {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)

		index := 0
		for _, k := range keys {
			if index != 0 {
				writeString(b, ",")
			}
			writeString(b, k)
			index++
		}

		writeString(b, colors.Reset)
	}
}

func writeString(b *bytes.Buffer, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}
