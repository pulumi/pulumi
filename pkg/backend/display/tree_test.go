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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
)

// Table Test using different terminal widths and heights to ensure that the display does not panic.
func TestTreeFrameSize(t *testing.T) {
	t.Parallel()

	// Table Test using different terminal widths and heights
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"narrow", 1, 100},
		{"short", 100, 1},
		{"small", 1, 1},
		{"normal", 100, 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			term := terminal.NewMockTerminal(&buf, tt.width, tt.height, true)
			treeRenderer, display := createRendererAndDisplay(term, true)

			// Fill the renderer with too many rows of strings to fit in the terminal.
			addManySystemEvents(display, 1000, strings.Repeat("a", 1000))
			addManyNormalEvents(display, 1000, strings.Repeat("a", 1000))

			// Required to get the frame to render.
			treeRenderer.markDirty()

			// This should not panic.
			treeRenderer.frame(false /* locked */, false /* done */)
		})
	}
}

func createRendererAndDisplay(term terminal.Terminal, initializeDisplay bool) (*treeRenderer, *ProgressDisplay) {
	treeRenderer := newInteractiveRenderer(term, "this-is-a-fake-permalink", Options{
		Color: colors.Always,
	}).(*treeRenderer)
	display := &ProgressDisplay{
		stack:                 tokens.MustParseStackName("stack"),
		eventUrnToResourceRow: make(map[resource.URN]ResourceRow),
		renderer:              treeRenderer,
	}
	treeRenderer.ticker.Stop()

	if initializeDisplay {
		treeRenderer.initializeDisplay(display)
	}

	return treeRenderer, display
}

func addManySystemEvents(display *ProgressDisplay, count int, message string) {
	for i := 0; i < count; i++ {
		display.handleSystemEvent(engine.StdoutEventPayload{
			Message: message,
			Color:   colors.Always,
		})
	}
}

func addManyNormalEvents(display *ProgressDisplay, count int, message string) {
	for i := 0; i < count; i++ {
		display.processNormalEvent(engine.NewEvent(
			engine.DiagEventPayload{
				URN:     resource.NewURN("stack", "project", "qualifier", "typ", fmt.Sprintf("row-%d", i)),
				Message: message,
			}))
	}
}

func TestTreeKeyboardHandling(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	term := terminal.NewMockTerminal(&buf, 80, 24, true)
	treeRenderer, display := createRendererAndDisplay(term, true)

	// Fill the renderer with strings that are too long to fit in the terminal.
	addManySystemEvents(display, 1000, strings.Repeat("a", 1000))
	addManyNormalEvents(display, 1000, strings.Repeat("a", 1000))

	// Required to get the frame to render.
	treeRenderer.markDirty()

	// This should not panic.
	treeRenderer.frame(false /* locked */, false /* done */)

	// manually move the current position to the middle
	treeRenderer.treeTableOffset = treeRenderer.maxTreeTableOffset / 2
	treeRenderer.markDirty()
	treeRenderer.frame(false /* locked */, false /* done */)

	tests := []struct {
		name             string
		key              string
		expectedChange   int
		expectedAbsolute int
	}{
		{"up arrow", terminal.KeyUp, -1, 0},
		{"VIM up", "k", -1, 0},
		{"Down arrow", terminal.KeyDown, 1, 0},
		{"VIM down", "j", 1, 0},
		{"Page Up", terminal.KeyPageUp, -24, 0},
		{"Page Down", terminal.KeyPageDown, 24, 0},
		{"Home", terminal.KeyHome, 0, 0},
		{"End", terminal.KeyEnd, 0, treeRenderer.maxTreeTableOffset},
		{"VIM home", "g", 0, 0},
		{"VIM end", "G", 0, treeRenderer.maxTreeTableOffset},
	}

	for _, tt := range tests {
		initialValue := treeRenderer.treeTableOffset
		treeRenderer.handleKey(tt.key)
		treeRenderer.frame(false /* locked */, false /* done */)

		if tt.expectedChange != 0 {
			assert.Equal(t, tt.expectedChange+initialValue, treeRenderer.treeTableOffset,
				"Current line was not moved to the expected value of %d(%d) for %v",
				tt.expectedChange+initialValue, tt.expectedChange, tt.name)
		} else {
			assert.Equal(t, tt.expectedAbsolute, treeRenderer.treeTableOffset,
				"Current line was not moved to the expected value of %d for %v", tt.expectedAbsolute, tt.name)
		}
	}
}

