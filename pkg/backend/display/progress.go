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

// nolint: goconst
package display

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/moby/term"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	sdkDisplay "github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

// DiagInfo contains the bundle of diagnostic information for a single resource.
type DiagInfo struct {
	ErrorCount, WarningCount, InfoCount, DebugCount int

	// The very last diagnostic event we got for this resource (regardless of severity). We'll print
	// this out in the non-interactive mode whenever we get new events. Importantly, we don't want
	// to print out the most significant diagnostic, as that means a flurry of event swill cause us
	// to keep printing out the most significant diagnostic over and over again.
	LastDiag *sdkDisplay.DiagEventPayload

	// The last error we received.  If we have an error, and we're in tree-view, we'll prefer to
	// show this over the last non-error diag so that users know about something bad early on.
	LastError *sdkDisplay.DiagEventPayload

	// All the diagnostic events we've heard about this resource.  We'll print the last diagnostic
	// in the status region while a resource is in progress.  At the end we'll print out all
	// diagnostics for a resource.
	//
	// Diagnostic events are bucketed by their associated stream ID (with 0 being the default
	// stream).
	StreamIDToDiagPayloads map[int32][]sdkDisplay.DiagEventPayload
}

// ProgressDisplay organizes all the information needed for a dynamically updated "progress" view of an update.
type ProgressDisplay struct {
	opts           Options
	progressOutput chan<- Progress

	// action is the kind of action (preview, update, refresh, etc) being performed.
	action apitype.UpdateKind
	// stack is the stack this progress pertains to.
	stack tokens.Name
	// proj is the project this progress pertains to.
	proj tokens.PackageName

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

	// The summary event from the sdkDisplay.  If we get this, we'll print this after all
	// normal resource events are heard.  That way we don't interfere with all the progress
	// messages we're outputting for them.
	summaryEventPayload *sdkDisplay.SummaryEventPayload

	// Any system events we've received.  They will be printed at the bottom of all the status rows
	systemEventPayloads []sdkDisplay.StdoutEventPayload

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

	// The width and height of the terminal.  Used so we can trim resource messages that are too long.
	terminalWidth  int
	terminalHeight int

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
	// policyPayloads is a collection of policy violation events for a single resource.
	policyPayloads []sdkDisplay.PolicyViolationEventPayload
)

