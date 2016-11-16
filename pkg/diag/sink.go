// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/golang/glog"
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
	return &defaultSink{}
}

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	errors   int
	warnings int
}

func (d *defaultSink) Count() int {
	return d.errors + d.warnings
}

func (d *defaultSink) Errors() int {
	return d.errors
}

func (d *defaultSink) Warnings() int {
	return d.warnings
}

func (d *defaultSink) Errorf(diag *Diag, args ...interface{}) {
	msg := d.stringify(diag, "error", args...)
	if glog.V(3) {
		glog.V(3).Infof("defaultSink::Error(%v)", msg)
	}
	fmt.Fprintln(os.Stderr, msg)
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.stringify(diag, "warning", args...)
	if glog.V(4) {
		glog.V(4).Infof("defaultSink::Warning(%v)", msg)
	}
	fmt.Fprintln(os.Stdout, msg)
}

// stringify stringifies a diagnostic in the usual way (e.g., "error: MU123: Mu.yaml:7:39: error goes here\n").
func (d *defaultSink) stringify(diag *Diag, prefix string, args ...interface{}) string {
	var buffer bytes.Buffer

	buffer.WriteString(prefix)
	buffer.WriteString(": ")

	if diag.ID > 0 {
		buffer.WriteString("MU")
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
		buffer.WriteString(": ")
	}

	if diag.Doc != nil {
		buffer.WriteString(diag.Doc.File)
		if diag.Loc != nil {
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Loc.Start.Row))
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Loc.Start.Col))
		}
		buffer.WriteString(": ")
	}

	buffer.WriteString(fmt.Sprintf(diag.Message, args...))
	buffer.WriteRune('\n')

	// TODO: support Clang-style caret diagnostics; e.g., see http://clang.llvm.org/diagnostics.html.

	return buffer.String()
}
