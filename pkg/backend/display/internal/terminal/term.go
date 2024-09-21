// Copyright 2022-2024, Pulumi Corporation.
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

type Terminal interface {
	io.WriteCloser

	IsRaw() bool
	Size() (width, height int, err error)

	ClearLine()
	ClearEnd()
	CarriageReturn()
	CursorUp(count int)
	CursorDown(count int)
	HideCursor()
	ShowCursor()

	ReadKey() (string, error)
}

var ErrNotATerminal = errors.New("not a terminal")

type terminal struct {
	fd   int
	info Info
	raw  bool
	save *term.State

	out io.Writer
	in  cancelreader.CancelReader
}

var _ Terminal = &terminal{}

func Open(in io.Reader, out io.Writer, raw bool) (Terminal, error) {
	type fileLike interface {
		Fd() uintptr
	}

	outFile, ok := out.(fileLike)
	if !ok {
		return nil, ErrNotATerminal
	}
	if outFile.Fd() > math.MaxInt32 {
		return nil, fmt.Errorf("file descriptor too large: %v", outFile.Fd())
	}
	//nolint:gosec // uintptr -> int conversion checked above
	outFd := int(outFile.Fd())

	width, height, err := term.GetSize(outFd)
	if err != nil {
		return nil, fmt.Errorf("getting dimensions: %w", err)
	}
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("unusable dimensions (%v x %v)", width, height)
	}

	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "vt102"
	}
	info := OpenInfo(termType)

	var save *term.State
	var inFile cancelreader.CancelReader
	if raw {
		if save, err = term.MakeRaw(outFd); err != nil {
			return nil, fmt.Errorf("enabling raw mode: %w", err)
		}
		if inFile, err = cancelreader.NewReader(in); err != nil {
			return nil, ErrNotATerminal
		}
	}

	return &terminal{
		fd:   outFd,
		info: info,
		raw:  raw,
		save: save,
		out:  out,
		in:   inFile,
	}, nil
}

func (t *terminal) IsRaw() bool {
	return t.raw
}

func (t *terminal) Close() error {
	t.in.Cancel()
	if t.save != nil {
		return term.Restore(t.fd, t.save)
	}
	return nil
}

func (t *terminal) Size() (width, height int, err error) {
	return term.GetSize(t.fd)
}

func (t *terminal) Write(b []byte) (int, error) {
	if !t.raw {
		return t.out.Write(b)
	}

	written := 0
	for {
		newline := bytes.IndexByte(b, '\n')
		if newline == -1 {
			w, err := t.out.Write(b)
			written += w
			return written, err
		}

		w, err := t.out.Write(b[:newline])
		written += w
		if err != nil {
			return written, err
		}

		if _, err = t.out.Write([]byte{'\r', '\n'}); err != nil {
			return written, err
		}
		written++

		b = b[newline+1:]
	}
}

func (t *terminal) ClearLine() {
	t.info.ClearLine(t.out)
}

func (t *terminal) ClearEnd() {
	t.info.ClearEnd(t.out)
}

func (t *terminal) CarriageReturn() {
	t.info.CarriageReturn(t.out)
}

func (t *terminal) CursorUp(count int) {
	t.info.CursorUp(t.out, count)
}

func (t *terminal) CursorDown(count int) {
	t.info.CursorDown(t.out, count)
}

func (t *terminal) HideCursor() {
	t.info.HideCursor(t.out)
}

func (t *terminal) ShowCursor() {
	t.info.ShowCursor(t.out)
}

type stateFunc func(b byte) stateFunc

// ansiKind
type ansiKind int

const (
	ansiError   ansiKind = iota // ansiError indicates a decoding error
	ansiKey                     // ansiKey indicates a normal keypress
	ansiEscape                  // ansiEscape indicates an ANSI escape sequence
	ansiControl                 // ansiControl indicates an ANSI control sequence
)

// ansiDecoder is responsible for decoding ANSI escape and control sequences as per ECMA-48 et. al.
//
//   - ANSI escape sequences are of the form "'\x1b' (intermediate bytes) <final byte>", where intermediate bytes are in
//     the range [0x20, 0x30) and the final byte is in the range [0x30, 0x7f)
//   - ANSI control sequences are of the form "'\x1b' '[' (parameter bytes) (intermediate bytes) <final byte>", where
//     parameter bytes are in the range [0x30, 0x40), intermediate bytes are in the range [0x20, 0x30), and the final
//     byte is in the range [0x40, 0x7f). Note that in most references (incl. ECMA-48), "'\x1b' '['" is referred to as
//     a Control Sequence Indicator, or CSI.
//
// Any sequence that is introduced with a byte that is not '\x1b' is treated as a normal keypress.
//
// No post-processing is done on the decoded sequences to ensure that e.g. the parameter count, etc. is valid--any such
// processing is up to the consumer.
type ansiDecoder struct {
	kind         ansiKind // the kind of the decoded sequence.
	params       []byte   // the decoded control sequence's parameter bytes, if any
	intermediate []byte   // the decoded escape or control sequence's intermediate bytes, if any.
	final        byte     // the final byte of the sequence.
}