func camelCase(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func simplifyTypeName(typ tokens.Type) string {
	typeString := string(typ)

	components := strings.Split(typeString, ":")
	if len(components) != 3 {
		return typeString
	}
	pkg, module, name := components[0], components[1], components[2]

	if len(name) == 0 {
		return typeString
	}

	lastSlashInModule := strings.LastIndexByte(module, '/')
	if lastSlashInModule == -1 {
		return typeString
	}
	file := module[lastSlashInModule+1:]

	if file != camelCase(name) {
		return typeString
	}

	return fmt.Sprintf("%v:%v:%v", pkg, module[:lastSlashInModule], name)
}

// getEventUrn returns the resource URN associated with an event, or the empty URN if this is not an
// event that has a URN.  If this is also a 'step' event, then this will return the step metadata as
// well.
func getEventUrnAndMetadata(event sdkDisplay.Event) (resource.URN, *sdkDisplay.StepEventMetadata) {
	switch event.Type {
	case sdkDisplay.ResourcePreEvent:
		payload := event.Payload().(sdkDisplay.ResourcePreEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	case sdkDisplay.ResourceOutputsEvent:
		payload := event.Payload().(sdkDisplay.ResourceOutputsEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	case sdkDisplay.ResourceOperationFailed:
		payload := event.Payload().(sdkDisplay.ResourceOperationFailedPayload)
		return payload.Metadata.URN, &payload.Metadata
	case sdkDisplay.DiagEvent:
		return event.Payload().(sdkDisplay.DiagEventPayload).URN, nil
	case sdkDisplay.PolicyViolationEvent:
		return event.Payload().(sdkDisplay.PolicyViolationEventPayload).ResourceURN, nil
	default:
		return "", nil
	}
}

// Converts the colorization tags in a progress message and then actually writes the progress
// message to the output stream.  This should be the only place in this file where we actually
// process colorization tags.
func (pd *ProgressDisplay) colorizeAndWriteProgress(progress Progress) {
	if progress.Message != "" {
		progress.Message = pd.opts.Color.Colorize(progress.Message)
	}

	if progress.Action != "" {
		progress.Action = pd.opts.Color.Colorize(progress.Action)
	}

	if progress.ID != "" {
		// don't repeat the same output if there is no difference between the last time we
		// printed it and now.
		lastProgress, has := pd.printedProgressCache[progress.ID]
		if has && lastProgress.Message == progress.Message && lastProgress.Action == progress.Action {
			return
		}

		pd.printedProgressCache[progress.ID] = progress
	}

	if !pd.isTerminal {
		// We're about to display something.  Reset our spinner so that it will go on the next line.
		pd.nonInteractiveSpinner.Reset()
	}

	pd.progressOutput <- progress
}

func (pd *ProgressDisplay) writeSimpleMessage(msg string) {
	pd.colorizeAndWriteProgress(makeMessageProgress(msg))
}

func (pd *ProgressDisplay) writeBlankLine() {
	pd.writeSimpleMessage(" ")
}

// ShowProgressEvents displays the engine events with docker's progress view.
func showProgressEvents(op string, action apitype.UpdateKind, stack tokens.Name, proj tokens.PackageName,
	events <-chan sdkDisplay.Event, done chan<- bool, opts Options, isPreview bool) {

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Create a ticker that will update all our status messages once a second.  Any
	// in-flight resources will get a varying .  ..  ... ticker appended to them to
	// let the user know what is still being worked on.
	var spinner cmdutil.Spinner
	var ticker *time.Ticker
	if stdout == os.Stdout && stderr == os.Stderr {
		spinner, ticker = cmdutil.NewSpinnerAndTicker(
			fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op),
			nil, opts.Color, 1 /*timesPerSecond*/)
	} else {
		spinner = &nopSpinner{}
		ticker = time.NewTicker(math.MaxInt64)
	}

	// The channel we push progress messages into, and which ShowProgressOutput pulls
	// from to display to the console.
	progressOutput := make(chan Progress)

	display := &ProgressDisplay{
		action:                 action,
		isPreview:              isPreview,
		opts:                   opts,
		stack:                  stack,
		proj:                   proj,
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

	// Assume we are not displaying in a terminal by default.
	display.isTerminal = false
	if stdout == os.Stdout {
		terminalWidth, terminalHeight, err := terminal.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			// If the terminal has a size, use it.
			display.isTerminal = opts.IsInteractive
			display.terminalWidth = terminalWidth
			display.terminalHeight = terminalHeight

			// Don't bother attempting to treat this display as a terminal if it has no width/height.
			if display.isTerminal && (display.terminalWidth == 0 || display.terminalHeight == 0) {
				display.isTerminal = false
				_, err = fmt.Fprintln(stderr, "Treating display as non-terminal due to 0 width/height.")
				contract.IgnoreError(err)
			}

			// Fetch the canonical stdout stream, configured appropriately.
			_, stdout, _ = term.StdStreams()
		}
	}

	go func() {
		display.processEvents(ticker, events)

		// no more progress events from this point on.  By closing the pipe, this will then cause
		// DisplayJSONMessagesToStream to finish once it processes the last message is receives from
		// pipeReader, causing DisplayEvents to finally complete.
		close(progressOutput)
	}()

	showProgressOutput(progressOutput, stdout, display.isTerminal)

	ticker.Stop()

	// let our caller know we're done.
	close(done)
}

// Gets the padding necessary to prepend to a message in order to keep it aligned in the
// terminal.
func (pd *ProgressDisplay) getMessagePadding(
	uncolorizedColumns []string, columnIndex int, maxColumnLengths []int) string {

	extraWhitespace := " "
	if columnIndex >= 0 && pd.isTerminal {
		column := uncolorizedColumns[columnIndex]
		maxLength := maxColumnLengths[columnIndex]
		extraWhitespace = messagePadding(column, maxLength, 2)
	}

	return extraWhitespace
}

// Gets the fully padded message to be shown.  The message will always include the ID of the
// status, then some amount of optional padding, then some amount of msgWithColors, then the
// suffix.  Importantly, if there isn't enough room to display all of that on the terminal, then
// the msg will be truncated to try to make it fit.
func (pd *ProgressDisplay) getPaddedMessage(
	colorizedColumns, uncolorizedColumns []string, maxColumnLengths []int) string {

	colorizedMessage := ""

	for i := 0; i < len(colorizedColumns); i++ {
		padding := pd.getMessagePadding(uncolorizedColumns, i-1, maxColumnLengths)
		colorizedMessage += padding + colorizedColumns[i]
	}

	if pd.isTerminal {
		// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
		// msgWithColors having the color code information embedded with it.  So we need to get
		// the right substring of it, assuming that embedded colors are just markup and do not
		// actually contribute to the length
		maxMsgLength := pd.terminalWidth - 1
		if maxMsgLength < 0 {
			maxMsgLength = 0
		}

		colorizedMessage = colors.TrimColorizedString(colorizedMessage, maxMsgLength)
	}

	return colorizedMessage
}

func (pd *ProgressDisplay) uncolorizeString(v string) string {
	uncolorized, has := pd.colorizedToUncolorized[v]
	if !has {
		uncolorized = colors.Never.Colorize(v)
		pd.colorizedToUncolorized[v] = uncolorized
	}

	return uncolorized
}

func (pd *ProgressDisplay) uncolorizeColumns(columns []string) []string {
	uncolorizedColumns := make([]string, len(columns))

	for i, v := range columns {
		uncolorizedColumns[i] = pd.uncolorizeString(v)
	}

	return uncolorizedColumns
}

func (pd *ProgressDisplay) refreshSingleRow(id string, row Row, maxColumnLengths []int) {
	colorizedColumns := row.ColorizedColumns()
	colorizedColumns[pd.suffixColumn] += row.ColorizedSuffix()
	pd.refreshColumns(id, colorizedColumns, maxColumnLengths)
}

func (pd *ProgressDisplay) refreshColumns(
	id string, colorizedColumns []string, maxColumnLengths []int) {

	uncolorizedColumns := pd.uncolorizeColumns(colorizedColumns)

	msg := pd.getPaddedMessage(colorizedColumns, uncolorizedColumns, maxColumnLengths)

	// getPaddedmessage trims our message and potentially trims it to "" which will trigger
	// asserts in later display functions if we try to show it
	if msg != "" {
		if pd.isTerminal {
			pd.colorizeAndWriteProgress(makeActionProgress(id, msg))
		} else {
			pd.writeSimpleMessage(msg)
		}
	}
}

// Ensure our stored dimension info is up to date.
func (pd *ProgressDisplay) updateTerminalDimensions() {
	// don't do any refreshing if we're not in a terminal
	if pd.isTerminal {
		currentTerminalWidth, currentTerminalHeight, err := terminal.GetSize(int(os.Stdout.Fd()))
		contract.IgnoreError(err)

		if currentTerminalWidth != pd.terminalWidth ||
			currentTerminalHeight != pd.terminalHeight {
			pd.terminalWidth = currentTerminalWidth
			pd.terminalHeight = currentTerminalHeight

			// also clear our display cache as we want to reprint all lines.
			pd.printedProgressCache = make(map[string]Progress)
		}
	}
}

type treeNode struct {
	row Row

	colorizedColumns []string
	colorizedSuffix  string

	childNodes []*treeNode
}

func (pd *ProgressDisplay) getOrCreateTreeNode(
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
	if urn != "" && urn != pd.stackUrn {
		var parentURN resource.URN

		res := row.Step().Res
		if res != nil {
			parentURN = res.Parent
		}

		parentRow, hasParentRow := pd.eventUrnToResourceRow[parentURN]

		if !hasParentRow {
			// If we haven't heard about this node's parent, then  just parent it to the stack.
			// Note: getting the parent row for the stack-urn will always succeed as we ensure that
			// such a row is always there in ensureHeaderAndStackRows
			parentURN = pd.stackUrn
			parentRow = pd.eventUrnToResourceRow[parentURN]
		}

		parentNode := pd.getOrCreateTreeNode(result, parentURN, parentRow, urnToTreeNode)
		parentNode.childNodes = append(parentNode.childNodes, node)
		return node
	}

	*result = append(*result, node)
	return node
}

func (pd *ProgressDisplay) generateTreeNodes() []*treeNode {
	result := []*treeNode{}

	result = append(result, &treeNode{
		row:              pd.headerRow,
		colorizedColumns: pd.headerRow.ColorizedColumns(),
	})

	urnToTreeNode := make(map[resource.URN]*treeNode)
	for urn, row := range pd.eventUrnToResourceRow {
		pd.getOrCreateTreeNode(&result, urn, row, urnToTreeNode)
	}

	return result
}

func (pd *ProgressDisplay) addIndentations(treeNodes []*treeNode, isRoot bool, indentation string) {

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
		pd.addIndentations(node.childNodes, false /*isRoot*/, nestedIndentation)
	}
}

func (pd *ProgressDisplay) convertNodesToRows(
	nodes []*treeNode, maxSuffixLength int, rows *[][]string, maxColumnLengths *[]int) {

	for _, node := range nodes {
		if len(*maxColumnLengths) == 0 {
			*maxColumnLengths = make([]int, len(node.colorizedColumns))
		}

		colorizedColumns := make([]string, len(node.colorizedColumns))
		uncolorisedColumns := pd.uncolorizeColumns(node.colorizedColumns)

		for i, colorizedColumn := range node.colorizedColumns {
			columnWidth := utf8.RuneCountInString(uncolorisedColumns[i])

			if i == pd.suffixColumn {
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

		pd.convertNodesToRows(node.childNodes, maxSuffixLength, rows, maxColumnLengths)
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

func (pd *ProgressDisplay) filterOutUnnecessaryNodesAndSetDisplayTimes(nodes []*treeNode) []*treeNode {
	result := []*treeNode{}

	for _, node := range nodes {
		node.childNodes = pd.filterOutUnnecessaryNodesAndSetDisplayTimes(node.childNodes)

		if node.row.HideRowIfUnnecessary() && len(node.childNodes) == 0 {
			continue
		}

		pd.displayOrderCounter++
		node.row.SetDisplayOrderIndex(pd.displayOrderCounter)
		result = append(result, node)
	}

	return result
}

func (pd *ProgressDisplay) refreshAllRowsIfInTerminal() {
	if pd.isTerminal && pd.headerRow != nil {
		// make sure our stored dimension info is up to date
		pd.updateTerminalDimensions()

		rootNodes := pd.generateTreeNodes()
		rootNodes = pd.filterOutUnnecessaryNodesAndSetDisplayTimes(rootNodes)
		sortNodes(rootNodes)
		pd.addIndentations(rootNodes, true /*isRoot*/, "")

		maxSuffixLength := 0
		for _, v := range pd.suffixesArray {
			runeCount := utf8.RuneCountInString(v)
			if runeCount > maxSuffixLength {
				maxSuffixLength = runeCount
			}
		}

		var rows [][]string
		var maxColumnLengths []int
		pd.convertNodesToRows(rootNodes, maxSuffixLength, &rows, &maxColumnLengths)

		removeInfoColumnIfUnneeded(rows)

		for i, row := range rows {
			pd.refreshColumns(fmt.Sprintf("%v", i), row, maxColumnLengths)
		}

		systemID := len(rows)

		printedHeader := false
		for _, payload := range pd.systemEventPayloads {
			msg := payload.Color.Colorize(payload.Message)
			lines := splitIntoDisplayableLines(msg)

			if len(lines) == 0 {
				continue
			}

			if !printedHeader {
				printedHeader = true
				pd.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID), " "))
				systemID++

				pd.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID),
					colors.Yellow+"System Messages"+colors.Reset))
				systemID++
			}

			for _, line := range lines {
				pd.colorizeAndWriteProgress(makeActionProgress(
					fmt.Sprintf("%v", systemID), fmt.Sprintf("  %s", line)))
				systemID++
			}
		}
	}
}

