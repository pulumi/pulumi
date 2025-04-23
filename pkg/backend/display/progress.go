// Copyright 2016-2023, Pulumi Corporation.
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
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dustin/go-humanize/english"
	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DiagInfo contains the bundle of diagnostic information for a single resource.
type DiagInfo struct {
	ErrorCount, WarningCount, InfoCount, DebugCount int

	// The very last diagnostic event we got for this resource (regardless of severity). We'll print
	// this out in the non-interactive mode whenever we get new events. Importantly, we don't want
	// to print out the most significant diagnostic, as that means a flurry of event swill cause us
	// to keep printing out the most significant diagnostic over and over again.
	LastDiag *engine.DiagEventPayload

	// The last error we received.  If we have an error, and we're in tree-view, we'll prefer to
	// show this over the last non-error diag so that users know about something bad early on.
	LastError *engine.DiagEventPayload

	// All the diagnostic events we've heard about this resource.  We'll print the last diagnostic
	// in the status region while a resource is in progress.  At the end we'll print out all
	// diagnostics for a resource.
	//
	// Diagnostic events are bucketed by their associated stream ID (with 0 being the default
	// stream).
	StreamIDToDiagPayloads map[int32][]engine.DiagEventPayload
}

type progressRenderer interface {
	io.Closer

	initializeDisplay(display *ProgressDisplay)
	tick()
	rowUpdated(row Row)
	systemMessage(payload engine.StdoutEventPayload)

	// A callback to be invoked whenever a ProgressEvent is emitted. The boolean
	// argument is true if and only if this is the first progress event received
	// with the given ID.
	progress(payload engine.ProgressEventPayload, first bool)

	done()
	println(line string)
}

// ProgressDisplay organizes all the information needed for a dynamically updated "progress" view of an update.
type ProgressDisplay struct {
	// eventMutex is used to synchronize access to eventUrnToResourceRow, which is accessed
	// by the treeRenderer
	eventMutex sync.RWMutex
	// stopwatchMutex is used to synchronixe access to opStopwatch, which is used to track the times
	// taken to perform actions on resources.
	stopwatchMutex sync.RWMutex

	opts Options

	renderer progressRenderer

	// action is the kind of action (preview, update, refresh, etc) being performed.
	action apitype.UpdateKind
	// stack is the stack this progress pertains to.
	stack tokens.StackName
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

	// The set of observed unchanged (same) resources we've seen. In the event
	// that we've not been explicitly asked to show unchanged resources
	// (ShowSameResources), we'll use this to show just a count of the number of
	// unchanged resources. This can be useful to indicate that we are still
	// making progress in the event that there are a large number of unchanged
	// resources (which otherwise would result in no visible change to the
	// display).
	sames map[resource.URN]bool

	// Whether or not we've seen outputs for the stack yet.
	seenStackOutputs bool

	// The summary event from the engine.  If we get this, we'll print this after all
	// normal resource events are heard.  That way we don't interfere with all the progress
	// messages we're outputting for them.
	summaryEventPayload *engine.SummaryEventPayload

	// Any system events we've received. They will be printed at the bottom of all
	// the status rows.
	systemEventPayloads []engine.StdoutEventPayload

	// Any active download progress events that we've received.
	progressEventPayloads map[string]engine.ProgressEventPayload

	// Used to record the order that rows are created in.  That way, when we present in a tree, we
	// can keep things ordered so they will not jump around.
	displayOrderCounter int

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

	// If all progress messages are done and we can print out the final display.
	done bool

	// True if one or more resource operations have failed.
	failed bool

	// The column that the suffix should be added to
	suffixColumn int

	// the list of suffixes to rotate through
	suffixesArray []string

	// Structure that tracks the time taken to perform an action on a resource.
	opStopwatch opStopwatch

	// Indicates whether we already printed the loading policy packs message.
	shownPolicyLoadEvent bool

	permalink string
}

type opStopwatch struct {
	start map[resource.URN]time.Time
	end   map[resource.URN]time.Time
}

func newOpStopwatch() opStopwatch {
	return opStopwatch{
		start: map[resource.URN]time.Time{},
		end:   map[resource.URN]time.Time{},
	}
}

// policyPayloads is a collection of policy violation events for a single resource.
var policyPayloads []engine.PolicyViolationEventPayload

