//go:build !js
// +build !js

package display

import (
	"io"

	"github.com/moby/term"
)

func EnableANSITerminal() (io.Writer, error) {
	// We run this method for its side-effects. On windows, this will enable the windows terminal
	// to understand ANSI escape codes.
	_, stdout, _ := term.StdStreams()
	return stdout, nil
}