func removeInfoColumnIfUnneeded(rows [][]string) {
	// If there have been no info messages, then don't print out the info column header.
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if row[len(row)-1] != "" {
			return
		}
	}

	firstRow := rows[0]
	firstRow[len(firstRow)-1] = ""
}

// Performs all the work at the end once we've heard about the last message from the sdkDisplay.
// Specifically, this will update the status messages for any resources, and will also then
// print out all final diagnostics. and finally will print out the summary.
func (pd *ProgressDisplay) processEndSteps() {
	// Figure out the rows that are currently in progress.
	inProgressRows := []ResourceRow{}

	for _, v := range pd.eventUrnToResourceRow {
		if !v.IsDone() {
			inProgressRows = append(inProgressRows, v)
		}
	}

	// Transition the display to the 'done' state.  This will transitively cause all
	// rows to become done.
	pd.done = true

	// Now print out all those rows that were in progress.  They will now be 'done'
	// since the display was marked 'done'.
	if !pd.isTerminal {
		for _, v := range inProgressRows {
			pd.refreshSingleRow("", v, nil)
		}
	}

	// Now refresh everything.  This ensures that we go back and remove things like the diagnostic
	// messages from a status message (since we're going to print them all) below.  Note, this will
	// only do something in a terminal.  This is what we want, because if we're not in a terminal we
	// don't really want to reprint any finished items we've already printed.
	pd.refreshAllRowsIfInTerminal()

	// Render several "sections" of output based on available data as applicable.
	pd.writeBlankLine()
	wroteDiagnosticHeader := pd.printDiagnostics()
	wrotePolicyViolations := pd.printPolicyViolations()
	pd.printOutputs()
	// If no policies violated, print policy packs applied.
	if !wrotePolicyViolations {
		pd.printSummary(wroteDiagnosticHeader)
	}
}

