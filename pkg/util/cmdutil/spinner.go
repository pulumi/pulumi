// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// NewSpinnerAndTicker returns a new Spinner and a ticker that will fire an event when the next call to Spinner.Tick()
// should be called.  NewSpinnerAndTicket takes into account if stdout is connected to a tty or not and returns either
// a nice animated spinner that updates quickly, using the specified ttyFrames, or a simple spinner that just prints a
// dot on each tick and updates slowly.
func NewSpinnerAndTicker(prefix string, ttyFrames []string) (Spinner, *time.Ticker) {
	if ttyFrames == nil {
		// If explicit tick frames weren't specified, default to unicode for Mac and ASCII for Windows/Linux.
		if Emoji {
			ttyFrames = DefaultEmojiSpinFrames
		} else {
			ttyFrames = DefaultASCIISpinFrames
		}
	}

	if Interactive() {
		return &ttySpinner{
			prefix: prefix,
			frames: ttyFrames,
		}, time.NewTicker(time.Second / 8)
	}

	return &dotSpinner{
		prefix: prefix,
	}, time.NewTicker(time.Second * 20)
}

// Spinner represents a very simple progress reporter.
type Spinner interface {
	// Print the next frame of the spinner. After Tick() has been called, there should be no writes to Stdout before
	// calling Reset().
	Tick()

	// Called to release ownership of stdout, so others may write to it.
	Reset()
}

var (
	// DefaultASCIISpinFrames is the default set of symbols to show while spinning in an ASCII TTY setting.
	DefaultASCIISpinFrames = []string{
		"|", "/", "-", "\\",
	}
	// DefaultEmojiSpinFrames is the default set of symbols to show while spinning in a Unicode-enabled TTY setting.
	DefaultEmojiSpinFrames = []string{
		"⠋", "⠙", "⠚", "⠒", "⠂", "⠂", "⠒", "⠲", "⠴", "⠦", "⠖", "⠒", "⠐", "⠐", "⠒", "⠓", "⠋",
	}
)

// ttySpinner is the spinner that can be used when standard out is a tty. When we are connected to a TTY we can erase
// characters we've written and provide a nice quick progress spinner.
type ttySpinner struct {
	prefix      string
	frames      []string
	index       int
	lastWritten int
}

func (spin *ttySpinner) Tick() {
	if spin.lastWritten > 0 {
		for i := 0; i < spin.lastWritten; i++ {
			fmt.Print("\b \b")
		}
	} else {
		fmt.Print(spin.prefix)
	}
	frame := spin.frames[spin.index]
	fmt.Print(frame)
	spin.lastWritten = utf8.RuneCountInString(frame)
	spin.index = (spin.index + 1) % len(spin.frames)
}

func (spin *ttySpinner) Reset() {
	if spin.lastWritten > 0 {
		for i := 0; i < len(spin.prefix)+spin.lastWritten; i++ {
			fmt.Print("\b \b")
		}
	}
	spin.index = 0
	spin.lastWritten = 0
}

// dotSpinner is the spinner that can be used when standard out is not a tty. In this case, we just write a single
// dot on each tick.
type dotSpinner struct {
	prefix     string
	hasWritten bool
}

func (spin *dotSpinner) Tick() {
	if !spin.hasWritten {
		fmt.Print(spin.prefix)
	}
	fmt.Printf(".")
	spin.hasWritten = true
}

func (spin *dotSpinner) Reset() {
	if spin.hasWritten {
		fmt.Println()
	}
	spin.hasWritten = false
}
