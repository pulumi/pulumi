// Copyright 2016-2022, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type treeRenderer struct {
	m sync.Mutex

	opts Options

	term       terminal.Terminal
	termWidth  int
	termHeight int

	permalink string

	dirty  bool // True if the display has changed since the last redraw.
	rewind int  // The number of lines we need to rewind to redraw the entire screen.

	nodes                 []*treeNode
	systemMessages        []string
	statusMessage         string
	statusMessageDeadline time.Time
	reflowed              bool // True if the table has been reflowed.

	ticker *time.Ticker
	keys   chan string
	closed chan bool

	treeTableOffset    int // The scroll offset into the tree table.
	maxTreeTableOffset int // The maximum scroll offset.
}

func newInteractiveRenderer(term terminal.Terminal, permalink string, opts Options) progressRenderer {
	// Something about the tree renderer--possibly the raw terminal--does not yet play well with Windows, so for now
	// we fall back to the legacy renderer on that platform.
	if !term.IsRaw() {
		return newInteractiveMessageRenderer(term, opts)
	}
	term.HideCursor()

	r := &treeRenderer{
		opts:      opts,
		term:      term,
		permalink: permalink,
		ticker:    time.NewTicker(16 * time.Millisecond),
		keys:      make(chan string),
		closed:    make(chan bool),
	}
	if opts.deterministicOutput {
		r.ticker.Stop()
	}
	go r.handleEvents()
	go r.pollInput()
	return r
}

func (r *treeRenderer) Close() error {
	r.term.ShowCursor()
	return r.term.Close()
}

func (r *treeRenderer) tick(display *ProgressDisplay) {
	r.render(display)
}

func (r *treeRenderer) rowUpdated(display *ProgressDisplay, _ Row) {
	r.render(display)
}

func (r *treeRenderer) systemMessage(display *ProgressDisplay, _ engine.StdoutEventPayload) {
	r.render(display)
}

func (r *treeRenderer) done(display *ProgressDisplay) {
	r.render(display)

	r.ticker.Stop()
	r.closed <- true
	close(r.closed)

	r.frame(false, true)
}

func (r *treeRenderer) showStatusMessage(msg string, duration time.Duration) {
	r.m.Lock()
	defer r.m.Unlock()

	r.statusMessage, r.statusMessageDeadline = msg, time.Now().Add(duration)
}

func (r *treeRenderer) print(text string) {
	_, err := r.term.Write([]byte(r.opts.Color.Colorize(text)))
	contract.IgnoreError(err)
}

func (r *treeRenderer) println(display *ProgressDisplay, text string) {
	r.print(text)
	r.print("\n")
}

func (r *treeRenderer) over(text string) {
	r.print(text)
	r.term.ClearEnd()
}

func (r *treeRenderer) overln(text string) {
	r.over(text)
	r.print("\n")
}

func (r *treeRenderer) render(display *ProgressDisplay) {
	r.m.Lock()
	defer r.m.Unlock()

	if display.headerRow == nil {
		return
	}

	// Render the resource tree table into rows.
	rootNodes := display.generateTreeNodes()
	rootNodes = display.filterOutUnnecessaryNodesAndSetDisplayTimes(rootNodes)
	sortNodes(rootNodes)
	r.nodes = rootNodes

	// Convert system events into lines.
	r.systemMessages = r.systemMessages[:0]
	for _, payload := range display.systemEventPayloads {
		msg := payload.Color.Colorize(payload.Message)
		r.systemMessages = append(r.systemMessages, splitIntoDisplayableLines(msg)...)
	}

	r.dirty = true
	if r.opts.deterministicOutput {
		r.frame(true, false)
	}
}

func (r *treeRenderer) markDirty() {
	r.m.Lock()
	defer r.m.Unlock()

	r.dirty = true
}

