// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

// NewSpinnerAndTicker returns a new Spinner and a ticker that will fire an event when the next call to Spinner.Tick()
// should be called. NewSpinnerAndTicket takes into account if stdout is connected to a tty or not and returns either a
// nice animated spinner that updates quickly or a simple spinner that just prints a dot on each tick and updates
// slowly.
func NewSpinnerAndTicker() (Spinner, *time.Ticker) {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		return &ttySpinner{}, time.NewTicker(time.Second / 8)
	}
	return &dotSpinner{}, time.NewTicker(time.Second * 20)
}

// Spinner represents a very simple progress reporter.
type Spinner interface {
	// Print the next frame of the spinner. After Tick() has been called, there should be no writes to Stdout before
	// calling Reset().
	Tick()

	// Called to release ownership of stdout, so others may write to it.
	Reset()
}

var spinFrames = []string{"|", "/", "-", "\\"}

// ttySpinner is the spinner that can be used when standard out is a tty. When we are connected to a TTY we can erase
// characters we've written and provide a nice quick progress spinner.
type ttySpinner struct {
	index int
}

func (spin *ttySpinner) Tick() {
	fmt.Printf("\r \r")
	fmt.Printf(spinFrames[spin.index])
	spin.index = (spin.index + 1) % len(spinFrames)
}

func (spin *ttySpinner) Reset() {
	fmt.Printf("\r \r")
	spin.index = 0
}

// dotSpinner is the spinner that can be used when standard out is not a tty. In this case, we just write a single
// dot on each tick.
type dotSpinner struct {
	hasWritten bool
}

func (spin *dotSpinner) Tick() {
	if !spin.hasWritten {
		fmt.Printf("still working")
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
