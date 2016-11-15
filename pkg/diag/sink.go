// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Count fetches the total number of diagnostics issued (errors plus warnings).
	Count() int
	// Errors fetches the number of errors issued.
	Errors() int
	// Warnings fetches the number of warnings issued.
	Warnings() int

	// Error issues a new error diagnostic.
	Errorf(diag *Diag, args ...interface{})
	// Warning issues a new warning diagnostic.
	Warningf(diag *Diag, args ...interface{})
}

// DefaultDiags returns a default sink that simply logs output to stderr/stdout.
func DefaultSink() Sink {
	return &defaultDiags{}
}

// defaultDiags is the default sink which logs output to stderr/stdout.
type defaultDiags struct {
	errors   int
	warnings int
}

func (d *defaultDiags) Count() int {
	return d.errors + d.warnings
}

func (d *defaultDiags) Errors() int {
	return d.errors
}

func (d *defaultDiags) Warnings() int {
	return d.warnings
}

func (d *defaultDiags) Errorf(diag *Diag, args ...interface{}) {
	fmt.Fprintln(os.Stdout, d.stringify(diag, "error", args...))
}

func (d *defaultDiags) Warningf(diag *Diag, args ...interface{}) {
	fmt.Fprintln(os.Stdout, d.stringify(diag, "warning", args...))
}

// stringify stringifies a diagnostic in the usual way (e.g., "error: MU123: Mu.yaml:7:39: error goes here\n").
func (d *defaultDiags) stringify(diag *Diag, prefix string, args ...interface{}) string {
	var buffer bytes.Buffer

	buffer.WriteString(prefix)
	buffer.WriteString(": ")

	if diag.ID > 0 {
		buffer.WriteString("MU")
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
		buffer.WriteString(": ")
	}

	if diag.File != "" {
		buffer.WriteString(diag.File)
		if diag.Filepos != nil {
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Filepos.Start.Row))
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Filepos.Start.Col))
		}
		buffer.WriteString(": ")
	}

	buffer.WriteString(fmt.Sprintf(diag.Message, args...))
	buffer.WriteRune('\n')

	// TODO: support Clang-style caret diagnostics; e.g., see http://clang.llvm.org/diagnostics.html.

	return buffer.String()
}
