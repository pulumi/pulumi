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
	"unicode/utf8"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type treeRenderer struct {
	m sync.Mutex

	opts Options

	display *ProgressDisplay
	term    terminal.Terminal

	permalink string

	dirty  bool // True if the display has changed since the last redraw.
	rewind int  // The number of lines we need to rewind to redraw the entire screen.

	treeTableRows         []string
	systemMessages        []string
	statusMessage         string
	statusMessageDeadline time.Time

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

func (r *treeRenderer) initializeDisplay(display *ProgressDisplay) {
	r.display = display
}

func (r *treeRenderer) tick() {
	r.markDirty()
}

func (r *treeRenderer) rowUpdated(_ Row) {
	r.markDirty()
}

func (r *treeRenderer) systemMessage(_ engine.StdoutEventPayload) {
	r.markDirty()
}

func (r *treeRenderer) done() {
	r.markDirty()

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

func (r *treeRenderer) println(text string) {
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

func (r *treeRenderer) render() {
	contract.Assertf(!r.m.TryLock(), "treeRenderer.render() MUST be called from within a locked context")

	if r.display.headerRow == nil {
		return
	}

	// Render the resource tree table into rows.
	rootNodes := r.display.generateTreeNodes()
	rootNodes = r.display.filterOutUnnecessaryNodesAndSetDisplayTimes(rootNodes)
	sortNodes(rootNodes)
	r.display.addIndentations(rootNodes, true /*isRoot*/, "")

	maxSuffixLength := 0
	for _, v := range r.display.suffixesArray {
		runeCount := utf8.RuneCountInString(v)
		if runeCount > maxSuffixLength {
			maxSuffixLength = runeCount
		}
	}

	var treeTableRows [][]string
	var maxColumnLengths []int
	r.display.convertNodesToRows(rootNodes, maxSuffixLength, &treeTableRows, &maxColumnLengths)
	removeInfoColumnIfUnneeded(treeTableRows)

	r.treeTableRows = r.treeTableRows[:0]
	for _, row := range treeTableRows {
		rendered := renderRow(row, maxColumnLengths)
		r.treeTableRows = append(r.treeTableRows, rendered)
	}

	// Convert system events into lines.
	r.systemMessages = r.systemMessages[:0]
	for _, payload := range r.display.systemEventPayloads {
		msg := payload.Color.Colorize(payload.Message)
		r.systemMessages = append(r.systemMessages, splitIntoDisplayableLines(msg)...)
	}
}

func (r *treeRenderer) markDirty() {
	r.m.Lock()
	defer r.m.Unlock()

	r.dirty = true
	if r.opts.deterministicOutput {
		r.frame(true, false)
	}
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

	if !done && !r.dirty {
		return
	}
	r.dirty = false

	contract.Assertf(r.display != nil, "treeRender.initializeDisplay MUST be called before rendering")
	r.render()

	termWidth, termHeight, err := r.term.Size()
	contract.IgnoreError(err)

	treeTableRows := r.treeTableRows
	systemMessages := r.systemMessages
	statusMessage := r.statusMessage

	var treeTableHeight int
	var treeTableHeader string
	if len(r.treeTableRows) > 0 {
		treeTableHeader, treeTableRows = treeTableRows[0], treeTableRows[1:]
		treeTableHeight = 1 + len(treeTableRows)
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
		r.maxTreeTableOffset = len(treeTableRows) - treeTableHeight + 1
		if r.maxTreeTableOffset < 0 {
			r.maxTreeTableOffset = 0
		}
		scrollable := r.maxTreeTableOffset != 0

		if r.treeTableOffset > r.maxTreeTableOffset {
			r.treeTableOffset = r.maxTreeTableOffset
		}

		if autoscroll {
			r.treeTableOffset = r.maxTreeTableOffset
		}

		if treeTableHeight <= 0 {
			// Ensure that the treeTableHeight is at least 1 to avoid going out of bounds.
			treeTableHeight = 1
		}
		if r.treeTableOffset+treeTableHeight-1 < len(treeTableRows) {
			treeTableRows = treeTableRows[r.treeTableOffset : r.treeTableOffset+treeTableHeight-1]
		} else if r.treeTableOffset < len(treeTableRows) {
			treeTableRows = treeTableRows[r.treeTableOffset:]
		}

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

		if padding < 0 {
			// Padding can potentially go negative on very small terminals.
			// This will cause a panic. To avoid this, we clamp the padding to 0.
			// The user won't be able to see anything anyway.
			padding = 0
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
	for _, row := range treeTableRows {
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
			case terminal.KeyUp, "k":
				if r.treeTableOffset > 0 {
					r.treeTableOffset--
				}
				r.markDirty()
			case terminal.KeyDown, "j":
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
			case terminal.KeyHome, "g":
				if r.treeTableOffset > 0 {
					r.treeTableOffset = 0
				}
				r.markDirty()
			case terminal.KeyEnd, "G":
				if r.treeTableOffset < r.maxTreeTableOffset {
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
