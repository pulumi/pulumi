// Copyright 2016 Pulumi, Inc. All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/diag/colors"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Count fetches the total number of diagnostics issued (errors plus warnings).
	Count() int
	// Infos fetches the number of informational messages issued.
	Infos() int
	// Errors fetches the number of errors issued.
	Errors() int
	// Warnings fetches the number of warnings issued.
	Warnings() int
	// Success returns true if this sink is currently error-free.
	Success() bool

	// Infof issues an informational message.
	Infof(diag *Diag, args ...interface{})
	// Errorf issues a new error diagnostic.
	Errorf(diag *Diag, args ...interface{})
	// Warningf issues a new warning diagnostic.
	Warningf(diag *Diag, args ...interface{})

	// Stringify stringifies a diagnostic in the usual way (e.g., "error: MU123: Coconut.yaml:7:39: error goes here\n").
	Stringify(diag *Diag, cat Category, args ...interface{}) string
	// StringifyLocation stringifies a source document location.
	StringifyLocation(doc *Document, loc *Location) string
}

// Category dictates the kind of diagnostic.
type Category string

const (
	Error   Category = "error"
	Warning          = "warning"
	Info             = "info"
)

// FormatOptions controls the output style and content.
type FormatOptions struct {
	Pwd    string // the working directory.
	Colors bool   // if true, output will be colorized.
}

// DefaultSink returns a default sink that simply logs output to stderr/stdout.
func DefaultSink(opts FormatOptions) Sink {
	return newDefaultSink(opts, map[Category]io.Writer{
		Info:    os.Stdout,
		Error:   os.Stderr,
		Warning: os.Stdout,
	})
}

func newDefaultSink(opts FormatOptions, writers map[Category]io.Writer) *defaultSink {
	contract.Assert(writers[Info] != nil)
	contract.Assert(writers[Error] != nil)
	contract.Assert(writers[Warning] != nil)
	return &defaultSink{
		opts:    opts,
		counts:  make(map[Category]int),
		writers: writers,
	}
}

const DefaultSinkIDPrefix = "COCO"

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	opts    FormatOptions          // a set of options that control output style and content.
	counts  map[Category]int       // the number of messages that have been issued per category.
	writers map[Category]io.Writer // the writers to use for each kind of diagnostic category.
}

func (d *defaultSink) Count() int    { return d.Infos() + d.Errors() + d.Warnings() }
func (d *defaultSink) Infos() int    { return d.counts[Info] }
func (d *defaultSink) Errors() int   { return d.counts[Error] }
func (d *defaultSink) Warnings() int { return d.counts[Warning] }
func (d *defaultSink) Success() bool { return d.Errors() == 0 }

func (d *defaultSink) Infof(diag *Diag, args ...interface{}) {
	msg := d.Stringify(diag, Info, args...)
	if glog.V(3) {
		glog.V(3).Infof("defaultSink::Info(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Info], msg)
	d.counts[Info]++
}

func (d *defaultSink) Errorf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(diag, Error, args...)
	if glog.V(3) {
		glog.V(3).Infof("defaultSink::Error(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Error], msg)
	d.counts[Error]++
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(diag, Warning, args...)
	if glog.V(4) {
		glog.V(4).Infof("defaultSink::Warning(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Warning], msg)
	d.counts[Warning]++
}

func (d *defaultSink) Stringify(diag *Diag, cat Category, args ...interface{}) string {
	var buffer bytes.Buffer

	// First print the location if there is one.
	if diag.Doc != nil || diag.Loc != nil {
		buffer.WriteString(d.StringifyLocation(diag.Doc, diag.Loc))
		buffer.WriteString(": ")
	}

	// Now print the message category's prefix (error/warning).
	if d.opts.Colors {
		switch cat {
		case Info:
			buffer.WriteString(colors.SpecInfo)
		case Error:
			buffer.WriteString(colors.SpecError)
		case Warning:
			buffer.WriteString(colors.SpecWarning)
		default:
			contract.Failf("Unrecognized diagnostic category: %v", cat)
		}
	}

	buffer.WriteString(string(cat))

	if diag.ID > 0 {
		buffer.WriteString(" ")
		buffer.WriteString(DefaultSinkIDPrefix)
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
	}

	buffer.WriteString(": ")

	if d.opts.Colors {
		buffer.WriteString(colors.Reset)
	}

	// Finally, actually print the message itself.
	if d.opts.Colors {
		buffer.WriteString(colors.SpecNote)
	}

	buffer.WriteString(fmt.Sprintf(diag.Message, args...))

	if d.opts.Colors {
		buffer.WriteString(colors.Reset)
	}

	buffer.WriteRune('\n')

	// TODO[pulumi/coconut#15]: support Clang-style expressive diagnostics.  This would entail, for example, using the
	//     buffer within the target document, to demonstrate the offending line/column range of code.

	s := buffer.String()

	// If colorization was requested, compile and execute the directives now.
	if d.opts.Colors {
		s = colors.ColorizeText(s)
	}

	return s
}

func (d *defaultSink) StringifyLocation(doc *Document, loc *Location) string {
	var buffer bytes.Buffer

	if doc != nil {
		if d.opts.Colors {
			buffer.WriteString(colors.SpecLocation)
		}

		file := doc.File
		if d.opts.Pwd != "" {
			// If a PWD is available, try to create a relative path.
			rel, err := filepath.Rel(d.opts.Pwd, file)
			if err == nil {
				file = rel
			}
		}
		buffer.WriteString(file)
	}

	if loc != nil && !loc.IsEmpty() {
		buffer.WriteRune('(')
		buffer.WriteString(strconv.Itoa(loc.Start.Line))
		buffer.WriteRune(',')
		buffer.WriteString(strconv.Itoa(loc.Start.Column))
		buffer.WriteRune(')')
	}

	var s string
	if doc != nil || loc != nil {
		if d.opts.Colors {
			buffer.WriteString(colors.Reset)
		}

		s = buffer.String()

		// If colorization was requested, compile and execute the directives now.
		if d.opts.Colors {
			s = colors.ColorizeText(s)
		}
	}

	return s
}
