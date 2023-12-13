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

// forked from: https://github.com/moby/moby/blob/master/pkg/jsonmessage/jsonmessage.go
// so we can customize parts of the display of our progress messages

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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

// Display displays the Progress to `out`. `termInfo` is non-nil if `out` is a terminal.
func (jm *Progress) Display(out io.Writer, termInfo terminal.Info) {
	var endl string
	if termInfo != nil && /*jm.Stream == "" &&*/ jm.Action != "" {
		termInfo.ClearLine(out)
		endl = "\r"
		fmt.Fprint(out, endl)
	}

	if jm.Action != "" && termInfo != nil {
		fmt.Fprintf(out, "%s%s", jm.Action, endl)
	} else {
		var msg string
		if jm.Action != "" {
			msg = jm.Action
		} else {
			msg = jm.Message
		}

		fmt.Fprintf(out, "%s%s\n", msg, endl)
	}
}

type messageRenderer struct {
	opts          Options
	isInteractive bool

	terminal       terminal.Terminal
	terminalWidth  int
	terminalHeight int

	// A spinner to use to show that we're still doing work even when no output has been
	// printed to the console in a while.
	nonInteractiveSpinner cmdutil.Spinner

	progressOutput chan<- Progress
	closed         <-chan bool

	// Cache of lines we've already printed.  We don't print a progress message again if it hasn't
	// changed between the last time we printed and now.
	printedProgressCache map[string]Progress
}

func newInteractiveMessageRenderer(term terminal.Terminal, opts Options) progressRenderer {
	r := newMessageRenderer(term, opts, true)
	r.terminal = term

	var err error
	r.terminalWidth, r.terminalHeight, err = term.Size()
	contract.IgnoreError(err)

	return r
}

func newNonInteractiveRenderer(stdout io.Writer, op string, opts Options) progressRenderer {
	spinner, ticker := cmdutil.NewSpinnerAndTicker(
		fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op),
		nil, opts.Color, 1 /*timesPerSecond*/)
	ticker.Stop()

	r := newMessageRenderer(stdout, opts, false)
	r.nonInteractiveSpinner = spinner
	return r
}

func newMessageRenderer(out io.Writer, opts Options, isInteractive bool) *messageRenderer {
	progressOutput, closed := make(chan Progress), make(chan bool)
	go func() {
		ShowProgressOutput(progressOutput, out, isInteractive)
		close(closed)
	}()

	return &messageRenderer{
		opts:                 opts,
		isInteractive:        isInteractive,
		progressOutput:       progressOutput,
		closed:               closed,
		printedProgressCache: make(map[string]Progress),
	}
}

func (r *messageRenderer) Close() error {
	close(r.progressOutput)
	<-r.closed
	return nil
}

// Converts the colorization tags in a progress message and then actually writes the progress
// message to the output stream.  This should be the only place in this file where we actually
// process colorization tags.
func (r *messageRenderer) colorizeAndWriteProgress(progress Progress) {
	if progress.Message != "" {
		progress.Message = r.opts.Color.Colorize(progress.Message)
	}

	if progress.Action != "" {
		progress.Action = r.opts.Color.Colorize(progress.Action)
	}

	if progress.ID != "" {
		// don't repeat the same output if there is no difference between the last time we
		// printed it and now.
		lastProgress, has := r.printedProgressCache[progress.ID]
		if has && lastProgress.Message == progress.Message && lastProgress.Action == progress.Action {
			return
		}

		r.printedProgressCache[progress.ID] = progress
	}

	if !r.isInteractive {
		// We're about to display something.  Reset our spinner so that it will go on the next line.
		r.nonInteractiveSpinner.Reset()
	}

	r.progressOutput <- progress
}

func (r *messageRenderer) writeSimpleMessage(msg string) {
	r.colorizeAndWriteProgress(makeMessageProgress(msg))
}

func (r *messageRenderer) println(display *ProgressDisplay, line string) {
	r.writeSimpleMessage(line)
}

func (r *messageRenderer) tick(display *ProgressDisplay) {
	if r.isInteractive {
		r.render(display, false)
	} else {
		// Update the spinner to let the user know that that work is still happening.
		r.nonInteractiveSpinner.Tick()
	}
}

