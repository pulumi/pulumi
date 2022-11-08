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
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	terminal "golang.org/x/term"

	gotty "github.com/ijc/Gotty"
	"github.com/muesli/cancelreader"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type treeRenderer struct {
	m sync.Mutex

	opts Options

	// The file descriptor, width, and height of the terminal. Used so we can trim resource messages that are too long.
	termFD     int
	termInfo   termInfo
	termWidth  int
	termHeight int
	termState  *terminal.State
	term       io.Writer

	inFile cancelreader.CancelReader

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

type fileLike interface {
	Fd() uintptr
}

func newInteractiveRenderer(in io.Reader, out io.Writer, opts Options) (progressRenderer, error) {
	if !opts.IsInteractive {
		return nil, fmt.Errorf("the tree display can only be used in interactive mode")
	}

	outFile, ok := out.(fileLike)
	if !ok {
		return nil, fmt.Errorf("stdout must be a terminal")
	}
	outFD := int(outFile.Fd())

	width, height, err := terminal.GetSize(outFD)
	if err != nil {
		return nil, fmt.Errorf("getting terminal dimensions: %w", err)
	}
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("terminal has unusable dimensions %v x %v", width, height)
	}

	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "vt102"
	}
	var info termInfo
	if info, err = gotty.OpenTermInfo(termType); err != nil {
		info = &noTermInfo{}
	}

	// Something about the tree renderer--possibly the raw terminal--does not yet play well with Windows, so for now
	// we fall back to the legacy renderer on that platform.
	if runtime.GOOS == "windows" {
		return newInteractiveMessageRenderer(out, opts, width, height, info), nil
	}

	inFile, err := cancelreader.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("preparing stdin: %w", err)
	}

	state, err := terminal.MakeRaw(outFD)
	if err != nil {
		return nil, fmt.Errorf("enabling raw terminal: %w", err)
	}

	r := &treeRenderer{
		opts:       opts,
		termFD:     outFD,
		termInfo:   info,
		termWidth:  width,
		termHeight: height,
		termState:  state,
		term:       out,
		inFile:     inFile,
		ticker:     time.NewTicker(16 * time.Millisecond),
		keys:       make(chan string),
		closed:     make(chan bool),
	}
	go r.handleEvents()
	go r.pollInput()
	return r, nil
}

func (r *treeRenderer) Close() error {
	r.inFile.Cancel()
	return terminal.Restore(r.termFD, r.termState)
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

	r.frame(true)
}

func (r *treeRenderer) println(display *ProgressDisplay, text string) {
	_, err := fmt.Fprint(r.term, r.opts.Color.Colorize(strings.ReplaceAll(text, "\n", "\r\n")))
	contract.IgnoreError(err)
	_, err = fmt.Fprint(r.term, "\r\n")
	contract.IgnoreError(err)
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
		r.treeTableRows = append(r.treeTableRows, r.renderRow(display, row, maxColumnLengths))
	}

	// Convert system events into lines.
	r.systemMessages = r.systemMessages[:0]
	for _, payload := range display.systemEventPayloads {
		msg := payload.Color.Colorize(payload.Message)
		r.systemMessages = append(r.systemMessages, splitIntoDisplayableLines(msg)...)
	}

	r.dirty = true
}

func (r *treeRenderer) markDirty() {
	r.m.Lock()
	defer r.m.Unlock()

	r.dirty = true
}

