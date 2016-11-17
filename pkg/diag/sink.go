// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	// Stringify stringifies a diagnostic in the usual way (e.g., "error: MU123: Mu.yaml:7:39: error goes here\n").
	Stringify(diag *Diag, prefix string, args ...interface{}) string
}

// DefaultDiags returns a default sink that simply logs output to stderr/stdout.
func DefaultSink(pwd string) Sink {
	return newDefaultSink(pwd, os.Stderr, os.Stdout)
}

func newDefaultSink(pwd string, errorW io.Writer, warningW io.Writer) *defaultSink {
	return &defaultSink{pwd: pwd, errorW: errorW, warningW: warningW}
}

const DefaultSinkIDPrefix = "MU"
const DefaultSinkErrorPrefix = "error"
const DefaultSinkWarningPrefix = "warning"

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	pwd      string    // an optional present working directory to which output paths will be relative to.
	errors   int       // the number of errors that have been issued.
	errorW   io.Writer // the output stream to use for errors.
	warnings int       // the number of warnings that have been issued.
	warningW io.Writer // the output stream to use for warnings.
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
	msg := d.Stringify(diag, DefaultSinkErrorPrefix, args...)
	if glog.V(3) {
		glog.V(3).Infof("defaultSink::Error(%v)", msg)
	}
	fmt.Fprintf(d.errorW, msg)
	d.errors++
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(diag, DefaultSinkWarningPrefix, args...)
	if glog.V(4) {
		glog.V(4).Infof("defaultSink::Warning(%v)", msg)
	}
	fmt.Fprintf(d.warningW, msg)
	d.warnings++
}

func (d *defaultSink) Stringify(diag *Diag, prefix string, args ...interface{}) string {
	var buffer bytes.Buffer

	buffer.WriteString(prefix)
	buffer.WriteString(": ")

	if diag.ID > 0 {
		buffer.WriteString(DefaultSinkIDPrefix)
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
		buffer.WriteString(": ")
	}

	if diag.Doc != nil {
		file := diag.Doc.File
		if d.pwd != "" {
			// If a PWD is available, try to create a relative path.
			rel, err := filepath.Rel(d.pwd, file)
			if err == nil {
				file = rel
			}
		}
		buffer.WriteString(file)

		if diag.Loc != nil && !diag.Loc.IsEmpty() {
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Loc.From.Ln))
			buffer.WriteRune(':')
			buffer.WriteString(strconv.Itoa(diag.Loc.From.Col))
		}
		buffer.WriteString(": ")
	}

	buffer.WriteString(fmt.Sprintf(diag.Message, args...))
	buffer.WriteRune('\n')

	// TODO: support Clang-style caret diagnostics; e.g., see http://clang.llvm.org/diagnostics.html.

	return buffer.String()
}