func (r *messageRenderer) renderRow(display *ProgressDisplay,
	id string, colorizedColumns []string, maxColumnLengths []int,
) {
	row := renderRow(colorizedColumns, maxColumnLengths)
	if r.isInteractive {
		// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
		// msgWithColors having the color code information embedded with it.  So we need to get
		// the right substring of it, assuming that embedded colors are just markup and do not
		// actually contribute to the length
		maxRowLength := r.terminalWidth - 1
		if maxRowLength < 0 {
			maxRowLength = 0
		}
		row = colors.TrimColorizedString(row, maxRowLength)
	}

	if row != "" {
		if r.isInteractive {
			r.colorizeAndWriteProgress(makeActionProgress(id, row))
		} else {
			r.writeSimpleMessage(row)
		}
	}
}

func (r *messageRenderer) rowUpdated(display *ProgressDisplay, row Row) {
	if r.isInteractive {
		// if we're in a terminal, then refresh everything so that all our columns line up
		r.render(display, false)
	} else {
		// otherwise, just print out this single row.
		colorizedColumns := row.ColorizedColumns()
		colorizedColumns[display.suffixColumn] += row.ColorizedSuffix()
		r.renderRow(display, "", colorizedColumns, nil)
	}
}

func (r *messageRenderer) systemMessage(display *ProgressDisplay, payload engine.StdoutEventPayload) {
	if r.isInteractive {
		// if we're in a terminal, then refresh everything.  The system events will come after
		// all the normal rows
		r.render(display, false)
	} else {
		// otherwise, in a non-terminal, just print out the actual event.
		r.writeSimpleMessage(renderStdoutColorEvent(payload, display.opts))
	}
}

func (r *messageRenderer) done(display *ProgressDisplay) {
	if r.isInteractive {
		r.render(display, false)
	}
}

func (r *messageRenderer) render(display *ProgressDisplay, done bool) {
	if !r.isInteractive || display.headerRow == nil {
		return
	}

	// make sure our stored dimension info is up to date
	r.updateTerminalDimensions()

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

	removeInfoColumnIfUnneeded(rows)

	for i, row := range rows {
		r.renderRow(display, strconv.Itoa(i), row, maxColumnLengths)
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
			r.colorizeAndWriteProgress(makeActionProgress(
				strconv.Itoa(systemID), " "))
			systemID++

			r.colorizeAndWriteProgress(makeActionProgress(
				strconv.Itoa(systemID),
				colors.Yellow+"System Messages"+colors.Reset))
			systemID++
		}

		for _, line := range lines {
			r.colorizeAndWriteProgress(makeActionProgress(
				strconv.Itoa(systemID), "  "+line))
			systemID++
		}
	}

	if done {
		r.println(display, "")
	}
}

// Ensure our stored dimension info is up to date.
func (r *messageRenderer) updateTerminalDimensions() {
	currentTerminalWidth, currentTerminalHeight, err := r.terminal.Size()
	contract.IgnoreError(err)

	if currentTerminalWidth != r.terminalWidth ||
		currentTerminalHeight != r.terminalHeight {
		r.terminalWidth = currentTerminalWidth
		r.terminalHeight = currentTerminalHeight

		// also clear our display cache as we want to reprint all lines.
		r.printedProgressCache = make(map[string]Progress)
	}
}

// ShowProgressOutput displays a progress stream from `in` to `out`, `isInteractive` describes if
// `out` is a terminal. If this is the case, it will print `\n` at the end of each line and move the
// cursor while displaying.
func ShowProgressOutput(in <-chan Progress, out io.Writer, isInteractive bool) {
	ids := make(map[string]int)

	var info terminal.Info
	if isInteractive {
		term := os.Getenv("TERM")
		if term == "" {
			term = "vt102"
		}
		info = terminal.OpenInfo(term)
	}

	for jm := range in {
		diff := 0

		if jm.Action != "" {
			if jm.ID == "" {
				contract.Failf("Must have an ID if we have an action! %s", jm.Action)
			}

			line, ok := ids[jm.ID]
			if !ok {
				// NOTE: This approach of using len(id) to
				// figure out the number of lines of history
				// only works as long as we clear the history
				// when we output something that's not
				// accounted for in the map, such as a line
				// with no ID.
				line = len(ids)
				ids[jm.ID] = line
				if info != nil {
					fmt.Fprintf(out, "\n")
				}
			}
			diff = len(ids) - line
			if info != nil {
				info.CursorUp(out, diff)
			}
		} else {
			// When outputting something that isn't progress
			// output, clear the history of previous lines. We
			// don't want progress entries from some previous
			// operation to be updated (for example, pull -a
			// with multiple tags).
			ids = make(map[string]int)
		}
		jm.Display(out, info)
		if jm.Action != "" && info != nil {
			info.CursorDown(out, diff)
		}
	}
}