// printDiagnostics prints a new "Diagnostics:" section with all of the diagnostics grouped by
// resource. If no diagnostics were emitted, prints nothing.
func (pd *ProgressDisplay) printDiagnostics() bool {
	// Since we display diagnostic information eagerly, we need to keep track of the first
	// time we wrote some output so we don't inadvertently print the header twice.
	wroteDiagnosticHeader := false
	for _, row := range pd.eventUrnToResourceRow {
		// The header for the diagnogistics grouped by resource, e.g. "aws:apigateway:RestApi (accountsApi):"
		wroteResourceHeader := false

		// Each row in the display corresponded with a resource, and that resource could have emitted
		// diagnostics to various streams.
		for id, payloads := range row.DiagInfo().StreamIDToDiagPayloads {
			if len(payloads) == 0 {
				continue
			}

			if id != 0 {
				// For the non-default stream merge all the messages from the stream into a single
				// message.
				p := pd.mergeStreamPayloadsToSinglePayload(payloads)
				payloads = []sdkDisplay.DiagEventPayload{p}
			}

			// Did we write any diagnostic information for the resource x stream?
			wrote := false
			for _, v := range payloads {
				if v.Ephemeral {
					continue
				}

				msg := pd.renderProgressDiagEvent(v, true /*includePrefix:*/)

				lines := splitIntoDisplayableLines(msg)
				if len(lines) == 0 {
					continue
				}

				// If we haven't printed the Diagnostics header, do so now.
				if !wroteDiagnosticHeader {
					wroteDiagnosticHeader = true
					pd.writeSimpleMessage(
						pd.opts.Color.Colorize(colors.SpecHeadline + "Diagnostics:" + colors.Reset))
				}
				// If we haven't printed the header for the resource, do so now.
				if !wroteResourceHeader {
					wroteResourceHeader = true
					columns := row.ColorizedColumns()
					pd.writeSimpleMessage("  " +
						pd.opts.Color.Colorize(
							colors.BrightBlue+columns[typeColumn]+" ("+columns[nameColumn]+"):"+colors.Reset))
				}

				for _, line := range lines {
					line = strings.TrimRightFunc(line, unicode.IsSpace)
					pd.writeSimpleMessage("    " + line)
				}

				wrote = true
			}

			if wrote {
				pd.writeBlankLine()
			}
		}

	}
	return wroteDiagnosticHeader
}

