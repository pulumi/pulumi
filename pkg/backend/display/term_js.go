//go:build js
// +build js

package display

import (
	"io"
	"os"
	"syscall/js"
)

var jsTerminal = js.Global().Get("terminal")

func getTerminalSize() (w int, h int, err error) {
	defer func() { catch(recover(), &err) }()

	dims := jsTerminal.Call("getSize")
	return dims.Get("width").Int(), dims.Get("height").Int(), nil
}

func EnableANSITerminal() (stdout io.Writer, err error) {
	defer func() { catch(recover(), &err) }()

	jsTerminal.Call("enableAnsiTerminal")
	return os.Stdout, nil
}

func catch(x interface{}, err *error) {
	if x != nil {
		e, ok := x.(error)
		if !ok {
			panic(x)
		}
		*err = e
	}
}