// +--------------------------------------------+
// | treetable header                           |
// | treetable contents...                      |
// | treetable footer                           |
// | system messages header                     |
// | system messages contents...                |
// | status message                             |
// +--------------------------------------------+
func (r *treeRenderer) frame(locked, done bool) {
	if !locked {
		r.m.Lock()
		defer r.m.Unlock()
	}

	// Determine whether or not the terminal has been resized since the last frame. If it has, we want to invalidate
	// the last reflow decision and re-render.
	termWidth, termHeight, err := r.term.Size()
	contract.IgnoreError(err)
	if termWidth != r.termWidth || termHeight != r.termHeight {
		r.termWidth, r.termHeight = termWidth, termHeight
		r.reflowed, r.dirty = false, true
	}

	if !done && !r.dirty {
		return
	}
	r.dirty = false

	systemMessages := r.systemMessages
	statusMessage := r.statusMessage

	treeTableRows := generateRows(r.nodes, r.reflowed)
	columnWidths := measureColumns(treeTableRows)

	// Figure out if we need to reflow the tree table.
	if !r.reflowed && rowWidth(columnWidths) > termWidth {
		treeTableRows = generateRows(r.nodes, true /*reflow*/)
		columnWidths = measureColumns(treeTableRows)
		r.reflowed = true
	}

	treeTable := make([]string, len(treeTableRows))
	for i, row := range treeTableRows {
		treeTable[i] = renderRow(row, columnWidths)
	}

	var treeTableHeight int
	var treeTableHeader string
	if len(treeTable) > 0 {
		treeTableHeader, treeTable = treeTable[0], treeTable[1:]
		treeTableHeight = 1 + len(treeTable)
	}

	systemMessagesHeight := len(systemMessages)
	if len(systemMessages) > 0 {
		systemMessagesHeight += 3 // Account for padding + title
	}

	statusMessageHeight := 0
	if !done && r.statusMessage != "" {
		statusMessageHeight = 1
	}

	// Enable autoscrolling if the display is scrolled to its maximum offset.
	autoscroll := r.treeTableOffset == r.maxTreeTableOffset

	// Layout the display. The extra '1' accounts for the fact that we terminate each line with a newline.
	totalHeight := treeTableHeight + systemMessagesHeight + statusMessageHeight + 1
	r.maxTreeTableOffset = 0

	// If this is not the final frame and the terminal is not large enough to show the entire display:
	// - If there are no system messages, devote the entire display to the tree table
	// - If there are system messages, devote the first two thirds of the display to the tree table and the
	//   last third to the system messages
	var treeTableFooter string
	if !done && totalHeight >= termHeight {
		if systemMessagesHeight > 0 {
			systemMessagesHeight = termHeight / 3
			if systemMessagesHeight <= 3 {
				systemMessagesHeight = 0
			} else {
				systemMessagesContentHeight := systemMessagesHeight - 3
				if len(systemMessages) > systemMessagesContentHeight {
					systemMessages = systemMessages[len(systemMessages)-systemMessagesContentHeight:]
				}
			}
		}

		// If there are no system messages and we have a status message to display, fold the status message into the
		// last line of the tree table (where the scroll indicator is displayed).
		mergeLastLine := systemMessagesHeight == 0 && statusMessageHeight != 0

		treeTableHeight = termHeight - systemMessagesHeight - statusMessageHeight - 1
		r.maxTreeTableOffset = len(treeTable) - treeTableHeight + 1
		scrollable := r.maxTreeTableOffset != 0

		if autoscroll {
			r.treeTableOffset = r.maxTreeTableOffset
		}

		treeTable = treeTable[r.treeTableOffset : r.treeTableOffset+treeTableHeight-1]

		totalHeight = treeTableHeight + systemMessagesHeight + statusMessageHeight + 1

		footer := ""
		if scrollable {
			upArrow := "  "
			if r.treeTableOffset != 0 {
				upArrow = "⬆ "
			}
			downArrow := "  "
			if r.treeTableOffset != r.maxTreeTableOffset {
				downArrow = "⬇ "
			}
			footer = colors.BrightBlue + fmt.Sprintf("%smore%s", upArrow, downArrow) + colors.Reset
		}
		padding := termWidth - colors.MeasureColorizedString(footer)

		// Combine any last-line content.
		prefix := ""
		if mergeLastLine {
			prefix = r.clampLine(statusMessage, padding-1) + " "
			padding -= colors.MeasureColorizedString(prefix)
			statusMessageHeight, statusMessage = 0, ""
		}

		treeTableFooter = r.opts.Color.Colorize(prefix + strings.Repeat(" ", padding) + footer)

		if systemMessagesHeight > 0 {
			treeTableFooter += "\n"
		}
	}

	// Re-home the cursor.
	r.print("\r")
	for ; r.rewind > 0; r.rewind-- {
		// If there is content that we won't overwrite, clear it.
		if r.rewind > totalHeight-1 {
			r.term.ClearEnd()
		}
		r.term.CursorUp(1)
	}
	r.rewind = totalHeight - 1

	// Render the tree table.
	r.overln(r.clampLine(treeTableHeader, termWidth))
	for _, row := range treeTable {
		r.overln(r.clampLine(row, termWidth))
	}
	if treeTableFooter != "" {
		r.over(treeTableFooter)
	}

	// Render the system messages.
	if systemMessagesHeight > 0 {
		r.overln("")
		r.overln(colors.Yellow + "System Messages" + colors.Reset)

		for _, line := range systemMessages {
			r.overln("  " + line)
		}
	}

	// Render the status message, if any.
	if statusMessageHeight != 0 {
		padding := termWidth - colors.MeasureColorizedString(statusMessage)

		r.overln("")
		r.over(statusMessage + strings.Repeat(" ", padding))
	}

	if done && totalHeight > 0 {
		r.overln("")
	}

	// Handle the status message timer. We do this at the end to ensure that any message is displayed for at least one
	// frame.
	if !r.statusMessageDeadline.IsZero() && r.statusMessageDeadline.Before(time.Now()) {
		r.statusMessage, r.statusMessageDeadline = "", time.Time{}
	}
}

