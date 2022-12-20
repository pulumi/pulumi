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

// nolint: goconst
package display

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/rivo/uniseg"
)

type treeRenderer struct {
	m sync.Mutex

	opts Options

	term terminal.Terminal

	dirty  bool // True if the display has changed since the last redraw.
	rewind int  // The number of lines we need to rewind to redraw the entire screen.

	treeTableRows  []string
	systemMessages []string

	ticker *time.Ticker
	keys   chan string
	closed chan bool

	treeTableOffset    int // The scroll offset into the tree table.
	maxTreeTableOffset int // The maximum scroll offset.
}

func newInteractiveRenderer(term terminal.Terminal, opts Options) progressRenderer {
	// Something about the tree renderer--possibly the raw terminal--does not yet play well with Windows, so for now
	// we fall back to the legacy renderer on that platform.
	if !term.IsRaw() {
		return newInteractiveMessageRenderer(term, opts)
	}

	r := &treeRenderer{
		opts:   opts,
		term:   term,
		ticker: time.NewTicker(16 * time.Millisecond),
		keys:   make(chan string),
		closed: make(chan bool),
	}
	if opts.deterministicOutput {
		r.ticker.Stop()
	}
	go r.handleEvents()
	go r.pollInput()
	return r
}

func (r *treeRenderer) Close() error {
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
	display.addIndentations(rootNodes, true /*isRoot*/, "")

	maxSuffixLength := 0
	for _, v := range display.suffixesArray {
		runeCount := utf8.RuneCountInString(v)
		if runeCount > maxSuffixLength {
			maxSuffixLength = runeCount
		}
	}

	var treeTableRows [][]string
	var maxColumnLengths []int
	display.convertNodesToRows(rootNodes, maxSuffixLength, &treeTableRows, &maxColumnLengths)
	removeInfoColumnIfUnneeded(treeTableRows)

	r.treeTableRows = r.treeTableRows[:0]
	for _, row := range treeTableRows {
		rendered := renderRow(row, maxColumnLengths)
		r.treeTableRows = append(r.treeTableRows, rendered)
	}

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

	termWidth, termHeight, err := r.term.Size()
	contract.IgnoreError(err)

	treeTableRows := r.treeTableRows
	systemMessages := r.systemMessages

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

	// Layout the display. The extra '1' accounts for the fact that we terminate each line with a newline.
	totalHeight := treeTableHeight + systemMessagesHeight + 1
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

		treeTableHeight = termHeight - systemMessagesHeight - 1
		r.maxTreeTableOffset = len(treeTableRows) - treeTableHeight + 1

		treeTableRows = treeTableRows[r.treeTableOffset : r.treeTableOffset+treeTableHeight-1]

		totalHeight = treeTableHeight + systemMessagesHeight + 1

		upArrow := "  "
		if r.treeTableOffset != 0 {
			upArrow = "⬆ "
		}
		downArrow := "  "
		if r.treeTableOffset != r.maxTreeTableOffset {
			downArrow = "⬇ "
		}
		footer := fmt.Sprintf("%smore%s", upArrow, downArrow)
		padding := termWidth - uniseg.GraphemeClusterCount(footer)
		treeTableFooter = strings.Repeat(" ", padding) + footer

		if systemMessagesHeight > 0 {
			treeTableFooter += "\n"
		}
	}

	// Re-home the cursor.
	r.print("\r")
	for ; r.rewind > 0; r.rewind-- {
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

	if done && totalHeight > 0 {
		r.overln("")
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
			case "ctrl+c":
				sigint()
			case "up":
				if r.treeTableOffset > 0 {
					r.treeTableOffset--
				}
				r.markDirty()
			case "down":
				if r.treeTableOffset < r.maxTreeTableOffset {
					r.treeTableOffset++
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