// printPolicyViolations prints a new "Policy Violation:" section with all of the violations
// grouped by policy pack. If no policy violations were encountered, prints nothing.
func (pd *ProgressDisplay) printPolicyViolations() bool {
	// Loop through every resource and gather up all policy violations encountered.
	var policyEvents []sdkDisplay.PolicyViolationEventPayload
	for _, row := range pd.eventUrnToResourceRow {
		policyPayloads := row.PolicyPayloads()
		if len(policyPayloads) == 0 {
			continue
		}
		policyEvents = append(policyEvents, policyPayloads...)
	}
	if len(policyEvents) == 0 {
		return false
	}
	// Sort policy events by: policy pack name, policy pack version, enforcement level,
	// policy name, and finally the URN of the resource.
	sort.SliceStable(policyEvents, func(i, j int) bool {
		eventI, eventJ := policyEvents[i], policyEvents[j]
		if packNameCmp := strings.Compare(
			eventI.PolicyPackName,
			eventJ.PolicyPackName); packNameCmp != 0 {
			return packNameCmp < 0
		}
		if packVerCmp := strings.Compare(
			eventI.PolicyPackVersion,
			eventJ.PolicyPackVersion); packVerCmp != 0 {
			return packVerCmp < 0
		}
		if enfLevelCmp := strings.Compare(
			string(eventI.EnforcementLevel),
			string(eventJ.EnforcementLevel)); enfLevelCmp != 0 {
			return enfLevelCmp < 0
		}
		if policyNameCmp := strings.Compare(
			eventI.PolicyName,
			eventJ.PolicyName); policyNameCmp != 0 {
			return policyNameCmp < 0
		}
		urnCmp := strings.Compare(
			string(eventI.ResourceURN),
			string(eventJ.ResourceURN))
		return urnCmp < 0
	})

	// Print every policy violation, printing a new header when necessary.
	pd.writeSimpleMessage(pd.opts.Color.Colorize(colors.SpecHeadline + "Policy Violations:" + colors.Reset))

	for _, policyEvent := range policyEvents {
		// Print the individual policy event.
		c := colors.SpecImportant
		if policyEvent.EnforcementLevel == apitype.Mandatory {
			c = colors.SpecError
		}

		policyNameLine := fmt.Sprintf("    %s[%s]  %s v%s %s %s (%s: %s)",
			c, policyEvent.EnforcementLevel,
			policyEvent.PolicyPackName,
			policyEvent.PolicyPackVersion, colors.Reset,
			policyEvent.PolicyName,
			policyEvent.ResourceURN.Type(),
			policyEvent.ResourceURN.Name())
		pd.writeSimpleMessage(policyNameLine)

		// The message may span multiple lines, so we massage it so it will be indented properly.
		message := strings.ReplaceAll(policyEvent.Message, "\n", "\n    ")
		messageLine := fmt.Sprintf("    %s", message)
		pd.writeSimpleMessage(messageLine)
	}
	return true
}

// printOutputs prints the Stack's outputs for the display in a new section, if appropriate.
func (pd *ProgressDisplay) printOutputs() {
	// Printing the stack's outputs wasn't desired.
	if pd.opts.SuppressOutputs {
		return
	}
	// Cannot display outputs for the stack if we don't know its URN.
	if pd.stackUrn == "" {
		return
	}

	stackStep := pd.eventUrnToResourceRow[pd.stackUrn].Step()

	props := getResourceOutputsPropertiesString(
		stackStep, 1, pd.isPreview, pd.opts.Debug,
		false /* refresh */, pd.opts.ShowSameResources)
	if props != "" {
		pd.writeSimpleMessage(colors.SpecHeadline + "Outputs:" + colors.Reset)
		pd.writeSimpleMessage(props)
	}
}

// printSummary prints the Stack's SummaryEvent in a new section if applicable.
func (pd *ProgressDisplay) printSummary(wroteDiagnosticHeader bool) {
	// If we never saw the SummaryEvent payload, we have nothing to do.
	if pd.summaryEventPayload == nil {
		return
	}

	msg := renderSummaryEvent(*pd.summaryEventPayload, wroteDiagnosticHeader, pd.opts)
	pd.writeSimpleMessage(msg)
}