func (r *treeRenderer) clampLine(line string, maxWidth int) string {
	// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
	// msgWithColors having the color code information embedded with it.  So we need to get
	// the right substring of it, assuming that embedded colors are just markup and do not
	// actually contribute to the length
	maxRowLength := maxWidth - 1
	if maxRowLength < 0 {
		maxRowLength = 0
	}
	return colors.TrimColorizedString(line, maxRowLength)
}

func (r *treeRenderer) handleEvents() {
	for {
		select {
		case <-r.ticker.C:
			r.frame(false, false)
		case key := <-r.keys:
			switch key {
			case terminal.KeyCtrlC:
				sigint()
			case terminal.KeyCtrlO:
				if r.permalink != "" {
					if err := browser.OpenURL(r.permalink); err != nil {
						r.showStatusMessage(colors.Red+"could not open browser"+colors.Reset, 5*time.Second)
					}
				}
			case terminal.KeyUp:
				if r.treeTableOffset > 0 {
					r.treeTableOffset--
				}
				r.markDirty()
			case terminal.KeyDown:
				if r.treeTableOffset < r.maxTreeTableOffset {
					r.treeTableOffset++
				}
				r.markDirty()
			case terminal.KeyPageUp:
				_, termHeight, err := r.term.Size()
				contract.IgnoreError(err)

				if r.treeTableOffset > termHeight {
					r.treeTableOffset -= termHeight
				} else {
					r.treeTableOffset = 0
				}
				r.markDirty()
			case terminal.KeyPageDown:
				_, termHeight, err := r.term.Size()
				contract.IgnoreError(err)

				if r.maxTreeTableOffset-r.treeTableOffset > termHeight {
					r.treeTableOffset += termHeight
				} else {
					r.treeTableOffset = r.maxTreeTableOffset
				}
				r.markDirty()
			}
		case <-r.closed:
			return
		}
	}
}

func (r *treeRenderer) pollInput() {
	for {
		key, err := r.term.ReadKey()
		if err == nil {
			r.keys <- key
		} else if errors.Is(err, io.EOF) {
			close(r.keys)
			return
		}
	}
}
