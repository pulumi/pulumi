package main

import (
	"bytes"
	"fmt"
	"strings"
)

type generator struct {
	indentSize int
}

func newGenerator() *generator {
	return &generator{}
}

func (g *generator) indented(f func()) {
	g.indentSize += 2
	f()
	g.indentSize -= 2
}

func (g *generator) indent(buffer *bytes.Buffer) {
	buffer.WriteString(strings.Repeat(" ", g.indentSize))
}

func (g *generator) write(buffer *bytes.Buffer, format string, args ...interface{}) {
	buffer.WriteString(fmt.Sprintf(format, args...))
}