// stateControlIntermediate decodes optional intermediate bytes and the final byte of a control sequence.
func (d *ansiDecoder) stateControlIntermediate(b byte) stateFunc {
	if b >= 0x20 && b < 0x30 {
		d.intermediate = append(d.intermediate, b)
		return d.stateControlIntermediate
	}
	if b >= 0x40 && b < 0x7f {
		d.kind = ansiControl
	}
	d.final = b
	return nil
}

// stateControl decodes optional parameter bytes of a control sequence.
func (d *ansiDecoder) stateControl(b byte) stateFunc {
	if b >= 0x30 && b < 0x40 {
		d.params = append(d.params, b)
		return d.stateControl
	}
	return d.stateControlIntermediate(b)
}

// stateEscapeIntermediate decodes optional intermediate bytes and the final byte of an escape sequence.
func (d *ansiDecoder) stateEscapeIntermediate(b byte) stateFunc {
	if b >= 0x20 && b < 0x30 {
		d.intermediate = append(d.intermediate, b)
		return d.stateEscapeIntermediate
	}
	if b >= 0x30 && b < 0x7f {
		d.kind = ansiEscape
	}
	d.final = b
	return nil
}

// stateEscape determines whether a sequence beginning with '\x1b' is an escape sequence or a control sequence.
func (d *ansiDecoder) stateEscape(b byte) stateFunc {
	if b == '[' {
		return d.stateControl
	}
	return d.stateEscapeIntermediate(b)
}

// stateInit is the initial state for the decoder.
func (d *ansiDecoder) stateInit(b byte) stateFunc {
	if b == 0x1b {
		return d.stateEscape
	}
	d.kind, d.final = ansiKey, b
	return nil
}

// decode decodes the next key, escape sequence, or control sequence from in. The results are left in the decoder.
func (d *ansiDecoder) decode(in io.Reader) error {
	state := d.stateInit
	for {
		var b [1]byte
		if _, err := in.Read(b[:]); err != nil {
			return err
		}

		next := state(b[0])
		if next == nil {
			return nil
		}
		state = next
	}
}

const (
	KeyCtrlC    = "ctrl+c"
	KeyCtrlO    = "ctrl+o"
	KeyDown     = "down"
	KeyEnd      = "end"
	KeyHome     = "home"
	KeyPageDown = "page-down"
	KeyPageUp   = "page-up"
	KeyUp       = "up"
)

// ReadKey reads a keypress from the terminal.
func (t *terminal) ReadKey() (string, error) {
	if t.in == nil {
		return "", io.EOF
	}

	// Decode an ANSI sequence from the input.
	var d ansiDecoder
	if err := d.decode(t.in); err != nil {
		if errors.Is(err, cancelreader.ErrCanceled) {
			err = io.EOF
		}
		return "", err
	}

	// Turn the decoded sequence into a key name.
	//
	// Some of these are described by ECMA-48, while others are described by the xterm or DEC docs:
	// - https://www.ecma-international.org/wp-content/uploads/ECMA-48_5th_edition_june_1991.pdf
	// - https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
	// - https://vt100.net/docs/vt510-rm/contents.html
	switch d.kind {
	case ansiKey:
		switch d.final {
		case 2: // Ctrl+B --- Vim key for page up (page back)
			return KeyPageUp, nil
		case 3: // ETX
			return KeyCtrlC, nil
		case 6: // Ctrl+F ---- Vim key for page down (page forward)
			return KeyPageDown, nil
		case 15: // SI
			return KeyCtrlO, nil
		}
		return string([]byte{d.final}), nil
	case ansiEscape:
		return fmt.Sprintf("<escape %v>", d.final), nil
	case ansiControl:
		switch d.final {
		case 'A':
			// CUU - Cursor Up: CSI (Pn) A
			return KeyUp, nil
		case 'B':
			// CUD - Cursor Down: CSI (Pn) B
			return KeyDown, nil
		case 'F':
			// Some terminals use CSI F for End, other use CSI 4 ~
			// Historically this is the SCO mapping for a vt220
			return KeyEnd, nil
		case 'H':
			// Some terminals use CSI H for home, other use CSI 1 ~
			// in VT100 terms this is CUP (Pl; Pc) H
			return KeyHome, nil
		case '~':
			// DECFNK - Function Key: CSI Ps1 (; Ps2) ~
			switch string(d.params) {
			case "1":
				// Some terminals use CSI H for home, other use CSI 1 ~
				// which is probably a reinterpretation of the DEC Find key
				return KeyHome, nil
			case "4":
				// Some terminals use CSI F for End, other use CSI 4 ~
				// which is probably a reinterpretation of the DEC Select key
				return KeyEnd, nil
			case "5":
				// Page Up: CSI 5 ~
				return KeyPageUp, nil
			case "6":
				// Page Down: CSI 6 ~
				return KeyPageDown, nil
			}
		}
		return fmt.Sprintf("<control %v>", d.final), nil
	case ansiError:
		return "", errors.New("invalid ANSI sequence")
	default:
		return "", errors.New("invalid control sequence")
	}
}