func (pd *ProgressDisplay) mergeStreamPayloadsToSinglePayload(
	payloads []sdkDisplay.DiagEventPayload) sdkDisplay.DiagEventPayload {
	buf := bytes.Buffer{}

	for _, p := range payloads {
		buf.WriteString(pd.renderProgressDiagEvent(p, false /*includePrefix:*/))
	}

	firstPayload := payloads[0]
	msg := buf.String()
	return sdkDisplay.DiagEventPayload{
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

func (pd *ProgressDisplay) processTick() {
	// Got a tick.  Update the progress display if we're in a terminal.  If we're not,
	// print a hearbeat message every 10 seconds after our last output so that the user
	// knows something is going on.  This is also helpful for hosts like jenkins that
	// often timeout a process if output is not seen in a while.
	pd.currentTick++

	if pd.isTerminal {
		pd.refreshAllRowsIfInTerminal()
	} else {
		// Update the spinner to let the user know that that work is still happening.
		pd.nonInteractiveSpinner.Tick()
	}
}

func (pd *ProgressDisplay) getRowForURN(urn resource.URN, metadata *sdkDisplay.StepEventMetadata) ResourceRow {
	// If there's already a row for this URN, return it.
	row, has := pd.eventUrnToResourceRow[urn]
	if has {
		return row
	}

	// First time we're hearing about this resource. Create an initial nearly-empty status for it.
	step := sdkDisplay.StepEventMetadata{URN: urn, Op: deploy.OpSame}
	if metadata != nil {
		step = *metadata
	}

	// If this is the first time we're seeing an event for the stack resource, check to see if we've already
	// recorded root events that we want to reassociate with this URN.
	if isRootURN(urn) {
		pd.stackUrn = urn

		if row, has = pd.eventUrnToResourceRow[""]; has {
			row.SetStep(step)
			pd.eventUrnToResourceRow[urn] = row
			delete(pd.eventUrnToResourceRow, "")
			return row
		}
	}

	row = &resourceRowData{
		display:              pd,
		tick:                 pd.currentTick,
		diagInfo:             &DiagInfo{},
		policyPayloads:       policyPayloads,
		step:                 step,
		hideRowIfUnnecessary: true,
	}

	pd.eventUrnToResourceRow[urn] = row

	pd.ensureHeaderAndStackRows()
	pd.resourceRows = append(pd.resourceRows, row)
	return row
}

func (pd *ProgressDisplay) processNormalEvent(event sdkDisplay.Event) {
	switch event.Type {
	case sdkDisplay.PreludeEvent:
		// A prelude event can just be printed out directly to the console.
		// Note: we should probably make sure we don't get any prelude events
		// once we start hearing about actual resource events.
		payload := event.Payload().(sdkDisplay.PreludeEventPayload)
		preludeEventString := renderPreludeEvent(payload, pd.opts)
		if pd.isTerminal {
			pd.processNormalEvent(sdkDisplay.NewEvent(sdkDisplay.DiagEvent, sdkDisplay.DiagEventPayload{
				Ephemeral: false,
				Severity:  diag.Info,
				Color:     cmdutil.GetGlobalColorization(),
				Message:   preludeEventString,
			}))
		} else {
			pd.writeSimpleMessage(preludeEventString)
		}
		return
	case sdkDisplay.SummaryEvent:
		// keep track of the summary event so that we can display it after all other
		// resource-related events we receive.
		payload := event.Payload().(sdkDisplay.SummaryEventPayload)
		pd.summaryEventPayload = &payload
		return
	case sdkDisplay.DiagEvent:
		msg := pd.renderProgressDiagEvent(event.Payload().(sdkDisplay.DiagEventPayload), true /*includePrefix:*/)
		if msg == "" {
			return
		}
	case sdkDisplay.StdoutColorEvent:
		pd.handleSystemEvent(event.Payload().(sdkDisplay.StdoutEventPayload))
		return
	}

	// At this point, all events should relate to resources.
	eventUrn, metadata := getEventUrnAndMetadata(event)

	// If we're suppressing reads from the tree-view, then convert notifications about reads into
	// ephemeral messages that will go into the info column.
	if metadata != nil && !pd.opts.ShowReads {
		if metadata.Op == deploy.OpReadDiscard || metadata.Op == deploy.OpReadReplacement {
			// just flat out ignore read discards/replace.  They're only relevant in the context of
			// 'reads', and we only present reads as an ephemeral diagnostic anyways.
			return
		}

		if metadata.Op == deploy.OpRead {
			// Don't show reads as operations on a specific resource.  It's an underlying detail
			// that we don't want to clutter up the display with.  However, to help users know
			// what's going on, we can show them as ephemeral diagnostic messages that are
			// associated at the top level with the stack.  That way if things are taking a while,
			// there's insight in the display as to what's going on.
			pd.processNormalEvent(sdkDisplay.NewEvent(sdkDisplay.DiagEvent, sdkDisplay.DiagEventPayload{
				Ephemeral: true,
				Severity:  diag.Info,
				Color:     cmdutil.GetGlobalColorization(),
				Message:   fmt.Sprintf("read %v %v", simplifyTypeName(eventUrn.Type()), eventUrn.Name()),
			}))
			return
		}
	}

	if eventUrn == "" {
		// If this event has no URN, associate it with the stack. Note that there may not yet be a stack resource, in
		// which case this is a no-op.
		eventUrn = pd.stackUrn
	}
	isRootEvent := eventUrn == pd.stackUrn

	row := pd.getRowForURN(eventUrn, metadata)

	// Don't bother showing certain events (for example, things that are unchanged). However
	// always show the root 'stack' resource so we can indicate that it's still running, and
	// also so we have something to attach unparented diagnostic events to.
	hideRowIfUnnecessary := metadata != nil && !shouldShow(*metadata, pd.opts) && !isRootEvent
	// Always show row if there's a policy violation event. Policy violations prevent resource
	// registration, so if we don't show the row, the violation gets attributed to the stack
	// resource rather than the resources whose policy failed.
	hideRowIfUnnecessary = hideRowIfUnnecessary || event.Type == sdkDisplay.PolicyViolationEvent
	if !hideRowIfUnnecessary {
		row.SetHideRowIfUnnecessary(false)
	}

	if event.Type == sdkDisplay.ResourcePreEvent {
		step := event.Payload().(sdkDisplay.ResourcePreEventPayload).Metadata
		row.SetStep(step)
	} else if event.Type == sdkDisplay.ResourceOutputsEvent {
		isRefresh := pd.getStepOp(row.Step()) == deploy.OpRefresh
		step := event.Payload().(sdkDisplay.ResourceOutputsEventPayload).Metadata

		// Is this the stack outputs event? If so, we'll need to print it out at the end of the plan.
		if step.URN == pd.stackUrn {
			pd.seenStackOutputs = true
		}

		row.SetStep(step)
		row.AddOutputStep(step)

		// If we're not in a terminal, we may not want to display this row again: if we're displaying a preview or if
		// this step is a no-op for a custom resource, refreshing this row will simply duplicate its earlier output.
		hasMeaningfulOutput := isRefresh ||
			!pd.isPreview && (step.Res == nil || step.Res.Custom && step.Op != deploy.OpSame)
		if !pd.isTerminal && !hasMeaningfulOutput {
			return
		}
	} else if event.Type == sdkDisplay.ResourceOperationFailed {
		row.SetFailed()
	} else if event.Type == sdkDisplay.DiagEvent {
		// also record this diagnostic so we print it at the end.
		row.RecordDiagEvent(event)
	} else if event.Type == sdkDisplay.PolicyViolationEvent {
		// also record this policy violation so we print it at the end.
		row.RecordPolicyViolationEvent(event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	if pd.isTerminal {
		// if we're in a terminal, then refresh everything so that all our columns line up
		pd.refreshAllRowsIfInTerminal()
	} else {
		// otherwise, just print out this single row.
		pd.refreshSingleRow("", row, nil)
	}
}

func (pd *ProgressDisplay) handleSystemEvent(payload sdkDisplay.StdoutEventPayload) {
	// Make sure we have a header to display
	pd.ensureHeaderAndStackRows()

	pd.systemEventPayloads = append(pd.systemEventPayloads, payload)

	if pd.isTerminal {
		// if we're in a terminal, then refresh everything.  The system events will come after
		// all the normal rows
		pd.refreshAllRowsIfInTerminal()
	} else {
		// otherwise, in a non-terminal, just print out the actual event.
		pd.writeSimpleMessage(renderStdoutColorEvent(payload, pd.opts))
	}
}

func (pd *ProgressDisplay) ensureHeaderAndStackRows() {
	if pd.headerRow == nil {
		// about to make our first status message.  make sure we present the header line first.
		pd.headerRow = &headerRowData{display: pd}
	}

	// we've added at least one row to the table.  make sure we have a row to designate the
	// stack if we haven't already heard about it yet.  This also ensures that as we build
	// the tree we can always guarantee there's a 'root' to parent anything to.
	_, hasStackRow := pd.eventUrnToResourceRow[pd.stackUrn]
	if hasStackRow {
		return
	}

	stackRow := &resourceRowData{
		display:              pd,
		tick:                 pd.currentTick,
		diagInfo:             &DiagInfo{},
		policyPayloads:       policyPayloads,
		step:                 sdkDisplay.StepEventMetadata{Op: deploy.OpSame},
		hideRowIfUnnecessary: false,
	}

	pd.eventUrnToResourceRow[pd.stackUrn] = stackRow
	pd.resourceRows = append(pd.resourceRows, stackRow)
}

func (pd *ProgressDisplay) processEvents(ticker *time.Ticker, events <-chan sdkDisplay.Event) {
	// Main processing loop.  The purpose of this func is to read in events from the engine
	// and translate them into Status objects and progress messages to be presented to the
	// command line.
	for {
		select {
		case <-ticker.C:
			pd.processTick()

		case event := <-events:
			if event.Type == "" || event.Type == sdkDisplay.CancelEvent {
				// Engine finished sending events.  Do all the final processing and return
				// from this local func.  This will print out things like full diagnostic
				// events, as well as the summary event from the sdkDisplay.
				pd.processEndSteps()
				return
			}

			pd.processNormalEvent(event)
		}
	}
}

func (pd *ProgressDisplay) renderProgressDiagEvent(payload sdkDisplay.DiagEventPayload, includePrefix bool) string {
	if payload.Severity == diag.Debug && !pd.opts.Debug {
		return ""
	}

	msg := payload.Message
	if includePrefix {
		msg = payload.Prefix + msg
	}

	return strings.TrimRightFunc(msg, unicode.IsSpace)
}

func (pd *ProgressDisplay) getStepDoneDescription(step sdkDisplay.StepEventMetadata, failed bool) string {
	makeError := func(v string) string {
		return colors.SpecError + "**" + v + "**" + colors.Reset
	}

	op := pd.getStepOp(step)

	if pd.isPreview {
		// During a preview, when we transition to done, we'll print out summary text describing the step instead of a
		// past-tense verb describing the step that was performed.
		return deploy.Color(op) + pd.getPreviewDoneText(step) + colors.Reset
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
			case deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				return "discarding failed"
			case deploy.OpImport, deploy.OpImportReplacement:
				return "importing failed"
			}
		} else {
			switch op {
			case deploy.OpSame:
				return ""
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
				// nolint: goconst
				return "read"
			case deploy.OpReadReplacement:
				return "read for replacement"
			case deploy.OpRefresh:
				return "refresh"
			case deploy.OpReadDiscard:
				return "discarded"
			case deploy.OpDiscardReplaced:
				return "discarded original"
			case deploy.OpImport:
				return "imported"
			case deploy.OpImportReplacement:
				return "imported replacement"
			}
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}

	if failed {
		return makeError(getDescription())
	}

	return deploy.Color(op) + getDescription() + colors.Reset
}

func (pd *ProgressDisplay) getPreviewText(step sdkDisplay.StepEventMetadata) string {
	switch step.Op {
	case deploy.OpSame:
		return ""
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
		// nolint: goconst
		return "read"
	case deploy.OpReadReplacement:
		return "read for replacement"
	case deploy.OpRefresh:
		return "refreshing"
	case deploy.OpReadDiscard:
		return "discard"
	case deploy.OpDiscardReplaced:
		return "discard original"
	case deploy.OpImport:
		return "import"
	case deploy.OpImportReplacement:
		return "import replacement"
	}

	contract.Failf("Unrecognized resource step op: %v", step.Op)
	return ""
}

// getPreviewDoneText returns a textual representation for this step, suitable for display during a preview once the
// preview has completed.
func (pd *ProgressDisplay) getPreviewDoneText(step sdkDisplay.StepEventMetadata) string {
	switch step.Op {
	case deploy.OpSame:
		return ""
	case deploy.OpCreate:
		return "create"
	case deploy.OpUpdate:
		return "update"
	case deploy.OpDelete:
		return "delete"
	case deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced, deploy.OpReadReplacement,
		deploy.OpDiscardReplaced:
		return "replace"
	case deploy.OpRead:
		// nolint: goconst
		return "read"
	case deploy.OpRefresh:
		return "refresh"
	case deploy.OpReadDiscard:
		return "discard"
	case deploy.OpImport, deploy.OpImportReplacement:
		return "import"
	}

	contract.Failf("Unrecognized resource step op: %v", step.Op)
	return ""
}

func (pd *ProgressDisplay) getStepOp(step sdkDisplay.StepEventMetadata) display.StepOp {
	op := step.Op

	// We will commonly hear about replacements as an actual series of steps.  i.e. 'create
	// replacement', 'replace', 'delete original'.  During the actual application of these steps we
	// want to see these individual steps.  However, both before we apply all of them, and after
	// they're all done, we want to show this as a single conceptual 'replace'/'replaced' step.
	//
	// Note: in non-interactive mode we can show these all as individual steps.  This only applies
	// to interactive mode, where there is only one line shown per resource, and we want it to be as
	// clear as possible
	if pd.isTerminal {
		// During preview, show the steps for replacing as a single 'replace' plan.
		// Once done, show the steps for replacing as a single 'replaced' step.
		// During update, we'll show these individual steps.
		if pd.isPreview || pd.done {
			if op == deploy.OpCreateReplacement || op == deploy.OpDeleteReplaced || op == deploy.OpDiscardReplaced {
				return deploy.OpReplace
			}
		}
	}

	return op
}

func (pd *ProgressDisplay) getStepOpLabel(step sdkDisplay.StepEventMetadata, done bool) string {
	return deploy.Prefix(pd.getStepOp(step), done) + colors.Reset
}

func (pd *ProgressDisplay) getStepInProgressDescription(step sdkDisplay.StepEventMetadata) string {
	op := pd.getStepOp(step)

	if isRootStack(step) && op == deploy.OpSame {
		// most of the time a stack is unchanged.  in that case we just show it as "running->done".
		// otherwise, we show what is actually happening to it.
		return "running"
	}

	getDescription := func() string {
		if pd.isPreview {
			return pd.getPreviewText(step)
		}

		switch op {
		case deploy.OpSame:
			return ""
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
		case deploy.OpReadDiscard:
			return "discarding"
		case deploy.OpDiscardReplaced:
			return "discarding original"
		case deploy.OpImport:
			return "importing"
		case deploy.OpImportReplacement:
			return "importing replacement"
		}

		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
	return deploy.ColorProgress(op) + getDescription() + colors.Reset
}