// getEventUrnAndMetadata returns the resource URN associated with an event, or the empty URN if this is not an
// event that has a URN.  If this is also a 'step' event, then this will return the step metadata as
// well.
func getEventUrnAndMetadata(event engine.Event) (resource.URN, *engine.StepEventMetadata) {
	//nolint:exhaustive // Only a subset of events have urns.
	switch event.Type {
	case engine.ResourcePreEvent:
		payload := event.Payload().(engine.ResourcePreEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	case engine.ResourceOutputsEvent:
		payload := event.Payload().(engine.ResourceOutputsEventPayload)
		return payload.Metadata.URN, &payload.Metadata
	case engine.ResourceOperationFailed:
		payload := event.Payload().(engine.ResourceOperationFailedPayload)
		return payload.Metadata.URN, &payload.Metadata
	case engine.DiagEvent:
		return event.Payload().(engine.DiagEventPayload).URN, nil
	case engine.PolicyRemediationEvent:
		return event.Payload().(engine.PolicyRemediationEventPayload).ResourceURN, nil
	case engine.PolicyViolationEvent:
		return event.Payload().(engine.PolicyViolationEventPayload).ResourceURN, nil
	default:
		return "", nil
	}
}

// ShowProgressEvents displays the engine events with docker's progress view.
func ShowProgressEvents(op string, action apitype.UpdateKind, stack tokens.StackName, proj tokens.PackageName,
	permalink string, events <-chan engine.Event, done chan<- bool, opts Options, isPreview bool,
) {
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	isInteractive, term := opts.IsInteractive, opts.term
	if isInteractive && term == nil {
		raw := runtime.GOOS != "windows"
		t, err := terminal.Open(stdin, stdout, raw)
		if err != nil {
			_, err = fmt.Fprintln(stderr, "Failed to open terminal; treating display as non-interactive (%w)", err)
			contract.IgnoreError(err)
			isInteractive = false
		} else {
			term = t
		}
	}

	var renderer progressRenderer
	if isInteractive {
		printPermalinkInteractive(term, opts, permalink, "")
		renderer = newInteractiveRenderer(term, permalink, opts)
	} else {
		printPermalinkNonInteractive(stdout, opts, permalink, "")
		renderer = newNonInteractiveRenderer(stdout, op, opts)
	}

	display := &ProgressDisplay{
		action:                action,
		isPreview:             isPreview,
		isTerminal:            isInteractive,
		opts:                  opts,
		renderer:              renderer,
		stack:                 stack,
		proj:                  proj,
		sames:                 make(map[resource.URN]bool),
		eventUrnToResourceRow: make(map[resource.URN]ResourceRow),
		suffixColumn:          int(statusColumn),
		suffixesArray:         []string{"", ".", "..", "..."},
		displayOrderCounter:   1,
		opStopwatch:           newOpStopwatch(),
		permalink:             permalink,
	}
	renderer.initializeDisplay(display)

	ticker := time.NewTicker(1 * time.Second)
	if opts.DeterministicOutput {
		ticker.Stop()
	}
	display.processEvents(ticker, events)
	contract.IgnoreClose(display.renderer)
	ticker.Stop()

	// let our caller know we're done.
	close(done)
}

// RenderProgressEvents renders the engine events as if to a terminal, providing a simple interface
// for rendering the progress of an update.
//
// A "simple" terminal is used which does not render control sequences. The simple terminal's output
// is written to opts.Stdout.
//
// For consistent output, these settings are enforced:
//
//	opts.Color = colors.Never
//	opts.RenderOnDirty = false
//	opts.IsInteractive = true
func RenderProgressEvents(
	op string,
	action apitype.UpdateKind,
	stack tokens.StackName,
	proj tokens.PackageName,
	permalink string,
	events <-chan engine.Event,
	done chan<- bool,
	opts Options,
	isPreview bool,
	width, height int,
) {
	o := opts
	if o.term == nil {
		o.term = terminal.NewSimpleTerminal(o.Stdout, width, height)
	}
	o.Color = colors.Never
	o.RenderOnDirty = false
	o.IsInteractive = true

	printPermalinkInteractive(o.term, o, permalink, "")
	renderer := newInteractiveRenderer(o.term, permalink, o)
	display := &ProgressDisplay{
		action:                action,
		isPreview:             isPreview,
		isTerminal:            true,
		opts:                  o,
		renderer:              renderer,
		stack:                 stack,
		proj:                  proj,
		sames:                 make(map[resource.URN]bool),
		eventUrnToResourceRow: make(map[resource.URN]ResourceRow),
		suffixColumn:          int(statusColumn),
		suffixesArray:         []string{"", ".", "..", "..."},
		displayOrderCounter:   1,
		opStopwatch:           newOpStopwatch(),
		permalink:             permalink,
	}
	renderer.initializeDisplay(display)

	display.processEvents(&time.Ticker{}, events)
	contract.IgnoreClose(display.renderer)

	// let our caller know we're done.
	close(done)
}

type CaptureProgressEvents struct {
	Buffer  *bytes.Buffer
	Stack   tokens.StackName
	Proj    tokens.PackageName
	display *ProgressDisplay
}

// NewCaptureProgressEvents renders the provided engine events channel to an internal buffer. It returns a
// CaptureProgressEvents instance that can be used to access both the output and the display instance after processing
// the events. This is useful for detecting whether a failure was detected in the display layer, e.g. used to send the
// output to Copilot if a failure was detected.
func NewCaptureProgressEvents(
	stack tokens.StackName,
	proj tokens.PackageName,
	opts Options,
	isPreview bool,
	action apitype.UpdateKind,
) *CaptureProgressEvents {
	buffer := bytes.NewBuffer([]byte{})
	width, height := 200, 80

	o := opts
	o.Color = colors.Never
	o.RenderOnDirty = false
	o.IsInteractive = true

	o.Stdout = buffer
	o.Stderr = io.Discard
	o.term = terminal.NewSimpleTerminal(buffer, width, height)

	permalink := ""

	printPermalinkInteractive(o.term, o, permalink, "")
	renderer := newInteractiveRenderer(o.term, permalink, o)
	display := &ProgressDisplay{
		action:                action,
		isPreview:             isPreview,
		isTerminal:            true,
		opts:                  o,
		renderer:              renderer,
		stack:                 stack,
		proj:                  proj,
		sames:                 make(map[resource.URN]bool),
		eventUrnToResourceRow: make(map[resource.URN]ResourceRow),
		suffixColumn:          int(statusColumn),
		suffixesArray:         []string{"", ".", "..", "..."},
		displayOrderCounter:   1,
		opStopwatch:           newOpStopwatch(),
		permalink:             permalink,
	}
	renderer.initializeDisplay(display)

	return &CaptureProgressEvents{
		Buffer:  buffer,
		Stack:   stack,
		Proj:    proj,
		display: display,
	}
}

func (r *CaptureProgressEvents) ProcessEvents(
	renderChan <-chan engine.Event,
	renderDone chan<- bool,
) {
	r.display.processEvents(&time.Ticker{}, renderChan)
	contract.IgnoreClose(r.display.renderer)
	close(renderDone)
}

func (r *CaptureProgressEvents) Output() []string {
	v := strings.TrimSpace(r.Buffer.String())
	if v == "" {
		return nil
	}
	return strings.Split(v, "\n")
}

func (r *CaptureProgressEvents) OutputIncludesFailure() bool {
	// If its an actual update we can use the failed flag
	if !r.display.isPreview {
		return r.display.failed
	}

	// If its a preview we need to check the resource rows for any failures
	for _, row := range r.display.resourceRows {
		diagInfo := row.DiagInfo()
		if diagInfo != nil && diagInfo.ErrorCount > 0 {
			return true
		}
	}

	return false
}

func (display *ProgressDisplay) println(line string) {
	display.renderer.println(line)
}

type treeNode struct {
	row Row

	colorizedColumns []string
	colorizedSuffix  string

	childNodes []*treeNode
}

func (display *ProgressDisplay) getOrCreateTreeNode(
	result *[]*treeNode, urn resource.URN, row ResourceRow, urnToTreeNode map[resource.URN]*treeNode,
) *treeNode {
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
	// We take the reader lock here because this is called from the renderer and reads from
	// the eventUrnToResourceRow map
	display.eventMutex.RLock()
	defer display.eventMutex.RUnlock()

	result := []*treeNode{}

	result = append(result, &treeNode{
		row:              display.headerRow,
		colorizedColumns: display.headerRow.ColorizedColumns(),
	})

	urnToTreeNode := make(map[resource.URN]*treeNode)
	eventRows := toResourceRows(display.eventUrnToResourceRow, display.opts.DeterministicOutput)
	for _, row := range eventRows {
		display.getOrCreateTreeNode(&result, row.Step().URN, row, urnToTreeNode)
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
	nodes []*treeNode, maxSuffixLength int, rows *[][]string, maxColumnLengths *[]int,
) {
	for _, node := range nodes {
		if len(*maxColumnLengths) == 0 {
			*maxColumnLengths = make([]int, len(node.colorizedColumns))
		}

		colorizedColumns := make([]string, len(node.colorizedColumns))

		for i, colorizedColumn := range node.colorizedColumns {
			columnWidth := colors.MeasureColorizedString(colorizedColumn)

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

// Performs all the work at the end once we've heard about the last message from the engine.
// Specifically, this will update the status messages for any resources, and will also then
// print out all final diagnostics. and finally will print out the summary.
func (display *ProgressDisplay) processEndSteps() {
	// Take the read lock here because we are reading from the eventUrnToResourceRow map
	display.eventMutex.RLock()
	defer display.eventMutex.RUnlock()

	// Figure out the rows that are currently in progress.
	var inProgressRows []ResourceRow
	if !display.isTerminal {
		for _, v := range display.eventUrnToResourceRow {
			if !v.IsDone() {
				inProgressRows = append(inProgressRows, v)
			}
		}
	}

	// Transition the display to the 'done' state.  This will transitively cause all
	// rows to become done.
	display.done = true

	// Now print out all those rows that were in progress.  They will now be 'done'
	// since the display was marked 'done'.
	if !display.isTerminal {
		if display.opts.DeterministicOutput {
			sortResourceRows(inProgressRows)
		}
		for _, v := range inProgressRows {
			display.renderer.rowUpdated(v)
		}
	}

	// Now refresh everything.  This ensures that we go back and remove things like the diagnostic
	// messages from a status message (since we're going to print them all) below.  Note, this will
	// only do something in a terminal.  This is what we want, because if we're not in a terminal we
	// don't really want to reprint any finished items we've already printed.
	display.renderer.done()

	display.printResourceDiffs()

	// Render the policies section; this will print all policy packs that ran plus any specific
	// policies that led to violations or remediations. This comes before diagnostics since policy
	// violations yield failures and it is better to see those in advance of the failure message.
	wroteMandatoryPolicyViolations := display.printPolicies()

	// Render the actual diagnostics streams (warnings, errors, etc).
	display.printDiagnostics()

	// Print output variables; this comes last, prior to the summary, since these are the final
	// outputs after having run all of the above.
	display.printOutputs()

	// Print a summary of resource operations unless there were mandatory policy violations.
	// In that case, we want to abruptly terminate the display so as not to confuse.
	if !wroteMandatoryPolicyViolations {
		display.printSummary()
	}
}

// printResourceDiffs prints a "Changes:" section with all of the resource diffs grouped by
// resource.printResourceChanges
func (display *ProgressDisplay) printResourceDiffs() {
	if !display.opts.ShowResourceChanges {
		return
	}

	wroteChangesHeader := false

	seen := make(map[resource.URN]engine.StepEventMetadata)
	eventRows := toResourceRows(display.eventUrnToResourceRow, display.opts.DeterministicOutput)
	for _, row := range eventRows {
		step := row.Step()

		if step.Op == deploy.OpSame {
			continue
		}

		var diffOutput bytes.Buffer
		renderDiff(
			&diffOutput,
			step,
			false,
			false,
			display.action == apitype.UpdateKind(apitype.Refresh),
			seen,
			display.opts,
		)

		diff := diffOutput.String()
		if diff == "" {
			continue
		}

		if !wroteChangesHeader {
			wroteChangesHeader = true
			display.println(colors.SpecHeadline + "Changes:" + colors.Reset)
		}

		columns := row.ColorizedColumns()
		display.println(
			"  " + colors.BrightBlue + columns[typeColumn] + " (" + columns[nameColumn] + "):" + colors.Reset)

		lines := splitIntoDisplayableLines(diff)
		for _, line := range lines {
			line = strings.TrimRightFunc(line, unicode.IsSpace)
			display.println("    " + line)
		}
		display.println("")
	}
}

// printDiagnostics prints a new "Diagnostics:" section with all of the diagnostics grouped by
// resource. If no diagnostics were emitted, prints nothing. Returns whether an error was encountered.
func (display *ProgressDisplay) printDiagnostics() {
	// Since we display diagnostic information eagerly, we need to keep track of the first
	// time we wrote some output so we don't inadvertently print the header twice.
	wroteDiagnosticHeader := false

	eventRows := toResourceRows(display.eventUrnToResourceRow, display.opts.DeterministicOutput)

	for _, row := range eventRows {
		// The header for the diagnostics grouped by resource, e.g. "aws:apigateway:RestApi (accountsApi):"
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
				p := display.mergeStreamPayloadsToSinglePayload(payloads)
				payloads = []engine.DiagEventPayload{p}
			}

			// Did we write any diagnostic information for the resource x stream?
			wrote := false
			for _, v := range payloads {
				if v.Ephemeral {
					continue
				}

				msg := display.renderProgressDiagEvent(v, true /*includePrefix:*/)

				lines := splitIntoDisplayableLines(msg)
				if len(lines) == 0 {
					continue
				}

				// If we haven't printed the Diagnostics header, do so now.
				if !wroteDiagnosticHeader {
					wroteDiagnosticHeader = true
					display.println(colors.SpecHeadline + "Diagnostics:" + colors.Reset)
				}
				// If we haven't printed the header for the resource, do so now.
				if !wroteResourceHeader {
					wroteResourceHeader = true
					columns := row.ColorizedColumns()
					display.println(
						"  " + colors.BrightBlue + columns[typeColumn] + " (" + columns[nameColumn] + "):" + colors.Reset)
				}

				for _, line := range lines {
					line = strings.TrimRightFunc(line, unicode.IsSpace)
					display.println("    " + line)
				}

				wrote = true
			}

			if wrote {
				display.println("")
			}
		}
	}

	// Print a link to Copilot to explain the failure.
	// ShowLinkToCopilot is an older, lighter feature that is now superseded by the _optional_ ShowCopilotFeatures.
	// ShowCopilotFeatures provides it's own link if enabled.
	showCopilotLink := display.opts.ShowLinkToCopilot && !display.opts.ShowCopilotFeatures
	// Check for SuppressPermalink ensures we don't print the link for DIY backends
	if wroteDiagnosticHeader && !display.opts.SuppressPermalink && showCopilotLink {
		display.println("    " +
			colors.SpecCreateReplacement + "[Pulumi Copilot]" + colors.Reset + " Would you like help with these diagnostics?")
		display.println("    " +
			colors.Underline + colors.Blue + ExplainFailureLink(display.permalink) + colors.Reset)
		display.println("")
	}
}

type policyPackSummary struct {
	HasCloudPack      bool
	LocalPaths        []string
	ViolationEvents   []engine.PolicyViolationEventPayload
	RemediationEvents []engine.PolicyRemediationEventPayload
}

func (display *ProgressDisplay) printPolicies() bool {
	if display.summaryEventPayload == nil || len(display.summaryEventPayload.PolicyPacks) == 0 {
		return false
	}

	var hadMandatoryViolations bool
	display.println(display.opts.Color.Colorize(colors.SpecHeadline + "Policies:" + colors.Reset))

	// Print policy packs that were run and any violations or remediations associated with them.
	// Gather up all policy packs and their associated violation and remediation events.
	policyPackInfos := make(map[string]policyPackSummary)

	// First initialize empty lists for all policy packs just to ensure they show if no events are found.
	for name, version := range display.summaryEventPayload.PolicyPacks {
		var summary policyPackSummary
		baseName, path := engine.GetLocalPolicyPackInfoFromEventName(name)
		var key string
		if baseName != "" {
			key = fmt.Sprintf("%s@v%s", baseName, version)
			if s, has := policyPackInfos[key]; has {
				summary = s
				summary.LocalPaths = append(summary.LocalPaths, path)
			} else {
				summary.LocalPaths = []string{path}
			}
		} else {
			key = fmt.Sprintf("%s@v%s", name, version)
			if s, has := policyPackInfos[key]; has {
				summary = s
				summary.HasCloudPack = true
			}
		}
		policyPackInfos[key] = summary
	}

	// Next associate all violation events with the corresponding policy pack in the list.
	for _, row := range display.eventUrnToResourceRow {
		for _, event := range row.PolicyPayloads() {
			key := fmt.Sprintf("%s@v%s", event.PolicyPackName, event.PolicyPackVersion)
			newInfo := policyPackInfos[key]
			newInfo.ViolationEvents = append(newInfo.ViolationEvents, event)
			policyPackInfos[key] = newInfo
		}
	}

	// Now associate all remediation events with the corresponding policy pack in the list.
	for _, row := range display.eventUrnToResourceRow {
		for _, event := range row.PolicyRemediationPayloads() {
			key := fmt.Sprintf("%s@v%s", event.PolicyPackName, event.PolicyPackVersion)
			newInfo := policyPackInfos[key]
			newInfo.RemediationEvents = append(newInfo.RemediationEvents, event)
			policyPackInfos[key] = newInfo
		}
	}

	// Enumerate all policy packs in a deterministic order:
	policyKeys := make([]string, len(policyPackInfos))
	policyKeyIndex := 0
	for key := range policyPackInfos {
		policyKeys[policyKeyIndex] = key
		policyKeyIndex++
	}
	sort.Strings(policyKeys)

	// Finally, print the policy pack info and any violations and any remediations for each one.
	for _, key := range policyKeys {
		info := policyPackInfos[key]

		// Print the policy pack status and name/version as a header:
		passFailWarn := "✅"
		for _, violation := range info.ViolationEvents {
			if violation.EnforcementLevel == apitype.Mandatory {
				passFailWarn = "❌"
				hadMandatoryViolations = true
				break
			}

			passFailWarn = "⚠️"
			// do not break; subsequent mandatory violations will override this.
		}

		var localMark string
		if len(info.LocalPaths) > 0 {
			localMark = " (local: "
			sort.Strings(info.LocalPaths)
			for i, path := range info.LocalPaths {
				if i > 0 {
					localMark += "; "
				}
				localMark += path
			}
			localMark += ")"

			if info.HasCloudPack {
				localMark += " + (cloud)"
			}
		}

		display.println(fmt.Sprintf("    %s %s%s%s%s", passFailWarn, colors.SpecInfo, key, colors.Reset, localMark))
		subItemIndent := "        "

		// First show any remediations since they happen first.
		if display.opts.ShowPolicyRemediations {
			// If the user has requested detailed remediations, print each one. Do not sort them -- show them in the
			// order in which events arrived, since for remediations, the order matters.
			for _, remediationEvent := range info.RemediationEvents {
				// Print the individual policy event.
				remediationLine := renderDiffPolicyRemediationEvent(
					remediationEvent, subItemIndent+"- ", false, display.opts)
				remediationLine = strings.TrimSuffix(remediationLine, "\n")
				if remediationLine != "" {
					display.println(remediationLine)
				}
			}
		} else {
			// Otherwise, simply print a summary of which remediations ran and how many resources were affected.
			policyNames := make([]string, 0)
			policyRemediationCounts := make(map[string]int)
			for _, e := range info.RemediationEvents {
				name := e.PolicyName
				if policyRemediationCounts[name] == 0 {
					policyNames = append(policyNames, name)
				}
				policyRemediationCounts[name]++
			}
			sort.Strings(policyNames)
			for _, policyName := range policyNames {
				count := policyRemediationCounts[policyName]
				display.println(fmt.Sprintf("%s- %s[remediate]  %s%s  (%d %s)",
					subItemIndent, colors.SpecInfo, policyName, colors.Reset,
					count, english.PluralWord(count, "resource", "")))
			}
		}

		// Next up, display all violations. Sort policy events by: policy pack name, policy pack version,
		// enforcement level, policy name, and finally the URN of the resource.
		sort.SliceStable(info.ViolationEvents, func(i, j int) bool {
			eventI, eventJ := info.ViolationEvents[i], info.ViolationEvents[j]
			if enfLevelCmp := strings.Compare(
				string(eventI.EnforcementLevel), string(eventJ.EnforcementLevel)); enfLevelCmp != 0 {
				return enfLevelCmp < 0
			}
			if policyNameCmp := strings.Compare(eventI.PolicyName, eventJ.PolicyName); policyNameCmp != 0 {
				return policyNameCmp < 0
			}
			return strings.Compare(string(eventI.ResourceURN), string(eventJ.ResourceURN)) < 0
		})
		for _, policyEvent := range info.ViolationEvents {
			// Print the individual policy event.
			policyLine := renderDiffPolicyViolationEvent(
				policyEvent, subItemIndent+"- ", subItemIndent+"  ", display.opts)
			policyLine = strings.TrimSuffix(policyLine, "\n")
			display.println(policyLine)
		}
	}

	display.println("")
	return hadMandatoryViolations
}

// printOutputs prints the Stack's outputs for the display in a new section, if appropriate.
func (display *ProgressDisplay) printOutputs() {
	// Printing the stack's outputs wasn't desired.
	if display.opts.SuppressOutputs {
		return
	}
	// Cannot display outputs for the stack if we don't know its URN.
	if display.stackUrn == "" {
		return
	}

	stackStep := display.eventUrnToResourceRow[display.stackUrn].Step()

	props := getResourceOutputsPropertiesString(
		stackStep, 1, display.isPreview, display.opts.Debug,
		false /* refresh */, display.opts.ShowSameResources, display.opts.ShowSecrets)
	if props != "" {
		display.println(colors.SpecHeadline + "Outputs:" + colors.Reset)
		display.println(props)
	}
}

// printSummary prints the Stack's SummaryEvent in a new section if applicable.
func (display *ProgressDisplay) printSummary() {
	// If we never saw the SummaryEvent payload, we have nothing to do.
	if display.summaryEventPayload == nil {
		return
	}

	msg := renderSummaryEvent(*display.summaryEventPayload, false, display.opts)
	display.println(msg)
}

func (display *ProgressDisplay) mergeStreamPayloadsToSinglePayload(
	payloads []engine.DiagEventPayload,
) engine.DiagEventPayload {
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

	display.renderer.tick()
}

func (display *ProgressDisplay) getRowForURN(urn resource.URN, metadata *engine.StepEventMetadata) ResourceRow {
	// Take the write lock here because this can write the the eventUrnToResourceRow map
	display.eventMutex.Lock()
	defer display.eventMutex.Unlock()

	// If there's already a row for this URN, return it.
	row, has := display.eventUrnToResourceRow[urn]
	if has {
		return row
	}

	// First time we're hearing about this resource. Create an initial nearly-empty status for it.
	step := engine.StepEventMetadata{URN: urn, Op: deploy.OpSame}
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
		policyPayloads:       policyPayloads,
		step:                 step,
		hideRowIfUnnecessary: true,
	}

	display.eventUrnToResourceRow[urn] = row

	display.ensureHeaderAndStackRows()
	display.resourceRows = append(display.resourceRows, row)
	return row
}

func (display *ProgressDisplay) processNormalEvent(event engine.Event) {
	//nolint:exhaustive // we are only interested in a subset of events
	switch event.Type {
	case engine.PreludeEvent:
		// A prelude event can just be printed out directly to the console.
		// Note: we should probably make sure we don't get any prelude events
		// once we start hearing about actual resource events.
		payload := event.Payload().(engine.PreludeEventPayload)
		preludeEventString := renderPreludeEvent(payload, display.opts)
		if display.isTerminal {
			display.processNormalEvent(engine.NewEvent(engine.DiagEventPayload{
				Ephemeral: false,
				Severity:  diag.Info,
				Color:     cmdutil.GetGlobalColorization(),
				Message:   preludeEventString,
			}))
		} else {
			display.println(preludeEventString)
		}
		return
	case engine.PolicyLoadEvent:
		if !display.shownPolicyLoadEvent {
			policyLoadEventString := colors.SpecInfo + "Loading policy packs..." + colors.Reset + "\n"
			display.println(policyLoadEventString)
			display.shownPolicyLoadEvent = true
		}
		return
	case engine.SummaryEvent:
		// keep track of the summary event so that we can display it after all other
		// resource-related events we receive.
		payload := event.Payload().(engine.SummaryEventPayload)
		display.summaryEventPayload = &payload
		return
	case engine.DiagEvent:
		msg := display.renderProgressDiagEvent(event.Payload().(engine.DiagEventPayload), true /*includePrefix:*/)
		if msg == "" {
			return
		}
	case engine.StartDebuggingEvent:
		return
	case engine.StdoutColorEvent:
		display.handleSystemEvent(event.Payload().(engine.StdoutEventPayload))
		return
	case engine.ProgressEvent:
		display.handleProgressEvent(event.Payload().(engine.ProgressEventPayload))
		return
	}

	// At this point, all events should relate to resources.
	eventUrn, metadata := getEventUrnAndMetadata(event)

	// If we're suppressing reads from the tree-view, then convert notifications about reads into
	// ephemeral messages that will go into the info column.
	if metadata != nil && !display.opts.ShowReads {
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
			display.processNormalEvent(engine.NewEvent(engine.DiagEventPayload{
				Ephemeral: true,
				Severity:  diag.Info,
				Color:     cmdutil.GetGlobalColorization(),
				Message:   fmt.Sprintf("read %v %v", eventUrn.Type().DisplayName(), eventUrn.Name()),
			}))
			return
		}
	}

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
	// Always show row if there's a policy violation event. Policy violations prevent resource
	// registration, so if we don't show the row, the violation gets attributed to the stack
	// resource rather than the resources whose policy failed.
	hideRowIfUnnecessary = hideRowIfUnnecessary || event.Type == engine.PolicyViolationEvent
	if !hideRowIfUnnecessary {
		row.SetHideRowIfUnnecessary(false)
	}

	if event.Type == engine.ResourcePreEvent {
		step := event.Payload().(engine.ResourcePreEventPayload).Metadata

		// Register the resource update start time to calculate duration
		// and time elapsed.
		start := time.Now()
		display.stopwatchMutex.Lock()
		display.opStopwatch.start[step.URN] = start

		// Clear out potential event end timings for prior operations on the same resource.
		delete(display.opStopwatch.end, step.URN)
		display.stopwatchMutex.Unlock()

		row.SetStep(step)
	} else if event.Type == engine.ResourceOutputsEvent {
		isRefresh := display.getStepOp(row.Step()) == deploy.OpRefresh
		step := event.Payload().(engine.ResourceOutputsEventPayload).Metadata

		// Register the resource update end time to calculate duration
		// to display.
		end := time.Now()
		display.stopwatchMutex.Lock()
		display.opStopwatch.end[step.URN] = end
		display.stopwatchMutex.Unlock()

		// Is this the stack outputs event? If so, we'll need to print it out at the end of the plan.
		if step.URN == display.stackUrn {
			display.seenStackOutputs = true
		}

		row.SetStep(step)
		row.AddOutputStep(step)

		if step.Op == deploy.OpSame {
			display.sames[step.URN] = true
		}

		// If we're not in a terminal, we may not want to display this row again: if we're displaying a preview or if
		// this step is a no-op for a custom resource, refreshing this row will simply duplicate its earlier output.
		hasMeaningfulOutput := isRefresh ||
			!display.isPreview && (step.Res == nil || step.Res.Custom && step.Op != deploy.OpSame)
		if !display.isTerminal && !hasMeaningfulOutput {
			return
		}
	} else if event.Type == engine.ResourceOperationFailed {
		display.failed = true
		row.SetFailed()
	} else if event.Type == engine.DiagEvent {
		// also record this diagnostic so we print it at the end.
		row.RecordDiagEvent(event)
	} else if event.Type == engine.PolicyViolationEvent {
		// also record this policy violation so we print it at the end.
		row.RecordPolicyViolationEvent(event)
	} else if event.Type == engine.PolicyRemediationEvent {
		// record this remediation so we print it at the end.
		row.RecordPolicyRemediationEvent(event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	display.renderer.rowUpdated(row)
}

func (display *ProgressDisplay) handleSystemEvent(payload engine.StdoutEventPayload) {
	// We need to take the writer lock here because ensureHeaderAndStackRows expects to be
	// called under the write lock.
	display.eventMutex.Lock()
	defer display.eventMutex.Unlock()

	// Make sure we have a header to display
	display.ensureHeaderAndStackRows()

	display.systemEventPayloads = append(display.systemEventPayloads, payload)

	display.renderer.systemMessage(payload)
}

func (display *ProgressDisplay) handleProgressEvent(payload engine.ProgressEventPayload) {
	// We need to take the writer lock here because ensureHeaderAndStackRows expects to be
	// called under the write lock.
	display.eventMutex.Lock()
	defer display.eventMutex.Unlock()

	// Make sure we have a header to display
	display.ensureHeaderAndStackRows()

	if display.progressEventPayloads == nil {
		display.progressEventPayloads = make(map[string]engine.ProgressEventPayload)
	}

	_, seen := display.progressEventPayloads[payload.ID]
	first := !seen

	if payload.Done {
		delete(display.progressEventPayloads, payload.ID)
	} else {
		display.progressEventPayloads[payload.ID] = payload
	}

	display.renderer.progress(payload, first)
}

func (display *ProgressDisplay) ensureHeaderAndStackRows() {
	contract.Assertf(!display.eventMutex.TryLock(), "ProgressDisplay.ensureHeaderAndStackRows MUST be called "+
		"under the write lock")
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
		policyPayloads:       policyPayloads,
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

// getStepStatus handles getting the value to put in the status column.
func (display *ProgressDisplay) getStepStatus(step engine.StepEventMetadata, done bool, failed bool) string {
	var status string
	if done {
		status = display.getStepDoneDescription(step, failed)
	} else {
		status = display.getStepInProgressDescription(step)
	}
	status = addRetainStatusFlag(status, step)
	return status
}

func (display *ProgressDisplay) getStepDoneDescription(step engine.StepEventMetadata, failed bool) string {
	makeError := func(v string) string {
		return colors.SpecError + "**" + v + "**" + colors.Reset
	}

	op := display.getStepOp(step)

	if display.isPreview {
		// During a preview, when we transition to done, we'll print out summary text describing the step instead of a
		// past-tense verb describing the step that was performed.
		return deploy.Color(op) + display.getPreviewDoneText(step) + colors.Reset
	}

	getDescription := func() string {
		var opText string
		if failed {
			switch op {
			case deploy.OpSame:
				opText = "failed"
			case deploy.OpCreate, deploy.OpCreateReplacement:
				opText = "creating failed"
			case deploy.OpUpdate:
				opText = "updating failed"
			case deploy.OpDelete, deploy.OpDeleteReplaced:
				opText = "deleting failed"
			case deploy.OpReplace:
				opText = "replacing failed"
			case deploy.OpRead, deploy.OpReadReplacement:
				opText = "reading failed"
			case deploy.OpRefresh:
				opText = "refreshing failed"
			case deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				opText = "discarding failed"
			case deploy.OpImport, deploy.OpImportReplacement:
				opText = "importing failed"
			case deploy.OpRemovePendingReplace:
				opText = ""
			default:
				contract.Failf("Unrecognized resource step op: %v", op)
				return ""
			}
		} else {
			switch op {
			case deploy.OpSame:
				opText = ""
			case deploy.OpCreate:
				opText = "created"
			case deploy.OpUpdate:
				opText = "updated"
			case deploy.OpDelete:
				opText = "deleted"
			case deploy.OpReplace:
				opText = "replaced"
			case deploy.OpCreateReplacement:
				opText = "created replacement"
			case deploy.OpDeleteReplaced:
				opText = "deleted original"
			case deploy.OpRead:
				opText = "read"
			case deploy.OpReadReplacement:
				opText = "read for replacement"
			case deploy.OpRefresh:
				opText = "refresh"
			case deploy.OpReadDiscard:
				opText = "discarded"
			case deploy.OpDiscardReplaced:
				opText = "discarded original"
			case deploy.OpImport:
				opText = "imported"
			case deploy.OpImportReplacement:
				opText = "imported replacement"
			case deploy.OpRemovePendingReplace:
				opText = ""
			default:
				contract.Failf("Unrecognized resource step op: %v", op)
				return ""
			}
		}
		if op == deploy.OpSame || display.opts.DeterministicOutput || display.opts.SuppressTimings {
			return opText
		}

		display.stopwatchMutex.RLock()
		start, ok := display.opStopwatch.start[step.URN]
		display.stopwatchMutex.RUnlock()

		if !ok {
			return opText
		}

		display.stopwatchMutex.RLock()
		end, ok := display.opStopwatch.end[step.URN]
		display.stopwatchMutex.RUnlock()

		if !ok {
			return opText
		}

		opDuration := end.Sub(start).Seconds()
		if opDuration < 1 {
			// Display a more fine-grain duration as the operation
			// has completed.
			return fmt.Sprintf("%s (%.2fs)", opText, opDuration)
		}
		return fmt.Sprintf("%s (%ds)", opText, int(opDuration))
	}

	if failed {
		return makeError(getDescription())
	}

	return deploy.Color(op) + getDescription() + colors.Reset
}

func (display *ProgressDisplay) getPreviewText(step engine.StepEventMetadata) string {
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
func (display *ProgressDisplay) getPreviewDoneText(step engine.StepEventMetadata) string {
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

// getStepOp returns the operation to display for the given step. Generally this
// will be the operation already attached to the step, but there are some cases
// where it makes sense for us to return a different operation. In particular,
// we often think of replacements as a series of steps, e.g.:
//
// * create replacement
// * replace
// * delete original
//
// When these steps are being applied, we want to display the individual steps
// to give the user the best indicator of what is happening currently. However,
// both before we apply all of them, and before they're all done, we want to
// show this sequence as a single conceptual "replace"/"replaced" step. We thus
// rewrite the operation to be "replace" in these cases.
//
// There are two cases where we do not want to rewrite any operations:
//
//   - If we are operating in a non-interactive mode (that is, effectively
//     outputting a log), we can afford to show all the individual steps, since we
//     are not limited to a single line per resource, and we want the list of
//     steps to be as clear and true to the actual operations as possible.
//
//   - If a resource operation fails (meaning the entire operation will likely
//     fail), we want the user to be left with as true a representation of where
//     we got to when the program terminates (e.g. we don't want to show
//     "replaced" when in fact we have only completed the first step of the
//     series).
func (display *ProgressDisplay) getStepOp(step engine.StepEventMetadata) display.StepOp {
	op := step.Op
	if !display.isTerminal {
		return op
	}

	if display.failed {
		return op
	}

	// Replacing replace series with replace:
	//
	// * During preview -- we'll show a single "replace" plan.
	// * During update -- we'll show the individual steps.
	// * When done -- we'll show a single "replaced" step.
	if display.isPreview || display.done {
		if op == deploy.OpCreateReplacement || op == deploy.OpDeleteReplaced || op == deploy.OpDiscardReplaced {
			return deploy.OpReplace
		}
	}

	return op
}

func (display *ProgressDisplay) getStepOpLabel(step engine.StepEventMetadata, done bool) string {
	return deploy.Prefix(display.getStepOp(step), done) + colors.Reset
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

		var opText string
		switch op {
		case deploy.OpSame:
			opText = ""
		case deploy.OpCreate:
			opText = "creating"
		case deploy.OpUpdate:
			opText = "updating"
		case deploy.OpDelete:
			opText = "deleting"
		case deploy.OpReplace:
			opText = "replacing"
		case deploy.OpCreateReplacement:
			opText = "creating replacement"
		case deploy.OpDeleteReplaced:
			opText = "deleting original"
		case deploy.OpRead:
			opText = "reading"
		case deploy.OpReadReplacement:
			opText = "reading for replacement"
		case deploy.OpRefresh:
			opText = "refreshing"
		case deploy.OpReadDiscard:
			opText = "discarding"
		case deploy.OpDiscardReplaced:
			opText = "discarding original"
		case deploy.OpImport:
			opText = "importing"
		case deploy.OpImportReplacement:
			opText = "importing replacement"
		case deploy.OpRemovePendingReplace:
			opText = ""
		default:
			contract.Failf("Unrecognized resource step op: %v", op)
			return ""
		}

		if op == deploy.OpSame || display.opts.DeterministicOutput || display.opts.SuppressTimings {
			return opText
		}

		// Calculate operation time elapsed.
		display.stopwatchMutex.RLock()
		start, ok := display.opStopwatch.start[step.URN]
		display.stopwatchMutex.RUnlock()

		if !ok {
			return opText
		}

		secondsElapsed := time.Since(start).Seconds()
		return fmt.Sprintf("%s (%ds)", opText, int(secondsElapsed))
	}
	return deploy.ColorProgress(op) + getDescription() + colors.Reset
}

// toResourceRows sorts a map of resources by their URN and returns the sorted rows.
func toResourceRows(rowMap map[resource.URN]ResourceRow, sorted bool) []ResourceRow {
	rows := make([]ResourceRow, 0, len(rowMap))
	for _, row := range rowMap {
		rows = append(rows, row)
	}
	if sorted {
		sortResourceRows(rows)
	}
	return rows
}

// sortResourceRows sorts the given rows in a deterministic order.
func sortResourceRows(rows []ResourceRow) {
	sort.Slice(rows, func(i, j int) bool {
		a := rows[i].Step()
		b := rows[j].Step()

		if a.Op == "same" && a.URN == "" {
			// This is the root stack event. Always sort it last
			return false
		}
		if b.Op == "same" && b.URN == "" {
			return true
		}
		if a.Res == nil {
			return false
		}
		if b.Res == nil {
			return true
		}

		return a.Res.URN < b.Res.URN
	})
}
