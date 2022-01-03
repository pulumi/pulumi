//go:build !js
// +build !js

package display

import (
	"io"
	"os"

	"github.com/moby/term"
	"golang.org/x/crypto/ssh/terminal"
)

func getTerminalSize() (int, int, error) {
	return terminal.GetSize(int(os.Stdout.Fd()))
}

func EnableANSITerminal() (io.Writer, error) {
	// We run this method for its side-effects. On windows, this will enable the windows terminal
	// to understand ANSI escape codes.
	_, stdout, _ := term.StdStreams()
	return stdout, nil
}
