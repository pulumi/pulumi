package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	CursorUp(count int)
	CursorDown(count int)

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

func Open(in io.Reader, out io.Writer, raw bool) (Terminal, error) {
	type fileLike interface {
		Fd() uintptr
	}

	outFile, ok := out.(fileLike)
	if !ok {
		return nil, ErrNotATerminal
	}
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

func (t *terminal) CursorUp(count int) {
	t.info.CursorUp(t.out, count)
}

func (t *terminal) CursorDown(count int) {
	t.info.CursorDown(t.out, count)
}

func (t *terminal) ReadKey() (string, error) {
	if t.in == nil {
		return "", io.EOF
	}

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
		if _, err := t.in.Read(b[:]); err != nil {
			if errors.Is(err, cancelreader.ErrCanceled) {
				err = io.EOF
			}
			return "", err
		}

		next, key := state(b[0])
		if next == nil {
			return key, nil
		}
		state = next
	}
}