// +--------------------------------------------+
// | treetable header                           |
// | treetable contents...                      |
// +--------------------------------------------+
func (r *treeRenderer) frame(done bool) {
	r.m.Lock()
	defer r.m.Unlock()

	if !done && !r.dirty {
		return
	}
	r.dirty = false

	// Make sure our stored dimension info is up to date
	r.updateTerminalDimensions()

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
	if !done && totalHeight >= r.termHeight {
		if systemMessagesHeight > 0 {
			systemMessagesHeight = r.termHeight / 3
			if systemMessagesHeight <= 3 {
				systemMessagesHeight = 0
			} else {
				systemMessagesContentHeight := systemMessagesHeight - 3
				if len(systemMessages) > systemMessagesContentHeight {
					systemMessages = systemMessages[len(systemMessages)-systemMessagesContentHeight:]
				}
			}
		}

		treeTableHeight = r.termHeight - systemMessagesHeight - 1
		r.maxTreeTableOffset = len(treeTableRows) - treeTableHeight - 1

		treeTableRows = treeTableRows[r.treeTableOffset : r.treeTableOffset+treeTableHeight-1]

		totalHeight = treeTableHeight + systemMessagesHeight + 1
	}

	// Re-home the cursor.
	for ; r.rewind > 0; r.rewind-- {
		cursorUp(r.term, r.termInfo, 1)
		clearLine(r.term, r.termInfo)
	}
	r.rewind = totalHeight - 1

	// Render the tree table.
	r.println(nil, treeTableHeader)
	for _, row := range treeTableRows {
		r.println(nil, row)
	}

	// Render the system messages.
	if systemMessagesHeight > 0 {
		r.println(nil, "")
		r.println(nil, colors.Yellow+"System Messages"+colors.Reset)

		for _, line := range systemMessages {
			r.println(nil, "  "+line)
		}
	}

	if done && totalHeight > 0 {
		r.println(nil, "")
	}
}

func (r *treeRenderer) renderRow(display *ProgressDisplay, colorizedColumns []string, maxColumnLengths []int) string {
	uncolorizedColumns := display.uncolorizeColumns(colorizedColumns)
	row := renderRow(colorizedColumns, uncolorizedColumns, maxColumnLengths)

	// Ensure we don't go past the end of the terminal.  Note: this is made complex due to
	// msgWithColors having the color code information embedded with it.  So we need to get
	// the right substring of it, assuming that embedded colors are just markup and do not
	// actually contribute to the length
	maxRowLength := r.termWidth - 1
	if maxRowLength < 0 {
		maxRowLength = 0
	}
	return colors.TrimColorizedString(row, maxRowLength)
}

// Ensure our stored dimension info is up to date.
func (r *treeRenderer) updateTerminalDimensions() {
	currentTermWidth, currentTermHeight, err := terminal.GetSize(r.termFD)
	contract.IgnoreError(err)

	if currentTermWidth != r.termWidth ||
		currentTermHeight != r.termHeight {
		r.termWidth = currentTermWidth
		r.termHeight = currentTermHeight
	}
}

func (r *treeRenderer) handleEvents() {
	for {
		select {
		case <-r.ticker.C:
			r.frame(false)
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
		key, err := readKey(r.inFile)
		if err == nil {
			r.keys <- key
		} else if errors.Is(err, cancelreader.ErrCanceled) || errors.Is(err, io.EOF) {
			close(r.keys)
			return
		}
	}
}

func readKey(r io.Reader) (string, error) {
	type stateFunc func(b byte) (stateFunc, string)

	var stateIntermediate stateFunc
	stateIntermediate = func(b byte) (stateFunc, string) {
		if b >= 0x20 && b < 0x30 {
			return stateIntermediate, ""
		}
		switch b {
		case 'A':
			return nil, "up"
		case 'B':
			return nil, "down"
		default:
			return nil, "<control>"
		}
	}
	var stateParameter stateFunc
	stateParameter = func(b byte) (stateFunc, string) {
		if b >= 0x30 && b < 0x40 {
			return stateParameter, ""
		}
		return stateIntermediate(b)
	}
	stateBracket := func(b byte) (stateFunc, string) {
		if b == '[' {
			return stateParameter, ""
		}
		return nil, "<control>"
	}
	stateEscape := func(b byte) (stateFunc, string) {
		if b == 0x1b {
			return stateBracket, ""
		}
		if b == 3 {
			return nil, "ctrl+c"
		}
		return nil, string([]byte{b})
	}

	state := stateEscape
	for {
		var b [1]byte
		if _, err := r.Read(b[:]); err != nil {
			return "", err
		}

		next, key := state(b[0])
		if next == nil {
			return key, nil
		}
		state = next
	}
}
