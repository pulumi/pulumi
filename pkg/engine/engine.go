package engine

import (
	"io"
	"os"
)

var (
	E Engine
)

type Engine struct {
	Stdout io.Writer
	Stderr io.Writer
}

func init() {
	E.Stdout = os.Stdout
	E.Stderr = os.Stderr
}
