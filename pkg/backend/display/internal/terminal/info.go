// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"

	gotty "github.com/ijc/Gotty"
)

type Info interface {
	Parse(attr string, params ...interface{}) (string, error)

	ClearEnd(out io.Writer)
	ClearLine(out io.Writer)
	CarriageReturn(out io.Writer)
	CursorUp(out io.Writer, count int)
	CursorDown(out io.Writer, count int)
	HideCursor(out io.Writer)
	ShowCursor(out io.Writer)
}

/* Satisfied by gotty.TermInfo as well as noTermInfo from below */
type termInfo interface {
	Parse(attr string, params ...interface{}) (string, error)
}

type noTermInfo int // canary used when no terminfo.

func (ti noTermInfo) Parse(attr string, params ...interface{}) (string, error) {
	return "", errors.New("noTermInfo")
}

type info struct {
	termInfo
}

var _ = Info(info{})

func OpenInfo(terminal string) Info {
	if i, err := gotty.OpenTermInfo(terminal); err == nil {
		return info{i}
	}
	return info{noTermInfo(0)}
}

func (i info) ClearLine(out io.Writer) {
	// el2 (clear whole line) is not exposed by terminfo.

	// First clear line from beginning to cursor
	if attr, err := i.Parse("el1"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[1K")
	}
	// Then clear line from cursor to end
	if attr, err := i.Parse("el"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[K")
	}
}

func (i info) ClearEnd(out io.Writer) {
	// clear line from cursor to end
	if attr, err := i.Parse("el"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[K")
	}
}

func (i info) CarriageReturn(out io.Writer) {
	fmt.Fprint(out, "\r")
}

func (i info) CursorUp(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	if attr, err := i.Parse("cuu", count); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[%dA", count)
	}
}

func (i info) CursorDown(out io.Writer, count int) {
	if count == 0 { // Should never be the case, but be tolerant
		return
	}
	if attr, err := i.Parse("cud", count); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[%dB", count)
	}
}

func (i info) HideCursor(out io.Writer) {
	if attr, err := i.Parse("civis"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	}
}

func (i info) ShowCursor(out io.Writer) {
	if attr, err := i.Parse("cnorm"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	}
}