func TestTreeRenderCallsFrameOnTick(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	term := terminal.NewMockTerminal(&buf, 80, 24, true)
	treeRenderer, display := createRendererAndDisplay(term, true)

	addManySystemEvents(display, 1000, strings.Repeat("a", 1000))

	// Mark a row as updated, this used to invoke render, it should now
	// only mark the renderer as dirty. This is only cleared when the
	// frame function is called. the ticker is currently stopped, so
	// this should never happen
	treeRenderer.rowUpdated(&resourceRowData{})

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Truef(t, treeRenderer.dirty, "Expecting the renderer to be dirty until we explicitly call frame")
		// the treeRenderer has never rendered, so the systemMessages array
		// should be empty at this point
		assert.Emptyf(t, treeRenderer.systemMessages,
			"Not expecting system messages to be populated until rendering happens.")
		assert.Equalf(t, "<%hide-cursor%>", buf.String(), "No content should have been written to the terminal yet")
	}()

	// This should trigger a render, and reset the dirty flag to false.
	// This is normally called by the ticker event in treeRenderer.eventHandler, but for the test
	// we have stopped the ticker.
	treeRenderer.frame(false, false)

	// If dirty is true here, then there was no render operation
	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Falsef(t, treeRenderer.dirty, "Expecting the renderer to not be dirty after a frame is called")

		// An observable consequence of rendering is that the treeRenderer now has an array of system messages
		assert.Equalf(t, 1000, len(treeRenderer.systemMessages),
			"Expecting 1000 system messages to now be in the tree renderer")
	}()

	// Check that at least one system message was written to the mock terminal,
	// and trimmed to the terminal width appropriately.
	terminalText := buf.String()
	assert.Contains(t, terminalText, "pulumi:pulumi:Stack")
	assert.Contains(t, terminalText, "System Messages")
	assert.Contains(t, terminalText, strings.Repeat("a", 70))
}

// Tests that when the tree renderer correctly clears stale rows when it renders
// a frame with fewer rows than the one which preceded it.
func TestTreeRenderRewindsCorrectly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	term := terminal.NewMockTerminal(&buf, 80, 24, true)
	treeRenderer, display := createRendererAndDisplay(term, true)

	addManySystemEvents(display, 10, strings.Repeat("a", 32))

	// Mark a row as updated, this used to invoke render, it should now
	// only mark the renderer as dirty. This is only cleared when the
	// frame function is called. the ticker is currently stopped, so
	// this should never happen
	treeRenderer.rowUpdated(&resourceRowData{})

	// This should trigger a render, and reset the dirty flag to false.
	// This is normally called by the ticker event in treeRenderer.eventHandler, but for the test
	// we have stopped the ticker.
	treeRenderer.frame(false, false)

	// 10 system messages, plus the table header, stack name, a blank line and the
	// system messages header.
	assert.Equal(t, 14, treeRenderer.rewind, "Expected 14 lines to have been written")

	// Hackily clear out the system messages.
	(func() {
		treeRenderer.display.eventMutex.Lock()
		defer treeRenderer.display.eventMutex.Unlock()

		treeRenderer.display.systemEventPayloads = nil
	})()

	// Render another frame.
	treeRenderer.rowUpdated(&resourceRowData{})
	treeRenderer.frame(false, false)

	// The table header and stack name.
	assert.Equal(t, 2, treeRenderer.rewind, "Expected 4 lines to have been written")

	// The 10 line clears that should have been emitted as part of rewinding the
	// output.
	assert.Contains(t, buf.String(), strings.Repeat("<%clear-to-end%>\n", 10))
}

func TestTreeRenderDoesntRenderBeforeItHasContent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	term := terminal.NewMockTerminal(&buf, 80, 24, true)
	treeRenderer, display := createRendererAndDisplay(term, false)

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Falsef(t, treeRenderer.dirty, "Expecting the renderer to not be dirty when initialized")
	}()

	// Call the external tick, this normally happens once a second
	display.processTick()

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Falsef(t, treeRenderer.dirty,
			"Expecting the renderer to not be after a tick without display initialized")
	}()

	treeRenderer.initializeDisplay(display)
	display.processTick()

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Falsef(t, treeRenderer.dirty,
			"Expecting the renderer to not be dirty after a tick before row headers are initialized")
	}()

	func() {
		display.eventMutex.Lock()
		defer display.eventMutex.Unlock()
		display.ensureHeaderAndStackRows()
	}()

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Falsef(t, treeRenderer.dirty,
			"Expecting the renderer to not be dirty after row headers are initialized, but before a tick")
	}()

	display.processTick()

	func() {
		treeRenderer.m.Lock()
		defer treeRenderer.m.Unlock()
		assert.Truef(t, treeRenderer.dirty,
			"Expecting the renderer to be dirty after headers are initialized, and after a tick has happened")
	}()
}
