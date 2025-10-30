// Copyright 2016-2025, Pulumi Corporation.
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

package terminal

import (
	"fmt"
	"io"

	"github.com/xo/terminfo"
)

// Info provides methods to print ANSI escape codes to a writer.
type Info interface {
	ClearEnd(out io.Writer)
	ClearLine(out io.Writer)
	CarriageReturn(out io.Writer)
	CursorUp(out io.Writer, count int)
	CursorDown(out io.Writer, count int)
	HideCursor(out io.Writer)
	ShowCursor(out io.Writer)
}

// OpenInfo returns a new Info instance for the given terminal, ensuring that the
// correct ANSI escape codes are used. If the terminal is not found in the
// terminfo database, a fallback implementation is used.
func OpenInfo(terminal string) Info {
	ti, err := terminfo.Load(terminal)
	if err != nil {
		return defaultInfo{}
	}
	return info{ti}
}

// info implements the Info interface, using the terminfo database to determine
// the correct terminal codes.
type info struct {
	ti *terminfo.Terminfo
}

var _ = Info(info{})

func (i info) ClearLine(out io.Writer) {
	// First clear line from beginning to cursor
	i.ti.Fprintf(out, terminfo.ClrBol)
	// Then clear line from cursor to end
	i.ti.Fprintf(out, terminfo.ClrEol)
}

func (i info) ClearEnd(out io.Writer) {
	// clear line from cursor to end
	i.ti.Fprintf(out, terminfo.ClrEol)
}

func (i info) CarriageReturn(out io.Writer) {
	fmt.Fprint(out, "\r")
}

func (i info) CursorUp(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	i.ti.Fprintf(out, terminfo.ParmUpCursor, count)
}

func (i info) CursorDown(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	i.ti.Fprintf(out, terminfo.ParmDownCursor, count)
}

func (i info) HideCursor(out io.Writer) {
	i.ti.Fprintf(out, terminfo.CursorInvisible)
}

func (i info) ShowCursor(out io.Writer) {
	i.ti.Fprintf(out, terminfo.CursorNormal)
}

// defaultInfo is a fallback implementation of Info interface for terminals not
// found in the terminfo database. The terminal codes should work reasonably
// well for any terminal.
type defaultInfo struct{}

var _ = Info(defaultInfo{})

func (i defaultInfo) ClearLine(out io.Writer) {
	// First clear line from beginning to cursor
	fmt.Fprintf(out, "\x1b[1K")
	// Then clear line from cursor to end
	fmt.Fprintf(out, "\x1b[K")
}

func (i defaultInfo) ClearEnd(out io.Writer) {
	// clear line from cursor to end
	fmt.Fprintf(out, "\x1b[K")
}

func (i defaultInfo) CarriageReturn(out io.Writer) {
	fmt.Fprint(out, "\r")
}

func (i defaultInfo) CursorUp(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	fmt.Fprintf(out, "\x1b[%dA", count)
}

func (i defaultInfo) CursorDown(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	fmt.Fprintf(out, "\x1b[%dB", count)
}

func (i defaultInfo) HideCursor(out io.Writer) {
}

func (i defaultInfo) ShowCursor(out io.Writer) {
}
